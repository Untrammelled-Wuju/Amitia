package system

import "github.com/gin-gonic/gin"

func registerContinuityRoutes(r *gin.RouterGroup, handler *Handler) {
	group := r.Group("/continuity", sharedCoreAdminOnly())
	group.GET("/threads", handler.ContinuityListThreads)
	group.POST("/threads", handler.ContinuityCreateThread)
	group.GET("/threads/:id", handler.ContinuityGetThread)
	group.PATCH("/threads/:id", handler.ContinuityUpdateThread)
	group.GET("/threads/:id/events", handler.ContinuityListEvents)
	group.GET("/threads/:id/waits", handler.ContinuityListWaits)
	group.POST("/threads/:id/waits", handler.ContinuityCreateWait)
	group.POST("/threads/:id/waits/:waitId/resolve", handler.ContinuityResolveWait)
	group.POST("/threads/:id/waits/:waitId/cancel", handler.ContinuityCancelWait)
	group.POST("/signals", handler.ContinuitySignal)
}
