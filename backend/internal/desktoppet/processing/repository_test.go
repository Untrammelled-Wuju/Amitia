// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "processing_test.db")
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

	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(migration.DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func newRepoFromDB(t *testing.T, db *gorm.DB) Repository {
	t.Helper()
	ctx := &app.AppContext{DB: db, Context: context.Background()}
	return NewRepository(db, ctx)
}

func nowStr() string {
	return time.Now().Format(desktopPetTimeFormat)
}

func TestCreateAndGetProcessingTask(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	task := &ProcessingTask{
		ID:                         "pt-1",
		GenerationTaskID:           "gt-1",
		ProcessingVersion:          1,
		Status:                     "queued",
		CurrentStage:               "queued",
		OutputWidth:                512,
		OutputHeight:               512,
		TargetCharacterHeightRatio: 0.8,
		AnchorMode:                 "feet_center",
		BackgroundMode:             "remove_background",
		OutputFormat:               "png",
		DefaultFPS:                 10,
	}

	if err := repo.CreateProcessingTask(db, task); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	got, err := repo.GetProcessingTask("pt-1")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.ID != "pt-1" {
		t.Fatalf("ID = %s, want pt-1", got.ID)
	}
	if got.GenerationTaskID != "gt-1" {
		t.Fatalf("GenerationTaskID = %s, want gt-1", got.GenerationTaskID)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued", got.Status)
	}
	if got.TargetCharacterHeightRatio != 0.8 {
		t.Fatalf("TargetCharacterHeightRatio = %v, want 0.8", got.TargetCharacterHeightRatio)
	}
}

func TestListProcessingTasksByGenerationTask(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	for i, id := range []string{"pt-a", "pt-b", "pt-c"} {
		if err := repo.CreateProcessingTask(db, &ProcessingTask{
			ID:                id,
			GenerationTaskID:  "gt-shared",
			ProcessingVersion: i + 1,
			Status:            "queued",
		}); err != nil {
			t.Fatalf("CreateProcessingTask[%d]: %v", i, err)
		}
	}

	tasks, err := repo.ListProcessingTasksByGenerationTask("gt-shared")
	if err != nil {
		t.Fatalf("ListProcessingTasksByGenerationTask: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3", len(tasks))
	}

	other, err := repo.ListProcessingTasksByGenerationTask("gt-other")
	if err != nil {
		t.Fatalf("ListProcessingTasksByGenerationTask other: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("len(other) = %d, want 0", len(other))
	}
}

func TestClaimProcessingTaskAtomicity(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID:                "pt-claim",
		GenerationTaskID:  "gt-1",
		ProcessingVersion: 1,
		Status:            "queued",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	leaseExpires := time.Now().Add(30 * time.Second).Format(desktopPetTimeFormat)
	ok, err := repo.ClaimProcessingTask("pt-claim", "worker-1", "processing-exec-1", leaseExpires)
	if err != nil {
		t.Fatalf("ClaimProcessingTask first: %v", err)
	}
	if !ok {
		t.Fatal("first claim should succeed")
	}

	got, err := repo.GetProcessingTask("pt-claim")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.Status != "processing" {
		t.Fatalf("Status = %s, want processing", got.Status)
	}
	if got.WorkerID != "worker-1" {
		t.Fatalf("WorkerID = %s, want worker-1", got.WorkerID)
	}
	if got.ExecutionID != "processing-exec-1" {
		t.Fatalf("ExecutionID = %s, want processing-exec-1", got.ExecutionID)
	}
	if got.CurrentStage != "validating_sources" {
		t.Fatalf("CurrentStage = %s, want validating_sources", got.CurrentStage)
	}
	if got.LeaseExpiresAt == "" {
		t.Fatal("LeaseExpiresAt should be set")
	}
	if got.LastHeartbeatAt == "" {
		t.Fatal("LastHeartbeatAt should be set")
	}

	ok2, err := repo.ClaimProcessingTask("pt-claim", "worker-2", "processing-exec-2", leaseExpires)
	if err != nil {
		t.Fatalf("ClaimProcessingTask second: %v", err)
	}
	if ok2 {
		t.Fatal("second claim should fail")
	}

	got2, err := repo.GetProcessingTask("pt-claim")
	if err != nil {
		t.Fatalf("GetProcessingTask after second claim: %v", err)
	}
	if got2.WorkerID != "worker-1" {
		t.Fatalf("WorkerID = %s, want worker-1 (unchanged)", got2.WorkerID)
	}
	if got2.ExecutionID != "processing-exec-1" {
		t.Fatalf("ExecutionID = %s, want processing-exec-1 (unchanged)", got2.ExecutionID)
	}
}

func TestClaimProcessingTask_NotQueuedFails(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID:                "pt-notqueued",
		GenerationTaskID:  "gt-1",
		ProcessingVersion: 1,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	leaseExpires := time.Now().Add(30 * time.Second).Format(desktopPetTimeFormat)
	ok, err := repo.ClaimProcessingTask("pt-notqueued", "worker-1", "processing-exec-1", leaseExpires)
	if err != nil {
		t.Fatalf("ClaimProcessingTask: %v", err)
	}
	if ok {
		t.Fatal("claim should fail for non-queued task")
	}
}

func TestListRecoverableProcessingTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	now := time.Now()
	pastStr := now.Add(-1 * time.Minute).Format(desktopPetTimeFormat)
	futureStr := now.Add(10 * time.Minute).Format(desktopPetTimeFormat)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID:               "pt-recover",
		GenerationTaskID: "gt-recover-1",
		Status:           "processing",
		LeaseExpiresAt:   pastStr,
	}); err != nil {
		t.Fatalf("CreateProcessingTask recover: %v", err)
	}
	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID:               "pt-active",
		GenerationTaskID: "gt-active-1",
		Status:           "processing",
		LeaseExpiresAt:   futureStr,
	}); err != nil {
		t.Fatalf("CreateProcessingTask active: %v", err)
	}
	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID:               "pt-queued",
		GenerationTaskID: "gt-queued-1",
		Status:           "queued",
		LeaseExpiresAt:   pastStr,
	}); err != nil {
		t.Fatalf("CreateProcessingTask queued: %v", err)
	}

	tasks, err := repo.ListRecoverableProcessingTasks()
	if err != nil {
		t.Fatalf("ListRecoverableProcessingTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].ID != "pt-recover" {
		t.Fatalf("tasks[0].ID = %s, want pt-recover", tasks[0].ID)
	}
}

func TestListQueuedProcessingTasks(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-q1", GenerationTaskID: "gt-q1", Status: "queued",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}
	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-q2", GenerationTaskID: "gt-q2", Status: "queued",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}
	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-p1", GenerationTaskID: "gt-p1", Status: "processing",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	tasks, err := repo.ListQueuedProcessingTasks()
	if err != nil {
		t.Fatalf("ListQueuedProcessingTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
}

func TestUpdateProcessingTaskStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-upd", GenerationTaskID: "gt-1", Status: "queued",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	if err := repo.UpdateProcessingTaskStatusNoTx("pt-upd", map[string]interface{}{
		"status":        "succeeded",
		"progress":      100,
		"current_stage": "completed",
	}); err != nil {
		t.Fatalf("UpdateProcessingTaskStatusNoTx: %v", err)
	}

	got, err := repo.GetProcessingTask("pt-upd")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.Status != "succeeded" {
		t.Fatalf("Status = %s, want succeeded", got.Status)
	}
	if got.Progress != 100 {
		t.Fatalf("Progress = %d, want 100", got.Progress)
	}
	if got.CurrentStage != "completed" {
		t.Fatalf("CurrentStage = %s, want completed", got.CurrentStage)
	}
}

func TestUpdateProcessingHeartbeatAndLease(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-hb", GenerationTaskID: "gt-1", Status: "processing",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	now := nowStr()
	if err := repo.UpdateProcessingHeartbeat("pt-hb", now); err != nil {
		t.Fatalf("UpdateProcessingHeartbeat: %v", err)
	}

	got, err := repo.GetProcessingTask("pt-hb")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.LastHeartbeatAt != now {
		t.Fatalf("LastHeartbeatAt = %s, want %s", got.LastHeartbeatAt, now)
	}

	lease := time.Now().Add(60 * time.Second).Format(desktopPetTimeFormat)
	if err := repo.RefreshProcessingLease("pt-hb", lease, now); err != nil {
		t.Fatalf("RefreshProcessingLease: %v", err)
	}

	got, err = repo.GetProcessingTask("pt-hb")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.LeaseExpiresAt != lease {
		t.Fatalf("LeaseExpiresAt = %s, want %s", got.LeaseExpiresAt, lease)
	}
}

func TestSetProcessingCancelRequested(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-cancel", GenerationTaskID: "gt-1", Status: "processing",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	now := nowStr()
	if err := repo.SetProcessingCancelRequested("pt-cancel", now); err != nil {
		t.Fatalf("SetProcessingCancelRequested: %v", err)
	}

	got, err := repo.GetProcessingTask("pt-cancel")
	if err != nil {
		t.Fatalf("GetProcessingTask: %v", err)
	}
	if got.CancelRequestedAt != now {
		t.Fatalf("CancelRequestedAt = %s, want %s", got.CancelRequestedAt, now)
	}
}

func TestGetProcessingTaskForUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingTask(db, &ProcessingTask{
		ID: "pt-fu", GenerationTaskID: "gt-1", Status: "queued",
	}); err != nil {
		t.Fatalf("CreateProcessingTask: %v", err)
	}

	got, err := repo.GetProcessingTaskForUpdate(db, "pt-fu")
	if err != nil {
		t.Fatalf("GetProcessingTaskForUpdate: %v", err)
	}
	if got.ID != "pt-fu" {
		t.Fatalf("ID = %s, want pt-fu", got.ID)
	}
}

func TestCreateAndListProcessingActions(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	actions := []ProcessingAction{
		{
			ID:                     "pa-1",
			ProcessingTaskID:       "pt-1",
			GenerationTaskActionID: "gta-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "待机",
			SourceAttemptNumber:    1,
			Status:                 "pending",
			FPS:                    10,
			FrameDurationMS:        100,
			AnchorX:                0.5,
			AnchorY:                0.92,
			Excluded:               0,
		},
		{
			ID:                     "pa-2",
			ProcessingTaskID:       "pt-1",
			GenerationTaskActionID: "gta-2",
			ActionKey:              "walk_left",
			ActionNameSnapshot:     "左移",
			SourceAttemptNumber:    1,
			Status:                 "pending",
			FPS:                    10,
			FrameDurationMS:        100,
			AnchorX:                0.5,
			AnchorY:                0.92,
			Excluded:               0,
		},
	}

	if err := repo.CreateProcessingActions(db, actions); err != nil {
		t.Fatalf("CreateProcessingActions: %v", err)
	}

	got, err := repo.ListProcessingActions("pt-1")
	if err != nil {
		t.Fatalf("ListProcessingActions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	ordered, err := repo.ListProcessingActionsOrdered("pt-1")
	if err != nil {
		t.Fatalf("ListProcessingActionsOrdered: %v", err)
	}
	if len(ordered) != 2 {
		t.Fatalf("len(ordered) = %d, want 2", len(ordered))
	}
}

func TestGetProcessingActionByActionKey(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingActions(db, []ProcessingAction{
		{
			ID:               "pa-1",
			ProcessingTaskID: "pt-1",
			ActionKey:        "idle_normal",
			Status:           "pending",
			AnchorX:          0.5,
			AnchorY:          0.92,
		},
	}); err != nil {
		t.Fatalf("CreateProcessingActions: %v", err)
	}

	got, err := repo.GetProcessingActionByActionKey("pt-1", "idle_normal")
	if err != nil {
		t.Fatalf("GetProcessingActionByActionKey: %v", err)
	}
	if got.ID != "pa-1" {
		t.Fatalf("ID = %s, want pa-1", got.ID)
	}
	if got.ActionKey != "idle_normal" {
		t.Fatalf("ActionKey = %s, want idle_normal", got.ActionKey)
	}
}

func TestUpdateProcessingAction(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingActions(db, []ProcessingAction{
		{ID: "pa-1", ProcessingTaskID: "pt-1", ActionKey: "idle", Status: "pending", AnchorX: 0.5, AnchorY: 0.92},
	}); err != nil {
		t.Fatalf("CreateProcessingActions: %v", err)
	}

	if err := repo.UpdateProcessingActionNoTx("pa-1", map[string]interface{}{
		"status":   "succeeded",
		"progress": 100,
	}); err != nil {
		t.Fatalf("UpdateProcessingActionNoTx: %v", err)
	}

	var got ProcessingAction
	if err := db.Where("id = ?", "pa-1").First(&got).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if got.Status != "succeeded" {
		t.Fatalf("Status = %s, want succeeded", got.Status)
	}
	if got.Progress != 100 {
		t.Fatalf("Progress = %d, want 100", got.Progress)
	}
}

func TestUpdateProcessingActionAttempt(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingActions(db, []ProcessingAction{
		{ID: "pa-1", ProcessingTaskID: "pt-1", ActionKey: "idle", Status: "pending", SourceAttemptNumber: 1, AnchorX: 0.5, AnchorY: 0.92},
	}); err != nil {
		t.Fatalf("CreateProcessingActions: %v", err)
	}

	if err := repo.UpdateProcessingActionAttempt(db, "pa-1", 3); err != nil {
		t.Fatalf("UpdateProcessingActionAttempt: %v", err)
	}

	var got ProcessingAction
	if err := db.Where("id = ?", "pa-1").First(&got).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if got.SourceAttemptNumber != 3 {
		t.Fatalf("SourceAttemptNumber = %d, want 3", got.SourceAttemptNumber)
	}
}

func TestSetActionExcluded(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingActions(db, []ProcessingAction{
		{ID: "pa-1", ProcessingTaskID: "pt-1", ActionKey: "idle", Status: "pending", Excluded: 0, AnchorX: 0.5, AnchorY: 0.92},
	}); err != nil {
		t.Fatalf("CreateProcessingActions: %v", err)
	}

	if err := repo.SetActionExcluded("pa-1", true); err != nil {
		t.Fatalf("SetActionExcluded true: %v", err)
	}

	var got ProcessingAction
	if err := db.Where("id = ?", "pa-1").First(&got).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if got.Excluded != 1 {
		t.Fatalf("Excluded = %d, want 1", got.Excluded)
	}

	if err := repo.SetActionExcluded("pa-1", false); err != nil {
		t.Fatalf("SetActionExcluded false: %v", err)
	}

	if err := db.Where("id = ?", "pa-1").First(&got).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if got.Excluded != 0 {
		t.Fatalf("Excluded = %d, want 0", got.Excluded)
	}
}

func TestCreateAndListProcessedFrames(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	frames := []ProcessedFrame{
		{
			ID:                 "pf-1",
			ProcessingActionID: "pa-1",
			SourceFrameID:      "gf-1",
			FrameIndex:         0,
			Status:             "pending",
		},
		{
			ID:                 "pf-2",
			ProcessingActionID: "pa-1",
			SourceFrameID:      "gf-2",
			FrameIndex:         1,
			Status:             "pending",
		},
	}

	if err := repo.CreateProcessedFrames(db, frames); err != nil {
		t.Fatalf("CreateProcessedFrames: %v", err)
	}

	got, err := repo.ListProcessedFramesByAction("pa-1")
	if err != nil {
		t.Fatalf("ListProcessedFramesByAction: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].FrameIndex != 0 {
		t.Fatalf("got[0].FrameIndex = %d, want 0", got[0].FrameIndex)
	}
	if got[1].FrameIndex != 1 {
		t.Fatalf("got[1].FrameIndex = %d, want 1", got[1].FrameIndex)
	}
}

func TestUpdateProcessedFrame(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessedFrames(db, []ProcessedFrame{
		{ID: "pf-1", ProcessingActionID: "pa-1", FrameIndex: 0, Status: "pending"},
	}); err != nil {
		t.Fatalf("CreateProcessedFrames: %v", err)
	}

	if err := repo.UpdateProcessedFrameNoTx("pf-1", map[string]interface{}{
		"status":         "succeeded",
		"processed_path": "/path/to/processed.png",
		"width":          512,
		"height":         512,
	}); err != nil {
		t.Fatalf("UpdateProcessedFrameNoTx: %v", err)
	}

	var got ProcessedFrame
	if err := db.Where("id = ?", "pf-1").First(&got).Error; err != nil {
		t.Fatalf("query frame: %v", err)
	}
	if got.Status != "succeeded" {
		t.Fatalf("Status = %s, want succeeded", got.Status)
	}
	if got.ProcessedPath != "/path/to/processed.png" {
		t.Fatalf("ProcessedPath = %s, want /path/to/processed.png", got.ProcessedPath)
	}
	if got.Width != 512 || got.Height != 512 {
		t.Fatalf("Width=%d Height=%d, want 512x512", got.Width, got.Height)
	}
}

func TestCreateAndGetPackage(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	pkg := &Package{
		ID:               "pkg-1",
		UserID:           "user-1",
		CharacterID:      "char-1",
		GenerationTaskID: "gt-1",
		ProcessingTaskID: "pt-1",
		Name:             "测试包",
		Version:          1,
		Status:           "draft",
		CanvasWidth:      512,
		CanvasHeight:     512,
		ActionCount:      3,
		IncludedActions:  "[]",
	}

	if err := repo.CreatePackage(pkg); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	got, err := repo.GetPackage("pkg-1")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if got.ID != "pkg-1" {
		t.Fatalf("ID = %s, want pkg-1", got.ID)
	}
	if got.Name != "测试包" {
		t.Fatalf("Name = %s, want 测试包", got.Name)
	}
	if got.Version != 1 {
		t.Fatalf("Version = %d, want 1", got.Version)
	}
}

func TestUpdatePackageStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreatePackage(&Package{
		ID: "pkg-1", UserID: "user-1", Status: "draft", Version: 1,
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	if err := repo.UpdatePackageStatus("pkg-1", map[string]interface{}{
		"status":       "published",
		"package_path": "/path/to/pkg.zip",
		"package_hash": "abc123",
	}); err != nil {
		t.Fatalf("UpdatePackageStatus: %v", err)
	}

	got, err := repo.GetPackage("pkg-1")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if got.Status != "published" {
		t.Fatalf("Status = %s, want published", got.Status)
	}
	if got.PackagePath != "/path/to/pkg.zip" {
		t.Fatalf("PackagePath = %s, want /path/to/pkg.zip", got.PackagePath)
	}
	if got.PackageHash != "abc123" {
		t.Fatalf("PackageHash = %s, want abc123", got.PackageHash)
	}
}

func TestListPackagesByUser(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	for i, id := range []string{"pkg-1", "pkg-2", "pkg-3"} {
		if err := repo.CreatePackage(&Package{
			ID: id, UserID: "user-1", ProcessingTaskID: id, Status: "draft", Version: 1,
		}); err != nil {
			t.Fatalf("CreatePackage[%d]: %v", i, err)
		}
	}
	if err := repo.CreatePackage(&Package{
		ID: "pkg-other", UserID: "user-2", ProcessingTaskID: "pt-other", Status: "draft", Version: 1,
	}); err != nil {
		t.Fatalf("CreatePackage other: %v", err)
	}

	packages, total, err := repo.ListPackagesByUser("user-1", 1, 10)
	if err != nil {
		t.Fatalf("ListPackagesByUser: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(packages) != 3 {
		t.Fatalf("len(packages) = %d, want 3", len(packages))
	}

	packages2, total2, err := repo.ListPackagesByUser("user-1", 1, 2)
	if err != nil {
		t.Fatalf("ListPackagesByUser page: %v", err)
	}
	if total2 != 3 {
		t.Fatalf("total2 = %d, want 3", total2)
	}
	if len(packages2) != 2 {
		t.Fatalf("len(packages2) = %d, want 2", len(packages2))
	}
}

func TestListPackagesByGenerationTask(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreatePackage(&Package{
		ID: "pkg-1", UserID: "user-1", GenerationTaskID: "gt-1", ProcessingTaskID: "pt-1", Status: "draft", Version: 1,
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if err := repo.CreatePackage(&Package{
		ID: "pkg-2", UserID: "user-1", GenerationTaskID: "gt-1", ProcessingTaskID: "pt-2", Status: "draft", Version: 1,
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if err := repo.CreatePackage(&Package{
		ID: "pkg-3", UserID: "user-1", GenerationTaskID: "gt-2", ProcessingTaskID: "pt-3", Status: "draft", Version: 1,
	}); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	packages, err := repo.ListPackagesByGenerationTask("gt-1")
	if err != nil {
		t.Fatalf("ListPackagesByGenerationTask: %v", err)
	}
	if len(packages) != 2 {
		t.Fatalf("len(packages) = %d, want 2", len(packages))
	}
}

func TestListSucceededActions(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := db.Create(&desktoppet.GenerationTaskAction{
		ID:        "gta-1",
		TaskID:    "gt-1",
		ActionKey: "idle_normal",
		Status:    "succeeded",
		SortOrder: 1,
	}).Error; err != nil {
		t.Fatalf("create gta-1: %v", err)
	}
	if err := db.Create(&desktoppet.GenerationTaskAction{
		ID:        "gta-2",
		TaskID:    "gt-1",
		ActionKey: "walk_left",
		Status:    "succeeded",
		SortOrder: 2,
	}).Error; err != nil {
		t.Fatalf("create gta-2: %v", err)
	}
	if err := db.Create(&desktoppet.GenerationTaskAction{
		ID:        "gta-3",
		TaskID:    "gt-1",
		ActionKey: "walk_right",
		Status:    "failed",
		SortOrder: 3,
	}).Error; err != nil {
		t.Fatalf("create gta-3: %v", err)
	}
	if err := db.Create(&desktoppet.GenerationTaskAction{
		ID:        "gta-4",
		TaskID:    "gt-1",
		ActionKey: "jump",
		Status:    "pending",
		SortOrder: 4,
	}).Error; err != nil {
		t.Fatalf("create gta-4: %v", err)
	}

	actions, err := repo.ListSucceededActions("gt-1")
	if err != nil {
		t.Fatalf("ListSucceededActions: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("len(actions) = %d, want 2", len(actions))
	}
	if actions[0].ID != "gta-1" {
		t.Fatalf("actions[0].ID = %s, want gta-1", actions[0].ID)
	}
	if actions[1].ID != "gta-2" {
		t.Fatalf("actions[1].ID = %s, want gta-2", actions[1].ID)
	}
	for _, a := range actions {
		if a.Status != "succeeded" {
			t.Fatalf("action %s status = %s, want succeeded", a.ID, a.Status)
		}
	}
}

func TestGetGenerationTask(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := db.Create(&desktoppet.GenerationTask{
		ID:           "gt-1",
		UserID:       "user-1",
		Name:         "生成任务",
		Status:       "succeeded",
		CurrentStage: "completed",
	}).Error; err != nil {
		t.Fatalf("create generation task: %v", err)
	}

	got, err := repo.GetGenerationTask("gt-1")
	if err != nil {
		t.Fatalf("GetGenerationTask: %v", err)
	}
	if got.ID != "gt-1" {
		t.Fatalf("ID = %s, want gt-1", got.ID)
	}
	if got.Name != "生成任务" {
		t.Fatalf("Name = %s, want 生成任务", got.Name)
	}
}

func TestListActionsByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	for i, id := range []string{"gta-1", "gta-2"} {
		if err := db.Create(&desktoppet.GenerationTaskAction{
			ID:        id,
			TaskID:    "gt-1",
			ActionKey: id,
			Status:    "succeeded",
			SortOrder: i + 1,
		}).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	actions, err := repo.ListActionsByTaskID("gt-1")
	if err != nil {
		t.Fatalf("ListActionsByTaskID: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("len(actions) = %d, want 2", len(actions))
	}
}

func TestListFramesByAction(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	for i, id := range []string{"gf-1", "gf-2"} {
		if err := db.Create(&desktoppet.GenerationFrame{
			ID:           id,
			TaskID:       "gt-1",
			TaskActionID: "gta-1",
			FrameIndex:   i,
			Status:       "succeeded",
		}).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	frames, err := repo.ListFramesByAction("gta-1")
	if err != nil {
		t.Fatalf("ListFramesByAction: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("len(frames) = %d, want 2", len(frames))
	}
	if frames[0].FrameIndex != 0 {
		t.Fatalf("frames[0].FrameIndex = %d, want 0", frames[0].FrameIndex)
	}
}

func TestCreateProcessingActions_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessingActions(db, []ProcessingAction{}); err != nil {
		t.Fatalf("CreateProcessingActions empty: %v", err)
	}
	if err := repo.CreateProcessingActions(db, nil); err != nil {
		t.Fatalf("CreateProcessingActions nil: %v", err)
	}
}

func TestCreateProcessedFrames_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if err := repo.CreateProcessedFrames(db, []ProcessedFrame{}); err != nil {
		t.Fatalf("CreateProcessedFrames empty: %v", err)
	}
	if err := repo.CreateProcessedFrames(db, nil); err != nil {
		t.Fatalf("CreateProcessedFrames nil: %v", err)
	}
}

func TestDB(t *testing.T) {
	db := setupTestDB(t)
	repo := newRepoFromDB(t, db)

	if repo.DB() != db {
		t.Fatal("DB() should return the underlying db")
	}
}
