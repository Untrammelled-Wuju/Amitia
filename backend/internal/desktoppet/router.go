// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/pkg/app"
)

func RegisterDesktopPetRouter(r *gin.RouterGroup, ctx *app.AppContext, registry *security.PathRootRegistry) {
	repo := NewRepository(ctx.DB, ctx)
	svc, err := NewService(repo, ctx.DB)
	if err != nil {
		panic(fmt.Errorf("failed to initialize desktoppet service: %w", err))
	}
	if registry == nil {
		registry = security.NewPathRootRegistry()
		if err := security.EnsureAllRequiredRoots(registry, config.AppCfg.Storage.DataDir); err != nil {
			panic(fmt.Errorf("initialize required storage roots: %w", err))
		}
	}
	responder := security.NewSafeArtifactResponder(registry)
	guard := security.NewSQLiteOwnershipGuard(ctx.DB)
	handler := NewHandler(svc, responder, guard)

	readGroup := r.Group("/desktop-pets")
	{
		readGroup.GET("/action-definitions", handler.GetActionDefinitions)
		readGroup.GET("/generation-tasks", handler.ListTasks)
		readGroup.GET("/generation-tasks/:taskId", handler.GetTask)
		readGroup.GET("/generation-tasks/:taskId/reference-image", handler.ReferenceImage)
		readGroup.GET("/generation-tasks/:taskId/events", handler.TaskEventsStream) // audit:ok: handler enforces actor authentication and task ownership
		readGroup.GET("/generation-tasks/:taskId/actions/:actionKey/frames/:frameIndex/image", handler.ActionFrameImage)
		readGroup.GET("/generation-tasks/:taskId/transitions", handler.GetTaskTransitions)
	}
}

func RegisterDesktopPetWriteRouter(r *gin.RouterGroup, ctx *app.AppContext, registry *security.PathRootRegistry) {
	repo := NewRepository(ctx.DB, ctx)
	svc, err := NewService(repo, ctx.DB)
	if err != nil {
		panic(fmt.Errorf("failed to initialize desktoppet service: %w", err))
	}
	responder := security.NewSafeArtifactResponder(registry)
	guard := security.NewSQLiteOwnershipGuard(ctx.DB)
	handler := NewHandler(svc, responder, guard)

	writeGroup := r.Group("/desktop-pets")
	{
		writeGroup.POST("/generation-tasks", handler.CreateTask)
		writeGroup.DELETE("/generation-tasks/:taskId", handler.DeleteTask)
		writeGroup.POST("/generation-tasks/:taskId/start", handler.StartTask)
		writeGroup.POST("/generation-tasks/:taskId/cancel", handler.CancelTask)
		writeGroup.POST("/generation-tasks/:taskId/actions/:actionKey/retry", handler.RetryAction)
	}
}
