package temporal

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterRouter(group *gin.RouterGroup, service *Service) {
	handler := NewHandler(service)
	write := security.SharedCoreDataWriteOnly()
	temporal := group.Group("/temporal")
	{
		temporal.GET("/profile", handler.GetUserProfile)
		temporal.PUT("/profile", write, handler.UpdateUserProfile)
		temporal.GET("/characters/:characterId/profile", handler.GetCharacterProfile)
		temporal.PUT("/characters/:characterId/profile", write, handler.UpdateCharacterProfile)
		temporal.GET("/snapshot", handler.GetSnapshot)
		temporal.GET("/diagnostics", security.SharedCoreAdminOnly(), handler.GetDiagnostics)
		temporal.GET("/anchors", handler.ListAnchors)
		temporal.POST("/anchors", write, handler.CreateAnchor)
		temporal.PUT("/anchors/:id", write, handler.UpdateAnchor)
		temporal.DELETE("/anchors/:id", write, handler.DeleteAnchor)
		temporal.POST("/anchors/:id/confirm", write, handler.ConfirmAnchor)
		temporal.GET("/events", handler.ListEvents)
		temporal.POST("/recompute", security.SharedCoreAdminOnly(), handler.Recompute)
		temporal.POST("/timezone-suggestion", write, handler.SuggestTimezone)
		temporal.POST("/timezone-suggestion/accept", write, handler.AcceptTimezoneSuggestion)
		temporal.POST("/timezone-suggestion/reject", write, handler.RejectTimezoneSuggestion)
	}
}
