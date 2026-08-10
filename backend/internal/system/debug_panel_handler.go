package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
	"time"
)

func (h *Handler) DebugPanel(c *gin.Context) {
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
	data := mindruntime.BuildDebugPanelData(h.reconciliation)
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
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
	data := mindruntime.BuildDebugPanelData(h.reconciliation)
	util.SuccessResponse(c, data.Consistency)
}

func (h *Handler) DebugExport(c *gin.Context) {
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
	export := mindruntime.BuildSanitizedExport(h.reconciliation)
	util.SuccessResponse(c, export)
}

func (h *Handler) ReconciliationScan(c *gin.Context) {
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
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
	scan, err := h.reconciliation.RunScan(c.Request.Context(), target, strategy, body.Cursor)
	if err != nil {
		util.ErrorResponse(c, 500, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, scan)
}

func (h *Handler) ReconciliationStatus(c *gin.Context) {
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
	scanID := c.Query("scanId")
	if scanID != "" {
		scan := h.reconciliation.GetScan(scanID)
		util.SuccessResponse(c, scan)
		return
	}
	scans := h.reconciliation.AllScans()
	util.SuccessResponse(c, map[string]interface{}{
		"status": string(h.reconciliation.Status()),
		"scans":  scans,
	})
}

func (h *Handler) ReconciliationRepair(c *gin.Context) {
	if h.reconciliation == nil {
		util.ErrorResponse(c, 503, "reconciliation engine not initialized", nil)
		return
	}
	var body struct {
		ScanID     string `json:"scanId"`
		DiffID     string `json:"diffId"`
		AutoRepair bool   `json:"autoRepair"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, "invalid request body", nil)
		return
	}
	scan := h.reconciliation.GetScan(body.ScanID)
	if scan == nil {
		util.ErrorResponse(c, 404, "scan not found", nil)
		return
	}
	repaired := int64(0)
	for i := range scan.Diffs {
		diff := scan.Diffs[i]
		if body.DiffID != "" && diff.ID != body.DiffID {
			continue
		}
		if body.AutoRepair && !diff.AutoRepairable {
			continue
		}
		if diff.Repaired {
			continue
		}
		confirmed, failReason := h.reconciliation.RepairDiff(c.Request.Context(), body.ScanID, diff)
		if !confirmed {
			util.ErrorResponse(c, 409, failReason, map[string]interface{}{
				"diffId": diff.ID,
			})
			return
		}
		scan.Diffs[i].Repaired = true
		scan.Diffs[i].RepairedAt = time.Now()
		repaired++
	}
	h.reconciliation.UpdateScanProgress(body.ScanID, 0, 0, repaired, 0, 0)
	util.SuccessResponse(c, map[string]interface{}{
		"scanId":        body.ScanID,
		"repairedCount": repaired,
	})
}
