// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"github.com/gin-gonic/gin"
)

func RegisterEpisodicRouter(r *gin.RouterGroup, svc Service) {
	handler := NewHandler(svc)

	r.GET("/episodic", handler.List)
	r.POST("/episodic", handler.Create)
	r.DELETE("/episodic/:id", handler.Delete)
	r.PUT("/episodic/:id/retention", handler.UpdateRetention)
	r.POST("/episodic/:id/restore", handler.Restore)
	r.GET("/episodic/by-user", handler.GetByUserID)
	r.GET("/episodic/:id/detail", handler.GetDetail)
	r.POST("/episodic/extract", handler.Extract)
	r.GET("/episodic/system-prompt", handler.SystemPrompt)
}
