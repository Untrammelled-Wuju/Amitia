// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package vision

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterVisionRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	svc := NewService(repo)
	handler := NewHandler(svc)

	g := r.Group("/vision")
	{
		g.GET("/configs", handler.List)
		g.GET("/configs/:id", handler.Get)
		g.POST("/configs", handler.Create)
		g.PUT("/configs/:id", handler.Update)
		g.DELETE("/configs/:id", handler.Delete)
		g.POST("/configs/:id/activate", handler.Activate)
		g.POST("/configs/:id/test", handler.TestConnection)
	}
}
