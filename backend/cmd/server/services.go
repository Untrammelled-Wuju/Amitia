// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
)

type AppServices struct {
	Graph         graph.Service
	Memory        memory.Service
	Profile       profile.Service
	Episodic      episodic.Service
	WorldBook     worldbook.Service
	Vision        vision.Service
	Companion     companion.Service
	Chat          chat.Service
	UnifiedEntry  *interaction.UnifiedEntry
	DataLifecycle *mindruntime.DataLifecycleCoordinator
	OutboxRuntime *interaction.OutboxRuntime
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
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, wbSvc, compressor, visionSvc, graphSvc, psycheStore)
	orchCfg := interaction.DefaultOrchestratorConfig()
	tracker := interaction.NewSQLiteInteractionTracker(ctx.DB)
	outbox := interaction.NewSQLiteOutboxStore(ctx.DB)
	deadStore := interaction.NewInMemoryDeadLetterStore()
	if err := tracker.InitSchema(); err != nil {
		panic("failed to init interaction tracker schema: " + err.Error())
	}
	if err := outbox.InitSchema(); err != nil {
		panic("failed to init outbox schema: " + err.Error())
	}
	outboxPublisher := interaction.OutboxPublisherFunc(func(record interaction.OutboxRecord) error {
		log.Info("interaction outbox event published id=", record.ID, " type=", record.EventType, " aggregate=", record.AggregateID)
		return nil
	})
	outboxRuntime := interaction.NewOutboxRuntime(outbox, deadStore, outboxPublisher, interaction.OutboxWorkerConfig{})
	orch := interaction.NewOrchestratorWithStores(orchCfg, chatSvc.(interaction.MessageProcessor), tracker, outbox)
	runtimeRegistry := interaction.NewContextLoaderRegistry()
	runtimeRegistry.Register(interaction.NewChannelContextLoader())
	runtimeRegistry.Register(interaction.NewConversationContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewPsycheContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewRelationshipContextLoader(ctx.DB))
	runtimeRegistry.Register(interaction.NewBeliefContextLoader(ctx.DB))
	orch.SetRuntimePipeline(interaction.NewRuntimePipeline(runtimeRegistry, interaction.NewPathClassifier(), interaction.NewTokenBudgetManager(2400)))
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
	entry := interaction.NewUnifiedEntry(orch, resolver)
	return &AppServices{
		Graph:         graphSvc,
		Memory:        memSvc,
		Profile:       profSvc,
		Episodic:      epiSvc,
		WorldBook:     wbSvc,
		Vision:        visionSvc,
		Companion:     compSvc,
		Chat:          chatSvc,
		UnifiedEntry:  entry,
		DataLifecycle: dataLifecycle,
		OutboxRuntime: outboxRuntime,
	}
}
