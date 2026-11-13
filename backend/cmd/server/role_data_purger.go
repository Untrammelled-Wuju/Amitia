package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type roleDataPurger struct {
	services *AppServices
}

func (p roleDataPurger) PurgeRoleData(ctx context.Context, roleID string) error {
	if p.services == nil || p.services.KernelContainer == nil || p.services.KernelContainer.DeviceRegistry == nil {
		return errors.New("角色关联数据清理入口不可用")
	}
	if roleID == "" {
		return errors.New("角色标识为空，已取消关联数据清理")
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	result, err := db.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1, revision=revision+1, updated_at=? WHERE role_id=? AND deleted=0`, time.Now().UTC().Format(time.RFC3339Nano), roleID)
	if err != nil {
		return fmt.Errorf("清理角色关联数据失败: %w", err)
	}
	if _, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("读取角色关联数据清理结果失败: %w", err)
	}
	return nil
}

func (p roleDataPurger) HasRoleDataResidue(ctx context.Context, roleID string) (bool, error) {
	if p.services == nil || p.services.KernelContainer == nil || p.services.KernelContainer.DeviceRegistry == nil {
		return false, errors.New("角色关联数据清理入口不可用")
	}
	if roleID == "" {
		return false, errors.New("角色标识为空，无法检查关联数据")
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE role_id=? AND deleted=0`, roleID).Scan(&count); err != nil {
		return false, fmt.Errorf("检查角色关联数据残留失败: %w", err)
	}
	return count > 0, nil
}
