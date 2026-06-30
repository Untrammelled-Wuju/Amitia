// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterFeedbackRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc)

	r.POST("/messages/:id/feedback", handler.Create)
	r.GET("/messages/:id/feedback", handler.GetByMessage)
	r.GET("/messages/feedback/stats", handler.Stats)
	r.GET("/messages/feedback/recent", handler.Recent)
	r.DELETE("/messages/feedback/:id", handler.Delete)
}
