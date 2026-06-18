package graph

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
)

func RegisterGraphRouter(r *gin.RouterGroup, cfg config.SurrealConfig) {
	client, err := NewClient(cfg)
	if err != nil {
		return
	}
	svc := NewService(client)
	handler := NewHandler(svc)

	g := r.Group("/graph")
	{
		g.GET("/node/:id/neighbors", handler.Neighbors)
		g.GET("/path", handler.FindPath)
		g.GET("/stats", handler.Stats)
		g.DELETE("/node/:id", handler.DeleteNode)
	}
}
