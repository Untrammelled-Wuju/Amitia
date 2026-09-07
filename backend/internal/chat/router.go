// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterChatRouter(r *gin.RouterGroup, ctx *app.AppContext, svc Service, entry *interaction.UnifiedEntry) {
	handler := NewHandlerWithUnifiedEntry(svc, entry)
	registerChatRoutes(r, handler)
}

func RegisterChatRouterWithDelivery(r *gin.RouterGroup, ctx *app.AppContext, svc Service, entry *interaction.UnifiedEntry, ds DeliveryStore) {
	handler := NewHandlerWithUnifiedEntryAndDelivery(svc, entry, ds)
	registerChatRoutes(r, handler)
}

func registerChatRoutes(r *gin.RouterGroup, handler *Handler) {
	r.POST("/chat", handler.Chat)

	chatsGroup := r.Group("/chats")
	{
		chatsGroup.GET("/stats", handler.Stats)
		chatsGroup.GET("/conversations", handler.ListConversations)
		chatsGroup.POST("/conversations", handler.CreateConversation)
		chatsGroup.GET("/conversations/:id/messages", handler.GetMessages)
		chatsGroup.DELETE("/conversations/:id", handler.DeleteConversation)
		chatsGroup.DELETE("/conversations/:id/messages", handler.DeleteMessages)
		chatsGroup.DELETE("/messages/:id", handler.DeleteSingleMessage)
		chatsGroup.GET("/search", handler.SearchMessages)
		chatsGroup.PUT("/conversations/:id/character", handler.ChangeCharacter)
		chatsGroup.DELETE("/all", handler.DeleteAllConversations)
		chatsGroup.GET("/conversations/:id/summary", handler.GetSummary)
		chatsGroup.PUT("/conversations/:id/summary", handler.UpdateSummary)
		chatsGroup.DELETE("/conversations/:id/summary", handler.DeleteSummary)
		chatsGroup.POST("/conversations/:id/summary/generate", handler.GenerateSummary)
		chatsGroup.POST("/cleanup/preview", handler.CleanupPreview)
		chatsGroup.POST("/cleanup/confirm", handler.CleanupConfirm)
		chatsGroup.POST("/cleanup/vacuum", handler.CleanupVacuum)
		chatsGroup.GET("/conversations/:id/compression-status", handler.CompressionStatus)
		chatsGroup.POST("/export", handler.Export)
	}
	modelGroup := r.Group("/model")
	{
		admin := security.SharedCoreAdminOnly()
		modelGroup.GET("/configs", admin, handler.ListModels)
		modelGroup.GET("/configs/:id", admin, handler.GetModel)
		modelGroup.POST("/configs", admin, handler.CreateModel)
		modelGroup.PUT("/configs/:id", admin, handler.UpdateModel)
		modelGroup.DELETE("/configs/:id", admin, handler.DeleteModel)
		modelGroup.POST("/configs/:id/activate", admin, handler.ActivateModel)
		modelGroup.POST("/configs/:id/active", admin, handler.ActivateModel)
		modelGroup.POST("/configs/:id/test", admin, handler.TestModel)
		modelGroup.POST("/test", admin, handler.TestModelStandalone)
		modelGroup.GET("/routes", admin, handler.GetModelRoutes)
		modelGroup.PUT("/routes", admin, handler.UpdateModelRoutes)
		modelGroup.POST("/detect-models", admin, handler.DetectModels)
		modelGroup.GET("/providers", handler.ListProviders)
		modelGroup.GET("/providers/:id/schema", handler.ProviderSchema)
	}
}
