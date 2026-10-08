package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) TaskRoles(ctx context.Context, request Request) ([]coordination.Role, coordination.ExecutionScope, string, error) {
	if e == nil || e.coordination == nil || e.data == nil || request.RequestID == "" || len(request.RequestID) > 128 || request.SpaceID != request.CoreID || request.DeviceID == "" {
		return nil, coordination.ExecutionScope{}, "", coordination.ErrWrongOwner
	}
	current, authority, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, "", request.RequestID)
	if err != nil {
		return nil, authority, "", err
	}
	defer finish()
	if err := e.coordination.RequireCapability(current, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return nil, authority, "", err
	}
	roles, err := e.data.Roles(current, authority)
	if err != nil {
		return nil, authority, "", err
	}
	if len(roles) > 256 {
		return nil, authority, "", coordination.ErrPendingLimit
	}
	for _, role := range roles {
		if role.ID == "" || len(role.ID) > 256 || role.Revision < 1 || len(role.Profile) > 64<<10 || !json.Valid(role.Profile) {
			return nil, authority, "", coordination.ErrRoleRequired
		}
	}
	policy, err := e.coordination.Get(current, authority.SpaceID, authority.TargetDeviceID)
	if err != nil {
		return nil, authority, "", err
	}
	return roles, authority, policy.SelectedRole, coordination.ValidateCurrent(current)
}

func (e *Engine) OpenTaskExecution(ctx context.Context, request Request) (context.Context, coordination.ExecutionScope, func(), error) {
	if e == nil || e.coordination == nil || e.data == nil || request.RequestID == "" || len(request.RequestID) > 128 || request.SpaceID != request.CoreID || request.DeviceID == "" {
		return ctx, coordination.ExecutionScope{}, nil, coordination.ErrWrongOwner
	}
	parent := ctx
	current, authority, closeInitial, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return ctx, authority, nil, err
	}
	defer closeInitial()
	if err := e.coordination.RequireCapability(current, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		return ctx, authority, nil, err
	}
	roles, err := e.data.Roles(current, authority)
	if err != nil {
		return ctx, authority, nil, err
	}
	requested := request.RoleID
	if requested == "" {
		policy, err := e.coordination.Get(current, authority.SpaceID, authority.TargetDeviceID)
		if err != nil {
			return ctx, authority, nil, err
		}
		requested = policy.SelectedRole
	}
	role, err := coordination.ResolveRole(requested, roles)
	if err != nil {
		return ctx, authority, nil, err
	}
	authority.RoleID, authority.RoleRevision = role.ID, role.Revision
	if request.ExpectedScope != nil {
		expected := *request.ExpectedScope
		expected.RequestID, expected.TurnID, expected.ExecutionID = authority.RequestID, authority.TurnID, authority.ExecutionID
		if expected != authority {
			return ctx, authority, nil, coordination.ErrScopeExpired
		}
	}
	key, err := json.Marshal([]string{authority.CoreID, authority.InitiatorDeviceID, authority.RequestID})
	if err != nil {
		return ctx, authority, nil, err
	}
	hash := sha256.Sum256(key)
	authority.ExecutionID, authority.TurnID = "task-execution-"+hex.EncodeToString(hash[:]), "task-turn-"+hex.EncodeToString(hash[:])
	restored, finish, err := e.coordination.Restore(parent, authority, e.data)
	if err != nil {
		return ctx, authority, nil, err
	}
	if err := e.coordination.RequireCapability(restored, authority.SpaceID, authority.InitiatorDeviceID, authority.TargetDeviceID, "task.execute"); err != nil {
		finish()
		return ctx, authority, nil, err
	}
	return restored, authority, finish, nil
}
