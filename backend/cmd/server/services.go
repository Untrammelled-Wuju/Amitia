// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/config"

	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	processingworker "github.com/u-ai/backend/internal/desktoppet/processing/worker"
	"github.com/u-ai/backend/internal/desktoppet/worker"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mcp"
	mcpauth "github.com/u-ai/backend/internal/mcp/auth"
	mcpclient "github.com/u-ai/backend/internal/mcp/client"
	mcpdependency "github.com/u-ai/backend/internal/mcp/dependency"
	mcpdiscovery "github.com/u-ai/backend/internal/mcp/discovery"
	mcpfeatures "github.com/u-ai/backend/internal/mcp/features"
	mcphost "github.com/u-ai/backend/internal/mcp/host"
	mcpmanager "github.com/u-ai/backend/internal/mcp/manager"
	"github.com/u-ai/backend/internal/mcp/protocol"
	mcpskill "github.com/u-ai/backend/internal/mcp/skill"
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
	"github.com/u-ai/backend/internal/temporal"
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
	DesktopPetWorker    *worker.Worker
	ProcessingWorker    *processingworker.Worker
	InstallationService installation.Service
	Reconciliation      *mindruntime.ReconciliationEngine
	CircuitBreakers     *mindruntime.CircuitBreakerRegistry
	VoiceEntry          *interaction.VoiceEntry
	Extension           *extension.Runtime
	Emote               *emote.Service
	Temporal            *temporal.Service
	RelTimeCoordinator  *temporal.RelationshipTimeCoordinator
	MCPRepository       *mcp.Repository
	MCPConnections      *mcpmanager.Manager
	MCPAuth             *mcpauth.Manager
	MCPDiscovery        *mcpdiscovery.Service
	MCPSkills           *mcpskill.Runtime
	MCPSecrets          mcpauth.SecretStore
	MCPFeatures         *mcpfeatures.Service
	MCPHost             *mcphost.Service
	MCPInteractions     *mcphost.Broker
	MCPDependencies     *mcpdependency.Service
}

type defaultCharacterProvider struct {
	repo character.Repository
}

func (p *defaultCharacterProvider) GetDefaultCharacterID(ctx context.Context) (string, error) {
	profile, err := p.repo.GetRuntimeProfile("")
	if err != nil {
		return "", err
	}
	return profile.CharacterID, nil
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
	temporalSvc := temporal.NewService(temporal.NewRepository(ctx.DB), temporal.SystemClock{})
	relTimeRepo := temporal.NewRelationshipTimeRepository(ctx.DB, temporal.SystemClock{})
	relTimeCoordinator := temporal.NewRelationshipTimeCoordinator(relTimeRepo, temporal.SystemClock{})
	temporalSvc.SetRelationshipTimeProvider(relTimeCoordinator)
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
	compSvc.AttachTemporalResolver(temporalSvc)
	if temporalSvc.FeatureFlags().RelationshipTimeEnabled {
		compSvc.AttachAssistantContactRecorder(relTimeCoordinator)
	}
	compressor := chat.NewCompressor(ctx.DB)
	psycheStore := psyche.NewSQLitePsycheStore(ctx.DB)
	if err := psycheStore.InitSchema(); err != nil {
		log.Error("failed to init psyche store schema:", err)
		panic("failed to init psyche store schema")
	}
	if err := ctx.DB.AutoMigrate(&chat.RelationshipStateRecord{}, &chat.NeedStateRecord{}); err != nil {
		log.Error("failed to init relationship/need store schema:", err)
		panic("failed to init relationship/need store schema")
	}
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, wbSvc, compressor, visionSvc, graphSvc, psycheStore)
	extensionRuntime, err := extension.NewRuntime(context.Background(), ctx.DB, "1.0.0")
	if err != nil {
		log.Error("failed to initialize skill runtime:", err)
		panic("failed to initialize skill runtime")
	}
	chatSvc.SetSkillRuntime(extensionRuntime)
	extensionRuntime.Workshop.SetModelGenerator(chatSvc)
	orchCfg := interaction.DefaultOrchestratorConfig()
	tracker := interaction.NewSQLiteInteractionTracker(ctx.DB)
	if err := tracker.InitSchema(); err != nil {
		log.Error("failed to init interaction tracker schema:", err)
		panic("failed to init interaction tracker schema")
	}
	newOutboxStore := newoutbox.NewSQLiteOutboxStore(ctx.DB, newoutbox.DefaultOutboxStoreConfig())
	if err := ctx.DB.AutoMigrate(&newoutbox.OutboxRecordModel{}, &newoutbox.DeadLetterRecordModel{}); err != nil {
		log.Error("failed to init outbox store schema:", err)
		panic("failed to init outbox store schema")
	}
	runtimeQueue := queue.NewSQLiteRuntimeQueueStore(ctx.DB)
	deliveryStore := delivery.NewSQLiteDeliveryStore(ctx.DB)
	deliveryWorker := delivery.NewWorker(deliveryStore, []delivery.ChannelAdapter{
		delivery.NewWebChannelAdapter(),
		delivery.NewQQChannelAdapter("http://127.0.0.1:19877"),
		delivery.NewWechatChannelAdapter("http://127.0.0.1:19876"),
	}, delivery.DefaultWorkerConfig())
	deliveryAdapter := &chatDeliveryAdapter{store: deliveryStore}
	chatSvc.SetDeliveryStore(deliveryAdapter)
	emoteSvc := emote.NewService(ctx.DB, deliveryStore)
	emoteDecision := emote.NewDecisionService(emoteSvc)
	chat.RegisterMessagePlanningHook(emoteDecision.Plan)

	outboxAdapter := &chatOutboxAdapter{store: newOutboxStore}
	chatSvc.SetOutboxStore(outboxAdapter)
	if err := runtimeQueue.InitSchema(); err != nil {
		log.Error("failed to init runtime queue schema:", err)
		panic("failed to init runtime queue schema")
	}

	dispatchedPublisher := newoutbox.NewDispatchedPublisher(newoutbox.LogOnlyPublisher())
	postProcessAdapter := &postProcessPublisherAdapter{chatSvc: chatSvc}
	dispatchedPublisher.Register("postprocess.pipeline.execute", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.context.trim", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.mood.recovery", postProcessAdapter)
	dispatchedPublisher.Register("postprocess.compressor.maybe", postProcessAdapter)

	newOutboxWorker := newoutbox.NewWorker(newOutboxStore, dispatchedPublisher, newoutbox.DefaultWorkerConfig())

	interactionPublisher := &noopPublisher{}
	dispatchedPublisher.Register("interaction.completed", interactionPublisher)
	dispatchedPublisher.Register("interaction.state_changed", interactionPublisher)
	dispatchedPublisher.Register("interaction.runtime_assembled", interactionPublisher)
	orch := interaction.NewOrchestratorWithStores(orchCfg, chatSvc.(interaction.MessageProcessor), tracker, newOutboxStore)
	if temporalSvc.FeatureFlags().RelationshipTimeEnabled {
		orch.SetRelationshipTimeCoordinator(relTimeCoordinator)
		chatSvc.SetRelationshipTimeCoordinator(relTimeCoordinator)
	}
	charRepo := character.NewRepository(ctx)
	runtimeRegistry := newRuntimeContextLoaderRegistry(ctx, charRepo, temporalSvc)
	runtimePipeline := interaction.NewRuntimePipeline(runtimeRegistry, interaction.NewPathClassifier(), interaction.NewTokenBudgetManager(2400))
	runtimePipeline.SetPersonalityCompiler(personality.NewCompiler(personality.DefaultCompilerConfig()))
	runtimePipeline.SetSafetyGovernor(safety.NewGovernor(safety.DefaultGovernorConfig()))
	runtimePipeline.SetBeliefResolver(belief.ResolveBelief)
	runtimePipeline.SetAppraisalEngine(appraisal.NewEngine(appraisal.DefaultAppraisalConfig()))
	runtimePipeline.SetBudgetController(budget.NewBudgetController(0.5))
	runtimePipeline.SetDecisionLayer(decision.DefaultCandidateRegistry(), decision.DefaultArbitrationLayer())
	orch.SetRuntimePipeline(runtimePipeline)
	deadlineCfg := mindruntime.DefaultDeadlineConfig
	deadlineCfg.TotalTimeout = 180 * time.Second
	deadlineCfg.GenerationTimeout = 120 * time.Second
	dp := mindruntime.NewDeadlinePropagator(deadlineCfg)
	orch.SetDeadlineProvider(func(ctx context.Context, requestID string) (context.Context, context.CancelFunc) {
		return dp.ContextWithDeadline(ctx, requestID, mindruntime.DeadlineStageGeneration)
	})
	resolver := interaction.NewScopeResolverWithDefaultChar(interaction.NewConversationScopeBindingLookup(ctx.DB), &defaultCharacterProvider{repo: charRepo})
	dataLifecycle := mindruntime.NewDataLifecycleCoordinator(ctx.DB)
	if err := dataLifecycle.InitSchema(); err != nil {
		log.Error("failed to init data lifecycle schema:", err)
		panic("failed to init data lifecycle schema")
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
	chatSvc.EnsureChannelConversation("wechat")
	chatSvc.EnsureChannelConversation("qq")

	entry := interaction.NewUnifiedEntry(orch, resolver, temporal.SystemClock{})
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
		log.Error("failed to init delivery store schema:", err)
		panic("failed to init delivery store schema")
	}
	configureWorkflowHost(extensionRuntime, chatSvc, memSvc, deliveryStore)
	mcpRepository := mcp.NewRepository(ctx.DB)
	mcpStorageDir := mcpDataDirectory(ctx)
	secretStore, err := mcpauth.NewEncryptedFileStore(filepath.Join(mcpStorageDir, "mcp-secrets.json"), filepath.Join(mcpStorageDir, "mcp-secrets.key"))
	if err != nil {
		panic("failed to initialize MCP secret store")
	}
	oauthManager := mcpauth.NewManager(nil, secretStore, mcpRepository)
	connectionManager := mcpmanager.New(mcpRepository, mcpmanager.DefaultFactory{Repository: mcpRepository, Secrets: secretStore, OAuth: oauthManager}, mcpmanager.Config{Connection: mcpclient.Config{ClientInfo: protocol.Implementation{Name: "amitia", Title: "Amitia", Version: "1.0.0"}, Capabilities: protocol.ClientCapabilities{Roots: map[string]any{"listChanged": true}, Sampling: map[string]any{}, Elicitation: map[string]any{}, Tasks: map[string]any{}}}})
	discoveryService := mcpdiscovery.New(mcpRepository, connectionManager)
	skillRuntime := mcpskill.New(mcpRepository, connectionManager, extensionRuntime)
	featureService := mcpfeatures.New(mcpRepository, connectionManager)
	interactionBroker := mcphost.NewBroker(chatSvc)
	hostService := mcphost.New(mcpRepository, connectionManager, mcphost.NewConfiguredRoots(mcpRepository), interactionBroker, interactionBroker)
	dependencyService := mcpdependency.New(mcpRepository, connectionManager, discoveryService, skillRuntime)
	extensionRuntime.AgentSkills.SetAfterRemove(func(ctx context.Context, extensionID string) {
		_, _ = dependencyService.Uninstall(ctx, extensionID)
	})
	connectionManager.RegisterReadyHandler(func(readyCtx context.Context, serverID string) {
		hostService.Attach(serverID)
		if discoverErr := discoveryService.Discover(readyCtx, serverID); discoverErr != nil {
			log.Warn("MCP capability discovery failed server=", serverID, " error=", discoverErr)
			return
		}
		if registerErr := skillRuntime.RegisterServer(readyCtx, serverID); registerErr != nil {
			log.Warn("MCP skill registration failed server=", serverID, " error=", registerErr)
		}
	})
	if err := skillRuntime.RegisterAll(context.Background()); err != nil {
		log.Warn("MCP skill restore warning: ", err)
	}
	if err := connectionManager.Restore(context.Background()); err != nil {
		log.Warn("MCP connection restore warning: ", err)
	}
	desktopPetRepo := desktoppet.NewRepository(ctx.DB, ctx)
	desktopPetRegistry := desktoppet.NewProviderRegistry()
	desktopPetWorker := worker.NewWorker(ctx.DB, desktopPetRepo, desktopPetRegistry)
	processingRepo := processing.NewRepository(ctx.DB, ctx)
	processingDataDir := mcpDataDirectory(ctx)
	processingWorker := processingworker.NewWorker(ctx.DB, processingRepo, processingDataDir)
	installationRepo := installation.NewRepository(ctx.DB, ctx)
	installationInstaller := installation.NewInstaller(installationRepo, processingRepo, charRepo, processingDataDir)
	installationUninstaller := installation.NewUninstaller(installationRepo, processingDataDir)
	installationService := installation.NewService(installationRepo, installationInstaller, installationUninstaller, processingRepo, charRepo, processingDataDir)
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
		DesktopPetWorker:    desktopPetWorker,
		ProcessingWorker:    processingWorker,
		InstallationService: installationService,
		Reconciliation:      reconciliationEngine,
		CircuitBreakers:     cbRegistry,
		VoiceEntry:          voiceEntry,
		Extension:           extensionRuntime,
		Emote:               emoteSvc,
		Temporal:            temporalSvc,
		RelTimeCoordinator:  relTimeCoordinator,
		MCPRepository:       mcpRepository,
		MCPConnections:      connectionManager,
		MCPAuth:             oauthManager,
		MCPDiscovery:        discoveryService,
		MCPSkills:           skillRuntime,
		MCPSecrets:          secretStore,
		MCPFeatures:         featureService,
		MCPHost:             hostService,
		MCPInteractions:     interactionBroker,
		MCPDependencies:     dependencyService,
	}
}

func mcpDataDirectory(ctx *app.AppContext) string {
	if config.AppCfg != nil && strings.TrimSpace(config.AppCfg.Storage.DataDir) != "" {
		return config.AppCfg.Storage.DataDir
	}
	var databases []struct {
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	if ctx != nil && ctx.DB != nil && ctx.DB.Raw("PRAGMA database_list").Scan(&databases).Error == nil {
		for _, database := range databases {
			if database.Name == "main" && strings.TrimSpace(database.File) != "" {
				return filepath.Dir(database.File)
			}
		}
	}
	return filepath.Join(".", "data")
}

func configureWorkflowHost(runtime *extension.Runtime, chatSvc chat.Service, memSvc memory.Service, deliveryStore *delivery.SQLiteDeliveryStore) {
	runtime.WorkflowHost.Schedule = func(ctx context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload map[string]interface{}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("日程参数无效: %w", err)
		}
		if payload["due_time"] == nil && payload["dueAt"] != nil {
			payload["due_time"] = payload["dueAt"]
		}
		idempotencyKey, _ := payload["idempotencyKey"].(string)
		normalized, _ := json.Marshal(payload)
		registered, err := runtime.Registry.Get(ctx, "dev.amitia.skill.create-schedule")
		if err != nil {
			return nil, nil, err
		}
		result, err := registered.Handler(ctx, extension.ExecuteSkillRequest{SkillID: registered.Definition.ID, Input: normalized, Scope: scope, IdempotencyKey: idempotencyKey})
		return result.Output, result.SideEffects, err
	}
	runtime.WorkflowHost.Notification = func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("通知参数无效: %w", err)
		}
		payload.Content = strings.TrimSpace(payload.Content)
		if payload.Content == "" || len([]rune(payload.Content)) > 4000 {
			return nil, nil, fmt.Errorf("通知内容长度必须为 1 到 4000 个字符")
		}
		conversation, err := chatSvc.GetConversation(scope.ConversationID)
		if err != nil {
			return nil, nil, err
		}
		if conversation.CharacterID != scope.CharacterID || conversation.Channel != scope.Channel || conversation.PeerID == "" {
			return nil, nil, fmt.Errorf("通知只能发送到当前角色和会话绑定的渠道")
		}
		body, _ := json.Marshal(map[string]string{"content": payload.Content})
		interactionID := scope.RequestID
		if interactionID == "" {
			interactionID = uuid.New().String()
		}
		intent := delivery.NewDeliveryIntent(interactionID, conversation.Channel, conversation.PeerID, "text", body)
		if err := deliveryStore.CreateIntent(intent); err != nil {
			return nil, nil, err
		}
		output, _ := json.Marshal(map[string]string{"intentId": intent.ID, "status": string(intent.Status)})
		return output, []extension.SideEffectRecord{{Type: "notification_send", TargetID: intent.ID, Confirmed: true}}, nil
	}
	runtime.WorkflowHost.MemoryCandidate = func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Key        string `json:"key"`
			Value      string `json:"value"`
			MemoryType string `json:"memoryType"`
			Importance int    `json:"importance"`
			Source     string `json:"source"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("候选记忆参数无效: %w", err)
		}
		candidate, err := memSvc.SubmitCandidate(&memory.SubmitCandidateRequest{Key: payload.Key, Value: payload.Value, MemoryType: payload.MemoryType, Importance: payload.Importance, SourceText: payload.Source, ConversationID: scope.ConversationID, CharacterID: scope.CharacterID})
		if err != nil {
			return nil, nil, err
		}
		output, _ := json.Marshal(map[string]interface{}{"candidateId": candidate.ID, "status": "pending_review"})
		return output, []extension.SideEffectRecord{{Type: "memory_candidate_write", TargetID: candidate.ID, Confirmed: true}}, nil
	}
	runtime.WorkflowHost.ContextContribution = func(_ context.Context, input json.RawMessage, scope extension.ExecutionScope) (json.RawMessage, []extension.SideEffectRecord, error) {
		var payload struct {
			Content    string `json:"content"`
			TokenLimit int    `json:"tokenLimit"`
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, nil, fmt.Errorf("上下文贡献参数无效: %w", err)
		}
		payload.Content = strings.TrimSpace(payload.Content)
		if payload.Content == "" || payload.TokenLimit < 1 || payload.TokenLimit > 1024 || len([]rune(payload.Content)) > payload.TokenLimit*8 {
			return nil, nil, fmt.Errorf("上下文贡献超出 1024 token 宿主限制")
		}
		output, _ := json.Marshal(map[string]interface{}{"content": payload.Content, "tokenLimit": payload.TokenLimit, "conversationId": scope.ConversationID})
		return output, []extension.SideEffectRecord{{Type: "context_injection", TargetID: scope.ConversationID, Confirmed: true}}, nil
	}
}

func newRuntimeContextLoaderRegistry(ctx *app.AppContext, charRepo character.Repository, temporalServices ...*temporal.Service) *interaction.ContextLoaderRegistry {
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
	if len(temporalServices) > 0 && temporalServices[0] != nil {
		runtimeRegistry.Register(interaction.NewTemporalContextLoader(temporalServices[0]))
	}
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

type noopPublisher struct{}

func (p *noopPublisher) Publish(record newoutbox.OutboxRecord) error {
	return nil
}
