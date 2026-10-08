package devicemesh

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) OpenTaskRead(ctx context.Context, proof coordination.TaskReadProof) (context.Context, func(), error) {
	actor, ok := auth.FromContext(ctx)
	if rt == nil || rt.Coordination == nil || !ok || actor == nil || actor.SpaceID.String() != proof.Scope.CoreID || actor.DeviceID == "" || actor.PrincipalType != auth.PrincipalTrustedDevice && (actor.PrincipalType != auth.PrincipalLocalUI || !actor.HasPermission(auth.PermSystemAdmin)) {
		return ctx, nil, coordination.ErrWrongOwner
	}
	if !actor.HasPermission(auth.PermSystemAdmin) && actor.DeviceID.String() != proof.Scope.InitiatorDeviceID && actor.DeviceID.String() != proof.Scope.TargetDeviceID {
		return ctx, nil, coordination.ErrWrongOwner
	}
	target := proof.Scope.TargetDeviceID
	if actor.HasPermission(auth.PermSystemAdmin) && proof.Scope.Coordinated && proof.Scope.ResourceOwnerID == proof.Scope.CoreID {
		target = actor.DeviceID.String()
	}
	current, scope, finish, err := rt.Coordination.Begin(ctx, proof.Scope.CoreID, actor.DeviceID.String(), target, proof.Scope.CoreID, "", "task-history-"+uuid.NewString())
	if err != nil {
		return ctx, nil, err
	}
	if actor.PrincipalType == auth.PrincipalTrustedDevice && actor.HasPermission(auth.PermSystemAdmin) {
		policy, err := rt.Coordination.Get(current, proof.Scope.CoreID, actor.DeviceID.String())
		if err != nil || !policy.Coordinated || !policy.Administrator || policy.PermissionRevision != scope.PermissionRevision {
			finish()
			return ctx, nil, coordination.ErrScopeExpired
		}
	}
	read, err := coordination.WithTaskReadAuthority(current, scope, proof)
	if err != nil {
		finish()
		return ctx, nil, err
	}
	return read, finish, nil
}

func (rt *Runtime) readTaskResource(ctx context.Context, current coordination.ExecutionScope, proof coordination.TaskReadProof, kind, id string) (*coordination.Resource, error) {
	if err := coordination.ValidateTaskReadProof(current, proof); err != nil {
		return nil, err
	}
	if err := coordination.ValidateTaskReadResourceID(proof, kind, id); err != nil {
		return nil, err
	}
	if err := rt.validateDataRoute(ctx, current); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	var port coordination.DataPort
	if proof.Scope.ResourceOwnerID == current.CoreID {
		port = rt.CoreDataPort
	} else if proof.Scope.ResourceOwnerID == rt.LocalDeviceID {
		port = rt.LocalDeviceDataPort
	}
	var result *coordination.Resource
	if port != nil {
		reader, ok := port.(coordination.TaskHistoryResourcePort)
		if !ok {
			return nil, coordination.ErrWrongOwner
		}
		var err error
		result, err = reader.ReadTaskResource(ctx, current, proof, kind, id)
		if err != nil {
			return nil, err
		}
	} else {
		if proof.Scope.ResourceOwnerID == current.CoreID {
			return nil, coordination.ErrWrongOwner
		}
		payload, err := json.Marshal(map[string]any{"operation": "task-history-resource", "scope": current, "taskRead": proof, "query": coordination.DataQuery{ResourceKind: kind, ResourceID: id}})
		if err != nil {
			return nil, err
		}
		reply, err := rt.InvokeDeviceHandlerWithRuntimeType(coordination.WithScope(ctx, current), runtimeidentity.SpaceID(current.SpaceID), runtimeidentity.DeviceID(current.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if err != nil {
			return nil, err
		}
		if len(reply.Structured) > 4<<20 {
			return nil, coordination.ErrPendingLimit
		}
		if err := json.Unmarshal(reply.Structured, &result); err != nil {
			return nil, err
		}
	}
	if err := coordination.ValidateTaskReadResource(proof, kind, id, result); err != nil {
		return nil, err
	}
	return result, coordination.ValidateCurrent(ctx)
}
