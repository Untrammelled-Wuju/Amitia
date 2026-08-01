package release

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, svc ReleaseService) {
	handler := NewHandler(svc)
	g := r.Group("/releases")
	{
		g.POST("/build", handler.BuildRelease)
		g.GET("/operations/:operationId", handler.GetBuildOperation)
		g.POST("/operations/:operationId/cancel", handler.CancelBuildOperation)
		g.GET("/list", handler.ListReleases)
		g.GET("/pet/:petId", handler.ListReleasesForPet)
		g.GET("/:releaseId", handler.GetRelease)
		g.GET("/:releaseId/files", handler.GetReleaseFiles)
		g.POST("/:releaseId/archive", handler.ArchiveRelease)
		g.POST("/:releaseId/revoke", handler.RevokeRelease)
		g.GET("/:releaseId/download", handler.DownloadRelease)
	}
	g2 := r.Group("/pets")
	{
		g2.GET("/identity", handler.GetPetIdentity)
	}
}
