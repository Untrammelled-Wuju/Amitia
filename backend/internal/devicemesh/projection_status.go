package devicemesh

import (
	"context"
	"errors"
	"fmt"

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
	if result.OwnerID != scope.ResourceOwnerID || result.RoleID != coordination.StorageRole(scope, "memory") || len(result.Layers) != 2 {
		return result, coordination.ErrWrongOwner
	}
	for i, layer := range result.Layers {
		if layer.Kind != []string{"vector", "graph"}[i] || layer.Total < 0 || layer.Current < 0 || layer.Current > layer.Total || layer.Pending != layer.Total-layer.Current || layer.Retrying < 0 || layer.Retrying > layer.Total || layer.NextAttemptAt < 0 {
			return result, coordination.ErrWrongOwner
		}
	}
	return result, rt.validateDataRoute(ctx, scope)
}

func (rt *Runtime) MemoryDeletionComplete(ctx context.Context, scope coordination.ExecutionScope, id string) (bool, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return false, err
	}
	var data coordination.DataPort
	if scope.Coordinated {
		data = rt.CoreDataPort
	} else if scope.TargetDeviceID == rt.LocalDeviceID {
		data = rt.LocalDeviceDataPort
	}
	if data != nil {
		port, ok := data.(interface {
			MemoryDeletionComplete(context.Context, coordination.ExecutionScope, string) (bool, error)
		})
		if !ok {
			return false, errors.New("数据所有者的记忆删除状态入口不可用")
		}
		complete, err := port.MemoryDeletionComplete(ctx, scope, id)
		if err != nil {
			return false, err
		}
		return complete, rt.validateDataRoute(ctx, scope)
	}
	var result struct {
		OwnerID  string `json:"ownerId"`
		RoleID   string `json:"roleId"`
		Complete bool   `json:"complete"`
	}
	if err := rt.callOwnedData(ctx, scope, "memory-deletion-status", coordination.DataQuery{ResourceID: id}, &result); err != nil {
		return false, err
	}
	if result.OwnerID != scope.ResourceOwnerID || result.RoleID != coordination.StorageRole(scope, "memory") {
		return false, coordination.ErrWrongOwner
	}
	return result.Complete, rt.validateDataRoute(ctx, scope)
}

func (rt *Runtime) CleanupMemoryDeletions(ctx context.Context, scope coordination.ExecutionScope) (int, error) {
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return 0, fmt.Errorf("清理路由校验失败: %w", err)
	}
	var data coordination.DataPort
	if scope.Coordinated {
		data = rt.CoreDataPort
	} else if scope.TargetDeviceID == rt.LocalDeviceID {
		data = rt.LocalDeviceDataPort
	}
	if data != nil {
		port, ok := data.(interface {
			CleanupMemoryDeletions(context.Context, coordination.ExecutionScope) (int, error)
		})
		if !ok {
			return 0, errors.New("数据所有者的记忆清理入口不可用")
		}
		count, err := port.CleanupMemoryDeletions(ctx, scope)
		if err != nil {
			return count, fmt.Errorf("本地清理失败: %w", err)
		}
		return count, rt.validateDataRoute(ctx, scope)
	}
	var result struct {
		OwnerID string `json:"ownerId"`
		Cleaned int    `json:"cleaned"`
	}
	if err := rt.callOwnedData(ctx, scope, "memory-deletion-cleanup", coordination.DataQuery{}, &result); err != nil {
		return 0, fmt.Errorf("远端清理失败: %w", err)
	}
	if result.OwnerID != scope.ResourceOwnerID || result.Cleaned < 0 {
		return 0, coordination.ErrWrongOwner
	}
	return result.Cleaned, rt.validateDataRoute(ctx, scope)
}
