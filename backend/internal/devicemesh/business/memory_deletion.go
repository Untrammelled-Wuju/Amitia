package business

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type MemoryDeletionStatus struct {
	ID               string                       `json:"id"`
	TargetID         string                       `json:"targetId"`
	TargetType       string                       `json:"targetType"`
	Scope            string                       `json:"scope"`
	RequestedAt      string                       `json:"requestedAt"`
	Status           string                       `json:"status"`
	ItemsCount       int                          `json:"itemsCount"`
	CleanedCount     int                          `json:"cleanedCount"`
	FailedCount      int                          `json:"failedCount"`
	RetrievalBlocked bool                         `json:"retrievalBlocked"`
	OwnerID          string                       `json:"ownerId"`
	Acknowledgement  coordination.Acknowledgement `json:"acknowledgement"`
}

type MemoryPrivacyDeletionPort interface {
	PreviewMemoryDeletion(context.Context, string) ([]coordination.PrivacyDeletionTarget, error)
	DeleteMemoryIntent(context.Context, string, string, coordination.PrivacyDeletionTarget, string) (MemoryDeletionStatus, error)
	DeletionStatus(context.Context, string) (MemoryDeletionStatus, error)
	DeletionStats(context.Context) (map[string]interface{}, error)
	CleanupDeletions(context.Context) (int, error)
	DeletionChecks(context.Context, string) ([]MemoryDeletionCheck, error)
}

type MemoryRelatedDeletionPort interface {
	DeleteRelatedMemories(context.Context, string, string) ([]MemoryDeletionStatus, error)
}

func (e *Engine) DeleteRelatedOwnedMemories(ctx context.Context, request Request, entries []MemoryConsumerEntry, targetType, targetID string) ([]MemoryDeletionStatus, error) {
	if targetID == "" || targetType != "character" && targetType != "conversation" {
		return nil, errors.New("关联记忆删除目标无效")
	}
	results := []MemoryDeletionStatus{}
	for _, entry := range entries {
		if entry.Kind != "memory" || entry.Deleted {
			continue
		}
		match := targetType == "character" && entry.RoleID == targetID
		if targetType == "conversation" {
			document, _, err := MemoryContent(entry)
			if err != nil {
				return results, err
			}
			match = MemoryText(document, "conversationId") == targetID
		}
		if !match {
			continue
		}
		current := request
		current.RoleID = entry.Scope.RoleID
		current.RequestID = "privacy-related/" + memoryManagementHash([]interface{}{targetType, targetID, entry.ID, entry.Revision})[:48]
		result, err := e.DeleteOwnedMemory(ctx, current, entry, "all")
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

type MemoryDeletionCheck struct {
	Kind     string    `json:"kind"`
	Passed   bool      `json:"passed"`
	Severity string    `json:"severity"`
	Detail   string    `json:"detail"`
	Evidence string    `json:"evidence,omitempty"`
	TestedAt time.Time `json:"testedAt"`
}

func (e *Engine) CheckMemoryDeletion(ctx context.Context, request Request, id string) ([]MemoryDeletionCheck, error) {
	var roleErr error
	request, roleErr = e.PrivacyRequestRole(ctx, request)
	if roleErr != nil {
		return nil, roleErr
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return nil, coordination.ErrWrongOwner
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		current := scope
		current.RoleID, current.RoleRevision = role.ID, role.Revision
		snapshot, err := e.memoryManagementSnapshot(coordination.WithScope(ctx, current), current, "checkpoint", "")
		if err != nil {
			return nil, err
		}
		for _, row := range snapshot.Resources {
			if !strings.HasPrefix(row.ID, "memory-deletion/") {
				continue
			}
			var info MemoryDeletionStatus
			if json.Unmarshal(row.Body, &info) != nil {
				return nil, coordination.ErrResourceVersion
			}
			if info.TargetID != id {
				continue
			}
			address, err := ParseMemoryDeletionID(info.ID)
			if err != nil {
				return nil, err
			}
			current.MemoryRoleID = address.Partition
			root, err := port.Resource(coordination.WithScope(ctx, current), current, "memory", id)
			if err != nil {
				return nil, err
			}
			blocked := root != nil && root.Deleted && root.OwnerID == current.ResourceOwnerID
			info, err = e.memoryDeletionProgress(coordination.WithScope(ctx, current), current, info)
			if err != nil {
				return nil, err
			}
			now := time.Now().UTC()
			return []MemoryDeletionCheck{{Kind: "post_deletion_recall", Passed: blocked, Severity: "high", Detail: "核验当前所有者的正式记忆记录已经删除，无法再参与召回", Evidence: id, TestedAt: now}, {Kind: "owned_memory_index_cleanup", Passed: info.Status == "completed", Severity: "high", Detail: "核验向量与图谱清理任务的实际完成状态", Evidence: info.Status, TestedAt: now}}, nil
		}
	}
	return nil, errors.New("未找到当前所有者的记忆删除记录")
}

type MemoryDeletionCleanupPort interface {
	CleanupMemoryDeletions(context.Context, coordination.ExecutionScope) (int, error)
}

func (e *Engine) CleanupMemoryDeletions(ctx context.Context, request Request) (int, error) {
	var roleErr error
	request, roleErr = e.PrivacyRequestRole(ctx, request)
	if roleErr != nil {
		return 0, roleErr
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return 0, err
	}
	defer finish()
	port, ok := e.data.(MemoryDeletionCleanupPort)
	if !ok {
		return 0, errors.New("记忆索引清理入口不可用")
	}
	return port.CleanupMemoryDeletions(ctx, scope)
}

type MemoryDeletionProjectionPort interface {
	MemoryDeletionComplete(context.Context, coordination.ExecutionScope, string) (bool, error)
}
type MemoryDeletionAddress struct {
	OwnerID     string `json:"ownerId"`
	RoleID      string `json:"roleId"`
	Partition   string `json:"partition"`
	OperationID string `json:"operationId"`
}

func ParseMemoryDeletionID(id string) (MemoryDeletionAddress, error) {
	if !strings.HasPrefix(id, "owned-memory-delete.") || len(id) > 2048 {
		return MemoryDeletionAddress{}, errors.New("记忆删除记录编号无效")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "owned-memory-delete."))
	var address MemoryDeletionAddress
	if err != nil || json.Unmarshal(raw, &address) != nil || address.OwnerID == "" || address.RoleID == "" || address.Partition == "" || address.OperationID == "" || len(address.OperationID) > 128 {
		return address, errors.New("记忆删除记录编号无效")
	}
	return address, nil
}

func (e *Engine) DeleteOwnedMemory(ctx context.Context, request Request, entry MemoryConsumerEntry, selection string) (MemoryDeletionStatus, error) {
	if selection != "memory" && selection != "all" {
		return MemoryDeletionStatus{}, errors.New("记忆删除范围必须为 memory 或 all")
	}
	request.ExpectedScope = &entry.Scope
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, true)
	if err != nil {
		return MemoryDeletionStatus{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryDeletionStatus{}, coordination.ErrWrongOwner
	}
	if entry.OwnerID != scope.ResourceOwnerID || entry.Kind != "memory" {
		return MemoryDeletionStatus{}, coordination.ErrWrongOwner
	}
	id := "owned-memory-delete." + base64.RawURLEncoding.EncodeToString(body(MemoryDeletionAddress{OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Partition: coordination.StorageRole(scope, "memory"), OperationID: request.RequestID}))
	receiptID := "memory-deletion/" + request.RequestID
	if previous, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return MemoryDeletionStatus{}, err
	} else if previous != nil {
		var info MemoryDeletionStatus
		if json.Unmarshal(previous.Body, &info) != nil || info.TargetID != entry.ID || info.Scope != selection {
			return MemoryDeletionStatus{}, coordination.ErrRequestConflict
		}
		return info, e.coordination.Validate(ctx, scope)
	}
	row, err := port.Resource(ctx, scope, "memory", entry.ID)
	if err != nil {
		return MemoryDeletionStatus{}, err
	}
	if row == nil || row.Deleted || row.Revision != entry.Revision {
		return MemoryDeletionStatus{}, coordination.ErrResourceVersion
	}
	info := MemoryDeletionStatus{ID: id, TargetID: entry.ID, TargetType: "memory", Scope: selection, RequestedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "cleaning", ItemsCount: 1, RetrievalBlocked: true, OwnerID: scope.ResourceOwnerID, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"memory/" + entry.ID: entry.Revision + 1, "checkpoint/" + receiptID: 1}}}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "memory", ID: entry.ID, RoleID: row.RoleID, ExpectedRevision: entry.Revision, Deleted: true, Body: body(nil)}, {Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(info)}}})
	if err != nil {
		return MemoryDeletionStatus{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || ack.Versions["memory/"+entry.ID] != entry.Revision+1 || ack.Versions["checkpoint/"+receiptID] != 1 {
		return MemoryDeletionStatus{}, coordination.ErrResourceVersion
	}
	info.Acknowledgement = ack
	return e.memoryDeletionProgress(ctx, scope, info)
}

func (e *Engine) memoryDeletionProgress(ctx context.Context, scope coordination.ExecutionScope, info MemoryDeletionStatus) (MemoryDeletionStatus, error) {
	if info.OwnerID != scope.ResourceOwnerID {
		return info, coordination.ErrWrongOwner
	}
	if port, ok := e.data.(MemoryDeletionProjectionPort); ok {
		complete, err := port.MemoryDeletionComplete(ctx, scope, info.TargetID)
		if err != nil {
			return info, err
		}
		if complete {
			info.Status = "completed"
			info.CleanedCount = info.ItemsCount
		}
	}
	return info, e.coordination.Validate(ctx, scope)
}

func (e *Engine) MemoryDeletionState(ctx context.Context, request Request, address MemoryDeletionAddress) (MemoryDeletionStatus, error) {
	request.RoleID = address.RoleID
	request.MemoryView = "role:" + address.Partition
	if address.Partition == coordination.GlobalMemoryRole {
		request.MemoryView = "global"
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryDeletionStatus{}, err
	}
	defer finish()
	if scope.ResourceOwnerID != address.OwnerID {
		return MemoryDeletionStatus{}, coordination.ErrWrongOwner
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryDeletionStatus{}, coordination.ErrWrongOwner
	}
	row, err := port.Resource(ctx, scope, "checkpoint", "memory-deletion/"+address.OperationID)
	if err != nil {
		return MemoryDeletionStatus{}, err
	}
	var info MemoryDeletionStatus
	if row == nil || row.Deleted || json.Unmarshal(row.Body, &info) != nil {
		return info, coordination.ErrResourceVersion
	}
	return e.memoryDeletionProgress(ctx, scope, info)
}

func (e *Engine) MemoryDeletionStatistics(ctx context.Context, request Request) (map[string]interface{}, error) {
	var roleErr error
	request, roleErr = e.PrivacyRequestRole(ctx, request)
	if roleErr != nil {
		return nil, roleErr
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer finish()
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return nil, err
	}
	var total, completed, pending int64
	for _, role := range roles {
		current := scope
		current.RoleID, current.RoleRevision = role.ID, role.Revision
		snapshot, err := e.memoryManagementSnapshot(coordination.WithScope(ctx, current), current, "checkpoint", "")
		if err != nil {
			return nil, err
		}
		for _, row := range snapshot.Resources {
			if !strings.HasPrefix(row.ID, "memory-deletion/") {
				continue
			}
			var info MemoryDeletionStatus
			if json.Unmarshal(row.Body, &info) != nil {
				return nil, coordination.ErrResourceVersion
			}
			address, err := ParseMemoryDeletionID(info.ID)
			if err != nil {
				return nil, err
			}
			current.MemoryRoleID = address.Partition
			info, err = e.memoryDeletionProgress(coordination.WithScope(ctx, current), current, info)
			if err != nil {
				return nil, err
			}
			total++
			if info.Status == "completed" {
				completed++
			} else {
				pending++
			}
		}
	}
	return map[string]interface{}{"memoryDeletions": total, "memoryCompleted": completed, "memoryPending": pending}, nil
}
