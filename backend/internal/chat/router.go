// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterChatRouter(r *gin.RouterGroup, ctx *app.AppContext, svc Service) {
	handler := NewHandler(svc)

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
		modelGroup.GET("/configs", handler.ListModels)
		modelGroup.GET("/configs/:id", handler.GetModel)
		modelGroup.POST("/configs", handler.CreateModel)
		modelGroup.PUT("/configs/:id", handler.UpdateModel)
		modelGroup.DELETE("/configs/:id", handler.DeleteModel)
		modelGroup.POST("/configs/:id/activate", handler.ActivateModel)
		modelGroup.POST("/configs/:id/active", handler.ActivateModel)
		modelGroup.POST("/configs/:id/test", handler.TestModel)
		modelGroup.POST("/test", handler.TestModelStandalone)
		modelGroup.GET("/routes", handler.GetModelRoutes)
		modelGroup.PUT("/routes", handler.UpdateModelRoutes)
		modelGroup.POST("/detect-models", handler.DetectModels)
		modelGroup.GET("/providers", handler.ListProviders)
		modelGroup.GET("/providers/:id/schema", handler.ProviderSchema)
	}
}
