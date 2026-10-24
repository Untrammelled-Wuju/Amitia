package release

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

func RegisterRoutes(r *gin.RouterGroup, svc ReleaseService, guard security.OwnershipGuard) {
	handler := NewHandler(svc, guard)
	g := r.Group("/desktop-pets/releases")
	{
		g.GET("", handler.ListReleases)
		g.GET("/pet/:petId", handler.ListReleasesForPet)
		g.GET("/:releaseId", handler.GetRelease)
		g.GET("/:releaseId/files", handler.GetReleaseFiles)
		g.POST("/:releaseId/archive", handler.ArchiveRelease)
		g.POST("/:releaseId/revoke", handler.RevokeRelease)
		g.GET("/:releaseId/download", handler.DownloadRelease)
	}
	g2 := r.Group("/desktop-pets/pets")
	{
		g2.GET("/identity", handler.GetPetIdentity)
	}
}
