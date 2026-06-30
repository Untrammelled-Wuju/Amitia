// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterProfileRouter(r *gin.RouterGroup, ctx *app.AppContext, graphSvc graph.Service) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, graphSvc)
	handler := NewHandler(svc)

	r.GET("/profiles", handler.List)
	r.POST("/profiles", handler.Create)
	r.PUT("/profiles/:id", handler.Update)
	r.DELETE("/profiles/:id", handler.Delete)
	r.GET("/profiles/by-user", handler.GetByUserID)
	r.POST("/profiles/extract", handler.Extract)
	r.GET("/profiles/system-prompt", handler.SystemPrompt)
}
