package business

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) BusinessUsage(ctx context.Context, request Request, since string) (coordination.BusinessUsage, error) {
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.DeviceID, request.CoreID, "", uuid.NewString())
	if err != nil {
		return coordination.BusinessUsage{}, err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return coordination.BusinessUsage{}, err
	}
	port, ok := e.data.(coordination.BusinessUsagePort)
	if !ok {
		return coordination.BusinessUsage{}, errors.New("Owner统计端口未接通")
	}
	result, err := port.BusinessUsage(coordination.WithScope(ctx, scope), scope, since)
	if err != nil {
		return result, err
	}
	if result.OwnerID != scope.ResourceOwnerID {
		return result, coordination.ErrWrongOwner
	}
	return result, e.coordination.Validate(ctx, scope)
}
