// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/pkg/app"
)

func registerDataPortabilityRoutes(r *gin.RouterGroup, handler *Handler) {
	registerStorageStatusRoutes(r, handler)
	registerDataPortabilityMigrationRoutes(r, handler)
	registerDataPortabilityBackupRoutes(r, handler)
}

func registerStorageStatusRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/storage/info", handler.StorageInfo)
	r.GET("/storage/migrations", handler.StorageMigrations)
	r.POST("/storage/migrations/check", handler.StorageMigrationsCheck)
}

func registerDataPortabilityMigrationRoutes(r *gin.RouterGroup, handler *Handler) {
	r.POST("/data-portability/exports/preview", handler.DataPortabilityPreviewExport)
	r.POST("/data-portability/exports", handler.DataPortabilityExport)
	r.GET("/data-portability/exports/:filename", handler.DataPortabilityExportDownload)
	r.POST("/data-portability/imports/inspect", handler.DataPortabilityInspectImport)
	r.POST("/data-portability/imports/:ticketId/commit", handler.DataPortabilityCommitImport)
	r.DELETE("/data-portability/imports/:ticketId", handler.DataPortabilityCancelImport)
}

func registerDataPortabilityBackupRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/data-portability/backups", handler.DataPortabilityBackups)
	r.POST("/data-portability/backups", handler.DataPortabilityCreateBackup)
	r.POST("/data-portability/backups/:name/restore/preview", handler.DataPortabilityPreviewRestore)
	r.POST("/data-portability/restores/:operationId", handler.DataPortabilityRestore)
	r.DELETE("/data-portability/restores/pending", handler.DataPortabilityCancelRestore)
	r.DELETE("/data-portability/backups/:name", handler.DataPortabilityDeleteBackup)
}

func RegisterDeviceDataPortabilityRoutes(r *gin.RouterGroup, ctx *app.AppContext, coord *dataportability.Coordinator) {
	if r == nil || ctx == nil {
		return
	}
	svc := NewService(ctx, runtimeprofile.ProfileDeviceAgent)
	if coord != nil {
		svc.SetDataPortabilityCoordinator(coord)
	}
	handler := &Handler{service: svc, db: ctx.DB}
	registerDataPortabilityBackupRoutes(r, handler)
}
