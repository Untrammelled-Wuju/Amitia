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

func (rt *Runtime) SubmitChannelDelivery(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChannelDeliveryRequest) error {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return err
	}
	if scope.TargetDeviceID == "" || request.RequestID != scope.RequestID || request.ConversationID == "" {
		return coordination.ErrWrongOwner
	}
	if scope.TargetDeviceID == rt.LocalDeviceID {
		port, ok := rt.LocalDeviceDataPort.(coordination.ChannelDeliveryPort)
		if !ok {
			return errors.New("目标设备渠道交付端口未接通")
		}
		return port.SubmitChannelDelivery(ctx, scope, request)
	}
	if rt.DeviceReg == nil {
		return coordination.ErrWrongOwner
	}
	if err := rt.DeviceReg.RequireTrustedDevice(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID)); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"operation": "channel-delivery", "scope": scope, "channelDelivery": request})
	if err != nil {
		return err
	}
	result, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID), capability.RuntimeTypeInternal, "coordination.data", payload, 30*time.Second)
	if err != nil {
		return err
	}
	var acknowledgement struct {
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(result.Structured, &acknowledgement); err != nil {
		return err
	}
	if acknowledgement.RequestID != request.RequestID {
		return coordination.ErrRequestConflict
	}
	return rt.validateDataRoute(ctx, scope)
}

func (rt *Runtime) ResumeChannelDeliveries(ctx context.Context) error {
	for _, data := range []coordination.DataPort{rt.CoreDataPort, rt.LocalDeviceDataPort} {
		port, ok := data.(coordination.ChannelDeliveryRecoveryPort)
		if !ok {
			continue
		}
		cursor := ""
		for {
			pending, err := port.PendingChannelDeliveries(ctx, cursor)
			if err != nil {
				return err
			}
			if len(pending) == 0 {
				break
			}
			for _, item := range pending {
				if item.Cursor <= cursor {
					return coordination.ErrRequestConflict
				}
				cursor = item.Cursor
				current, finish, err := rt.Coordination.Restore(ctx, item.Scope, rt)
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					continue
				}
				if err := rt.SubmitChannelDelivery(current, item.Scope, item.Request); err != nil {
					finish()
					if ctx.Err() != nil {
						return ctx.Err()
					}
					continue
				}
				err = port.CompleteChannelDelivery(current, item)
				finish()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
