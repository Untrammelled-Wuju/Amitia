// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func RegisterHealthRouter(r *gin.RouterGroup, cbRegistry *mindruntime.CircuitBreakerRegistry, dataLifecycle *mindruntime.DataLifecycleCoordinator, reconciliation *mindruntime.ReconciliationEngine) {
	r.GET("/health/circuit-breakers", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		reports := cbRegistry.AllHealthReports()
		result := make([]gin.H, 0, len(reports))
		for _, report := range reports {
			result = append(result, gin.H{
				"name":    report.Name,
				"healthy": report.Healthy,
				"state":   string(report.CircuitBreaker.Status()),
				"message": report.CheckMessage,
			})
		}
		util.SuccessResponse(c, result)
	})

	r.POST("/health/circuit-breakers/:name/reset", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		name := c.Param("name")
		cb := cbRegistry.Get(name)
		if cb == nil {
			util.ErrorResponse(c, response.NotFound, "breaker not found", nil)
			return
		}
		cb.Reset()
		util.SuccessMsgResponse(c, "breaker reset", gin.H{"name": name, "state": string(cb.Status())})
	})

	r.GET("/health/data-lifecycle", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		stats := dataLifecycle.Stats()
		util.SuccessResponse(c, stats)
	})

	r.GET("/health/reconciliation", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		scans := reconciliation.AllScans()
		util.SuccessResponse(c, gin.H{
			"status": string(reconciliation.Status()),
			"scans":  scans,
		})
	})

	r.POST("/health/reconciliation/run", security.SharedCoreAdminOnly(), func(c *gin.Context) {
		var body struct {
			Target   string `json:"target"`
			Strategy string `json:"strategy"`
		}
		c.ShouldBindJSON(&body)
		target := mindruntime.ReconciliationTarget(body.Target)
		strategy := mindruntime.ReconciliationStrategy(body.Strategy)
		if strategy == "" {
			strategy = mindruntime.StrategyManualConfirm
		}
		scan, err := reconciliation.RunScan(c.Request.Context(), target, strategy, "")
		if err != nil {
			util.ErrorResponse(c, response.InternalError, err.Error(), nil)
			return
		}
		util.SuccessMsgResponse(c, "scan started", scan)
	})
}
