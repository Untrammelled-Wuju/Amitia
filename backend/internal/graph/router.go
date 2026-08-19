// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
)

func RegisterGraphRouter(r *gin.RouterGroup, cfg config.SurrealConfig) {
	var svc Service
	client, err := NewClient(cfg)
	if err != nil {
		log.Warn("Graph SurrealDB 初始连接失败，将使用延迟连接:", err)
		svc = NewRetryingService(cfg)
	} else {
		svc = NewService(client)
	}
	handler := NewHandler(svc)

	g := r.Group("/graph")
	{
		g.GET("/node/:id/neighbors", handler.Neighbors)
		g.GET("/path", handler.FindPath)
		g.GET("/stats", handler.Stats)
		g.GET("/nodes", handler.AllNodes)
		g.GET("/edges", handler.AllEdges)
		g.DELETE("/node/:id", handler.DeleteNode)
	}
}
