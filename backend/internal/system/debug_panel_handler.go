package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) DebugPanel(c *gin.Context) {
	data := mindruntime.BuildDebugPanelData()
	util.SuccessResponse(c, data)
}

func (h *Handler) DebugCircuitBreakers(c *gin.Context) {
	reports := mindruntime.DefaultCircuitBreakerRegistry.AllHealthReports()
	cbSnapshots := make([]mindruntime.CircuitBreakerSnapshot, 0)
	for _, report := range reports {
		if report.CircuitBreaker != nil {
			cbSnapshots = append(cbSnapshots, mindruntime.CircuitBreakerSnapshot{
				Name:        report.Name,
				State:       string(report.CircuitBreaker.Status()),
				Failures:    report.CircuitBreaker.Failures,
				TotalCalls:  report.CircuitBreaker.TotalCalls,
				LastFailure: report.CircuitBreaker.LastFailure,
			})
		}
	}
	util.SuccessResponse(c, map[string]interface{}{
		"circuitBreakers": cbSnapshots,
		"count":           len(cbSnapshots),
	})
}

func (h *Handler) DebugConsistency(c *gin.Context) {
	data := mindruntime.BuildDebugPanelData()
	util.SuccessResponse(c, data.Consistency)
}

func (h *Handler) DebugExport(c *gin.Context) {
	export := mindruntime.BuildSanitizedExport()
	util.SuccessResponse(c, export)
}

func (h *Handler) ReconciliationScan(c *gin.Context) {
	var body struct {
		Target   string `json:"target"`
		Strategy string `json:"strategy"`
		Cursor   string `json:"cursor"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "invalid request body", nil)
		return
	}
	target := mindruntime.ReconciliationTarget(body.Target)
	strategy := mindruntime.ReconciliationStrategy(body.Strategy)
	if target == "" {
		target = mindruntime.ReconciliationSQLiteQdrant
	}
	if strategy == "" {
		strategy = mindruntime.StrategyAutoRebuild
	}
	scan := mindruntime.DefaultReconciliationEngine.StartScan(target, strategy, body.Cursor)
	util.SuccessResponse(c, scan)
}

func (h *Handler) ReconciliationStatus(c *gin.Context) {
	scanID := c.Query("scanId")
	if scanID != "" {
		scan := mindruntime.DefaultReconciliationEngine.GetScan(scanID)
		util.SuccessResponse(c, scan)
		return
	}
	scans := mindruntime.DefaultReconciliationEngine.AllScans()
	util.SuccessResponse(c, map[string]interface{}{
		"status": string(mindruntime.DefaultReconciliationEngine.Status()),
		"scans":  scans,
	})
}

func (h *Handler) ReconciliationRepair(c *gin.Context) {
	var body struct {
		ScanID    string `json:"scanId"`
		DiffID    string `json:"diffId"`
		AutoRepair bool   `json:"autoRepair"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "invalid request body", nil)
		return
	}
	scan := mindruntime.DefaultReconciliationEngine.GetScan(body.ScanID)
	if scan == nil {
		util.ErrorResponse(c, 404, "scan not found", nil)
		return
	}
	repaired := int64(0)
	for i := range scan.Diffs {
		diff := &scan.Diffs[i]
		if body.DiffID != "" && diff.ID != body.DiffID {
			continue
		}
		if body.AutoRepair && !diff.AutoRepairable {
			continue
		}
		if !diff.Repaired {
			diff.Repaired = true
			repaired++
		}
	}
	mindruntime.DefaultReconciliationEngine.UpdateScanProgress(body.ScanID, 0, 0, repaired, 0, 0)
	util.SuccessResponse(c, map[string]interface{}{
		"scanId":        body.ScanID,
		"repairedCount": repaired,
	})
}
