// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterDesktopPetRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB, ctx)
	svc := NewService(repo, ctx.DB)
	handler := NewHandler(svc)

	g := r.Group("/desktop-pets")
	{
		g.GET("/action-definitions", handler.GetActionDefinitions)
		g.POST("/generation-tasks", handler.CreateTask)
		g.GET("/generation-tasks", handler.ListTasks)
		g.GET("/generation-tasks/:taskId", handler.GetTask)
		g.DELETE("/generation-tasks/:taskId", handler.DeleteTask)
		g.GET("/generation-tasks/:taskId/reference-image", handler.ReferenceImage)
	}
}
