// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) MaintenanceStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetMaintenanceStatus())
}

func (h *Handler) MaintenanceDiagnose(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceDiagnose())
}

func (h *Handler) MaintenanceExportDiagnostic(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceExportDiagnostic())
}

func (h *Handler) MaintenanceReloadConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceReloadConfig())
}

func (h *Handler) MaintenanceRestartBridge(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceRestartBridge())
}
