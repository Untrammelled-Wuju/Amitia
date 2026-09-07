// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/pkg/app"
)

func registerStorageRoutes(r *gin.RouterGroup, handler *Handler) {
	r.POST("/storage/export-amitia", handler.StorageExportAmitia)
	r.GET("/storage/export-download/:filename", handler.StorageExportDownload)
	r.POST("/storage/import-user-data", handler.StorageImportUserData)
	r.POST("/storage/import-amitia", handler.StorageImportAmitia)
	r.GET("/storage/backups", handler.StorageBackups)
	r.POST("/storage/backups", handler.StorageCreateBackup)
	r.POST("/storage/backups/:name/restore", handler.StorageRestoreBackup)
	r.DELETE("/storage/backups/:name", handler.StorageDeleteBackup)
	r.DELETE("/storage/all", handler.StorageDeleteAll)
	r.GET("/storage/info", handler.StorageInfo)
	r.GET("/storage/migrations", handler.StorageMigrations)
	r.POST("/storage/migrations/check", handler.StorageMigrationsCheck)
}

// RegisterDeviceStorageRoutes exposes the same physical backup surface on the
// loopback Device Agent. The caller must attach device-local authentication to
// r before registering these routes.
func RegisterDeviceStorageRoutes(r *gin.RouterGroup, ctx *app.AppContext, coord *dataportability.Coordinator) {
	if r == nil || ctx == nil {
		return
	}
	svc := NewService(ctx, runtimeprofile.ProfileDeviceAgent)
	if coord != nil {
		svc.SetDataPortabilityCoordinator(coord)
	}
	handler := &Handler{service: svc, db: ctx.DB}
	registerStorageRoutes(r, handler)
}
