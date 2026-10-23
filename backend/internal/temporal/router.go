package temporal

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterRouter(group *gin.RouterGroup, service *Service) {
	handler := NewHandler(service)
	temporal := group.Group("/temporal")
	{
		temporal.GET("/profile", handler.GetUserProfile)
		temporal.PUT("/profile", handler.UpdateUserProfile)
		temporal.GET("/characters/:characterId/profile", handler.GetCharacterProfile)
		temporal.PUT("/characters/:characterId/profile", handler.UpdateCharacterProfile)
		temporal.GET("/snapshot", handler.GetSnapshot)
		temporal.GET("/diagnostics", security.SharedCoreAdminOnly(), handler.GetDiagnostics)
		temporal.GET("/anchors", handler.ListAnchors)
		temporal.POST("/anchors", handler.CreateAnchor)
		temporal.PUT("/anchors/:id", handler.UpdateAnchor)
		temporal.DELETE("/anchors/:id", handler.DeleteAnchor)
		temporal.POST("/anchors/:id/confirm", handler.ConfirmAnchor)
		temporal.GET("/events", handler.ListEvents)
		temporal.POST("/recompute", security.SharedCoreAdminOnly(), handler.Recompute)
		temporal.POST("/timezone-suggestion", handler.SuggestTimezone)
		temporal.POST("/timezone-suggestion/accept", handler.AcceptTimezoneSuggestion)
		temporal.POST("/timezone-suggestion/reject", handler.RejectTimezoneSuggestion)
	}
}
