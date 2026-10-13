// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

func (h *Handler) PrivacyScan(c *gin.Context) {
	var req struct {
		Scope []string `json:"scope"`
	}
	// An empty body remains valid for backwards compatibility and scans the
	// historical default scope. Updated clients always send their selected scope.
	_ = c.ShouldBindJSON(&req)
	memories := business.MemoryPrivacySnapshot{}
	for _, item := range normalizePrivacyScopes(req.Scope) {
		if item != privacyScopeMemories {
			continue
		}
		if h.memoryConsumer == nil {
			util.ErrorResponse(c, response.OperationFailed, "统一记忆服务尚未就绪", nil)
			return
		}
		entries, err := h.memoryConsumer.Read(c.Request.Context(), "", "memory", true)
		if err == nil {
			memories, err = business.ScanMemoryPrivacy(entries, matchPrivacyPattern)
		}
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "记忆隐私扫描失败", err.Error())
			return
		}
		h.privacyMemoryMu.Lock()
		h.privacyMemoryEntries = map[string]business.MemoryConsumerEntry{}
		for _, entry := range entries {
			h.privacyMemoryEntries[entry.ID] = entry
		}
		h.privacyMemoryMu.Unlock()
	}
	var result map[string]interface{}
	if !h.commitAdministratorAction(c, func() error {
		svc, ok := h.service.(*service)
		if !ok {
			return errors.New("统一隐私扫描服务尚未就绪")
		}
		result = svc.privacyScan(req.Scope, memories)
		return nil
	}) {
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) scannedPrivacyMemory(id string) (business.MemoryConsumerEntry, error) {
	h.privacyMemoryMu.RLock()
	defer h.privacyMemoryMu.RUnlock()
	entry, exists := h.privacyMemoryEntries[id]
	if !exists {
		return business.MemoryConsumerEntry{}, errors.New("请重新扫描隐私后再进行记忆脱敏")
	}
	return entry, nil
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
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
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
	maskedTargets := make([]privacyMaskTarget, 0, len(targets))
	var updated int64
	for _, target := range targets {
		if target.SourceTable != privacyScopeMemories {
			continue
		}
		if h.memoryConsumer == nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", "memory service unavailable")
			return
		}
		entry, err := h.scannedPrivacyMemory(target.ID)
		if err == nil {
			_, content, parseErr := business.MemoryContent(entry)
			err = parseErr
			if err == nil {
				content["value"] = business.MemoryJSON("[已脱敏]")
				content["sensitivityLevel"] = business.MemoryJSON("restricted")
				content["allowProactiveMention"] = business.MemoryJSON(false)
				_, err = h.memoryConsumer.Change(c.Request.Context(), entry, map[string]json.RawMessage{"content": business.MemoryJSON(content), "allowContextUse": business.MemoryJSON(false)}, false, "privacy-mask/"+entry.ID+"/"+fmt.Sprint(entry.Revision))
			}
		}
		if err != nil {
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
		if err := configwrite.Transaction(h.db.WithContext(c.Request.Context()), func(tx *gorm.DB) error {
			for _, target := range messageTargets {
				result := tx.Table("messages").Where("id = ?", target.ID).Updates(map[string]interface{}{
					"content":      "[已脱敏]",
					"safety_level": "masked",
				})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected > 0 {
					updated += result.RowsAffected
					maskedTargets = append(maskedTargets, target)
				}
			}
			return nil
		}); err != nil {
			util.ErrorResponse(c, response.OperationFailed, "privacy mask failed", err.Error())
			return
		}
	}

	if svc, ok := h.service.(*service); ok {
		if !h.commitAdministratorAction(c, func() error { svc.markPrivacyFindingsMasked(maskedTargets); return nil }) {
			return
		}
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
	if c.Request.Method == "DELETE" {
		if !h.commitAdministratorAction(c, func() error {
			svc, ok := h.service.(*service)
			if !ok {
				return errors.New("隐私扫描服务不可用")
			}
			svc.privacyMu.Lock()
			svc.privacyScans = nil
			svc.privacyMu.Unlock()
			return nil
		}) {
			return
		}
		util.SuccessResponse(c, gin.H{"deleted": true})
		return
	}
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
	if strings.EqualFold(req.TargetType, "memory") {
		if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
			util.ErrorResponse(c, 409, err.Error(), nil)
			return
		}
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			util.ErrorResponse(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		selection := string(req.Scope)
		if selection == "" {
			selection = "all"
		}
		result, err := port.DeleteMemory(c.Request.Context(), req.TargetID, selection)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "记忆删除失败", err.Error())
			return
		}
		util.SuccessResponse(c, result)
		return
	}
	var memoryDeletions []business.MemoryDeletionStatus
	targetType := strings.ToLower(strings.TrimSpace(req.TargetType))
	if (targetType == "character" || targetType == "conversation") && (req.Scope == mindruntime.DeletionScopeMemory || req.Scope == mindruntime.DeletionScopeAll) {
		if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
			util.ErrorResponse(c, 409, err.Error(), nil)
			return
		}
		port, ok := h.memoryConsumer.(business.MemoryRelatedDeletionPort)
		if !ok {
			util.ErrorResponse(c, 503, "关联记忆删除入口不可用", nil)
			return
		}
		var err error
		memoryDeletions, err = port.DeleteRelatedMemories(c.Request.Context(), targetType, req.TargetID)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "关联记忆删除未全部完成", gin.H{"error": err.Error(), "memoryDeletions": memoryDeletions})
			return
		}
		if req.Scope == mindruntime.DeletionScopeMemory {
			util.SuccessResponse(c, gin.H{"memoryDeletions": memoryDeletions, "targetType": targetType, "targetId": req.TargetID})
			return
		}
	}
	var tombstone mindruntime.DeletionTombstone
	if !h.commitAdministratorAction(c, func() error {
		if h.dataLifecycle == nil {
			return errors.New("数据生命周期服务不可用")
		}
		var err error
		tombstone, err = h.dataLifecycle.RequestDeletion(req)
		return err
	}) {
		return
	}
	if memoryDeletions != nil {
		result := gin.H{}
		encoded, _ := json.Marshal(tombstone)
		_ = json.Unmarshal(encoded, &result)
		result["memoryDeletions"] = memoryDeletions
		util.SuccessResponse(c, result)
		return
	}
	util.SuccessResponse(c, tombstone)
}

func (h *Handler) PrivacyDeletionStatus(c *gin.Context) {
	id := c.Param("id")
	if strings.HasPrefix(id, "owned-memory-delete.") {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			util.ErrorResponse(c, 503, "统一记忆删除入口不可用", nil)
			return
		}
		result, err := port.DeletionStatus(c.Request.Context(), id)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "读取记忆删除状态失败", err.Error())
			return
		}
		util.SuccessResponse(c, result)
		return
	}
	tombstone, ok := h.dataLifecycle.GetTombstone(id)
	if !ok {
		util.ErrorResponse(c, response.DataNotFound, "tombstone not found", nil)
		return
	}
	util.SuccessResponse(c, tombstone)
}

func (h *Handler) PrivacyDeletionStats(c *gin.Context) {
	stats := h.dataLifecycle.Stats()
	if port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort); ok {
		owned, err := port.DeletionStats(c.Request.Context())
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "读取记忆删除统计失败", err.Error())
			return
		}
		for key, value := range owned {
			stats[key] = value
		}
		if total, ok := owned["memoryDeletions"].(int64); ok {
			count, _ := stats["tombstones"].(int)
			stats["tombstones"] = int64(count) + total
		}
		if completed, ok := owned["memoryCompleted"].(int64); ok {
			count, _ := stats["completed"].(int)
			stats["completed"] = int64(count) + completed
		}
	}
	util.SuccessResponse(c, stats)
}

func (h *Handler) PrivacyDeletionCleanup(c *gin.Context) {
	ownedCount := 0
	if port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort); ok {
		if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
			util.ErrorResponse(c, 409, err.Error(), nil)
			return
		}
		var err error
		ownedCount, err = port.CleanupDeletions(c.Request.Context())
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "记忆索引清理失败", err.Error())
			return
		}
	}
	var results []mindruntime.OutboxCleanupItem
	if !h.commitAdministratorAction(c, func() error {
		if h.dataLifecycle == nil {
			return errors.New("数据生命周期服务不可用")
		}
		var err error
		results, err = h.dataLifecycle.ExecuteOutboxCleanup()
		return err
	}) {
		return
	}
	util.SuccessResponse(c, gin.H{"cleaned": len(results) + ownedCount, "memoryCleaned": ownedCount, "items": results})
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
	if strings.EqualFold(req.TargetType, "memory") {
		port, ok := h.memoryConsumer.(business.MemoryPrivacyDeletionPort)
		if !ok {
			util.ErrorResponse(c, 503, "统一记忆删除核验入口不可用", nil)
			return
		}
		checks, err := port.DeletionChecks(c.Request.Context(), req.TargetID)
		if err != nil {
			util.ErrorResponse(c, response.OperationFailed, "记忆删除核验失败", err.Error())
			return
		}
		util.SuccessResponse(c, checks)
		return
	}
	var results []mindruntime.SecurityTestResult
	if !h.commitAdministratorAction(c, func() error {
		if h.dataLifecycle == nil {
			return errors.New("数据生命周期服务不可用")
		}
		results = h.dataLifecycle.RunAllSecurityTests(mindruntime.DeletionRequest{
			TargetID: req.TargetID, TargetType: req.TargetType,
		})
		return nil
	}) {
		return
	}
	util.SuccessResponse(c, results)
}
