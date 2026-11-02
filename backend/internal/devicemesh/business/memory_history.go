package business

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type MemoryHistoryVersion struct {
	Revision  int64           `json:"revision"`
	CreatedAt string          `json:"createdAt"`
	Body      json.RawMessage `json:"body"`
}

type MemoryHistory struct {
	MemoryID string                 `json:"memoryId"`
	Versions []MemoryHistoryVersion `json:"versions"`
}

func memoryHistoryMutation(ctx context.Context, port coordination.ResourcePort, scope coordination.ExecutionScope, resource coordination.Resource, now time.Time) (coordination.Mutation, error) {
	id := "memory-history/" + resource.ID
	history := MemoryHistory{MemoryID: resource.ID}
	existing, err := port.Resource(ctx, scope, "checkpoint", id)
	if err != nil {
		return coordination.Mutation{}, err
	}
	revision := int64(0)
	if existing != nil && !existing.Deleted {
		if err := json.Unmarshal(existing.Body, &history); err != nil {
			return coordination.Mutation{}, err
		}
		revision = existing.Revision
	}
	history.MemoryID = resource.ID
	history.Versions = append(history.Versions, MemoryHistoryVersion{Revision: resource.Revision, CreatedAt: now.UTC().Format(time.RFC3339Nano), Body: append(json.RawMessage(nil), resource.Body...)})
	if len(history.Versions) > 20 {
		history.Versions = history.Versions[len(history.Versions)-20:]
	}
	return coordination.Mutation{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, ExpectedRevision: revision, Body: body(history)}, nil
}

func (e *Engine) OwnedMemoryHistory(ctx context.Context, request Request, id string) (MemoryHistory, coordination.ExecutionScope, error) {
	if id == "" {
		return MemoryHistory{}, coordination.ExecutionScope{}, errors.New("记忆历史缺少记录编号")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryHistory{}, scope, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryHistory{}, scope, errors.New("数据所有者不支持记忆历史")
	}
	resource, err := port.Resource(ctx, scope, "checkpoint", "memory-history/"+id)
	if err != nil {
		return MemoryHistory{}, scope, err
	}
	if resource == nil || resource.Deleted {
		return MemoryHistory{MemoryID: id, Versions: []MemoryHistoryVersion{}}, scope, nil
	}
	var history MemoryHistory
	if err := json.Unmarshal(resource.Body, &history); err != nil {
		return MemoryHistory{}, scope, err
	}
	if history.MemoryID != id {
		return MemoryHistory{}, scope, coordination.ErrWrongOwner
	}
	return history, scope, e.coordination.Validate(ctx, scope)
}

func memoryHistorySnapshot(ctx context.Context, port coordination.ResourcePort, scope coordination.ExecutionScope, id string, revision int64) (map[string]json.RawMessage, error) {
	resource, err := port.Resource(ctx, scope, "checkpoint", "memory-history/"+id)
	if err != nil {
		return nil, err
	}
	if resource == nil || resource.Deleted {
		return nil, coordination.ErrResourceVersion
	}
	var history MemoryHistory
	if err := json.Unmarshal(resource.Body, &history); err != nil {
		return nil, err
	}
	for _, version := range history.Versions {
		if version.Revision != revision {
			continue
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(version.Body, &document); err != nil {
			return nil, err
		}
		return document, nil
	}
	return nil, coordination.ErrResourceVersion
}
