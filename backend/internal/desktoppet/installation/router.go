package installation

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

func RegisterRoutes(r *gin.RouterGroup, svc Service, guard security.OwnershipGuard) {
	handler := NewHandler(svc, guard)
	g := r.Group("/desktop-pets")
	{
		g.POST("/packages/:packageId/install", handler.InstallPackage)
		g.GET("/installations", handler.ListInstallations)
		g.GET("/installations/:installationId", handler.GetInstallation)
		g.POST("/installations/:installationId/enable", handler.EnableInstallation)
		g.POST("/installations/:installationId/disable", handler.DisableInstallation)
		g.PATCH("/installations/:installationId/default-action", handler.UpdateDefaultAction)
		g.PATCH("/installations/:installationId/settings", handler.UpdateRuntimeSettings)
		g.POST("/installations/:installationId/recenter", handler.Recenter)
		g.POST("/installations/:installationId/actions/:actionKey/play", handler.PlayAction)
	}
}
