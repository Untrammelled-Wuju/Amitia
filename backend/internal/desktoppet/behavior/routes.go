package behavior

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, svc *BehaviorService) {
	handler := NewHandler(svc)
	g := r.Group("/desktop-pet/behavior")
	{
		g.GET("/state/:userId/:characterId", handler.GetBehaviorState)
		g.GET("/metrics", handler.GetMetrics)
		g.POST("/simulate", handler.SimulateEvent)
		g.POST("/reconcile", handler.TriggerReconcile)
		g.PUT("/shadow-mode", handler.SetShadowMode)
		g.PUT("/runtime-command", handler.SetRuntimeCommand)
		g.GET("/bindings", handler.ListBindings)
		g.POST("/bindings", handler.CreateBinding)
		g.GET("/bindings/:id", handler.GetBinding)
		g.PUT("/bindings/:id", handler.UpdateBinding)
		g.DELETE("/bindings/:id", handler.DeleteBinding)
	}
}
