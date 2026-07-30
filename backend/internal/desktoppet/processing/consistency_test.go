// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupConsistencyTestDB(t *testing.T) *gorm.DB {
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
	ctx := &app.AppContext{DB: db, Context: context.Background()}
	return NewRepository(db, ctx)
}

func seedGenTaskForConsistency(t *testing.T, db *gorm.DB, taskID string) {
	t.Helper()
	task := &desktoppet.GenerationTask{
		ID:     taskID,
		UserID: "u1",
		Name:   "一致性测试生成任务",
		Status: "succeeded",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create gen task: %v", err)
	}
}

func seedConsistencyProcessingTask(t *testing.T, db *gorm.DB, taskID, genTaskID, status, executionID string) *ProcessingTask {
	t.Helper()
	task := &ProcessingTask{
		ID:                taskID,
		GenerationTaskID:  genTaskID,
		ProcessingVersion: 1,
		Status:            status,
		CurrentStage:      status,
		ExecutionID:       executionID,
		OutputWidth:       512,
		OutputHeight:      512,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create processing task: %v", err)
	}
	return task
}

func seedConsistencyAction(t *testing.T, db *gorm.DB, actionID, ptTaskID, actionKey string, rowVersion int64) *ProcessingAction {
	t.Helper()
	action := &ProcessingAction{
		ID:                actionID,
		ProcessingTaskID:  ptTaskID,
		ActionKey:         actionKey,
		Status:            "pending",
		RowVersion:        rowVersion,
		ProcessingAttempt: 0,
	}
	if err := db.Create(action).Error; err != nil {
		t.Fatalf("create processing action: %v", err)
	}
	return action
}

func TestEnsureProcessingActions_Idempotent(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "queued", "")

	actions := []ProcessingAction{
		{ID: "pa-1", ProcessingTaskID: "pt-1", ActionKey: "idle_normal", Status: "pending"},
		{ID: "pa-2", ProcessingTaskID: "pt-1", ActionKey: "wave", Status: "pending"},
	}

	if err := repo.EnsureProcessingActions(db, actions); err != nil {
		t.Fatalf("first EnsureProcessingActions: %v", err)
	}

	if err := repo.EnsureProcessingActions(db, actions); err != nil {
		t.Fatalf("second EnsureProcessingActions: %v", err)
	}

	list, err := repo.ListProcessingActions("pt-1")
	if err != nil {
		t.Fatalf("ListProcessingActions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(list))
	}
}

func TestUpdateProcessingTaskOwned_OwnershipCheck(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "processing", "exec-1")

	ok, err := repo.UpdateProcessingTaskOwned("pt-1", "exec-1", map[string]interface{}{
		"progress": 50,
	})
	if err != nil {
		t.Fatalf("UpdateProcessingTaskOwned with correct exec: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct execution_id")
	}

	ok, err = repo.UpdateProcessingTaskOwned("pt-1", "exec-wrong", map[string]interface{}{
		"progress": 80,
	})
	if err != nil {
		t.Fatalf("UpdateProcessingTaskOwned with wrong exec: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false with wrong execution_id")
	}

	task, _ := repo.GetProcessingTask("pt-1")
	if task.Progress != 50 {
		t.Fatalf("expected progress=50, got %d", task.Progress)
	}
}

func TestRefreshProcessingLeaseOwned_OwnershipCheck(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "processing", "exec-1")

	future := nowStr()

	ok, err := repo.RefreshProcessingLeaseOwned("pt-1", "exec-1", future, future)
	if err != nil {
		t.Fatalf("RefreshProcessingLeaseOwned correct: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct execution_id")
	}

	ok, err = repo.RefreshProcessingLeaseOwned("pt-1", "exec-wrong", future, future)
	if err != nil {
		t.Fatalf("RefreshProcessingLeaseOwned wrong: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false with wrong execution_id")
	}
}

func TestRecoverExpiredProcessingTask_CAS(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")

	expired := "2020-01-01 00:00:00"
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "processing", "exec-1")
	db.Model(&ProcessingTask{}).Where("id = ?", "pt-1").Updates(map[string]interface{}{
		"lease_expires_at": expired,
	})

	now := nowStr()

	ok, err := repo.RecoverExpiredProcessingTask("pt-1", "exec-1", expired, now)
	if err != nil {
		t.Fatalf("RecoverExpiredProcessingTask correct: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true with correct exec+lease")
	}

	task, _ := repo.GetProcessingTask("pt-1")
	if task.Status != "queued" {
		t.Fatalf("expected status=queued, got %s", task.Status)
	}
	if task.ExecutionID != "" {
		t.Fatalf("expected execution_id cleared, got %s", task.ExecutionID)
	}

	ok, _ = repo.RecoverExpiredProcessingTask("pt-1", "exec-1", expired, now)
	if ok {
		t.Fatal("expected ok=false on second recovery (task already queued)")
	}
}

func TestBeginProcessingActionAttempt_OptimisticLock(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "processing", "exec-1")
	action := seedConsistencyAction(t, db, "pa-1", "pt-1", "idle_normal", 0)

	attempt, err := repo.BeginProcessingActionAttempt(db, action.ID, 0, "exec-1", 1)
	if err != nil {
		t.Fatalf("BeginProcessingActionAttempt first: %v", err)
	}
	if attempt.AttemptNumber != 1 {
		t.Fatalf("expected attempt_number=1, got %d", attempt.AttemptNumber)
	}
	if attempt.ExecutionID != "exec-1" {
		t.Fatalf("expected execution_id=exec-1, got %s", attempt.ExecutionID)
	}

	_, err = repo.BeginProcessingActionAttempt(db, action.ID, 0, "exec-2", 1)
	if err == nil {
		t.Fatal("expected error on stale row_version, got nil")
	}

	attempt2, err := repo.BeginProcessingActionAttempt(db, action.ID, 1, "exec-2", 1)
	if err != nil {
		t.Fatalf("BeginProcessingActionAttempt with correct row_version: %v", err)
	}
	if attempt2.AttemptNumber != 2 {
		t.Fatalf("expected attempt_number=2, got %d", attempt2.AttemptNumber)
	}

	updated, _ := repo.GetProcessingActionByActionKey("pt-1", "idle_normal")
	if updated.ProcessingAttempt != 2 {
		t.Fatalf("expected processing_attempt=2, got %d", updated.ProcessingAttempt)
	}
	if updated.RowVersion != 2 {
		t.Fatalf("expected row_version=2, got %d", updated.RowVersion)
	}
	if updated.ActiveExecutionID != "exec-2" {
		t.Fatalf("expected active_execution_id=exec-2, got %s", updated.ActiveExecutionID)
	}
}

func TestListProcessingActionAttempts_Ordered(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "processing", "exec-1")
	action := seedConsistencyAction(t, db, "pa-1", "pt-1", "idle_normal", 0)

	repo.BeginProcessingActionAttempt(db, action.ID, 0, "exec-1", 1)
	repo.BeginProcessingActionAttempt(db, action.ID, 1, "exec-1", 1)

	attempts, err := repo.ListProcessingActionAttempts(action.ID)
	if err != nil {
		t.Fatalf("ListProcessingActionAttempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attempts))
	}
	if attempts[0].AttemptNumber != 2 {
		t.Fatalf("expected first attempt_number=2 (DESC), got %d", attempts[0].AttemptNumber)
	}

	latest, err := repo.GetLatestProcessingActionAttempt(action.ID)
	if err != nil {
		t.Fatalf("GetLatestProcessingActionAttempt: %v", err)
	}
	if latest.AttemptNumber != 2 {
		t.Fatalf("expected latest attempt_number=2, got %d", latest.AttemptNumber)
	}
}

func TestUniqueConstraint_ProcessingActionActionKey(t *testing.T) {
	db := setupConsistencyTestDB(t)
	repo := newConsistencyRepo(t, db)
	seedGenTaskForConsistency(t, db, "gt-1")
	seedConsistencyProcessingTask(t, db, "pt-1", "gt-1", "queued", "")

	actions := []ProcessingAction{
		{ID: "pa-1", ProcessingTaskID: "pt-1", ActionKey: "idle_normal", Status: "pending"},
	}
	if err := repo.EnsureProcessingActions(db, actions); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	dup := []ProcessingAction{
		{ID: "pa-dup", ProcessingTaskID: "pt-1", ActionKey: "idle_normal", Status: "pending"},
	}
	err := repo.CreateProcessingActions(db, dup)
	if err == nil {
		t.Fatal("expected unique constraint violation, got nil")
	}
}
