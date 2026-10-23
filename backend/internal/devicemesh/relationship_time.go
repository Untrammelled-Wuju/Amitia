package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"time"
)

func (rt *Runtime) RelationshipTime(ctx context.Context, scope coordination.ExecutionScope, request coordination.RelationshipTimeRequest) (json.RawMessage, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	var local coordination.DataPort
	if scope.Coordinated && scope.ResourceOwnerID == scope.CoreID {
		local = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
		local = rt.LocalDeviceDataPort
	}
	var result json.RawMessage
	var err error
	if local != nil {
		port, ok := local.(coordination.RelationshipTimePort)
		if !ok {
			return nil, errors.New("Owner 关系时间端口未接通")
		}
		result, err = port.RelationshipTime(ctx, scope, request)
	} else {
		if scope.ResourceOwnerID != scope.TargetDeviceID {
			return nil, coordination.ErrWrongOwner
		}
		payload, marshalErr := json.Marshal(map[string]any{"operation": "relationship-time", "scope": scope, "relationshipTime": request})
		if marshalErr != nil {
			return nil, marshalErr
		}
		reply, invokeErr := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.ResourceOwnerID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if invokeErr != nil {
			return nil, invokeErr
		}
		if len(reply.Structured) > 256<<10 {
			return nil, coordination.ErrPendingLimit
		}
		result = json.RawMessage(reply.Structured)
	}
	if err != nil {
		return nil, err
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	return result, nil
}
