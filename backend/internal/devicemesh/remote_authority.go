package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) fenceRemoteAuthority(ctx context.Context, space, device string, revision int64, sources []string) error {
	for _, source := range sources {
		scope := coordination.ExecutionScope{SpaceID: space, CoreID: space, AuthorizationRealm: space, TargetDeviceID: source, ResourceOwnerID: source, RoleOwnerID: source}
		payload, err := json.Marshal(map[string]any{"operation": "authority-fence", "scope": scope, "fencedDeviceId": device, "closedPermissionRevision": revision})
		if err != nil {
			return err
		}
		reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(space), runtimeidentity.DeviceID(source), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if err != nil {
			return err
		}
		var ack struct {
			DeviceID string `json:"deviceId"`
			Revision int64  `json:"closedPermissionRevision"`
		}
		if json.Unmarshal(reply.Structured, &ack) != nil || ack.DeviceID != device || ack.Revision != revision {
			return errors.New("设备未确认远端权限屏障")
		}
	}
	return nil
}

func (rt *Runtime) ReconcileRemoteDevice(ctx context.Context, space, target string) error {
	connection, ok := rt.Hub.GetByDevice(runtimeidentity.SpaceID(space), runtimeidentity.DeviceID(target))
	if !ok || connection == nil {
		return coordination.ErrAuthorityUnconfirmed
	}
	return rt.Coordination.ReconcileRemoteAuthority(ctx, space, target, connection.SessionID.String(), connection.Generation, func(ctx context.Context, ids []string) error {
		scope := coordination.ExecutionScope{SpaceID: space, CoreID: space, AuthorizationRealm: space, TargetDeviceID: target, ResourceOwnerID: target, RoleOwnerID: target}
		payload, err := json.Marshal(map[string]any{"operation": "authority-reconcile", "scope": scope, "cancelledCallIds": ids})
		if err != nil {
			return err
		}
		reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(space), runtimeidentity.DeviceID(target), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if err != nil {
			return err
		}
		var ack struct {
			IDs []string `json:"cancelledCallIds"`
		}
		if json.Unmarshal(reply.Structured, &ack) != nil || !slices.Equal(ack.IDs, ids) {
			return errors.New("设备未确认旧连接数据请求已停止")
		}
		return nil
	})
}
