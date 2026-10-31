// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

func RegisterModelRouter(r *gin.RouterGroup, svc Service) {
	handler := NewHandler(svc)
	modelGroup := r.Group("/model")
	{
		modelGroup.GET("/available", handler.AvailableModels)
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
		modelGroup.GET("/local-models", admin, handler.ListLocalModels)
		modelGroup.POST("/local-models", admin, handler.UploadLocalModel)
		modelGroup.DELETE("/local-models/:id", admin, handler.DeleteLocalModel)
		modelGroup.GET("/providers", handler.ListProviders)
		modelGroup.GET("/providers/:id/schema", handler.ProviderSchema)
	}
}
