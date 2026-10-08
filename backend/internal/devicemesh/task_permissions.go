package devicemesh

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) TargetTaskPermissions(ctx context.Context, request task_runtime.SourceTaskPermissionRequest) error {
	authority, owned := coordination.FromContext(ctx)
	if !owned || authority != request.Scope {
		return coordination.ErrWrongOwner
	}
	owner := authority.TargetDeviceID
	if authority.Coordinated {
		owner = authority.CoreID
	}
	if authority.ResourceOwnerID != owner || authority.RoleOwnerID != owner {
		return coordination.ErrWrongOwner
	}
	if err := rt.validateDataRoute(ctx, authority); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"operation": "task-permissions", "scope": authority, "taskPermission": request})
	if err != nil || len(payload) > (512<<10)+(128<<10) {
		return coordination.ErrPendingLimit
	}
	reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(authority.SpaceID), runtimeidentity.DeviceID(authority.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return err
	}
	var ack task_runtime.SourceTaskPermissionAcknowledgement
	if len(reply.Structured) > 64<<10 || json.Unmarshal(reply.Structured, &ack) != nil {
		return task_runtime.NewTaskError(task_runtime.ErrTaskPermissionDenied, "目标设备权限确认无效")
	}
	if err := rt.validateDataRoute(ctx, authority); err != nil {
		return err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	return task_runtime.ValidateSourceTaskPermissionAcknowledgement(request, ack)
}
