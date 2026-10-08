// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/sync"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterCharacterRouter(r *gin.RouterGroup, ctx *app.AppContext, chatTester ChatTester) {
	RegisterCharacterRouterWithRecorder(r, ctx, chatTester, nil)
}

func RegisterCharacterRouterWithRecorder(r *gin.RouterGroup, ctx *app.AppContext, chatTester ChatTester, recorder sync.ChangeRecorder) {
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx, recorder)
	handler := NewHandler(svc)
	handler.chatTester = chatTester
	admin := security.SharedCoreAdminOnly()
	intent := handler.guardRoleAuthority()

	r.GET("/characters", handler.List)
	r.GET("/characters/authority", handler.Authority)
	r.POST("/characters/generate-card", handler.GenerateCard)
	r.GET("/characters/:id", handler.Get)
	r.GET("/characters/:id/card-data", intent, handler.GetCardData)
	r.PUT("/characters/:id/card-data", admin, intent, handler.UpdateCardData)
	r.POST("/characters", admin, intent, handler.Create)
	r.PUT("/characters/:id", admin, intent, handler.Update)
	r.DELETE("/characters/:id", admin, intent, handler.Delete)
	r.POST("/characters/:id/active", admin, intent, handler.SetActive)
	r.POST("/characters/:id/test", handler.Test)

	r.POST("/characters/:id/export-pack", handler.ExportPack)
	r.POST("/characters/import-pack/preview", handler.ImportPackPreview)
	r.POST("/characters/import-pack/confirm", admin, intent, handler.ImportPackConfirm)
	r.GET("/characters/packs/history", handler.PacksHistory)

	r.POST("/characters/import-card/preview", handler.ImportPackPreview)
	r.POST("/characters/import-card/confirm", admin, intent, handler.ImportPackConfirm)
	r.GET("/characters/:id/export-card", handler.ExportCardV2)

	r.GET("/character-templates", handler.ListTemplates)
	r.GET("/character-templates/:id", handler.GetTemplate)
	r.POST("/character-templates/:id/create-character", admin, intent, handler.CreateFromTemplate)
	r.GET("/companion/role-profile", handler.GetRoleProfile)
	r.PUT("/companion/role-profile", admin, intent, handler.UpdateRoleProfile)
	r.POST("/characters/:id/avatar", admin, intent, handler.UploadAvatar)
	r.GET("/characters/:id/avatar", handler.GetAvatar)
}
