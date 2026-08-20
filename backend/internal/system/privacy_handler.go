// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) PrivacyScan(c *gin.Context) { util.SuccessResponse(c, h.service.PrivacyScan()) }

func (h *Handler) PrivacyMask(c *gin.Context) {
	var req struct {
		IDs          []uint `json:"ids"`
		ConfirmToken string `json:"confirmToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ConfirmToken != "确认脱敏" {
		util.ErrorResponse(c, response.InvalidParams, "confirmToken must be 确认脱敏", nil)
		return
	}
	if len(req.IDs) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"masked": true, "maskedCount": 0, "updatedSourceRecords": 0})
		return
	}
	result := h.db.Table("messages").Where("id IN ?", req.IDs).Updates(map[string]interface{}{
		"content":      "[已脱敏]",
		"safety_level": "masked",
	})
	if result.Error != nil {
		util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", result.Error.Error())
		return
	}
	if svc, ok := h.service.(*service); ok {
		svc.markPrivacyFindingsMasked(req.IDs)
	}
	util.SuccessResponse(c, map[string]interface{}{
		"masked":               true,
		"maskedCount":          result.RowsAffected,
		"updatedSourceRecords": result.RowsAffected,
	})
}

func (h *Handler) PrivacyScanResults(c *gin.Context) {
	util.SuccessResponse(c, h.service.PrivacyScanResults())
}

func (h *Handler) PrivacyScanResultsGet(c *gin.Context) {
	util.SuccessResponse(c, h.service.PrivacyScanResults())
}

func (h *Handler) PrivacyScanResultByID(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetPrivacyScanResult(c.Param("id")))
}

func (h *Handler) PrivacyDeletionRequest(c *gin.Context) {
	var req mindruntime.DeletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "invalid request", nil)
		return
	}
	tombstone, err := h.dataLifecycle.RequestDeletion(req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "deletion request failed", err.Error())
		return
	}
	util.SuccessResponse(c, tombstone)
}

func (h *Handler) PrivacyDeletionStatus(c *gin.Context) {
	id := c.Param("id")
	tombstone, ok := h.dataLifecycle.GetTombstone(id)
	if !ok {
		util.ErrorResponse(c, response.DataNotFound, "tombstone not found", nil)
		return
	}
	util.SuccessResponse(c, tombstone)
}

func (h *Handler) PrivacyDeletionStats(c *gin.Context) {
	stats := h.dataLifecycle.Stats()
	util.SuccessResponse(c, stats)
}

func (h *Handler) PrivacyDeletionCleanup(c *gin.Context) {
	results, err := h.dataLifecycle.ExecuteOutboxCleanup()
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "deletion cleanup failed", err.Error())
		return
	}
	util.SuccessResponse(c, gin.H{"cleaned": len(results), "items": results})
}

func (h *Handler) PrivacyDeletionSecurityTests(c *gin.Context) {
	var req struct {
		TargetID   string `json:"targetId"`
		TargetType string `json:"targetType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "invalid request", nil)
		return
	}
	results := h.dataLifecycle.RunAllSecurityTests(mindruntime.DeletionRequest{
		TargetID: req.TargetID, TargetType: req.TargetType,
	})
	util.SuccessResponse(c, results)
}
