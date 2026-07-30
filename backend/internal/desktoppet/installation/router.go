package installation

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, svc Service) {
	handler := NewHandler(svc)
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

func RegisterReleaseRoutes(r *gin.RouterGroup, releaseSvc ReleaseService) {
	handler := NewReleaseHandler(releaseSvc)
	g := r.Group("/desktop-pets")
	{
		g.POST("/releases/build", handler.BuildRelease)
		g.POST("/releases/import", handler.ImportPackage)
		g.GET("/releases", handler.ListReleases)
		g.GET("/releases/:releaseId", handler.GetRelease)
		g.GET("/pets", handler.ListPets)
		g.GET("/pets/:petId", handler.GetPet)
		g.GET("/pets/:petId/releases", handler.ListReleasesByPet)
		g.POST("/pets/:petId/releases/:releaseId/install", handler.InstallRelease)
		g.POST("/installations/:installationId/upgrade", handler.UpgradeInstallation)
		g.POST("/installations/:installationId/switch", handler.SwitchInstallation)
		g.POST("/installations/:installationId/repair", handler.RepairInstallation)
		g.DELETE("/installations/:installationId", handler.UninstallInstallation)
	}
}
