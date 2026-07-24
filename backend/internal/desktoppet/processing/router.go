// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterProcessingRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB, ctx)
	svc := NewService(repo, ctx.DB, ctx, config.AppCfg.Storage.DataDir)
	handler := NewHandler(svc)

	g := r.Group("/desktop-pets")
	{
		g.POST("/generation-tasks/:taskId/process", handler.CreateProcessingTask)
		g.GET("/processing-tasks/:processingTaskId", handler.GetProcessingTask)
		g.POST("/processing-tasks/:processingTaskId/cancel", handler.CancelProcessingTask)
		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/retry", handler.RetryProcessingAction)
		g.POST("/processing-tasks/:processingTaskId/package", handler.CreatePackage)
		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/switch-attempt", handler.SwitchAttempt)
		g.POST("/processing-tasks/:processingTaskId/actions/:actionKey/exclude", handler.ExcludeAction)
		g.GET("/processing-tasks/:processingTaskId/events", handler.ProcessingEventsStream)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/frames/:frameIndex/processed-image", handler.ProcessedFrameImage)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/frames/:frameIndex/source-image", handler.SourceFrameImage)
		g.GET("/processing-tasks/:processingTaskId/actions/:actionKey/preview", handler.ActionPreview)
	}
}
