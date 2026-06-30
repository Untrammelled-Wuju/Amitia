// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterEpisodicRouter(r *gin.RouterGroup, ctx *app.AppContext, graphSvc graph.Service) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, graphSvc)
	handler := NewHandler(svc)

	r.GET("/episodic", handler.List)
	r.POST("/episodic", handler.Create)
	r.DELETE("/episodic/:id", handler.Delete)
	r.GET("/episodic/by-user", handler.GetByUserID)
	r.GET("/episodic/:id/detail", handler.GetDetail)
	r.POST("/episodic/extract", handler.Extract)
	r.GET("/episodic/system-prompt", handler.SystemPrompt)
}
