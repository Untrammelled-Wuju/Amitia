// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterWorldBookRouter(r *gin.RouterGroup, ctx *app.AppContext, graphSvc graph.Service) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, graphSvc)
	handler := NewHandler(svc)

	r.GET("/world-book", handler.List)
	r.POST("/world-book", handler.Create)
	r.PUT("/world-book/:id", handler.Update)
	r.DELETE("/world-book/:id", handler.Delete)
	r.POST("/world-book/match", handler.TestMatch)
	r.DELETE("/world-book", handler.DeleteAll)
	r.GET("/world-book/system-prompt", handler.SystemPrompt)
}
