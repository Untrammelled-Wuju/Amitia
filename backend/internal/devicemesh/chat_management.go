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

func (rt *Runtime) ManageChat(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest) (map[string]any, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	owner := request.ConversationOwnerID
	if owner == "" {
		owner = scope.ResourceOwnerID
		request.ConversationOwnerID = owner
	}
	if owner != scope.ResourceOwnerID && (!scope.Coordinated || owner != scope.TargetDeviceID) {
		return nil, coordination.ErrWrongOwner
	}
	var local coordination.DataPort
	if owner == scope.CoreID && scope.Coordinated {
		local = rt.CoreDataPort
	} else if owner == rt.LocalDeviceID {
		local = rt.LocalDeviceDataPort
	}
	var result map[string]any
	var err error
	if local != nil {
		port, ok := local.(coordination.ChatManagementPort)
		if !ok {
			return nil, errors.New("Owner 聊天管理端口未接通")
		}
		result, err = port.ManageChat(ctx, scope, request)
	} else {
		if owner != scope.TargetDeviceID {
			return nil, coordination.ErrWrongOwner
		}
		payload, marshalErr := coordination.ChatManagementPayload(scope, request)
		if marshalErr != nil {
			return nil, marshalErr
		}
		reply, invokeErr := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(owner), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if invokeErr != nil {
			return nil, invokeErr
		}
		if len(reply.Structured) > 4<<20 {
			return nil, coordination.ErrPendingLimit
		}
		err = json.Unmarshal(reply.Structured, &result)
	}
	if err != nil {
		return nil, err
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	return result, nil
}
