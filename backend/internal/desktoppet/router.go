// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterDesktopPetRouter(r *gin.RouterGroup, ctx *app.AppContext) {
	repo := NewRepository(ctx.DB, ctx)
	svc := NewService(repo, ctx.DB)
	registry := security.NewPathRootRegistry()
	_ = registry.Register(security.RootGenerationArtifacts, filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "generation-artifacts"))
	_ = registry.Register(security.RootQualityReports, filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "quality-reports"))
	_ = registry.Register(security.RootReleasePublished, filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "releases"))
	_ = registry.Register(security.RootImportQuarantine, filepath.Join(config.AppCfg.Storage.DataDir, "desktop-pets", "import-quarantine"))
	responder := security.NewSafeArtifactResponder(registry)
	guard := security.NewSQLiteOwnershipGuard(ctx.DB)
	handler := NewHandler(svc, responder, guard)

	g := r.Group("/desktop-pets")
	{
		g.GET("/action-definitions", handler.GetActionDefinitions)
		g.POST("/generation-tasks", handler.CreateTask)
		g.GET("/generation-tasks", handler.ListTasks)
		g.GET("/generation-tasks/:taskId", handler.GetTask)
		g.DELETE("/generation-tasks/:taskId", handler.DeleteTask)
		g.GET("/generation-tasks/:taskId/reference-image", handler.ReferenceImage)
		g.POST("/generation-tasks/:taskId/start", handler.StartTask)
		g.POST("/generation-tasks/:taskId/cancel", handler.CancelTask)
		g.POST("/generation-tasks/:taskId/actions/:actionKey/retry", handler.RetryAction)
		g.GET("/generation-tasks/:taskId/events", handler.TaskEventsStream)
		g.GET("/generation-tasks/:taskId/actions/:actionKey/frames/:frameIndex/image", handler.ActionFrameImage)
		g.GET("/generation-tasks/:taskId/transitions", handler.GetTaskTransitions)
	}
}
