// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterCharacterRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)
	handler := NewHandler(svc)

	r.GET("/characters", handler.List)
	r.GET("/characters/:id", handler.Get)
	r.POST("/characters", handler.Create)
	r.PUT("/characters/:id", handler.Update)
	r.DELETE("/characters/:id", handler.Delete)
	r.POST("/characters/:id/active", handler.SetActive)
	r.POST("/characters/:id/test", handler.Test)
	r.POST("/characters/:id/export-pack", handler.ExportPack)
	r.POST("/characters/import-pack/preview", handler.ImportPackPreview)
	r.POST("/characters/import-pack/confirm", handler.ImportPackConfirm)
	r.GET("/characters/packs/history", handler.PacksHistory)
	r.GET("/character-templates", handler.ListTemplates)
	r.GET("/character-templates/:id", handler.GetTemplate)
	r.POST("/character-templates/:id/create-character", handler.CreateFromTemplate)
	r.GET("/companion/role-profile", handler.GetRoleProfile)
	r.PUT("/companion/role-profile", handler.UpdateRoleProfile)
}
