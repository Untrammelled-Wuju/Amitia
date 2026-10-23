package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type privacyDeletionReference struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	RoleID         string `json:"roleId"`
	Revision       int64  `json:"revision"`
	Legacy         bool   `json:"legacy"`
	ConversationID string `json:"conversationId,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

type privacyDeletionJob struct {
	ExecutionScope coordination.ExecutionScope `json:"executionScope"`
	Attempts       int                         `json:"attempts"`
	NextRetryAt    time.Time                   `json:"nextRetryAt"`
	business.MemoryDeletionStatus
	InitiatorID      string                     `json:"initiatorId"`
	RoleID           string                     `json:"roleId"`
	Reason           string                     `json:"reason,omitempty"`
	LastError        string                     `json:"lastError,omitempty"`
	References       []privacyDeletionReference `json:"references"`
	ReceiptID        string                     `json:"receiptId"`
	PrimaryCompleted bool                       `json:"primaryCompleted"`
}

var privacyDeletionLocks sync.Map

func (p *meshLocalDataPort) privacyDeletionRecords(ctx context.Context, scope coordination.ExecutionScope) ([]ownedChatManagementRecord, error) {
	records, err := p.privacyChatRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := []ownedChatManagementRecord{}
	for _, row := range records {
		if row.Legacy {
			if scope.Coordinated && p.ownerID == scope.CoreID && p.services.DeviceMesh != nil && scope.TargetDeviceID == p.services.DeviceMesh.LocalDeviceID {
				continue
			}
			result = append(result, row)
		}
	}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT kind,resource_id,role_id,revision,body,source_id FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0 AND NOT(kind='checkpoint' AND resource_id LIKE 'privacy-deletion/%') ORDER BY role_id,kind,resource_id`, p.ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row ownedChatManagementRecord
		var raw []byte
		if err := rows.Scan(&row.Kind, &row.ID, &row.RoleID, &row.Revision, &raw, &row.SourceID); err != nil {
			return nil, err
		}
		if json.Unmarshal(raw, &row.Body) != nil {
			return nil, errors.New("删除范围包含无法解析的资源")
		}
		result = append(result, row)
		if len(result) > 32768 {
			return nil, coordination.ErrPendingLimit
		}
	}
	return result, rows.Err()
}

func validPrivacyDeletionInput(input *coordination.PrivacyDeletionRequest) error {
	if input == nil || input.TargetID == "" || len(input.TargetID) > 512 || len(input.Reason) > 8192 {
		return errors.New("删除目标无效")
	}
	if input.TargetType != "message" && input.TargetType != "conversation" && input.TargetType != "character" {
		return errors.New("删除类型无效")
	}
	switch input.Selection {
	case "all", "memory", "belief", "relation", "trace":
		return nil
	}
	return errors.New("删除范围无效")
}

func (p *meshLocalDataPort) managedPrivacyDeletion(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest) (map[string]any, error) {
	lock, _ := privacyDeletionLocks.LoadOrStore(p.ownerID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	input := request.PrivacyDeletion
	if request.Operation == "privacy/deletion-stats" || request.Operation == "privacy/deletion-cleanup" || request.Operation == "privacy/deletion-status" || request.Operation == "privacy/deletion-checks" {
		return p.privacyDeletionQuery(ctx, scope, request)
	}
	if err := validPrivacyDeletionInput(input); err != nil {
		return nil, err
	}
	records, err := p.privacyDeletionRecords(ctx, scope)
	if err != nil {
		return nil, err
	}
	targets := []coordination.PrivacyDeletionTarget{}
	fingerprint := chatManagementFingerprint(records)
	seen := map[string]bool{}
	for _, row := range records {
		match := row.ID == input.TargetID && row.Kind == input.TargetType
		targetRole := row.RoleID
		if row.Legacy && targetRole == "" {
			targetRole = role
		}
		if input.TargetType == "character" {
			match = targetRole == input.TargetID
		}
		if !match {
			continue
		}
		key := fmt.Sprintf("%t/%s", row.Legacy, targetRole)
		if input.TargetType == "character" {
			key = targetRole
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, coordination.PrivacyDeletionTarget{ID: input.TargetID, Kind: input.TargetType, OwnerID: p.ownerID, RoleID: targetRole, Legacy: row.Legacy && input.TargetType != "character", Revision: row.Revision, Fingerprint: fingerprint, ExecutionScope: scope})
	}
	if request.Operation == "privacy/deletion-preview" {
		return map[string]any{"targets": targets}, nil
	}
	if request.Operation != "privacy/deletion-request" || request.RequestID == "" || len(request.RequestID) > 128 || input.Target == nil {
		return nil, errors.New("删除必须先确认原始数据归属和版本")
	}
	receipt := "privacy-deletion/" + request.RequestID
	previous, err := p.store.Get(ctx, "checkpoint", receipt)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		var job privacyDeletionJob
		if json.Unmarshal(previous.Body, &job) != nil || job.InitiatorID != scope.InitiatorDeviceID || job.TargetID != input.TargetID || job.TargetType != input.TargetType || job.Scope != input.Selection {
			return nil, coordination.ErrRequestConflict
		}
		return p.advancePrivacyDeletion(ctx, scope, job, previous.Revision)
	}
	selected := false
	for _, target := range targets {
		if target.ID == input.Target.ID && target.Kind == input.Target.Kind && target.OwnerID == input.Target.OwnerID && target.RoleID == input.Target.RoleID && target.Legacy == input.Target.Legacy && target.Revision == input.Target.Revision && target.Fingerprint == input.Target.Fingerprint {
			selected = true
			break
		}
	}
	if !selected {
		return nil, coordination.ErrResourceVersion
	}
	references := privacyDeletionSelection(records, *input.Target, input.Selection)
	encoded, _ := json.Marshal([]string{p.ownerID, receipt})
	job := privacyDeletionJob{MemoryDeletionStatus: business.MemoryDeletionStatus{ID: "owned-chat-delete." + base64.RawURLEncoding.EncodeToString(encoded), TargetID: input.TargetID, TargetType: input.TargetType, Scope: input.Selection, RequestedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "pending", ItemsCount: len(references), OwnerID: p.ownerID}, InitiatorID: scope.InitiatorDeviceID, RoleID: scope.RoleID, ExecutionScope: scope, Reason: input.Reason, ReceiptID: receipt, References: references}
	if err := p.savePrivacyDeletion(ctx, scope, job, 0); err != nil {
		return nil, err
	}
	return p.advancePrivacyDeletion(ctx, scope, job, 1)
}

func privacyDeletionSelection(records []ownedChatManagementRecord, target coordination.PrivacyDeletionTarget, selection string) []privacyDeletionReference {
	conversations, requests := map[string]bool{}, map[string]bool{}
	if target.Kind == "conversation" {
		conversations[target.ID] = true
	}
	for _, row := range records {
		if target.Kind == "character" && row.RoleID == target.ID && row.Kind == "conversation" {
			conversations[row.ID] = true
		}
		if target.Kind == "message" && row.Kind == "message" && row.ID == target.ID && row.RoleID == target.RoleID && row.Legacy == target.Legacy {
			conversations[chatManagementString(row.Body, "conversationId")] = true
			if id := chatManagementString(row.Body, "requestId"); id != "" {
				requests[id] = true
			}
		}
	}
	sources := map[string]bool{}
	for _, row := range records {
		if row.Kind != "memory" {
			continue
		}
		linked := target.Kind == "character" && (row.RoleID == target.RoleID || row.RoleID == coordination.GlobalMemoryRole)
		if target.Kind == "conversation" {
			linked = chatManagementString(row.Body, "conversationId") == target.ID
		}
		if target.Kind == "message" {
			metadata, _ := row.Body["executionScope"].(map[string]any)
			id := strings.TrimSuffix(chatManagementString(metadata, "requestId"), "|memory")
			linked = id != "" && requests[id]
		}
		if linked {
			sources[row.ID] = true
		}
	}
	result := []privacyDeletionReference{}
	for _, row := range records {
		originalRole := row.RoleID
		if originalRole == coordination.GlobalMemoryRole {
			if metadata, ok := row.Body["executionScope"].(map[string]any); ok {
				originalRole = chatManagementString(metadata, "roleId")
			}
		}
		if originalRole != target.RoleID {
			continue
		}
		conversation := chatManagementString(row.Body, "conversationId")
		requestID := chatManagementString(row.Body, "requestId")
		if metadata, ok := row.Body["executionScope"].(map[string]any); ok && requestID == "" {
			requestID = chatManagementString(metadata, "requestId")
		}
		primary := (row.Kind == "message" && row.ID == target.ID || row.Kind == "conversation" && row.ID == target.ID) && row.Legacy == target.Legacy
		linked := target.Kind == "character" || conversations[conversation] && conversation != "" || row.SourceID != "" && sources[row.SourceID]
		if target.Kind == "message" {
			if row.Kind == "message" {
				linked = primary
			}
			if coordination.MemoryKind(row.Kind) {
				linked = requests[strings.TrimSuffix(requestID, "|memory")] && requestID != "" || row.SourceID != "" && sources[row.SourceID]
			}
		}
		if row.Kind == "checkpoint" {
			for id := range requests {
				if row.ID == "turn/"+id || row.ID == "memory/"+id {
					linked = true
				}
			}
		}
		if !primary && !linked {
			continue
		}
		include := selection == "all"
		if row.Kind == "project" || row.Kind == "worldbook" || row.Kind == "character" {
			include = false
		}
		if selection == "memory" {
			include = coordination.MemoryKind(row.Kind) || row.Kind == "summary" || row.Kind == "working"
		}
		if selection == "belief" {
			include = row.Kind == "memory"
		}
		if selection == "relation" {
			include = row.Kind == "graph" || row.Kind == coordination.RuntimeStateKindRelationship
		}
		if selection == "trace" {
			include = row.Kind == "checkpoint" || row.Kind == "tool-result" || row.Kind == "summary" || row.Kind == "working"
		}
		if row.Legacy && row.Kind != "message" && row.Kind != "conversation" {
			include = false
		}
		if row.Legacy && selection != "all" {
			include = false
		}
		if include {
			result = append(result, privacyDeletionReference{Kind: row.Kind, ID: row.ID, RoleID: row.RoleID, Revision: row.Revision, Legacy: row.Legacy, ConversationID: conversation, RequestID: requestID})
		}
	}
	return result
}

func (p *meshLocalDataPort) savePrivacyDeletion(ctx context.Context, scope coordination.ExecutionScope, job privacyDeletionJob, revision int64) error {
	current := scope
	current.ResourceOwnerID, current.RoleOwnerID = p.ownerID, p.ownerID
	current.RequestID = fmt.Sprintf("%s/status/%d", job.ReceiptID, revision)
	raw, _ := json.Marshal(job)
	return coordination.CommitCurrent(ctx, func() error {
		return p.store.ApplyPrivacyDeletion(ctx, []coordination.Commit{{Scope: current, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: job.ReceiptID, RoleID: scope.RoleID, ExpectedRevision: revision, Body: raw}}}})
	})
}

func (p *meshLocalDataPort) advancePrivacyDeletion(ctx context.Context, scope coordination.ExecutionScope, job privacyDeletionJob, revision int64) (map[string]any, error) {
	if !job.PrimaryCompleted {
		if err := p.deletePrivacyLegacy(ctx, scope, job); err != nil {
			job.LastError = err.Error()
			job.Status = "failed"
			job.FailedCount = 1
			_ = p.savePrivacyDeletion(ctx, scope, job, revision)
			return privacyDeletionResponse(job), nil
		}
		job.PrimaryCompleted = true
	}
	commits := []coordination.Commit{}
	for _, ref := range job.References {
		if ref.Legacy {
			continue
		}
		row, err := p.store.Get(ctx, ref.Kind, ref.ID)
		if err != nil {
			return nil, err
		}
		if row == nil || row.RoleID != ref.RoleID {
			return nil, coordination.ErrResourceVersion
		}
		if row.Deleted {
			continue
		}
		if row.Revision != ref.Revision {
			return nil, coordination.ErrResourceVersion
		}
		local := scope
		local.ResourceOwnerID, local.RoleOwnerID, local.RoleID = p.ownerID, p.ownerID, ref.RoleID
		if coordination.MemoryKind(ref.Kind) {
			local.RoleID = job.RoleID
			local.MemoryRoleID = ref.RoleID
		}
		local.RequestID = fmt.Sprintf("%s/delete/%s/%s/%d", job.ReceiptID, ref.Kind, ref.ID, ref.Revision)
		commits = append(commits, coordination.Commit{Scope: local, Mutations: []coordination.Mutation{{Kind: ref.Kind, ID: ref.ID, RoleID: ref.RoleID, SourceID: row.SourceID, ExpectedRevision: ref.Revision, Deleted: true, KeepConversationMemories: job.Scope != "all", Body: json.RawMessage(`null`)}}})
	}
	job.Status, job.RetrievalBlocked, job.FailedCount, job.LastError = "cleaning", true, 0, ""
	job.Acknowledgement = coordination.Acknowledgement{OwnerID: p.ownerID, RequestID: job.ReceiptID, Versions: map[string]int64{"checkpoint/" + job.ReceiptID: revision + 1}}
	current := scope
	current.ResourceOwnerID, current.RoleOwnerID = p.ownerID, p.ownerID
	current.RequestID = fmt.Sprintf("%s/blocked/%d", job.ReceiptID, revision)
	raw, _ := json.Marshal(job)
	commits = append(commits, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: job.ReceiptID, RoleID: scope.RoleID, ExpectedRevision: revision, Body: raw}}})
	err := coordination.CommitCurrent(ctx, func() error { return p.store.ApplyPrivacyDeletion(ctx, commits) })
	if err != nil {
		return nil, err
	}
	cleanupScope := scope
	cleanupScope.ResourceOwnerID, cleanupScope.RoleOwnerID = p.ownerID, p.ownerID
	if _, err := p.CleanupMemoryDeletions(ctx, cleanupScope); err != nil {
		job.LastError = err.Error()
	}
	return p.privacyDeletionProgress(ctx, scope, job, revision+1)
}

func privacyDeletionResponse(job privacyDeletionJob) map[string]any {
	raw, _ := json.Marshal(job.MemoryDeletionStatus)
	var result map[string]any
	_ = json.Unmarshal(raw, &result)
	result["lastError"] = job.LastError
	result["attempts"], result["maxAttempts"], result["nextRetryAt"], result["automaticRetryStopped"] = job.Attempts, 5, job.NextRetryAt, job.Attempts >= 5 && job.Status != "completed"
	return result
}

func (p *meshLocalDataPort) deletePrivacyLegacy(ctx context.Context, scope coordination.ExecutionScope, job privacyDeletionJob) error {
	selections := map[string]*chat.ManagedHistorySelection{}
	for _, ref := range job.References {
		if !ref.Legacy {
			continue
		}
		selected := selections[ref.RoleID]
		if selected == nil {
			selected = &chat.ManagedHistorySelection{EraseContent: true, KeepConversations: job.TargetType == "message", ConversationRevisions: map[string]int64{}, MessageRevisions: map[string]int64{}}
			selections[ref.RoleID] = selected
		}
		if ref.Kind == "message" {
			var message chat.Message
			if err := p.services.DB.WithContext(ctx).Where("id=? AND character_id=?", ref.ID, ref.RoleID).First(&message).Error; err != nil {
				return err
			}
			if message.DeletedAt != nil && message.Revision == ref.Revision+1 && message.Content == "" {
				continue
			}
			if message.DeletedAt != nil || message.Revision != ref.Revision {
				return coordination.ErrResourceVersion
			}
			selected.MessageRevisions[ref.ID] = ref.Revision
			var conversation chat.Conversation
			if err := p.services.DB.WithContext(ctx).Where("id=? AND deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", message.ConversationID, p.legacySpaceID).First(&conversation).Error; err != nil {
				return err
			}
			selected.ConversationRevisions[conversation.ID] = conversation.Revision
		}
	}
	for role, selected := range selections {
		if len(selected.MessageRevisions) == 0 {
			continue
		}
		if err := chat.DeleteOwnedManagedHistory(ctx, p.services.DB, p.services.Chat, p.legacySpaceID, role, *selected); err != nil {
			return err
		}
	}
	for _, ref := range job.References {
		if ref.Kind != "message" {
			continue
		}
		if err := p.services.DB.WithContext(ctx).Exec("DELETE FROM message_feedback WHERE space_id=? AND owner_id=? AND core_id=? AND role_id=? AND message_id=?", p.legacySpaceID, p.ownerID, scope.CoreID, ref.RoleID, ref.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func (p *meshLocalDataPort) privacyDeletionProgress(ctx context.Context, scope coordination.ExecutionScope, job privacyDeletionJob, revision int64) (map[string]any, error) {
	complete := job.PrimaryCompleted && job.RetrievalBlocked
	blocked := job.PrimaryCompleted
	for _, ref := range job.References {
		if ref.Legacy {
			if ref.Kind == "message" {
				var count int64
				if err := p.services.DB.WithContext(ctx).Model(&chat.Message{}).Where("id=? AND character_id=? AND (deleted_at IS NULL OR content<>'' OR reasoning_content<>'')", ref.ID, ref.RoleID).Count(&count).Error; err != nil {
					return nil, err
				}
				complete = complete && count == 0
				blocked = blocked && count == 0
			}
			continue
		}
		row, err := p.store.Get(ctx, ref.Kind, ref.ID)
		if err != nil {
			return nil, err
		}
		if row == nil || !row.Deleted || string(row.Body) != "null" {
			blocked = false
			complete = false
			continue
		}
		if ref.Kind == "vector" || ref.Kind == "graph" {
			ready, err := p.store.PrivacyDeletionProjectionComplete(ctx, ref.Kind, ref.ID)
			if err != nil {
				return nil, err
			}
			complete = complete && ready
		}
		if ref.Kind == "memory" {
			ready, err := p.store.MemoryDeletionComplete(ctx, ref.RoleID, ref.ID)
			if err != nil {
				return nil, err
			}
			complete = complete && ready
		}
	}
	if !complete && job.Status == "completed" {
		job.Status, job.FailedCount, job.CleanedCount, job.RetrievalBlocked, job.LastError = "failed", 1, 0, blocked, "检测到原始记录或索引状态发生变化，需要重新核验"
		if err := p.savePrivacyDeletion(ctx, scope, job, revision); err != nil {
			return nil, err
		}
		return privacyDeletionResponse(job), nil
	}
	job.RetrievalBlocked = blocked
	if complete && job.Status != "completed" {
		job.Status, job.CleanedCount, job.FailedCount = "completed", job.ItemsCount, 0
		if err := p.savePrivacyDeletion(ctx, scope, job, revision); err != nil {
			return nil, err
		}
	}
	if !complete && job.LastError != "" {
		if err := p.savePrivacyDeletion(ctx, scope, job, revision); err != nil {
			return nil, err
		}
	}
	return privacyDeletionResponse(job), nil
}

func (p *meshLocalDataPort) privacyDeletionQuery(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest) (map[string]any, error) {
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT revision,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id LIKE 'privacy-deletion/%' AND deleted=0`, p.ownerID)
	if err != nil {
		return nil, err
	}
	type entry struct {
		revision int64
		job      privacyDeletionJob
	}
	entries := []entry{}
	for rows.Next() {
		var item entry
		var raw []byte
		if err := rows.Scan(&item.revision, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if json.Unmarshal(raw, &item.job) != nil {
			rows.Close()
			return nil, coordination.ErrResourceVersion
		}
		if item.job.InitiatorID == scope.InitiatorDeviceID {
			entries = append(entries, item)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	stats := map[string]any{"tombstones": 0, "completed": 0, "pending": 0, "failed": 0, "cleaned": 0}
	checks := []business.MemoryDeletionCheck{}
	for _, item := range entries {
		job := item.job
		if request.Operation == "privacy/deletion-status" && (request.PrivacyDeletion == nil || request.PrivacyDeletion.ID != job.ID) {
			continue
		}
		if request.Operation == "privacy/deletion-checks" && (request.PrivacyDeletion == nil || request.PrivacyDeletion.TargetID != job.TargetID || request.PrivacyDeletion.TargetType != job.TargetType) {
			continue
		}
		var result map[string]any
		if request.Operation == "privacy/deletion-cleanup" && job.Status != "completed" {
			result, err = p.advancePrivacyDeletion(ctx, scope, job, item.revision)
		} else {
			result, err = p.privacyDeletionProgress(ctx, scope, job, item.revision)
		}
		if err != nil {
			return nil, err
		}
		if request.Operation == "privacy/deletion-status" {
			return result, nil
		}
		stats["tombstones"] = stats["tombstones"].(int) + 1
		switch result["status"] {
		case "completed":
			stats["completed"] = stats["completed"].(int) + 1
		case "failed":
			stats["failed"] = stats["failed"].(int) + 1
		default:
			stats["pending"] = stats["pending"].(int) + 1
		}
		if request.Operation == "privacy/deletion-cleanup" && job.Status != "completed" && result["status"] == "completed" {
			stats["cleaned"] = stats["cleaned"].(int) + 1
		}
		if request.Operation == "privacy/deletion-checks" {
			checks = append(checks, business.MemoryDeletionCheck{Kind: "owner_storage_deletion", Passed: result["status"] == "completed", Severity: "critical", Detail: "核验实际持有方的原始记录、派生资源及索引清理状态", Evidence: job.ID, TestedAt: time.Now().UTC()})
		}
	}
	if request.Operation == "privacy/deletion-status" {
		return nil, gorm.ErrRecordNotFound
	}
	if request.Operation == "privacy/deletion-checks" {
		if len(checks) == 0 {
			return nil, errors.New("未找到当前数据持有方的删除回执，不能判定通过")
		}
		return map[string]any{"checks": checks}, nil
	}
	return stats, nil
}

func runOwnedPrivacyDeletions(ctx context.Context, p *meshLocalDataPort) {
	run := func() {
		rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT revision,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id LIKE 'privacy-deletion/%' AND deleted=0`, p.ownerID)
		if err != nil {
			return
		}
		type pending struct {
			revision int64
			job      privacyDeletionJob
		}
		jobs := []pending{}
		for rows.Next() {
			var item pending
			var raw []byte
			if rows.Scan(&item.revision, &raw) == nil && json.Unmarshal(raw, &item.job) == nil && item.job.Status != "completed" && item.job.Attempts < 5 && !item.job.NextRetryAt.After(time.Now()) {
				jobs = append(jobs, item)
			}
		}
		rows.Close()
		lock, _ := privacyDeletionLocks.LoadOrStore(p.ownerID, &sync.Mutex{})
		for _, item := range jobs {
			if ctx.Err() != nil {
				return
			}
			lock.(*sync.Mutex).Lock()
			row, err := p.store.Get(ctx, "checkpoint", item.job.ReceiptID)
			if err == nil && row != nil && row.Revision == item.revision {
				result, err := p.advancePrivacyDeletion(ctx, item.job.ExecutionScope, item.job, item.revision)
				if err != nil || result["status"] != "completed" {
					current, readErr := p.store.Get(ctx, "checkpoint", item.job.ReceiptID)
					if readErr == nil && current != nil {
						var job privacyDeletionJob
						if json.Unmarshal(current.Body, &job) == nil {
							job.Attempts++
							job.NextRetryAt = time.Now().Add(time.Second * 30 * time.Duration(1<<min(job.Attempts, 5)))
							if err != nil {
								job.LastError = err.Error()
							}
							_ = p.savePrivacyDeletion(ctx, item.job.ExecutionScope, job, current.Revision)
						}
					}
				}
			}
			lock.(*sync.Mutex).Unlock()
		}
	}
	run()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
