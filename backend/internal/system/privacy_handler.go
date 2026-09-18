// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) PrivacyScan(c *gin.Context) {
	var req struct {
		Scope []string `json:"scope"`
	}
	// An empty body remains valid for backwards compatibility and scans the
	// historical default scope. Updated clients always send their selected scope.
	_ = c.ShouldBindJSON(&req)
	util.SuccessResponse(c, h.service.PrivacyScan(req.Scope))
}

func (h *Handler) PrivacyMask(c *gin.Context) {
	var req struct {
		IDs   []interface{} `json:"ids"`
		Items []struct {
			ID          interface{} `json:"id"`
			SourceTable string      `json:"sourceTable"`
		} `json:"items"`
		ConfirmToken string `json:"confirmToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ConfirmToken != "确认脱敏" {
		util.ErrorResponse(c, response.InvalidParams, "confirmToken must be 确认脱敏", nil)
		return
	}

	targets := make([]privacyMaskTarget, 0, len(req.IDs)+len(req.Items))
	seen := map[string]bool{}
	appendTarget := func(idValue interface{}, sourceTable string) {
		id := normalizePrivacyMaskID(idValue)
		sourceTable = strings.TrimSpace(sourceTable)
		if sourceTable == "" {
			sourceTable = privacyScopeMessages
		}
		if sourceTable != privacyScopeMessages && sourceTable != privacyScopeMemories && sourceTable != privacyScopeImportItems {
			return
		}
		key := sourceTable + "\x00" + id
		if id == "" || seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, privacyMaskTarget{ID: id, SourceTable: sourceTable})
	}
	for _, id := range req.IDs {
		appendTarget(id, privacyScopeMessages)
	}
	for _, item := range req.Items {
		appendTarget(item.ID, item.SourceTable)
	}
	if len(targets) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"masked": true, "maskedCount": 0, "updatedSourceRecords": 0})
		return
	}

	// Memories have derived vector/graph state, so they must go through the
	// canonical memory mutation service instead of a raw SQL update. Disabling
	// context/proactive use makes the existing update path synchronously evict
	// derived retrieval state while preserving the masked record for audit/UI.
	maskedValue := "[已脱敏]"
	restricted := "restricted"
	allowContextUse := false
	allowProactiveMention := false
	maskedTargets := make([]privacyMaskTarget, 0, len(targets))
	var updated int64
	for _, target := range targets {
		if target.SourceTable != privacyScopeMemories {
			continue
		}
		if h.memorySvc == nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", "memory service unavailable")
			return
		}
		if _, err := h.memorySvc.Update(target.ID, &memory.UpdateMemoryRequest{
			Value:                 &maskedValue,
			SensitivityLevel:      &restricted,
			AllowContextUse:       &allowContextUse,
			AllowProactiveMention: &allowProactiveMention,
		}); err != nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", err.Error())
			return
		}
		updated++
		maskedTargets = append(maskedTargets, target)
	}

	// Message/import records have no vector/graph derivative, so a single DB
	// transaction is sufficient and keeps those source updates atomic.
	messageTargets := make([]privacyMaskTarget, 0, len(targets))
	for _, target := range targets {
		if target.SourceTable == privacyScopeMessages || target.SourceTable == privacyScopeImportItems {
			messageTargets = append(messageTargets, target)
		}
	}
	if len(messageTargets) > 0 {
		tx := h.db.Begin()
		if tx.Error != nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", tx.Error.Error())
			return
		}
		for _, target := range messageTargets {
			result := tx.Table("messages").Where("id = ?", target.ID).Updates(map[string]interface{}{
				"content":      "[已脱敏]",
				"safety_level": "masked",
			})
			if result.Error != nil {
				tx.Rollback()
				util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", result.Error.Error())
				return
			}
			if result.RowsAffected > 0 {
				updated += result.RowsAffected
				maskedTargets = append(maskedTargets, target)
			}
		}
		if err := tx.Commit().Error; err != nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", err.Error())
			return
		}
	}

	if svc, ok := h.service.(*service); ok {
		svc.markPrivacyFindingsMasked(maskedTargets)
	}
	util.SuccessResponse(c, map[string]interface{}{
		"masked":               true,
		"maskedCount":          updated,
		"updatedSourceRecords": updated,
	})
}

func normalizePrivacyMaskID(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
	}
	return strings.TrimSpace(fmt.Sprint(value))
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
