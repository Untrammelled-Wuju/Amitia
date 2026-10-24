package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestLegacyContinuityBindingConflictDoesNotSelectAnExecutionOwner(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), p, continuityTestModel{})
	now := time.Now().UTC()
	for index := 0; index < 2; index++ {
		thread := continuity.Thread{ID: fmt.Sprint("conflicting-goal-", index), SpaceID: "core", CharacterID: "one", Title: "旧目标", Status: continuity.ThreadStatusActive, Confidence: 1, Revision: 1, CreatedAt: now, UpdatedAt: now, LastActiveAt: now}
		if err := p.services.DB.Create(&thread).Error; err != nil {
			t.Fatal(err)
		}
		binding := continuity.NewThreadBinding(thread.ID, "task_run", "legacy-task", "execution", "legacy", 1)
		if err := p.services.DB.Create(&binding).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateLegacyContinuity(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err == nil {
		t.Fatal("legacy conflict silently selected a goal")
	}
	var migrated int
	var legacy int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='continuity'`).Scan(&migrated); err != nil || migrated != 0 {
		t.Fatalf("conflicting migration partially committed: %d %v", migrated, err)
	}
	if err := p.services.DB.Table("continuity_thread_bindings").Count(&legacy).Error; err != nil || legacy != 2 {
		t.Fatalf("legacy evidence changed: %d %v", legacy, err)
	}
	if err := p.services.DB.Model(&continuity.ThreadBinding{}).Where("thread_id=?", "conflicting-goal-1").Update("role", "dependency").Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := migrateLegacyContinuity(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err != nil {
			t.Fatalf("reviewed execution/dependency migration failed: %v", err)
		}
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind='continuity' AND deleted=0`).Scan(&migrated); err != nil || migrated != 2 {
		t.Fatalf("migration lost goals: %d %v", migrated, err)
	}
}
