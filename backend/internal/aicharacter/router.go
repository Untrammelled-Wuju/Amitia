// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package aicharacter

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterAICharacterRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	svc := NewService(ctx)
	handler := NewHandler(svc)

	r.GET("/ai/character/default", handler.GetDefault)
	r.GET("/ai/character/presets", handler.GetPresets)
	r.GET("/ai/character/presets/:id", handler.GetPreset)
	r.GET("/ai/character/:id", handler.GetCharacter)
	r.POST("/ai/character/:id/set-default", handler.SetDefault)
	r.POST("/ai/character/preview-prompt", handler.PreviewPrompt)
	r.POST("/ai/character/reset-default", handler.ResetDefault)
	r.POST("/ai/character/save", handler.SaveCharacter)
}
