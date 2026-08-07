// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package maintenance

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

const (
	PermMaintenanceRead   = "desktop_pet.maintenance.read"
	PermBackupCreate      = "desktop_pet.backup.create"
	PermExportCreate      = "desktop_pet.export.create"
	PermMigrationRun      = "desktop_pet.migration.run"
	PermCutoverExecute    = "desktop_pet.cutover.execute"
)

func RegisterMaintenanceRouter(r *gin.RouterGroup, handler *Handler) {
	maintenance := r.Group("/maintenance")
	{
		maintenance.GET("/doctor",
			security.RequirePermission(PermMaintenanceRead),
			handler.Doctor,
		)
		maintenance.POST("/backup",
			security.RequirePermission(PermBackupCreate),
			handler.CreateBackup,
		)
		maintenance.POST("/export",
			security.RequirePermission(PermExportCreate),
			handler.CreateExport,
		)
		maintenance.POST("/migrations/:planId/run",
			security.RequirePermission(PermMigrationRun),
			handler.RunMigration,
		)
		maintenance.GET("/migrations/:operationId",
			security.RequirePermission(PermMaintenanceRead),
			handler.GetMigration,
		)
		maintenance.POST("/cutover/read",
			security.RequirePermission(PermCutoverExecute),
			handler.CutoverRead,
		)
		maintenance.POST("/cutover/write",
			security.RequirePermission(PermCutoverExecute),
			handler.CutoverWrite,
		)
	}
}
