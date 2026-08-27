package installation

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

func RegisterRoutes(r *gin.RouterGroup, coord coordinator.InstallationCoordinator, repo RepositoryV2, guard security.OwnershipGuard) {
	handler := NewCoordinatorHandler(coord, repo, guard)
	g := r.Group("/desktop-pets")
	{
		g.POST("/packages/:packageId/install", handler.InstallPackage)
		g.POST("/pets/:petId/releases/:releaseId/install", handler.InstallRelease)
		g.GET("/installations", handler.ListInstallations)
		g.GET("/installations/:installationId", handler.GetInstallation)
		g.GET("/installations/:installationId/settings", handler.GetRuntimeSettings)
		g.POST("/installations/:installationId/enable", handler.EnableInstallation)
		g.POST("/installations/:installationId/disable", handler.DisableInstallation)
		g.POST("/installations/:installationId/switch", handler.SwitchRelease)
		g.POST("/installations/:installationId/upgrade", handler.Upgrade)
		g.POST("/installations/:installationId/downgrade", handler.Downgrade)
		g.POST("/installations/:installationId/repair", handler.Repair)
		g.DELETE("/installations/:installationId", handler.Uninstall)
		g.PATCH("/installations/:installationId/default-action", handler.UpdateDefaultAction)
		g.PATCH("/installations/:installationId/settings", handler.UpdateRuntimeSettings)
		g.POST("/installations/:installationId/recenter", handler.Recenter)
		g.POST("/installations/:installationId/actions/:actionKey/play", handler.PlayAction)
		g.GET("/operations/:operationId", handler.GetOperationStatus)
		g.POST("/operations/:operationId/cancel", handler.CancelOperation)
	}
}
