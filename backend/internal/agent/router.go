// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAgentRouter(r *gin.RouterGroup, ctx *app.AppContext, unifiedEntry *interaction.UnifiedEntry) {
	svc := NewService(ctx, unifiedEntry)
	handler := NewHandler(svc)

	r.POST("/agent/test", handler.Test)
	r.GET("/agent/context-preview", handler.ContextPreview)
	r.POST("/agent/webhook", handler.Webhook)
}
