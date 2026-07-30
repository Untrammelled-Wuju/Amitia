// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupConsistencyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "consistency_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}

	migrations := migration.DefaultMigrations()
	if err := migration.MarkAllMigrationsApplied(db, migrations); err != nil {
		t.Fatalf("mark migrations applied: %v", err)
	}
	return db
}

func newConsistencyRepo(t *testing.T, db *gorm.DB) Repository {
	t.Helper()
	ctx := &app.AppContext{DB: db}
	return NewRepository(db, ctx)
}

func consistencyNowStr() string {
	return time.Now().Format(desktopPetTimeFormat)
}

func seedConsistencyGenTask(t *testing.T, db *gorm.DB, taskID, status, executionID string) *GenerationTask {
	t.Helper()
	task := &GenerationTask{
		ID:          taskID,
		UserID:      "u1",
		Name:        "一致性测试任务",
		Status:      status,
		ExecutionID: executionID,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create gen task: %v", err)
	}
	return task
}

func seedConsistencyTaskAction(t *testing.T, db *gorm.DB, actionID, taskID, actionKey string) *GenerationTaskAction {
	t.Helper()
	action := &GenerationTaskAction{
		ID:         actionID,
		TaskID:     taskID,
		ActionKey:  actionKey,
		Status:     "pending",
		SortOrder:  1,
		FrameCount: 4,
	}
	if err := db.Create(action).Error; err != nil {
		t.Fatalf("create task action: %v", err)
	}
	return action
}

func TestEnsureGenerationFrames_Idempotent(t *testing.T) {
	db := setupConsistencyDB(t)
	repo := newConsistencyRepo(t, db)
	seedConsistencyGenTask(t, db, "gt-1", "processing", "exec-1")
	action := seedConsistencyTaskAction(t, db, "gta-1", "gt-1", "idle_normal")

	frames := []GenerationFrame{
		{ID: "gf-1", TaskID: "gt-1", TaskActionID: action.ID, FrameIndex: 0, Status: "pending", GenerationAttempt: 1, ProviderAttempt: 0},
		{ID: "gf-2", TaskID: "gt-1", TaskActionID: action.ID, FrameIndex: 1, Status: "pending", GenerationAttempt: 1, ProviderAttempt: 0},
	}

	if err := repo.EnsureGenerationFrames(db, frames); err != nil {
		t.Fatalf("first EnsureGenerationFrames: %v", err)
	}

	if err := repo.EnsureGenerationFrames(db, frames); err != nil {
		t.Fatalf("second EnsureGenerationFrames: %v", err)
	}

	list, err := repo.ListFramesByAction(action.ID)
	if err != nil {
		t.Fatalf("ListFramesByAction: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(list))
	}
}

func TestUpdateTaskOwned_OwnershipCheck(t *testing.T) {
	db := setupConsistencyDB(t)
	repo := newConsistencyRepo(t, db)
	seedConsistencyGenTask(t, db, "gt-1", "processing", "exec-1")

	ok, err := repo.UpdateTaskOwned("gt-1", "exec-1", map[string]interface{}{
		"progress": 50,
	})
	if err != nil {
		t.Fatalf("UpdateTaskOwned correct exec: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct execution_id")
	}

	ok, err = repo.UpdateTaskOwned("gt-1", "exec-wrong", map[string]interface{}{
		"progress": 80,
	})
	if err != nil {
		t.Fatalf("UpdateTaskOwned wrong exec: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false with wrong execution_id")
	}

	task, _ := repo.GetTaskByID("gt-1")
	if task.Progress != 50 {
		t.Fatalf("expected progress=50, got %d", task.Progress)
	}
}

func TestRefreshLeaseOwned_OwnershipCheck(t *testing.T) {
	db := setupConsistencyDB(t)
	repo := newConsistencyRepo(t, db)
	seedConsistencyGenTask(t, db, "gt-1", "processing", "exec-1")

	future := consistencyNowStr()

	ok, err := repo.RefreshLeaseOwned("gt-1", "exec-1", future, future)
	if err != nil {
		t.Fatalf("RefreshLeaseOwned correct: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct execution_id")
	}

	ok, err = repo.RefreshLeaseOwned("gt-1", "exec-wrong", future, future)
	if err != nil {
		t.Fatalf("RefreshLeaseOwned wrong: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false with wrong execution_id")
	}
}

func TestRecoverExpiredTask_CAS(t *testing.T) {
	db := setupConsistencyDB(t)
	repo := newConsistencyRepo(t, db)

	expired := "2020-01-01 00:00:00"
	seedConsistencyGenTask(t, db, "gt-1", "processing", "exec-1")
	db.Model(&GenerationTask{}).Where("id = ?", "gt-1").Updates(map[string]interface{}{
		"lease_expires_at": expired,
	})

	now := consistencyNowStr()

	ok, err := repo.RecoverExpiredTask("gt-1", "exec-1", expired, now)
	if err != nil {
		t.Fatalf("RecoverExpiredTask correct: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct exec+lease")
	}

	task, _ := repo.GetTaskByID("gt-1")
	if task.Status != "queued" {
		t.Fatalf("expected status=queued, got %s", task.Status)
	}
	if task.ExecutionID != "" {
		t.Fatalf("expected execution_id cleared, got %s", task.ExecutionID)
	}

	ok, _ = repo.RecoverExpiredTask("gt-1", "exec-1", expired, now)
	if ok {
		t.Fatal("expected ok=false on second recovery")
	}
}

func TestUniqueConstraint_TaskActionActionKey(t *testing.T) {
	db := setupConsistencyDB(t)
	repo := newConsistencyRepo(t, db)
	seedConsistencyGenTask(t, db, "gt-1", "queued", "")

	actions := []GenerationTaskAction{
		{ID: "gta-1", TaskID: "gt-1", ActionKey: "idle_normal", Status: "pending", SortOrder: 1, FrameCount: 4},
	}
	if err := repo.CreateTaskActions(db, actions); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	dup := []GenerationTaskAction{
		{ID: "gta-dup", TaskID: "gt-1", ActionKey: "idle_normal", Status: "pending", SortOrder: 2, FrameCount: 4},
	}
	err := repo.CreateTaskActions(db, dup)
	if err == nil {
		t.Fatal("expected unique constraint violation, got nil")
	}
}
