// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package vision

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterVisionRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB)
	svc := NewService(repo)
	handler := NewHandler(svc)

	g := r.Group("/vision")
	{
		g.GET("/configs", security.SharedCoreAdminOnly(), handler.List)
		g.GET("/configs/:id", security.SharedCoreAdminOnly(), handler.Get)
		g.POST("/configs", security.SharedCoreAdminOnly(), handler.Create)
		g.PUT("/configs/:id", security.SharedCoreAdminOnly(), handler.Update)
		g.DELETE("/configs/:id", security.SharedCoreAdminOnly(), handler.Delete)
		g.POST("/configs/:id/activate", security.SharedCoreAdminOnly(), handler.Activate)
		g.POST("/configs/:id/test", security.SharedCoreAdminOnly(), handler.TestConnection)
		g.GET("/providers", handler.GetProviders)
	}
}
