// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	service := NewService(ctx.DB)
	handler := NewHandler(service)
	group := r.Group("/user-feedback")
	group.GET("/settings", handler.Settings)
	group.PUT("/settings", handler.SaveSettings)
	group.POST("/diagnostics/preview", handler.Preview)
	group.POST("/tickets", handler.Create)
	group.GET("/tickets", handler.List)
	group.POST("/tickets/:id/retry", handler.Retry)
	service.StartWorker()
}
