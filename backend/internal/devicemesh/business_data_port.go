package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) validateDataRoute(ctx context.Context, scope coordination.ExecutionScope) error {
	if current, _, historical := coordination.TaskReadAuthority(ctx); historical && scope != current {
		return coordination.ErrWrongOwner
	}
	if rt.Coordination == nil || scope.CoreID == "" || scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID {
		return coordination.ErrWrongOwner
	}
	return rt.Coordination.Validate(ctx, scope)
}

func (rt *Runtime) callOwnedData(ctx context.Context, scope coordination.ExecutionScope, operation string, query coordination.DataQuery, result any) error {
	if operation == "task-definition" || operation == "task-catalog-entry" {
		if scope.ResourceOwnerID != scope.TargetDeviceID || (scope.RoleOwnerID != scope.TargetDeviceID && scope.RoleOwnerID != scope.CoreID) {
			return coordination.ErrWrongOwner
		}
	} else if operation == "history" || operation == "history-list" || operation == "history-list-page" || operation == "history-roles" {
		if (scope.ResourceOwnerID != scope.TargetDeviceID && scope.ResourceOwnerID != scope.CoreID) || (scope.RoleOwnerID != scope.TargetDeviceID && scope.RoleOwnerID != scope.CoreID) {
			return coordination.ErrWrongOwner
		}
	} else if scope.ResourceOwnerID != scope.TargetDeviceID || (scope.RoleOwnerID != scope.TargetDeviceID && scope.RoleOwnerID != scope.CoreID) {
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
	if scope.ResourceOwnerID != scope.TargetDeviceID || scope.TargetDeviceID == scope.CoreID {
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
	if scope.ResourceOwnerID != scope.TargetDeviceID || scope.TargetDeviceID == scope.CoreID {
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

func (rt *Runtime) LocalMemoryArchiveEmpty(ctx context.Context, scope coordination.ExecutionScope) (bool, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return false, err
	}
	if scope.Coordinated || scope.TargetDeviceID != rt.LocalDeviceID || scope.ResourceOwnerID != rt.LocalDeviceID {
		return false, nil
	}
	roles, err := rt.Roles(ctx, scope)
	if err != nil || len(roles) != 0 {
		return false, err
	}
	hasRecords, err := business.HasLiveMemoryArchiveRecords(ctx, rt.DB, scope.ResourceOwnerID)
	return !hasRecords, err
}

func (rt *Runtime) Roles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, fmt.Errorf("校验角色读取范围失败: %w", err)
	}
	if scope.RoleOwnerID == scope.CoreID && rt.CoreDataPort != nil {
		roles, err := rt.CoreDataPort.Roles(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("读取中心角色目录失败 roleOwner=%q resourceOwner=%q localDevice=%q: %w", scope.RoleOwnerID, scope.ResourceOwnerID, rt.LocalDeviceID, err)
		}
		return roles, nil
	}
	if scope.TargetDeviceID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		roles, err := rt.LocalDeviceDataPort.Roles(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("读取本机角色目录失败: %w", err)
		}
		return roles, nil
	}
	var roles []coordination.Role
	if err := rt.callOwnedData(ctx, scope, "roles", coordination.DataQuery{}, &roles); err != nil {
		return nil, fmt.Errorf("读取远程角色目录失败: %w", err)
	}
	return roles, nil
}

func (rt *Runtime) GroupState(ctx context.Context, scope coordination.ExecutionScope, conversationID string) (coordination.GroupState, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.GroupState{}, fmt.Errorf("校验会话分组读取范围失败: %w", err)
	}
	var port coordination.DataPort
	if scope.ResourceOwnerID == scope.CoreID {
		port = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	if port != nil {
		reader, ok := port.(coordination.GroupStatePort)
		if !ok {
			return coordination.GroupState{}, coordination.ErrWrongOwner
		}
		state, err := reader.GroupState(ctx, scope, conversationID)
		if err != nil {
			return coordination.GroupState{}, fmt.Errorf("读取本机会话分组失败 resourceOwner=%q roleOwner=%q localDevice=%q: %w", scope.ResourceOwnerID, scope.RoleOwnerID, rt.LocalDeviceID, err)
		}
		return state, nil
	}
	var result coordination.GroupState
	if err := rt.callOwnedData(ctx, scope, "group-state", coordination.DataQuery{ConversationID: conversationID}, &result); err != nil {
		return coordination.GroupState{}, fmt.Errorf("读取远程会话分组失败: %w", err)
	}
	if result.ConversationID != "" && (result.ConversationID != conversationID || result.RoleOwnerID != scope.RoleOwnerID) {
		return coordination.GroupState{}, coordination.ErrWrongOwner
	}
	return result, nil
}

func (rt *Runtime) ListOwnedRoleCatalog(ctx context.Context, scope coordination.ExecutionScope, includeDisabled bool) ([]coordination.Role, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	var local coordination.DataPort
	if scope.RoleOwnerID == scope.CoreID {
		local = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
		local = rt.LocalDeviceDataPort
	} else {
		var result []coordination.Role
		if err := rt.callOwnedData(ctx, scope, "role-catalog", coordination.DataQuery{IncludeDisabled: includeDisabled}, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	port, ok := local.(coordination.RoleCatalogPort)
	if !ok {
		return nil, errors.New("Owner 角色目录未接通")
	}
	return port.ListOwnedRoleCatalog(ctx, scope, includeDisabled)
}

func (rt *Runtime) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.DataSnapshot{}, fmt.Errorf("校验数据快照范围失败: %w", err)
	}
	roles, err := rt.Roles(ctx, scope)
	if err != nil {
		return coordination.DataSnapshot{}, fmt.Errorf("读取数据快照角色失败: %w", err)
	}
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil {
		return coordination.DataSnapshot{}, fmt.Errorf("解析数据快照角色失败: %w", err)
	}
	if role.Revision != scope.RoleRevision {
		return coordination.DataSnapshot{}, coordination.ErrScopeExpired
	}
	query.Role = &role
	var snapshot coordination.DataSnapshot
	splitOwners := scope.Coordinated && scope.ResourceOwnerID == scope.TargetDeviceID && scope.ResourceOwnerID != scope.CoreID && scope.RoleOwnerID == scope.CoreID && query.ResourceKind == "" && rt.CoreDataPort != nil
	deviceQuery := query
	if splitOwners {
		deviceQuery.Role = &role
	}
	if scope.ResourceOwnerID == scope.CoreID {
		if rt.CoreDataPort == nil {
			return snapshot, errors.New("Core 数据服务未初始化")
		}
		snapshot, err = rt.CoreDataPort.Snapshot(ctx, scope, query)
		if err != nil {
			err = fmt.Errorf("读取中心数据快照失败: %w", err)
		}
	} else if scope.ResourceOwnerID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		port, ok := rt.LocalDeviceDataPort.(coordination.RoleSnapshotPort)
		if !ok {
			return snapshot, errors.New("本机数据服务不支持角色快照")
		}
		snapshot, err = port.SnapshotWithRole(ctx, scope, deviceQuery, role)
		if err != nil {
			err = fmt.Errorf("读取本机数据快照失败: %w", err)
		}
	} else {
		err = rt.callOwnedData(ctx, scope, "snapshot", deviceQuery, &snapshot)
		if err != nil {
			err = fmt.Errorf("读取远程数据快照失败: resourceOwner=%s localDevice=%s targetDevice=%s: %w", scope.ResourceOwnerID, rt.LocalDeviceID, scope.TargetDeviceID, err)
		}
	}
	if err != nil {
		return snapshot, err
	}
	if splitOwners {
		filtered := snapshot.Resources[:0]
		for _, resource := range snapshot.Resources {
			if !coordination.CoreOwnedKind(resource.Kind) {
				filtered = append(filtered, resource)
			}
		}
		snapshot.Resources = filtered
		snapshot.RecallEvidence = nil
		coreScope := scope
		coreScope.ResourceOwnerID = scope.CoreID
		coreScope.RoleOwnerID = scope.CoreID
		coreQuery := query
		coreQuery.ConversationID = ""
		coreQuery.RequestID = ""
		coreQuery.ListConversations = false
		coreQuery.Role = &role
		coreQuery.SkipRelationshipTime = true
		coreSnapshot, coreErr := rt.CoreDataPort.Snapshot(ctx, coreScope, coreQuery)
		if coreErr != nil {
			return snapshot, fmt.Errorf("读取中心记忆快照失败: %w", coreErr)
		}
		for _, resource := range coreSnapshot.Resources {
			if coordination.CoreOwnedKind(resource.Kind) {
				snapshot.Resources = append(snapshot.Resources, resource)
			}
		}
		if len(coreSnapshot.RecallEvidence) > 0 {
			snapshot.RecallEvidence = append([]coordination.ResourceVersion(nil), coreSnapshot.RecallEvidence...)
		}
		if snapshot.NextCursors == nil {
			snapshot.NextCursors = make(map[string]string)
		}
		for key, value := range coreSnapshot.NextCursors {
			if coordination.CoreOwnedKind(key) {
				snapshot.NextCursors[key] = value
			}
		}
		if snapshot.RuntimeContext == nil {
			snapshot.RuntimeContext = make(map[string]json.RawMessage)
		}
		for key, value := range coreSnapshot.RuntimeContext {
			snapshot.RuntimeContext[key] = value
		}
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return snapshot, fmt.Errorf("校验数据快照归属失败: %w", err)
	}
	return snapshot, nil
}

func (rt *Runtime) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if err := rt.validateDataRoute(ctx, commit.Scope); err != nil {
		return coordination.Acknowledgement{}, fmt.Errorf("校验数据提交范围失败: %w", err)
	}
	if err := rt.Coordination.ValidateCommitAuthorities(ctx, commit); err != nil {
		return coordination.Acknowledgement{}, fmt.Errorf("校验数据提交权限失败: %w", err)
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, commit.Scope); err != nil {
		return coordination.Acknowledgement{}, fmt.Errorf("校验数据提交角色失败: %w", err)
	}
	if commit.Scope.Coordinated && commit.Scope.RoleOwnerID == commit.Scope.CoreID && commit.Scope.ResourceOwnerID != commit.Scope.CoreID && rt.CoreDataPort != nil {
		var deviceMutations, coreMutations []coordination.Mutation
		for _, mutation := range commit.Mutations {
			if coordination.CoreOwnedKind(mutation.Kind) {
				coreMutations = append(coreMutations, mutation)
			} else {
				deviceMutations = append(deviceMutations, mutation)
			}
		}
		if len(coreMutations) > 0 {
			return rt.commitSplitOwners(ctx, commit, deviceMutations, coreMutations)
		}
	}
	return rt.commitOwnedPartition(ctx, commit)
}

func (rt *Runtime) commitSplitOwners(ctx context.Context, commit coordination.Commit, deviceMutations, coreMutations []coordination.Mutation) (coordination.Acknowledgement, error) {
	ownerID := commit.Scope.ResourceOwnerID
	if commit.MemoryTransfer != nil && len(deviceMutations) > 0 {
		return coordination.Acknowledgement{}, errors.New("记忆转移不能与设备专属数据混合提交")
	}
	deviceCommit := commit
	deviceCommit.Mutations = deviceMutations
	deviceCommit.Dependencies = filterOwnedDependencies(commit.Dependencies, false)
	var deviceAck coordination.Acknowledgement
	var err error
	if len(deviceMutations) > 0 {
		deviceAck, err = rt.commitOwnedPartition(ctx, deviceCommit)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
	}
	coreScope := commit.Scope
	coreScope.ResourceOwnerID = commit.Scope.CoreID
	coreScope.RoleOwnerID = commit.Scope.CoreID
	coreCommit := commit
	coreCommit.Scope = coreScope
	coreCommit.Mutations = coreMutations
	coreCommit.Dependencies = filterOwnedDependencies(commit.Dependencies, true)
	coreCommit.AdditionalAuthorities = nil
	if err := rt.Coordination.ValidateCommitAuthorities(ctx, coreCommit); err != nil {
		return coordination.Acknowledgement{}, fmt.Errorf("校验中心数据提交权限失败: %w", err)
	}
	coreAck, err := rt.commitOwnedPartition(ctx, coreCommit)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	combined := coordination.Acknowledgement{RequestID: commit.Scope.RequestID, OwnerID: ownerID, Versions: make(map[string]int64)}
	for _, ack := range []coordination.Acknowledgement{deviceAck, coreAck} {
		if ack.RequestID == "" {
			continue
		}
		if ack.RequestID != commit.Scope.RequestID {
			return coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
		for key, revision := range ack.Versions {
			combined.Versions[key] = revision
		}
	}
	return combined, nil
}

func filterOwnedDependencies(dependencies []coordination.ResourceVersion, core bool) []coordination.ResourceVersion {
	filtered := make([]coordination.ResourceVersion, 0, len(dependencies))
	for _, dependency := range dependencies {
		if coordination.CoreOwnedKind(dependency.Kind) == core {
			filtered = append(filtered, dependency)
		}
	}
	return filtered
}

func (rt *Runtime) commitOwnedPartition(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if commit.Scope.ResourceOwnerID == commit.Scope.CoreID {
		if rt.CoreDataPort == nil {
			return coordination.Acknowledgement{}, errors.New("Core 数据服务未初始化")
		}
		ack, err := rt.CoreDataPort.Commit(ctx, commit)
		if err != nil {
			return coordination.Acknowledgement{}, fmt.Errorf("保存中心数据失败: %w", err)
		}
		return ack, nil
	}
	if commit.Scope.ResourceOwnerID == rt.LocalDeviceID && rt.LocalDeviceDataPort != nil {
		ack, err := rt.LocalDeviceDataPort.Commit(ctx, commit)
		if err != nil {
			return coordination.Acknowledgement{}, fmt.Errorf("保存本机设备数据失败: %w", err)
		}
		return ack, nil
	}
	ack, err := rt.CommitDeviceData(ctx, commit)
	if err != nil {
		return coordination.Acknowledgement{}, fmt.Errorf("保存远程设备数据失败: %w", err)
	}
	return ack, nil
}

var _ coordination.DataPort = (*Runtime)(nil)

func (rt *Runtime) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	if current, proof, historical := coordination.TaskReadAuthority(ctx); historical {
		if scope != proof.Scope {
			return nil, coordination.ErrWrongOwner
		}
		return rt.readTaskResource(ctx, current, proof, kind, id)
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	if err := coordination.ValidateRoleRevision(ctx, rt, scope); err != nil {
		return nil, err
	}
	var port coordination.DataPort
	if scope.ResourceOwnerID == scope.CoreID {
		port = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
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
	if result != nil && (result.OwnerID != scope.ResourceOwnerID || result.RoleID != coordination.StorageRole(scope, kind) || result.Kind != kind || result.ID != id) {
		return nil, coordination.ErrWrongOwner
	}
	return result, nil
}

func (rt *Runtime) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	scope := reply.Scope
	if scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID {
		return coordination.ErrWrongOwner
	}
	if scope.ResourceOwnerID == scope.CoreID {
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
		return nil, fmt.Errorf("校验历史快照权限失败: %w", err)
	}
	if (scope.ResourceOwnerID != scope.TargetDeviceID && scope.ResourceOwnerID != scope.CoreID) || scope.TargetDeviceID == scope.CoreID || query.ConversationID == "" && query.HistoricalRoleID == "" {
		return nil, fmt.Errorf("历史快照范围无效 target=%s resourceOwner=%s core=%s: %w", scope.TargetDeviceID, scope.ResourceOwnerID, scope.CoreID, coordination.ErrWrongOwner)
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		if port, ok := rt.LocalDeviceDataPort.(coordination.HistoricalDataPort); ok {
			return port.HistoricalSnapshot(ctx, scope, query)
		}
	}
	var snapshot *coordination.DataSnapshot
	if err := rt.callOwnedData(ctx, scope, "history", query, &snapshot); err != nil {
		return nil, fmt.Errorf("读取远程历史快照失败: %w", err)
	}
	if snapshot == nil {
		return nil, nil
	}
	historical := scope
	historical.ResourceOwnerID = scope.TargetDeviceID
	historical.RoleOwnerID = scope.RoleOwnerID
	historical.RoleID = snapshot.Role.ID
	historical.MemoryRoleID = snapshot.Role.ID
	historical.MemoryViewAll = false
	historical.RoleRevision = snapshot.Role.Revision
	if query.HistoricalRoleID != "" && snapshot.Role.ID != query.HistoricalRoleID {
		return nil, coordination.ErrRoleRequired
	}
	historicalValidation := historical
	historicalValidation.Coordinated = false
	if err := coordination.ValidateSnapshot(historicalValidation, *snapshot); err != nil {
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
