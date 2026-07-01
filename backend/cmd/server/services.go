package main

import (
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/pkg/app"
)

type AppServices struct {
	Graph     graph.Service
	Memory    memory.Service
	Profile   profile.Service
	Episodic  episodic.Service
	WorldBook worldbook.Service
	Vision    vision.Service
	Companion companion.Service
	Chat      chat.Service
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
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, wbSvc, compressor, visionSvc, graphSvc)
	return &AppServices{
		Graph:     graphSvc,
		Memory:    memSvc,
		Profile:   profSvc,
		Episodic:  epiSvc,
		WorldBook: wbSvc,
		Vision:    visionSvc,
		Companion: compSvc,
		Chat:      chatSvc,
	}
}
