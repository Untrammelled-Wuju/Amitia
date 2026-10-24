package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"gorm.io/gorm"
)

func migrateLegacyContinuity(ctx context.Context, db *gorm.DB, engine *business.Engine, spaceID, deviceID, coreID string) error {
	if db == nil || engine == nil || strings.TrimSpace(spaceID) == "" || strings.TrimSpace(deviceID) == "" || strings.TrimSpace(coreID) == "" {
		return nil
	}
	if !db.Migrator().HasTable("continuity_threads") {
		return nil
	}
	if db.Migrator().HasTable("continuity_thread_bindings") {
		var conflict struct {
			BindingID string `gorm:"column:binding_id"`
		}
		result := db.WithContext(ctx).Table("continuity_thread_bindings AS binding").
			Select("binding.binding_id").
			Joins("JOIN continuity_threads thread ON thread.id=binding.thread_id").
			Where("binding.binding_type=? AND binding.role=?", "task_run", "execution").
			Group("binding.binding_id").Having("COUNT(DISTINCT binding.thread_id)>1").Limit(1).Scan(&conflict)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return fmt.Errorf("旧持续事项存在任务执行归属冲突，保留原记录并停止迁移，请先核对原始绑定")
		}
	}
	var threads []continuity.Thread
	if err := db.WithContext(ctx).Order("created_at ASC, id ASC").Find(&threads).Error; err != nil {
		return err
	}
	for _, thread := range threads {
		var bindings []continuity.ThreadBinding
		if db.Migrator().HasTable("continuity_thread_bindings") {
			if err := db.WithContext(ctx).Where("thread_id = ?", thread.ID).Order("created_at ASC, id ASC").Find(&bindings).Error; err != nil {
				return err
			}
		}
		var waits []continuity.Wait
		if db.Migrator().HasTable("continuity_waits") {
			if err := db.WithContext(ctx).Where("thread_id = ?", thread.ID).Order("created_at ASC, id ASC").Find(&waits).Error; err != nil {
				return err
			}
		}
		var events []continuity.ThreadEvent
		if db.Migrator().HasTable("continuity_thread_events") {
			if err := db.WithContext(ctx).Where("thread_id = ?", thread.ID).Order("occurred_at ASC, id ASC").Limit(256).Find(&events).Error; err != nil {
				return err
			}
		}
		document := business.OwnedContinuity{Thread: thread, Bindings: bindings, Waits: waits, Events: events}
		request := business.Request{
			SpaceID:        spaceID,
			DeviceID:       deviceID,
			TargetDeviceID: deviceID,
			CoreID:         coreID,
			RoleID:         thread.CharacterID,
			RequestID:      "continuity-migration/" + thread.ID,
		}
		if err := engine.ImportContinuity(ctx, request, document); err != nil {
			return fmt.Errorf("migrate continuity thread %s: %w", thread.ID, err)
		}
	}
	return nil
}
