package reminder

import "github.com/gin-gonic/gin"

func RegisterRouter(r *gin.RouterGroup, service *Service, localDeviceID string) *Handler {
	handler := NewHandler(service, localDeviceID)
	registerReminderRoutes(r, handler)
	return handler
}

func registerReminderRoutes(r *gin.RouterGroup, h *Handler) {
	group := r.Group("/reminders")
	group.GET("", h.List)
	group.POST("", h.Create)
	group.GET("/:id", h.Find)
	group.PUT("/:id", h.Update)
	group.DELETE("/:id", h.Change)
	group.POST("/:id/action", h.Change)
	group.POST("/:id/toggle", h.Toggle)
	group.POST("/:id/test", h.Test)
	group.POST("/:id/trigger", h.Trigger)
	group.GET("/status", h.Status)
	group.GET("/channels", h.Channels)
	group.GET("/trigger-history", h.History)
	group.GET("/prospective", h.Prospective)
	group.GET("/queue-summary", h.QueueSummary)
	group.GET("/cleanup-config", h.GetCleanupConfig)
	group.PUT("/cleanup-config", h.SetCleanupConfig)
	group.POST("/clear-backpressure", h.ClearBackpressure)
}
