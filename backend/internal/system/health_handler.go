// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) Health(c *gin.Context) {
	health := h.service.Health()
	checks := map[string]interface{}{}
	ready := true
	if database, ok := health["database"].(string); ok {
		checks["database"] = database
		if database != "ok" {
			ready = false
		}
	}
	orchestratorReady := h.unifiedEntry != nil && h.unifiedEntry.IsOrchestratorReady()
	checks["orchestrator"] = map[string]interface{}{"ready": orchestratorReady}
	if !orchestratorReady {
		ready = false
	}
	backpressure := interaction.BackpressureStatus("")
	if h.unifiedEntry != nil {
		backpressure = h.unifiedEntry.GetBackpressureStatus()
	}
	checks["unifiedEntry"] = map[string]interface{}{"ready": h.unifiedEntry != nil, "backpressure": backpressure}
	if h.unifiedEntry == nil || backpressure == interaction.BackpressureShedding {
		ready = false
	}
	health["ready"] = ready
	health["health"] = ready
	health["checks"] = checks
	if !ready {
		c.JSON(503, gin.H{"code": 503, "data": health, "msg": "服务尚未就绪"})
		return
	}
	util.SuccessResponse(c, health)
}

func (h *Handler) Diagnostics(c *gin.Context) { util.SuccessResponse(c, h.service.Diagnostics()) }

func (h *Handler) RunDiagnostics(c *gin.Context) { util.SuccessResponse(c, h.service.RunDiagnostics()) }
