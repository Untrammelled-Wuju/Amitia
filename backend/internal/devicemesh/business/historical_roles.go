package business

import (
	"context"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) HistoricalRoles(ctx context.Context, request Request) ([]coordination.HistoricalRole, coordination.ExecutionScope, error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return nil, scope, err
	}
	defer finish()
	if !scope.Coordinated {
		return nil, scope, coordination.ErrWrongOwner
	}
	if scope.TargetDeviceID == scope.CoreID {
		return []coordination.HistoricalRole{}, scope, nil
	}
	port, ok := e.data.(coordination.HistoricalRolesPort)
	if !ok {
		return nil, scope, errors.New("设备历史记忆目录不可用")
	}
	roles, err := port.HistoricalRoles(ctx, scope)
	if err != nil {
		return nil, scope, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return nil, scope, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return nil, scope, err
	}
	return roles, scope, nil
}
