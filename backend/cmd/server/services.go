// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/mindruntime"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/personality"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/psyche/appraisal"
	"github.com/u-ai/backend/internal/psyche/budget"
	"github.com/u-ai/backend/internal/qdrant"
	"github.com/u-ai/backend/internal/queue"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
)

type AppServices struct {
	DeliveryStore       *delivery.SQLiteDeliveryStore
	ChatDeliveryAdapter chat.DeliveryStore
	DeliveryWorker      *delivery.Worker
	Graph               graph.Service
	Memory              memory.Service
	Profile             profile.Service
	Episodic            episodic.Service
	WorldBook           worldbook.Service
	Vision              vision.Service
	Companion           companion.Service
	Chat                chat.Service
	UnifiedEntry        *interaction.UnifiedEntry
	DataLifecycle       *mindruntime.DataLifecycleCoordinator
	RuntimeQueue        *queue.SQLiteRuntimeQueueStore
	NewOutbox           *newoutbox.SQLiteOutboxStore
	OutboxWorker        *newoutbox.Worker
	Reconciliation      *mindruntime.ReconciliationEngine
	CircuitBreakers     *mindruntime.CircuitBreakerRegistry
	VoiceEntry          *interaction.VoiceEntry
}

type reflectionMemoryServiceAdapter struct {
	memory memory.Service
}

func (a reflectionMemoryServiceAdapter) CreateReflectionMemory(req interaction.ReflectionMemoryCreateRequest) error {
	_, err := a.memory.Create(&memory.CreateMemoryRequest{
		CharacterID:           req.CharacterID,
		MemoryType:            req.MemoryType,
		Key:                   req.Key,
		Value:                 req.Value,
		Importance:            req.Importance,
		Confidence:            req.Confidence,
		SourceMsgID:           req.SourceMsgID,
		SourceConvID:          req.SourceConvID,
		VerifiedStatus:        req.VerifiedStatus,
		Source:                req.Source,
		SensitivityLevel:      req.SensitivityLevel,
		AllowProactiveMention: req.AllowProactiveMention,
		RequiresConfirmation:  req.RequiresConfirmation,
		Scope:                 req.Scope,
	})
	return err
}

func NewAppServices(ctx *app.AppContext, graphSvc graph.Service) *AppServices {
	memRepo := memory.NewRepository(ctx)
	memSvc := memory.NewService(memRepo, ctx, graphSvc)
	profRepo := profile.NewRepository(ctx)
	profSvc := profile.NewService(profRepo, ctx, graphSvc)
	epiRepo := episodic.NewRepository(ctx)
	epiSvc := episodic.NewService(epiRepo, ctx, graphSvc)
	wbRepo := worldbook.NewRepository(ctx)
	wbSvc := worldbook.NewService(wbRepo, ctx, graphSvc)
	visionRepo := vision.NewRepository(ctx.DB)
	visionSvc := vision.NewService(visionRepo)
	compSvc := companion.NewService(ctx)
	compressor := chat.NewCompressor(ctx.DB)
	psycheStore := psyche.NewSQLitePsycheStore(ctx.DB)
	if err := psycheStore.InitSchema(); err != nil {
		panic("failed to init psyche store schema: " + err.Error())
	}
	if err := ctx.DB.AutoMigrate(&chat.RelationshipStateRecord{}, &chat.NeedStateRecord{}); err != nil {
		panic("failed to init relationship/need store schema: " + err.Error())
	}
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, wbSvc, compressor, visionSvc, graphSvc, psycheStore)
	orchCfg := interaction.DefaultOrchestratorConfig()
	tracker := interaction.NewSQLiteInteractionTracker(ctx.DB)
	if err := tracker.InitSchema(); err != nil {
		panic("failed to init interaction tracker schema: " + err.Error())
	}
	newOutboxStore := newoutbox.NewSQLiteOutboxStore(ctx.DB, newoutbox.DefaultOutboxStoreConfig())
	if err := ctx.DB.AutoMigrate(&newoutbox.OutboxRecordModel{}, &newoutbox.DeadLetterRecordModel{}); err != nil {
		panic("failed to init outbox store schema: " + err.Error())
	}
	runtimeQueue := queue.NewSQLiteRuntimeQueueStore(ctx.DB)
	deliveryStore := delivery.NewSQLiteDeliveryStore(ctx.DB)
	deliveryWorker := delivery.NewWorker(deliveryStore, []delivery.ChannelAdapter{
		delivery.NewQQChannelAdapter("http://127.0.0.1:9877"),
		delivery.NewWechatChannelAdapter("http://127.0.0.1:9876"),
	}, delivery.DefaultWorkerConfig())
	deliveryAdapter := &chatDeliveryAdapter{store: deliveryStore}
	chatSvc.SetDeliveryStore(deliveryAdapter)

	outboxAdapter := &chatOutboxAdapter{store: newOutboxStore}
	chatSvc.SetOutboxStore(outboxAdapter)
	if err := runtimeQueue.InitSchema(); err != nil {
		panic("failed to init runtime queue schema: " + err.Error())
	}

	dispatchedPublisher := newoutbox.NewDispatchedPublisher(newoutbox.LogOnlyPublisher())
	postProcessAdapter := &postProcessPublisherAdapter{chatSvc: chatSvc}
	dispatchedPublisher.Register("postprocess.pipeline.execute", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.context.trim", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.mood.recovery", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.compressor.maybe", postProcessAdapter)

	newOutboxWorker := newoutbox.NewWorker(newOutboxStore, dispatchedPublisher, newoutbox.DefaultWorkerConfig())
	orch := interaction.NewOrchestrator(orchCfg, chatSvc.(interaction.MessageProcessor))
	charRepo := character.NewRepository(ctx)
	runtimeRegistry := newRuntimeContextLoaderRegistry(ctx, charRepo)
	runtimePipeline := interaction.NewRuntimePipeline(runtimeRegistry, interaction.NewPathClassifier(), interaction.NewTokenBudgetManager(2400))
	runtimePipeline.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))
	runtimePipeline.SetSafetyGovernor(safety.NewGovernor(safety.DefaultGovernorConfig()))
	runtimePipeline.SetBeliefResolver(belief.ResolveBelief)
	runtimePipeline.SetAppraisalEngine(appraisal.NewEngine(appraisal.DefaultAppraisalConfig()))
	runtimePipeline.SetBudgetController(budget.NewBudgetController(0.5))
	orch.SetRuntimePipeline(runtimePipeline)
	deadlineCfg := mindruntime.DefaultDeadlineConfig
	deadlineCfg.TotalTimeout = 180 * time.Second
	deadlineCfg.GenerationTimeout = 120 * time.Second
	dp := mindruntime.NewDeadlinePropagator(deadlineCfg)
	orch.SetDeadlineProvider(func(ctx context.Context, requestID string) (context.Context, context.CancelFunc) {
		return dp.ContextWithDeadline(ctx, requestID, mindruntime.DeadlineStageGeneration)
	})
	resolver := interaction.NewScopeResolver(interaction.NewConversationScopeBindingLookup(ctx.DB))
	dataLifecycle := mindruntime.NewDataLifecycleCoordinator(ctx.DB)
	if err := dataLifecycle.InitSchema(); err != nil {
		panic("failed to init data lifecycle schema: " + err.Error())
	}
	dataLifecycle.SetOutboxCleanupExecutor(mindruntime.NewDefaultOutboxCleanupExecutor(ctx.DB, graphSvc))
	if coordinatorSetter, ok := interface{}(memSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	if coordinatorSetter, ok := interface{}(profSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	if coordinatorSetter, ok := interface{}(epiSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	entry := interaction.NewUnifiedEntry(orch, resolver)
	compSvc.AttachUnifiedEntry(entry)
	compSvc.AttachDeliveryStore(deliveryStore)
	if coordinatorSetter, ok := interface{}(compSvc).(interface {
		SetDataLifecycleCoordinator(*mindruntime.DataLifecycleCoordinator)
	}); ok {
		coordinatorSetter.SetDataLifecycleCoordinator(dataLifecycle)
	}
	reconciliationEngine := mindruntime.NewReconciliationEngine(mindruntime.DefaultReconciliationConfig())
	graphReconAdapter := &graphReconciliationAdapter{graphSvc: graphSvc}
	qdrantReconAdapter := &qdrantReconciliationAdapter{qdrantClient: qdrant.NewQdrantClient()}
	if err := mindruntime.RegisterRuntimeReconciliationCheckers(reconciliationEngine, ctx.DB, graphReconAdapter, qdrantReconAdapter); err != nil {
		log.Warn("reconciliation checkers registration warning: ", err)
	}
	cbRegistry := mindruntime.NewCircuitBreakerRegistry()
	cbRegistry.Register("qdrant", mindruntime.DefaultCircuitBreakerConfig())
	cbRegistry.Register("surrealdb", mindruntime.DefaultCircuitBreakerConfig())
	cbRegistry.Register("model_api", mindruntime.DefaultCircuitBreakerConfig())
	voiceEntry := interaction.NewVoiceEntryWithUnifiedEntry(orch, entry)
	if err := deliveryStore.InitSchema(); err != nil {
		panic("failed to init delivery store schema: " + err.Error())
	}
	return &AppServices{
		Graph:               graphSvc,
		ChatDeliveryAdapter: deliveryAdapter,
		Memory:              memSvc,
		Profile:             profSvc,
		Episodic:            epiSvc,
		WorldBook:           wbSvc,
		Vision:              visionSvc,
		Companion:           compSvc,
		Chat:                chatSvc,
		UnifiedEntry:        entry,
		DataLifecycle:       dataLifecycle,
		RuntimeQueue:        runtimeQueue,
		NewOutbox:           newOutboxStore,
		DeliveryStore:       deliveryStore,
		DeliveryWorker:      deliveryWorker,
		OutboxWorker:        newOutboxWorker,
		Reconciliation:      reconciliationEngine,
		CircuitBreakers:     cbRegistry,
		VoiceEntry:          voiceEntry,
	}
}

func newRuntimeContextLoaderRegistry(ctx *app.AppContext, charRepo character.Repository) *interaction.ContextLoaderRegistry {
	runtimeRegistry := interaction.NewContextLoaderRegistry()
	runtimeRegistry.Register(interaction.NewRoleRuntimeProfileContextLoader(charRepo))
	runtimeRegistry.Register(interaction.NewChannelContextLoader())
	runtimeRegistry.Register(interaction.NewConversationContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewPsycheContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewRelationshipContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewBeliefContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewLifeContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewNeedContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewUnresolvedThreadContextLoader(ctx.DB))
	return runtimeRegistry
}

type chatOutboxAdapter struct {
	store *newoutbox.SQLiteOutboxStore
}

func (a *chatOutboxAdapter) AppendOutbox(aggregateID, eventType string, payload []byte) error {
	record := newoutbox.OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     payload,
		Status:      newoutbox.OutboxStatusPending,
		MaxRetries:  newoutbox.DefaultMaxRetries,
		AvailableAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	return a.store.Append(record)
}

func (a *chatOutboxAdapter) AppendOutboxWithKey(aggregateID, eventType, idempotencyKey string, payload []byte) error {
	record := newoutbox.OutboxRecord{
		ID:             uuid.New().String(),
		AggregateID:    aggregateID,
		EventType:      eventType,
		Payload:        payload,
		Status:         newoutbox.OutboxStatusPending,
		MaxRetries:     newoutbox.DefaultMaxRetries,
		AvailableAt:    time.Now(),
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
	}
	return a.store.Append(record)
}

type chatDeliveryAdapter struct {
	store *delivery.SQLiteDeliveryStore
}

func (a *chatDeliveryAdapter) CreateDeliveryIntent(interactionID, channel, peerID, contentType string, payload []byte) error {
	intent := delivery.NewDeliveryIntent(interactionID, channel, peerID, contentType, payload)
	return a.store.CreateIntent(intent)
}

func (a *chatDeliveryAdapter) CreateOutputLease(interactionID, characterID, userID, channel string) error {
	lease := delivery.NewOutputLease(interactionID, characterID, userID, channel)
	return a.store.CreateLease(lease)
}

func (a *chatDeliveryAdapter) AcquireOutputLease(interactionID, characterID, userID, channel string) (string, string, error) {
	lease := delivery.NewOutputLease(interactionID, characterID, userID, channel)
	if err := a.store.CreateLease(lease); err != nil {
		return "", "", err
	}
	return lease.ID, lease.OwnerToken, nil
}

func (a *chatDeliveryAdapter) ReleaseOutputLease(leaseID, ownerToken string) error {
	return a.store.ReleaseLease(leaseID, ownerToken)
}

func (a *chatDeliveryAdapter) PreemptActiveOutputLeases(characterID string) error {
	_, err := a.store.PreemptActiveLeasesByCharacter(characterID)
	return err
}

type graphReconciliationAdapter struct {
	graphSvc graph.Service
}

func (a *graphReconciliationAdapter) Name() string { return "graph" }

func (a *graphReconciliationAdapter) CheckSideEffectExists(ctx context.Context, aggregateID, eventType string) (bool, error) {
	if a.graphSvc == nil {
		return false, nil
	}
	nodes, err := a.graphSvc.GetAllNodes(aggregateID)
	if err != nil {
		return false, err
	}
	return len(nodes) > 0, nil
}

type qdrantReconciliationAdapter struct {
	qdrantClient *qdrant.QdrantClient
}

func (a *qdrantReconciliationAdapter) Name() string { return "qdrant" }

func (a *qdrantReconciliationAdapter) CheckSideEffectExists(ctx context.Context, aggregateID, eventType string) (bool, error) {
	if a.qdrantClient == nil {
		return false, nil
	}
	_, err := a.qdrantClient.SearchWithFilter(ctx, "memory_embeddings", nil, qdrant.QdrantFilter{CharacterID: aggregateID}, 1)
	if err != nil {
		return false, nil
	}
	return true, nil
}

type postProcessPublisherAdapter struct {
	chatSvc chat.Service
}

func (a *postProcessPublisherAdapter) Publish(record newoutbox.OutboxRecord) error {
	return a.chatSvc.ReplayPostProcess(record.EventType, record.Payload)
}
