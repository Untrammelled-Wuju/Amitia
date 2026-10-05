package business

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) Projections(ctx context.Context, request Request, rebuild bool, expected *coordination.ExecutionScope) (coordination.ProjectionStatus, coordination.ExecutionScope, error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return coordination.ProjectionStatus{}, scope, err
	}
	defer finish()
	if rebuild {
		if expected == nil {
			return coordination.ProjectionStatus{}, scope, coordination.ErrScopeExpired
		}
		previous := *expected
		previous.RequestID, previous.ExecutionID, previous.TurnID = scope.RequestID, scope.ExecutionID, scope.TurnID
		if previous != scope {
			return coordination.ProjectionStatus{}, scope, coordination.ErrScopeExpired
		}
	}
	port, ok := e.data.(coordination.ProjectionPort)
	if !ok {
		return coordination.ProjectionStatus{}, scope, errors.New("数据所有者的索引管理端口不可用")
	}
	result, err := port.ProjectionState(ctx, scope, rebuild)
	if err != nil {
		return result, scope, err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return result, scope, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return result, scope, err
	}
	return result, scope, nil
}
