// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAgentRouter(r *gin.RouterGroup, ctx *app.AppContext, profSvc profile.Service, epiSvc episodic.Service, graphSvc graph.Service) {
	memRepo := memory.NewRepository(ctx)
	memSvc := memory.NewService(memRepo, ctx, graphSvc)
	chatSvc := chat.NewService(chat.NewRepository(ctx), ctx, memSvc, profSvc, epiSvc, nil, nil, nil, graphSvc)
	svc := NewService(ctx, chatSvc)
	handler := NewHandler(svc)

	r.POST("/agent/test", handler.Test)
	r.GET("/agent/context-preview", handler.ContextPreview)
	r.POST("/agent/webhook", handler.Webhook)
}
