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
