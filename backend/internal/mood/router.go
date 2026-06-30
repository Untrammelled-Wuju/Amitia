// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterMoodRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	svc := NewService(ctx)
	handler := NewHandler(svc)

	r.GET("/moods", handler.List)
	r.GET("/moods/conversations/:id", handler.GetByConversation)
	r.DELETE("/moods/:id", handler.Delete)
	r.DELETE("/moods/conversations/:id", handler.DeleteByConversation)
}
