package reminder

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"gorm.io/gorm"
)

func RegisterRouter(r *gin.RouterGroup, db *gorm.DB, dispatcher Dispatcher) *Handler {
	service := NewService(db, dispatcher)
	handler := NewHandler(service)
	group := r.Group("/reminders")
	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.PUT("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)
	group.POST("/:id/toggle", handler.Toggle)
	group.POST("/:id/test", handler.Test)
	group.POST("/:id/trigger", handler.Trigger)
	group.GET("/status", handler.Status)
	group.GET("/trigger-history", handler.History)
	group.GET("/prospective", handler.Prospective)
	group.GET("/queue-summary", handler.QueueSummary)
	group.GET("/cleanup-config", security.SharedCoreAdminOnly(), handler.GetCleanupConfig)
	group.PUT("/cleanup-config", security.SharedCoreAdminOnly(), handler.SetCleanupConfig)
	group.POST("/clear-backpressure", security.SharedCoreAdminOnly(), handler.ClearBackpressure)
	service.Start(context.Background())
	return handler
}
