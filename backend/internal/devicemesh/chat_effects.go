package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) CommitOwnedChatEffects(ctx context.Context, scope coordination.ExecutionScope, request coordination.OwnedChatEffectsRequest) error {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	var local coordination.DataPort
	if scope.Coordinated && scope.ResourceOwnerID == scope.CoreID {
		local = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
		local = rt.LocalDeviceDataPort
	}
	if local != nil {
		port, ok := local.(coordination.OwnerPostCommitPort)
		if !ok {
			return errors.New("Owner 对话后处理端口未接通")
		}
		if err := port.CommitOwnedChatEffects(ctx, scope, request); err != nil {
			return err
		}
	} else {
		if scope.ResourceOwnerID != scope.TargetDeviceID {
			return coordination.ErrWrongOwner
		}
		payload, err := json.Marshal(map[string]any{"operation": "chat-effects", "scope": scope, "chatEffects": request})
		if err != nil {
			return err
		}
		if _, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.ResourceOwnerID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second); err != nil {
			return err
		}
	}
	return rt.validateDataRoute(ctx, scope)
}
