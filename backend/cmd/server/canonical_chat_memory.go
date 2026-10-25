package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
	"time"
)

func memorySubmissionFingerprint(values ...string) string {
	encoded, _ := json.Marshal(values)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

type canonicalChatMemoryRuntime struct {
	engine            *business.Engine
	spaceID, deviceID string
	begin             chat.InferenceAuthority
}

func (r *canonicalChatMemoryRuntime) inferenceContext(ctx context.Context) (context.Context, func(), error) {
	if err := context.Cause(ctx); err != nil {
		return nil, nil, err
	}
	if r.begin == nil {
		return ctx, func() {}, nil
	}
	return r.begin(ctx)
}

func (r *canonicalChatMemoryRuntime) request(spaceID, role string) (business.Request, error) {
	if spaceID != r.spaceID || role == "" {
		return business.Request{}, coordination.ErrWrongOwner
	}
	return business.Request{SpaceID: r.spaceID, DeviceID: r.deviceID, CoreID: r.spaceID, RoleID: role, RequestID: "chat-memory"}, nil
}

func (r *canonicalChatMemoryRuntime) Recall(ctx context.Context, spaceID, role, query string, limit int) ([]business.HybridSearchResult, error) {
	request, err := r.request(spaceID, role)
	if err != nil {
		return nil, err
	}
	result, err := r.engine.SearchOwnedMemories(ctx, request, business.MemoryManagementQuery{Query: query, Mode: "hybrid", Limit: limit, ContextUsable: true})
	if err != nil {
		return nil, err
	}
	rows := []business.HybridSearchResult{}
	for _, resource := range result.Resources {
		if !coordination.ResourceUsable(resource.Body, time.Now()) {
			continue
		}
		var document struct {
			Key     string `json:"key"`
			Kind    string `json:"kind"`
			Content struct {
				Value      string `json:"value"`
				Text       string `json:"text"`
				MemoryType string `json:"memoryType"`
				Importance int    `json:"importance"`
			} `json:"content"`
		}
		if json.Unmarshal(resource.Body, &document) != nil {
			return nil, errors.New("记忆记录格式无效")
		}
		value := document.Content.Value
		if value == "" {
			value = document.Content.Text
		}
		var record business.Memory
		_ = json.Unmarshal(resource.Body, &record)
		record.ID, record.SpaceID, record.CharacterID, record.Key, record.Value, record.MemoryType, record.Importance, record.Version = resource.ID, r.spaceID, resource.RoleID, document.Key, value, document.Content.MemoryType, document.Content.Importance, int(resource.Revision)
		sourceType := "memory"
		if document.Kind == "episodic" {
			sourceType = "episodic"
		}
		rows = append(rows, business.HybridSearchResult{Memory: record, Score: result.Scores[resource.ID], SourceType: sourceType})
	}
	return rows, nil
}

type ownedMemoryConsumers struct {
	runtime *canonicalChatMemoryRuntime
}

func (p *ownedMemoryConsumers) DeleteRelatedMemories(ctx context.Context, targetType, targetID string) ([]business.MemoryDeletionStatus, error) {
	entries, err := p.Read(ctx, "", "memory", true)
	if err != nil {
		return nil, err
	}
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.DeleteRelatedOwnedMemories(ctx, request, entries, targetType, targetID)
}

func (p *ownedMemoryConsumers) PreviewMemoryDeletion(ctx context.Context, id string) ([]coordination.PrivacyDeletionTarget, error) {
	entries, err := p.Read(ctx, "", "memory", true)
	if err != nil {
		return nil, err
	}
	targets := []coordination.PrivacyDeletionTarget{}
	for _, entry := range entries {
		if entry.ID == id {
			targets = append(targets, coordination.PrivacyDeletionTarget{ID: id, Kind: "memory", OwnerID: entry.OwnerID, RoleID: entry.RoleID, Revision: entry.Revision, Fingerprint: privacyMemoryFingerprint(entry.Body), ExecutionScope: entry.Scope})
		}
	}
	return targets, nil
}

func (p *ownedMemoryConsumers) DeleteMemoryIntent(ctx context.Context, id, selection string, target coordination.PrivacyDeletionTarget, requestID string) (business.MemoryDeletionStatus, error) {
	if target.Kind != "memory" || target.ID != id {
		return business.MemoryDeletionStatus{}, coordination.ErrWrongOwner
	}
	replayCtx, request, finish, err := p.authority(ctx, target.ExecutionScope.RoleID)
	if err != nil {
		return business.MemoryDeletionStatus{}, err
	}
	defer finish()
	previous, previousErr := p.runtime.engine.MemoryDeletionState(replayCtx, request, business.MemoryDeletionAddress{OwnerID: target.OwnerID, RoleID: target.ExecutionScope.RoleID, Partition: target.RoleID, OperationID: requestID})
	if previousErr == nil {
		if previous.TargetID != id || previous.Scope != selection {
			return business.MemoryDeletionStatus{}, coordination.ErrRequestConflict
		}
		return previous, nil
	}
	if !errors.Is(previousErr, coordination.ErrResourceVersion) {
		return business.MemoryDeletionStatus{}, previousErr
	}
	entries, err := p.Read(ctx, "", "memory", true)
	if err != nil {
		return business.MemoryDeletionStatus{}, err
	}
	for _, entry := range entries {
		if entry.ID != id || entry.OwnerID != target.OwnerID || entry.RoleID != target.RoleID || entry.Revision != target.Revision || privacyMemoryFingerprint(entry.Body) != target.Fingerprint {
			continue
		}
		current, original := entry.Scope, target.ExecutionScope
		current.RequestID, current.ExecutionID, current.TurnID = original.RequestID, original.ExecutionID, original.TurnID
		if current != original {
			return business.MemoryDeletionStatus{}, coordination.ErrScopeExpired
		}
		ctx, request, finish, err := p.authority(ctx, entry.Scope.RoleID)
		if err != nil {
			return business.MemoryDeletionStatus{}, err
		}
		defer finish()
		entry.Scope = target.ExecutionScope
		request.RequestID = requestID
		request.MemoryView = "role:" + target.RoleID
		return p.runtime.engine.DeleteOwnedMemory(ctx, request, entry, selection)
	}
	return business.MemoryDeletionStatus{}, coordination.ErrResourceVersion
}

func (p *ownedMemoryConsumers) DeletionStatus(ctx context.Context, id string) (business.MemoryDeletionStatus, error) {
	address, err := business.ParseMemoryDeletionID(id)
	if err != nil {
		return business.MemoryDeletionStatus{}, err
	}
	ctx, request, finish, err := p.authority(ctx, address.RoleID)
	if err != nil {
		return business.MemoryDeletionStatus{}, err
	}
	defer finish()
	return p.runtime.engine.MemoryDeletionState(ctx, request, address)
}

func (p *ownedMemoryConsumers) DeletionStats(ctx context.Context) (map[string]interface{}, error) {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.MemoryDeletionStatistics(ctx, request)
}

func (p *ownedMemoryConsumers) CleanupDeletions(ctx context.Context) (int, error) {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return 0, err
	}
	defer finish()
	return p.runtime.engine.CleanupMemoryDeletions(ctx, request)
}

func (p *ownedMemoryConsumers) DeletionChecks(ctx context.Context, id string) ([]business.MemoryDeletionCheck, error) {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.CheckMemoryDeletion(ctx, request, id)
}

func (p *ownedMemoryConsumers) Archive(ctx context.Context, role string) (business.MemoryArchive, error) {
	ctx, request, finish, err := p.authority(ctx, role)
	if err != nil {
		return business.MemoryArchive{}, err
	}
	defer finish()
	return p.runtime.engine.ExportMemoryArchive(ctx, request, role)
}

func (p *ownedMemoryConsumers) RestoreArchive(ctx context.Context, input dataportability.ImportRequest, archive business.MemoryArchive) error {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return err
	}
	defer finish()
	return p.runtime.engine.RestoreMemoryArchive(ctx, request, input, archive)
}

func (p *ownedMemoryConsumers) Link(ctx context.Context, role, source, target, relation string, weight float64, requestID string) (coordination.Acknowledgement, error) {
	ctx, request, finish, err := p.authority(ctx, role)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	request.RequestID = requestID
	return p.runtime.engine.LinkOwnedMemories(ctx, request, source, target, relation, weight)
}

func (p *ownedMemoryConsumers) Links(ctx context.Context, role, id string, depth int) ([]coordination.Resource, error) {
	ctx, request, finish, err := p.authority(ctx, role)
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.OwnedMemoryLinks(ctx, request, id, depth)
}

func (p *ownedMemoryConsumers) DeleteEntries(ctx context.Context, entries []business.MemoryConsumerEntry, requestID string) (int64, error) {
	var deleted int64
	for _, entry := range entries {
		if _, err := p.Change(ctx, entry, nil, true, requestID+"/"+memorySubmissionFingerprint(entry.ID)[:32]); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (p *ownedMemoryConsumers) ExecuteMemory(ctx context.Context, execution tool.ToolExecutionContext, name string, args map[string]interface{}) (business.MemoryToolResponse, error) {
	ctx, request, finish, err := p.authority(ctx, execution.CharacterID)
	if err != nil {
		return business.MemoryToolResponse{}, err
	}
	defer finish()
	if execution.SpaceID != "" && execution.SpaceID != request.SpaceID {
		return business.MemoryToolResponse{}, coordination.ErrWrongOwner
	}
	if execution.RequestID == "" || execution.ToolCallID == "" {
		return business.MemoryToolResponse{}, errors.New("记忆工具缺少执行标识")
	}
	request.RequestID = "memory-tool/" + memorySubmissionFingerprint(execution.RequestID, execution.ToolCallID, name)[:48]
	request.ConversationID = execution.ConversationID
	return p.runtime.engine.MemoryTool(ctx, request, name, args)
}

func (p *ownedMemoryConsumers) Beliefs(ctx context.Context, role string, limit int) ([]business.MemoryConsumerEntry, error) {
	ctx, request, finish, err := p.authority(ctx, role)
	if err != nil {
		return nil, err
	}
	defer finish()
	result, err := p.runtime.engine.SearchOwnedMemories(ctx, request, business.MemoryManagementQuery{ContextUsable: true, Mode: "keyword", Sort: "importance_desc", Limit: limit})
	if err != nil {
		return nil, err
	}
	entries := []business.MemoryConsumerEntry{}
	for _, resource := range result.Resources {
		entry := business.MemoryConsumerEntry{Resource: resource, Scope: result.Scope}
		if business.MemoryActive(entry) {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (p *ownedMemoryConsumers) authority(ctx context.Context, role string) (context.Context, business.Request, func(), error) {
	if p.runtime == nil || p.runtime.engine == nil {
		return ctx, business.Request{}, func() {}, errors.New("统一记忆服务尚未就绪")
	}
	if scope, ok := coordination.FromContext(ctx); ok {
		if role != "" && scope.RoleID != "" && scope.RoleID != role {
			return ctx, business.Request{}, func() {}, coordination.ErrWrongOwner
		}
		selectedRole := scope.RoleID
		if selectedRole == "" {
			selectedRole = role
		}
		return ctx, business.Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, CoreID: scope.CoreID, TargetDeviceID: scope.TargetDeviceID, RoleID: selectedRole, RequestID: uuid.NewString()}, func() {}, nil
	}
	ctx, finish, err := p.runtime.inferenceContext(ctx)
	return ctx, business.Request{SpaceID: p.runtime.spaceID, DeviceID: p.runtime.deviceID, CoreID: p.runtime.spaceID, RoleID: role, RequestID: uuid.NewString()}, finish, err
}

func (p *ownedMemoryConsumers) Read(ctx context.Context, role, kind string, all bool) ([]business.MemoryConsumerEntry, error) {
	ctx, request, finish, err := p.authority(ctx, role)
	if err != nil {
		return nil, err
	}
	defer finish()
	if all {
		request, err = p.runtime.engine.PrivacyRequestRole(ctx, request)
		if err != nil {
			return nil, err
		}
		request.MemoryView = "all"
	}
	entries := []business.MemoryConsumerEntry{}
	cursor := ""
	var authority string
	for page := 0; page < 256; page++ {
		result, err := p.runtime.engine.Query(ctx, request, coordination.DataQuery{ResourceKind: kind, Management: true, Cursor: cursor, Limit: 128})
		if err != nil {
			return nil, err
		}
		scope := result.Scope
		scope.RequestID, scope.ExecutionID, scope.TurnID = "", "", ""
		stamp := string(business.MemoryJSON(scope))
		if authority != "" && authority != stamp {
			return nil, coordination.ErrScopeExpired
		}
		authority = stamp
		for _, resource := range result.Snapshot.Resources {
			if resource.Kind != kind || resource.Deleted {
				continue
			}
			entryScope := result.Scope
			if kind == "memory" && entryScope.MemoryViewAll {
				entryScope.MemoryViewAll = false
				entryScope.MemoryRoleID = resource.RoleID
			}
			entries = append(entries, business.MemoryConsumerEntry{Resource: resource, Scope: entryScope})
		}
		next := result.Snapshot.NextCursors[kind]
		if next == "" {
			return entries, nil
		}
		if next == cursor {
			return nil, errors.New("记忆分页未推进")
		}
		cursor = next
	}
	return nil, coordination.ErrPendingLimit
}

func (p *ownedMemoryConsumers) Change(ctx context.Context, entry business.MemoryConsumerEntry, changes map[string]json.RawMessage, deleted bool, requestID string) (coordination.Acknowledgement, error) {
	if entry.OwnerID != entry.Scope.ResourceOwnerID {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	ctx, request, finish, err := p.authority(ctx, entry.Scope.RoleID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	request.RequestID, request.ExpectedScope = requestID, &entry.Scope
	return p.runtime.engine.Edit(ctx, request, business.EditRequest{ExpectedScope: &entry.Scope, ExpectedRevision: entry.Revision, RequestID: requestID, Kind: entry.Kind, ID: entry.ID, RoleID: entry.Scope.RoleID, TargetDeviceID: entry.Scope.TargetDeviceID, Changes: changes, Deleted: deleted})
}

func (p *ownedMemoryConsumers) PrivacyMessages(ctx context.Context) ([]coordination.PrivacyMessage, error) {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.PrivacyMessages(ctx, request)
}

func (p *ownedMemoryConsumers) MaskPrivacyMessage(ctx context.Context, item coordination.PrivacyMessage, id string) error {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return err
	}
	defer finish()
	return p.runtime.engine.MaskPrivacyMessage(ctx, request, item, id)
}

func (p *ownedMemoryConsumers) PrivacyDeletion(ctx context.Context, operation string, input coordination.PrivacyDeletionRequest, id string) (map[string]any, error) {
	ctx, request, finish, err := p.authority(ctx, "")
	if err != nil {
		return nil, err
	}
	defer finish()
	return p.runtime.engine.ManagePrivacyDeletion(ctx, request, operation, input, id)
}

func privacyMemoryFingerprint(raw json.RawMessage) string {
	var document map[string]json.RawMessage
	if json.Unmarshal(raw, &document) != nil {
		return ""
	}
	for _, key := range []string{"effectiveStrength", "effectiveRetentionLevel", "effectiveDecayState"} {
		delete(document, key)
	}
	encoded, _ := json.Marshal(document)
	return memorySubmissionFingerprint(string(encoded))
}
