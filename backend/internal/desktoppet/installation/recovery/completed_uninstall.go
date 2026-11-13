package recovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"gorm.io/gorm"
)

func completedUninstallCandidates(db *gorm.DB) *gorm.DB {
	return db.Table("desktop_pet_installation_operations AS o").
		Joins("JOIN desktop_pet_installations i ON i.id = o.installation_id AND i.space_id = o.space_id AND i.device_id = o.device_id").
		Joins("JOIN desktop_pet_installation_runtime_projections p ON p.space_id = o.space_id AND p.device_id = o.device_id").
		Where("o.operation_type = ? AND o.status = ? AND o.stage = ? AND o.desired_revision > 0", operation.TypeUninstall, operation.OpStatusCompleted, operation.OpStageCompleted).
		Where("i.status = ? AND i.lifecycle_state = ? AND i.is_active = 0 AND i.desired_state = ? AND i.runtime_sync_state = ?",
			installation.StatusUninstalling, installation.LifecycleUninstalling, installation.DesiredDisabled, installation.SyncConfirmed).
		Where("p.applied_desired_revision >= o.desired_revision AND p.runtime_sync_state = ?", projection.SyncStateApplied).
		Where(`((p.actual_release_id = '' AND p.actual_visible = 0) OR
			(p.applied_desired_revision > o.desired_revision AND p.installation_id <> '' AND p.installation_id <> i.id AND p.actual_release_id <> '' AND p.actual_release_id <> i.current_release_id))`).
		Where(`NOT EXISTS (SELECT 1 FROM desktop_pet_device_active_installation_bindings b
			WHERE b.space_id = o.space_id AND b.device_id = o.device_id AND b.installation_id = i.id)`).
		Where(`NOT EXISTS (SELECT 1 FROM desktop_pet_runtime_desired_states d
			WHERE d.space_id = o.space_id AND d.device_id = o.device_id AND d.installation_id = i.id
			AND (d.desired_enabled <> 0 OR d.desired_visible <> 0))`)
}

func (f *ProductionRuntimeFinalizer) RecoverCompletedUninstalls(ctx context.Context, limit int) error {
	var ops []operation.InstallationOperation
	if err := completedUninstallCandidates(f.db.WithContext(ctx)).Select("o.*").Order("o.updated_at, o.id").Limit(limit).Find(&ops).Error; err != nil {
		return err
	}
	var result error
	for _, op := range ops {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if err := f.FinalizeRuntimeApplied(ctx, &op); err != nil {
			result = errors.Join(result, fmt.Errorf("recover completed uninstall %s: %w", op.ID, err))
		}
	}
	return result
}
