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

func (rt *Runtime) BusinessUsage(ctx context.Context, scope coordination.ExecutionScope, since string) (coordination.BusinessUsage, error) {
	var result coordination.BusinessUsage
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return result, err
	}
	var local coordination.DataPort
	if scope.Coordinated && scope.ResourceOwnerID == scope.CoreID {
		local = rt.CoreDataPort
	} else if scope.ResourceOwnerID == rt.LocalDeviceID {
		local = rt.LocalDeviceDataPort
	}
	if local != nil {
		port, ok := local.(coordination.BusinessUsagePort)
		if !ok {
			return result, errors.New("Owner统计端口未接通")
		}
		var err error
		result, err = port.BusinessUsage(ctx, scope, since)
		if err != nil {
			return result, err
		}
	} else {
		if scope.ResourceOwnerID != scope.TargetDeviceID {
			return result, coordination.ErrWrongOwner
		}
		payload, err := json.Marshal(map[string]any{"operation": "usage", "scope": scope, "usageSince": since})
		if err != nil {
			return result, err
		}
		reply, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.ResourceOwnerID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
		if err != nil {
			return result, err
		}
		if len(reply.Structured) > 256<<10 {
			return result, coordination.ErrPendingLimit
		}
		if err := json.Unmarshal(reply.Structured, &result); err != nil {
			return result, err
		}
	}
	if result.OwnerID != scope.ResourceOwnerID {
		return result, coordination.ErrWrongOwner
	}
	return result, rt.validateDataRoute(ctx, scope)
}
