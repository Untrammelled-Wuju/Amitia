// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) PrivacyScan(c *gin.Context) {
	var req struct {
		Scope []string `json:"scope"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		privacyError(c, 400, "扫描参数无效", nil)
		return
	}
	svc := h.privacyService(c)
	memories := business.MemoryPrivacySnapshot{}
	for _, item := range normalizePrivacyScopes(req.Scope) {
		if item != privacyScopeMemories {
			continue
		}
		if h.memoryConsumer == nil {
			privacyError(c, 503, "统一记忆服务尚未就绪", nil)
			return
		}
		entries, err := h.memoryConsumer.Read(c.Request.Context(), "", "memory", true)
		if err == nil {
			memories, err = business.ScanMemoryPrivacy(entries, matchPrivacyPattern)
		}
		if err != nil {
			privacyError(c, 503, "记忆隐私扫描失败", err.Error())
			return
		}
		svc.privacyMu.Lock()
		if svc.privacyMemoryEntries == nil {
			svc.privacyMemoryEntries = map[string]business.MemoryConsumerEntry{}
		}
		for _, entry := range entries {
			svc.privacyMemoryEntries[entry.ID] = entry
		}
		svc.privacyMu.Unlock()
	}
	messages := []privacyMessageRow{}
	receipts := map[string]coordination.PrivacyMessage{}
	for _, selected := range normalizePrivacyScopes(req.Scope) {
		if selected != privacyScopeMessages && selected != privacyScopeImportItems {
			continue
		}
		port, ok := h.memoryConsumer.(business.ChatPrivacyPort)
		if !ok {
			privacyError(c, 503, "统一消息隐私入口不可用", nil)
			return
		}
		entries, err := port.PrivacyMessages(c.Request.Context())
		if err != nil {
			privacyError(c, 503, "消息隐私扫描未完成", err.Error())
			return
		}
		for _, entry := range entries {
			var document map[string]interface{}
			if json.Unmarshal(entry.Body, &document) != nil {
				privacyError(c, 503, "消息隐私数据无效", nil)
				return
			}
			raw, _ := json.Marshal([]interface{}{entry.OwnerID, entry.RoleID, entry.Legacy, entry.ID, entry.Scope})
			digest := sha256.Sum256(raw)
			id := "privacy-message/" + hex.EncodeToString(digest[:])
			source := ""
			if entry.Imported {
				source = "import"
			}
			text, _ := document["content"].(string)
			role, _ := document["role"].(string)
			conversation, _ := document["conversationId"].(string)
			stamp, _ := document["createdAt"].(string)
			safety, _ := document["safetyLevel"].(string)
			messages = append(messages, privacyMessageRow{ID: id, ConversationID: conversation, Role: role, Content: text, CreatedAt: stamp, SafetyLevel: safety, MessageSource: source})
			receipts[id] = entry
		}
		break
	}
	svc.privacyMu.Lock()
	if svc.privacyMessages == nil {
		svc.privacyMessages = map[string]coordination.PrivacyMessage{}
	}
	for id, entry := range receipts {
		svc.privacyMessages[id] = entry
	}
	svc.privacyMu.Unlock()
	var result map[string]interface{}
	if !h.commitAdministratorAction(c, func() error {
		result = svc.privacyScan(req.Scope, memories, messages)
		for _, raw := range result["findings"].([]interface{}) {
			finding, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := finding["id"].(string)
			if entry, exists := receipts[id]; exists {
				finding["recordId"], finding["ownerId"], finding["characterId"], finding["revision"], finding["legacy"], finding["executionScope"] = entry.ID, entry.OwnerID, entry.RoleID, entry.Revision, entry.Legacy, entry.Scope
			}
		}
		svc.finalizePrivacyScan(result)
		return nil
	}) {
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) scannedPrivacyMemory(c *gin.Context, id string) (business.MemoryConsumerEntry, error) {
	svc := h.privacyService(c)
	svc.privacyMu.RLock()
	defer svc.privacyMu.RUnlock()
	entry, exists := svc.privacyMemoryEntries[id]
	if !exists {
		return business.MemoryConsumerEntry{}, errors.New("请重新扫描隐私后再进行记忆脱敏")
	}
	return entry, nil
}

func (h *Handler) PrivacyMask(c *gin.Context) {
	svc := h.privacyService(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 320<<10)
	var req struct {
		IDs   []interface{} `json:"ids"`
		Items []struct {
			ID          interface{} `json:"id"`
			SourceTable string      `json:"sourceTable"`
		} `json:"items"`
		ConfirmToken string `json:"confirmToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ConfirmToken != "确认脱敏" {
		privacyError(c, 400, "confirmToken must be 确认脱敏", nil)
		return
	}
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		privacyError(c, 409, err.Error(), nil)
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

	maskedTargets := make([]privacyMaskTarget, 0, len(targets))
	var updated int64
	for _, target := range targets {
		if target.SourceTable != privacyScopeMemories {
			continue
		}
		if h.memoryConsumer == nil {
			privacyError(c, 503, "privacy mask failed", "memory service unavailable")
			return
		}
		entry, err := h.scannedPrivacyMemory(c, target.ID)
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
			privacyError(c, 503, "privacy mask failed", err.Error())
			return
		}
		updated++
		maskedTargets = append(maskedTargets, target)
		svc.markPrivacyFindingsMasked([]privacyMaskTarget{target})
	}

	for _, target := range targets {
		if target.SourceTable == privacyScopeMemories {
			continue
		}
		port, ok := h.memoryConsumer.(business.ChatPrivacyPort)
		if !ok {
			privacyError(c, 503, "统一消息脱敏入口不可用", nil)
			return
		}
		svc.privacyMu.RLock()
		entry, exists := svc.privacyMessages[target.ID]
		svc.privacyMu.RUnlock()
		if !exists || (target.SourceTable == privacyScopeImportItems) != entry.Imported {
			privacyError(c, 409, "请重新扫描后再进行消息脱敏", nil)
			return
		}
		if err := port.MaskPrivacyMessage(c.Request.Context(), entry, uuid.NewString()); err != nil {
			privacyError(c, 409, "消息脱敏失败，请重新扫描", err.Error())
			return
		}
		updated++
		maskedTargets = append(maskedTargets, target)
		svc.markPrivacyFindingsMasked([]privacyMaskTarget{target})
	}

	if svc := h.privacyService(c); svc != nil {
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
			svc := h.privacyService(c)
			svc.privacyMu.Lock()
			svc.privacyScans = nil
			svc.privacyMessages = nil
			svc.privacyMemoryEntries = nil
			svc.privacyMu.Unlock()
			return nil
		}) {
			return
		}
		util.SuccessResponse(c, gin.H{"deleted": true})
		return
	}
	util.SuccessResponse(c, h.privacyService(c).PrivacyScanResults())
}

func (h *Handler) PrivacyScanResultsGet(c *gin.Context) {
	util.SuccessResponse(c, h.privacyService(c).PrivacyScanResults())
}

func (h *Handler) PrivacyScanResultByID(c *gin.Context) {
	util.SuccessResponse(c, h.privacyService(c).GetPrivacyScanResult(c.Param("id")))
}

func (h *Handler) PrivacyAuthority() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" || h.canonicalMemory == nil {
			c.AbortWithStatusJSON(503, gin.H{"message": "隐私数据归属入口尚未就绪"})
			return
		}
		invoked := false
		err := h.canonicalMemory.WithPrivacyAuthority(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: h.memoryCoreID}, func(ctx context.Context) error {
			c.Request = c.Request.WithContext(ctx)
			invoked = true
			c.Next()
			return nil
		})
		if err != nil && !c.Writer.Written() {
			c.AbortWithStatusJSON(409, gin.H{"message": err.Error()})
			return
		}
		if !invoked {
			c.Abort()
		}
	}
}

func (h *Handler) privacyService(c *gin.Context) *service {
	scope, _ := coordination.FromContext(c.Request.Context())
	keyBytes, _ := json.Marshal([]interface{}{scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, scope.CoreID, scope.ProviderEpoch, scope.ModeRevision, scope.PermissionRevision})
	key := string(keyBytes)
	h.privacyServicesMu.Lock()
	defer h.privacyServicesMu.Unlock()
	if h.privacyServices == nil {
		h.privacyServices = map[string]*service{}
	}
	if h.privacyServices[key] == nil {
		h.privacyServices[key] = &service{}
	}
	return h.privacyServices[key]
}

func privacyError(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, gin.H{"code": code, "message": message, "msg": message, "data": data})
}
