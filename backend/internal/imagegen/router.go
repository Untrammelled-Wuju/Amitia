// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imagegen

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterImageGenRouter(r *gin.RouterGroup, svc Service) {
	handler := NewHandler(svc)

	g := r.Group("/imagegen")
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
