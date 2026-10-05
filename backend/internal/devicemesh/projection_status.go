package devicemesh

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (rt *Runtime) ProjectionState(ctx context.Context, scope coordination.ExecutionScope, rebuild bool) (coordination.ProjectionStatus, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return coordination.ProjectionStatus{}, err
	}
	var data coordination.DataPort
	if scope.Coordinated {
		data = rt.CoreDataPort
	} else if scope.TargetDeviceID == rt.LocalDeviceID {
		data = rt.LocalDeviceDataPort
	}
	var result coordination.ProjectionStatus
	var err error
	if data != nil {
		port, ok := data.(coordination.ProjectionPort)
		if !ok {
			return result, errors.New("数据所有者的索引管理端口不可用")
		}
		result, err = port.ProjectionState(ctx, scope, rebuild)
	} else {
		operation := "projection-status"
		if rebuild {
			operation = "projection-rebuild"
		}
		err = rt.callOwnedData(ctx, scope, operation, coordination.DataQuery{}, &result)
	}
	if err != nil {
		return result, err
	}
	if result.OwnerID != scope.ResourceOwnerID || result.RoleID != scope.RoleID || len(result.Layers) != 2 {
		return result, coordination.ErrWrongOwner
	}
	for i, layer := range result.Layers {
		if layer.Kind != []string{"vector", "graph"}[i] || layer.Total < 0 || layer.Current < 0 || layer.Current > layer.Total || layer.Pending != layer.Total-layer.Current || layer.Retrying < 0 || layer.Retrying > layer.Total || layer.NextAttemptAt < 0 {
			return result, coordination.ErrWrongOwner
		}
	}
	return result, rt.validateDataRoute(ctx, scope)
}
