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
	h.administratorAction(c, "diagnose", "")
}

func (h *Handler) MaintenanceExportDiagnostic(c *gin.Context) {
	h.administratorAction(c, "diagnostic-export", "")
}

func (h *Handler) MaintenanceReloadConfig(c *gin.Context) {
	h.updateAdministratorSettings(c, "reload", false)
}
