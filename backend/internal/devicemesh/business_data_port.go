package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) validateDataRoute(ctx context.Context, scope coordination.ExecutionScope) error {
	if rt.Coordination == nil || scope.CoreID == "" || scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID {
		return coordination.ErrWrongOwner
	}
	return rt.Coordination.Validate(ctx, scope)
}

func (rt *Runtime) callOwnedData(ctx context.Context, scope coordination.ExecutionScope, operation string, query coordination.DataQuery, result any) error {
	if operation == "task-definition" {
		owner := scope.TargetDeviceID
		if scope.Coordinated {
			owner = scope.CoreID
		}
		if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner {
			return coordination.ErrWrongOwner
		}
	} else if operation == "history" || operation == "history-list" || operation == "history-list-page" || operation == "history-roles" {
		if !scope.Coordinated || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID {
			return coordination.ErrWrongOwner
		}
	} else if scope.Coordinated || scope.ResourceOwnerID != scope.TargetDeviceID || scope.RoleOwnerID != scope.TargetDeviceID {
		return coordination.ErrWrongOwner
	}
	payload, err := json.Marshal(map[string]any{"operation": operation, "scope": scope, "query": query})
	if err != nil {
		return err
	}
	reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return err
	}
	if len(reply.Structured) > 4<<20 {
		return coordination.ErrPendingLimit
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	return json.Unmarshal(reply.Structured, result)
}

func (rt *Runtime) TargetTaskDefinition(ctx context.Context, scope coordination.ExecutionScope, id string) (task_runtime.TargetTaskDefinitionPin, error) {
	var result task_runtime.TargetTaskDefinitionPin
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return result, err
	}
	if id == "" || len(id) > 256 {
		return result, coordination.ErrWrongOwner
	}
	if err := rt.callOwnedData(ctx, scope, "task-definition", coordination.DataQuery{ResourceID: id}, &result); err != nil {
		return result, err
	}
	if result.DeviceID != scope.TargetDeviceID || result.TaskID != id {
		return result, coordination.ErrWrongOwner
	}
	return result, coordination.ValidateCurrent(ctx)
}

func (rt *Runtime) HistoricalConversationPage(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.HistoricalConversationPage, error) {
	var result coordination.HistoricalConversationPage
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return result, err
	}
	if !scope.Coordinated || scope.TargetDeviceID == scope.CoreID {
		return result, nil
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.HistoricalConversationPagePort); ok {
			return port.HistoricalConversationPage(ctx, scope, query)
		}
	}
	if err := rt.callOwnedData(ctx, scope, "history-list-page", query, &result); err != nil {
		return result, err
	}
	if len(result.Conversations) > 256 || len(result.NextCursor) > 4096 {
		return result, coordination.ErrPendingLimit
	}
	for _, raw := range result.Conversations {
		var row struct {
			OwnerID string `json:"ownerId"`
			ID      string `json:"id"`
		}
		if json.Unmarshal(raw, &row) != nil || row.OwnerID != scope.TargetDeviceID || row.ID == "" {
			return result, coordination.ErrWrongOwner
		}
	}
	return result, nil
}

func (rt *Runtime) HistoricalConversations(ctx context.Context, scope coordination.ExecutionScope) ([]json.RawMessage, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	if !scope.Coordinated || scope.TargetDeviceID == scope.CoreID {
		return nil, nil
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.HistoricalConversationPort); ok {
			return port.HistoricalConversations(ctx, scope)
		}
	}
	var rows []json.RawMessage
	if err := rt.callOwnedData(ctx, scope, "history-list", coordination.DataQuery{ListConversations: true, Limit: 128}, &rows); err != nil {
		return nil, err
	}
	if len(rows) > 256 {
		return nil, coordination.ErrPendingLimit
	}
	for _, raw := range rows {
		var row struct {
			OwnerID string `json:"ownerId"`
			ID      string `json:"id"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if row.OwnerID != scope.TargetDeviceID || row.ID == "" {
			return nil, coordination.ErrWrongOwner
		}
	}
	return rows, nil
}

func (rt *Runtime) Roles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	if scope.Coordinated {
		if rt.CoreDataPort == nil {
			return nil, errors.New("Core 数据服务未初始化")
		}
		return rt.CoreDataPort.Roles(ctx, scope)
	}
	if scope.TargetDeviceID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		return rt.LocalDeviceDataPort.Roles(ctx, scope)
	}
	var roles []coordination.Role
	if err := rt.callOwnedData(ctx, scope, "roles", coordination.DataQuery{}, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (rt *Runtime) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.DataSnapshot{}, err
	}
	var snapshot coordination.DataSnapshot
	var err error
	if scope.Coordinated {
		if rt.CoreDataPort == nil {
			return snapshot, errors.New("Core 数据服务未初始化")
		}
		snapshot, err = rt.CoreDataPort.Snapshot(ctx, scope, query)
	} else if scope.TargetDeviceID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		snapshot, err = rt.LocalDeviceDataPort.Snapshot(ctx, scope, query)
	} else {
		err = rt.callOwnedData(ctx, scope, "snapshot", query, &snapshot)
	}
	if err != nil {
		return snapshot, err
	}
	return snapshot, coordination.ValidateSnapshot(scope, snapshot)
}

func (rt *Runtime) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if err := rt.validateDataRoute(ctx, commit.Scope); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, commit.Scope); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if commit.Scope.Coordinated {
		if rt.CoreDataPort == nil {
			return coordination.Acknowledgement{}, errors.New("Core 数据服务未初始化")
		}
		return rt.CoreDataPort.Commit(ctx, commit)
	}
	if commit.Scope.TargetDeviceID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		return rt.LocalDeviceDataPort.Commit(ctx, commit)
	}
	return rt.CommitDeviceData(ctx, commit)
}

var _ coordination.DataPort = (*Runtime)(nil)

func (rt *Runtime) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	var port coordination.DataPort
	if scope.Coordinated {
		port = rt.CoreDataPort
	} else if scope.TargetDeviceID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	var result *coordination.Resource
	if port != nil {
		reader, ok := port.(coordination.ResourcePort)
		if !ok {
			return nil, errors.New("数据来源不支持记录查询")
		}
		var err error
		result, err = reader.Resource(ctx, scope, kind, id)
		if err != nil {
			return nil, err
		}
	} else if err := rt.callOwnedData(ctx, scope, "resource", coordination.DataQuery{ResourceKind: kind, ResourceID: id}, &result); err != nil {
		return nil, err
	}
	if result != nil && (result.OwnerID != scope.ResourceOwnerID || result.RoleID != scope.RoleID || result.Kind != kind || result.ID != id) {
		return nil, coordination.ErrWrongOwner
	}
	return result, nil
}

func (rt *Runtime) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	scope := reply.Scope
	if scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID {
		return coordination.ErrWrongOwner
	}
	if scope.Coordinated {
		port, ok := rt.CoreDataPort.(coordination.InterruptionPort)
		if !ok {
			return errors.New("Core 缺少中断保存端口")
		}
		return port.SaveInterrupted(ctx, reply)
	}
	if scope.ResourceOwnerID != scope.TargetDeviceID {
		return coordination.ErrWrongOwner
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.InterruptionPort); ok {
			return port.SaveInterrupted(ctx, reply)
		}
	}
	if err := rt.DeviceReg.RequireTrustedDevice(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID)); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"operation": "interrupted", "interrupted": reply})
	if err != nil {
		return err
	}
	result, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 5*time.Second)
	if err != nil {
		return err
	}
	var ack struct {
		Saved bool `json:"saved"`
	}
	if err := json.Unmarshal(result.Structured, &ack); err != nil {
		return err
	}
	if !ack.Saved {
		return errors.New("中断记录尚未确认保存")
	}
	return nil
}

func (rt *Runtime) HistoricalSnapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (*coordination.DataSnapshot, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	if !scope.Coordinated || scope.TargetDeviceID == scope.CoreID || query.ConversationID == "" && query.HistoricalRoleID == "" {
		return nil, nil
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.HistoricalDataPort); ok {
			return port.HistoricalSnapshot(ctx, scope, query)
		}
	}
	var snapshot *coordination.DataSnapshot
	if err := rt.callOwnedData(ctx, scope, "history", query, &snapshot); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	historical := scope
	historical.ResourceOwnerID = scope.TargetDeviceID
	historical.RoleOwnerID = scope.TargetDeviceID
	historical.RoleID = snapshot.Role.ID
	historical.RoleRevision = snapshot.Role.Revision
	if query.HistoricalRoleID != "" && snapshot.Role.ID != query.HistoricalRoleID {
		return nil, coordination.ErrRoleRequired
	}
	if err := coordination.ValidateSnapshot(historical, *snapshot); err != nil {
		return nil, err
	}
	for _, resource := range snapshot.Resources {
		if resource.Kind == "checkpoint" || resource.Kind == "continuity" {
			return nil, coordination.ErrWrongOwner
		}
	}
	return snapshot, nil
}

func (rt *Runtime) HistoricalRoles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.HistoricalRole, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	if !scope.Coordinated || scope.TargetDeviceID == scope.CoreID {
		return nil, coordination.ErrWrongOwner
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.HistoricalRolesPort); ok {
			return port.HistoricalRoles(ctx, scope)
		}
	}
	var roles []coordination.HistoricalRole
	if err := rt.callOwnedData(ctx, scope, "history-roles", coordination.DataQuery{}, &roles); err != nil {
		return nil, err
	}
	if len(roles) > 4096 {
		return nil, coordination.ErrPendingLimit
	}
	seen := map[string]bool{}
	for _, role := range roles {
		if role.ID == "" || len(role.ID) > 512 || len(role.Name) > 1024 || seen[role.ID] {
			return nil, coordination.ErrWrongOwner
		}
		seen[role.ID] = true
	}
	return roles, nil
}
