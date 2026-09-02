// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/androiduiagent"
	"github.com/u-ai/backend/internal/browseragent"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAgentRouter(r *gin.RouterGroup, ctx *app.AppContext, unifiedEntry *interaction.UnifiedEntry, toolFacade *extensionkernel.ToolFacade) {
	svc := NewService(ctx, unifiedEntry, toolFacade)
	handler := NewHandler(svc)
	if runner, ok := svc.(androiduiagent.Runner); ok {
		androiduiagent.SetRunner(runner)
	}
	if runner, ok := svc.(browseragent.Runner); ok {
		browseragent.SetRunner(runner)
	}

	r.POST("/agent/test", handler.Test)
	r.GET("/agent/context-preview", handler.ContextPreview)
	r.POST("/agent/webhook", handler.Webhook)
}
