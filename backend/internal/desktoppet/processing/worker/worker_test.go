// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "worker_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	sqlPath := filepath.Join("..", "..", "..", "..", "data", "sql.sql")
	if err := migration.ApplyInitialSQLFile(db, sqlPath); err != nil {
		t.Fatalf("apply initial sql: %v", err)
	}

	runner := migration.Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(migration.DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func newWorkerRepo(t *testing.T, db *gorm.DB) processing.Repository {
	t.Helper()
	ctx := &app.AppContext{DB: db, Context: context.Background()}
	return processing.NewRepository(db, ctx)
}

func workerNowStr() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func seedWorkerGenerationTask(t *testing.T, db *gorm.DB, taskID, userID, status string) *desktoppet.GenerationTask {
	t.Helper()
	task := &desktoppet.GenerationTask{
		ID:     taskID,
		UserID: userID,
		Name:   "处理测试任务",
		Status: status,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create generation task %s: %v", taskID, err)
	}
	return task
}

func seedWorkerAction(t *testing.T, db *gorm.DB, actionID, taskID, actionKey, status string, frameCount int) desktoppet.GenerationTaskAction {
	t.Helper()
	action := desktoppet.GenerationTaskAction{
		ID:                    actionID,
		TaskID:                taskID,
		ActionKey:             actionKey,
		ActionNameSnapshot:    actionKey,
		Status:                status,
		CurrentAttempt:        1,
		FrameCount:            frameCount,
		GenerationSpecVersion: "v1",
		SortOrder:             1,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action %s: %v", actionID, err)
	}
	return action
}

func seedWorkerFrame(t *testing.T, db *gorm.DB, frameID, taskID, taskActionID, resultImagePath, resultHash string, frameIndex, attemptNumber int, status string) {
	t.Helper()
	frame := desktoppet.GenerationFrame{
		ID:              frameID,
		TaskID:          taskID,
		TaskActionID:    taskActionID,
		FrameIndex:      frameIndex,
		AttemptNumber:   attemptNumber,
		Status:          status,
		ResultImagePath: resultImagePath,
		ResultHash:      resultHash,
	}
	if err := db.Create(&frame).Error; err != nil {
		t.Fatalf("create frame %s: %v", frameID, err)
	}
}

func writeWorkerPNGWithContent(t *testing.T, dataDir, relPath string, width, height, frameIndex int) string {
	t.Helper()
	absPath := filepath.Join(dataDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", absPath, err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	r := 255
	if frameIndex%2 == 1 {
		r = 200
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x >= width/4 && x < width*3/4 && y >= height/4 && y < height*3/4 {
				img.Set(x, y, color.NRGBA{R: uint8(r), G: 100, B: 100, A: 255})
			} else {
				img.Set(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(absPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write file %s: %v", absPath, err)
	}
	return absPath
}

func fileSHA256Hex(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func createProcessingTask(t *testing.T, repo processing.Repository, db *gorm.DB, task *processing.ProcessingTask) {
	t.Helper()
	if err := repo.CreateProcessingTask(db, task); err != nil {
		t.Fatalf("create processing task %s: %v", task.ID, err)
	}
}

func TestClaimProcessingTaskAtomicity(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:                "pt-claim",
		GenerationTaskID:  "gt-1",
		ProcessingVersion: 1,
		Status:            "queued",
	})

	leaseExpires := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
	ok, err := repo.ClaimProcessingTask("pt-claim", "worker-1", "processing-exec-1", leaseExpires)
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if !ok {
		t.Fatal("first claim should succeed")
	}

	got, err := repo.GetProcessingTask("pt-claim")
	if err != nil {
		t.Fatalf("get task: %v", err)
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

	ok2, err := repo.ClaimProcessingTask("pt-claim", "worker-2", "processing-exec-2", leaseExpires)
	if err != nil {
		t.Fatalf("second claim failed: %v", err)
	}
	if ok2 {
		t.Fatal("second claim should fail")
	}

	got2, err := repo.GetProcessingTask("pt-claim")
	if err != nil {
		t.Fatalf("get task after second claim: %v", err)
	}
	if got2.WorkerID != "worker-1" {
		t.Fatalf("WorkerID = %s, want worker-1 (unchanged)", got2.WorkerID)
	}
	if got2.ExecutionID != "processing-exec-1" {
		t.Fatalf("ExecutionID = %s, want processing-exec-1 (unchanged)", got2.ExecutionID)
	}

	_ = w
}

func TestHeartbeatUpdate(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:               "pt-hb",
		GenerationTaskID: "gt-1",
		Status:           "processing",
	})

	now := workerNowStr()
	if err := repo.UpdateProcessingHeartbeat("pt-hb", now); err != nil {
		t.Fatalf("UpdateProcessingHeartbeat: %v", err)
	}

	got, err := repo.GetProcessingTask("pt-hb")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.LastHeartbeatAt != now {
		t.Fatalf("LastHeartbeatAt = %s, want %s", got.LastHeartbeatAt, now)
	}

	lease := time.Now().Add(60 * time.Second).Format("2006-01-02 15:04:05")
	if err := repo.RefreshProcessingLease("pt-hb", lease, now); err != nil {
		t.Fatalf("RefreshProcessingLease: %v", err)
	}

	got, err = repo.GetProcessingTask("pt-hb")
	if err != nil {
		t.Fatalf("get task after refresh: %v", err)
	}
	if got.LeaseExpiresAt != lease {
		t.Fatalf("LeaseExpiresAt = %s, want %s", got.LeaseExpiresAt, lease)
	}

	_ = w
}

func TestListRecoverableProcessingTasks(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	now := time.Now()
	pastStr := now.Add(-1 * time.Minute).Format("2006-01-02 15:04:05")
	futureStr := now.Add(10 * time.Minute).Format("2006-01-02 15:04:05")

	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:               "pt-recover",
		GenerationTaskID: "gt-1",
		Status:           "processing",
		LeaseExpiresAt:   pastStr,
	})
	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:               "pt-active",
		GenerationTaskID: "gt-1",
		Status:           "processing",
		LeaseExpiresAt:   futureStr,
	})
	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:               "pt-queued",
		GenerationTaskID: "gt-1",
		Status:           "queued",
		LeaseExpiresAt:   pastStr,
	})

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

	_ = w
}

func TestProcessActionFlow(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	taskID := "gt-process-action"
	seedWorkerGenerationTask(t, db, taskID, "user-1", "succeeded")

	actionKey := "idle_normal"
	actionID := "gta-1"
	seedWorkerAction(t, db, actionID, taskID, actionKey, "succeeded", 2)

	for i := 0; i < 2; i++ {
		relPath := fmt.Sprintf("desktop-pets/generation-tasks/%s/frames/frame-%04d.png", taskID, i+1)
		absPath := writeWorkerPNGWithContent(t, dataDir, relPath, 64, 64, i)
		hash := fileSHA256Hex(t, absPath)
		seedWorkerFrame(t, db, fmt.Sprintf("gf-%d", i+1), taskID, actionID, relPath, hash, i, 1, "succeeded")
	}

	pt := &processing.ProcessingTask{
		ID:                         "pt-process",
		GenerationTaskID:           taskID,
		ProcessingVersion:          1,
		Status:                     "processing",
		OutputWidth:                512,
		OutputHeight:               512,
		TargetCharacterHeightRatio: 0.8,
		BackgroundMode:             "remove_background",
		DefaultFPS:                 10,
	}
	createProcessingTask(t, repo, db, pt)

	source, err := w.validator.ValidateSources(taskID, "user-1")
	if err != nil {
		t.Fatalf("validate sources: %v", err)
	}

	pa := &processing.ProcessingAction{
		ID:                     "pa-1",
		ProcessingTaskID:       pt.ID,
		GenerationTaskActionID: actionID,
		ActionKey:              actionKey,
		ActionNameSnapshot:     actionKey,
		SourceAttemptNumber:    1,
		Status:                 "pending",
		SourceFrameCount:       2,
		FPS:                    10,
		LoopType:               "loop",
	}
	if err := repo.CreateProcessingActions(db, []processing.ProcessingAction{*pa}); err != nil {
		t.Fatalf("create processing action: %v", err)
	}

	paLoaded, err := repo.GetProcessingActionByActionKey(pt.ID, actionKey)
	if err != nil {
		t.Fatalf("get processing action: %v", err)
	}

	ctx := context.Background()
	if err := w.processAction(ctx, pt, paLoaded, source); err != nil {
		t.Fatalf("processAction failed: %v", err)
	}

	framesDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", actionKey, "frames")
	entries, err := os.ReadDir(framesDir)
	if err != nil {
		t.Fatalf("read frames dir: %v", err)
	}
	frameCount := 0
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			frameCount++
		}
	}
	if frameCount == 0 {
		t.Fatalf("expected at least 1 frame, got 0")
	}

	actionJSONPath := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", actionKey, "action.json")
	if _, err := os.Stat(actionJSONPath); err != nil {
		t.Fatalf("action.json not exists: %v", err)
	}

	previewPath := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", actionKey, "preview.png")
	if _, err := os.Stat(previewPath); err != nil {
		t.Fatalf("preview.png not exists: %v", err)
	}

	got, err := repo.GetProcessingActionByActionKey(pt.ID, actionKey)
	if err != nil {
		t.Fatalf("get processing action after process: %v", err)
	}
	if got.Status != "succeeded" {
		t.Fatalf("action status = %s, want succeeded", got.Status)
	}
	if got.Progress != 100 {
		t.Fatalf("action progress = %d, want 100", got.Progress)
	}
}

func TestFinalizeTaskStatus(t *testing.T) {
	tests := []struct {
		name       string
		actions    []string
		cancelled  bool
		processErr error
		wantStatus string
	}{
		{
			name:       "all_succeeded",
			actions:    []string{"succeeded", "succeeded"},
			wantStatus: "succeeded",
		},
		{
			name:       "partial_succeeded",
			actions:    []string{"succeeded", "failed"},
			wantStatus: "partially_succeeded",
		},
		{
			name:       "all_failed",
			actions:    []string{"failed", "failed"},
			processErr: fmt.Errorf("processing error"),
			wantStatus: "failed",
		},
		{
			name:       "cancelled_with_success",
			actions:    []string{"succeeded", "pending"},
			cancelled:  true,
			wantStatus: "partially_succeeded",
		},
		{
			name:       "cancelled_no_success",
			actions:    []string{"pending", "pending"},
			cancelled:  true,
			wantStatus: "cancelled",
		},
		{
			name:       "no_actions_failed",
			actions:    []string{},
			processErr: fmt.Errorf("no actions"),
			wantStatus: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupWorkerTestDB(t)
			repo := newWorkerRepo(t, db)
			dataDir := t.TempDir()
			w := NewWorker(db, repo, dataDir)

			pt := &processing.ProcessingTask{
				ID:                "pt-finalize",
				GenerationTaskID:  "gt-finalize",
				ProcessingVersion: 1,
				Status:            "processing",
			}
			createProcessingTask(t, repo, db, pt)

			if tt.cancelled {
				if err := repo.SetProcessingCancelRequested(pt.ID, workerNowStr()); err != nil {
					t.Fatalf("set cancel requested: %v", err)
				}
			}

			for i, status := range tt.actions {
				pa := processing.ProcessingAction{
					ID:               fmt.Sprintf("pa-%d", i),
					ProcessingTaskID: pt.ID,
					ActionKey:        fmt.Sprintf("action_%d", i),
					Status:           status,
				}
				if err := repo.CreateProcessingActions(db, []processing.ProcessingAction{pa}); err != nil {
					t.Fatalf("create action %d: %v", i, err)
				}
			}

			w.finalizeTask(pt.ID, tt.processErr)

			got, err := repo.GetProcessingTask(pt.ID)
			if err != nil {
				t.Fatalf("get task: %v", err)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("Status = %s, want %s", got.Status, tt.wantStatus)
			}
			if got.Progress != 100 {
				t.Fatalf("Progress = %d, want 100", got.Progress)
			}
			if got.CompletedAt == "" {
				t.Fatal("CompletedAt should be set")
			}
		})
	}
}

func TestCancelDetection(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	pt := &processing.ProcessingTask{
		ID:               "pt-cancel-detect",
		GenerationTaskID: "gt-1",
		Status:           "processing",
	}
	createProcessingTask(t, repo, db, pt)

	if w.isCancelled(pt.ID) {
		t.Fatal("should not be cancelled initially")
	}

	if err := repo.SetProcessingCancelRequested(pt.ID, workerNowStr()); err != nil {
		t.Fatalf("set cancel requested: %v", err)
	}

	if !w.isCancelled(pt.ID) {
		t.Fatal("should be cancelled after SetProcessingCancelRequested")
	}
}

func TestRecoverStuckTasks(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	pastStr := time.Now().Add(-1 * time.Minute).Format("2006-01-02 15:04:05")

	pt := &processing.ProcessingTask{
		ID:               "pt-recover-stuck",
		GenerationTaskID: "gt-1",
		Status:           "processing",
		LeaseExpiresAt:   pastStr,
		ExecutionID:      "processing-old-exec",
		WorkerID:         "worker-old",
		LastHeartbeatAt:  pastStr,
	}
	createProcessingTask(t, repo, db, pt)

	activePt := &processing.ProcessingTask{
		ID:               "pt-active-stuck",
		GenerationTaskID: "gt-1",
		Status:           "processing",
		LeaseExpiresAt:   time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05"),
	}
	createProcessingTask(t, repo, db, activePt)

	w.recoverStuckTasks(context.Background())

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get recovered task: %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued", got.Status)
	}
	if got.CurrentStage != "queued" {
		t.Fatalf("CurrentStage = %s, want queued", got.CurrentStage)
	}
	if got.ExecutionID != "" {
		t.Fatalf("ExecutionID = %s, want empty", got.ExecutionID)
	}
	if got.WorkerID != "" {
		t.Fatalf("WorkerID = %s, want empty", got.WorkerID)
	}
	if got.LeaseExpiresAt != "" {
		t.Fatalf("LeaseExpiresAt = %s, want empty", got.LeaseExpiresAt)
	}
	if got.LastHeartbeatAt != "" {
		t.Fatalf("LastHeartbeatAt = %s, want empty", got.LastHeartbeatAt)
	}

	activeGot, err := repo.GetProcessingTask(activePt.ID)
	if err != nil {
		t.Fatalf("get active task: %v", err)
	}
	if activeGot.Status != "processing" {
		t.Fatalf("active task Status = %s, want processing (unchanged)", activeGot.Status)
	}
}

func TestRecoverStuckTasks_Empty(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	createProcessingTask(t, repo, db, &processing.ProcessingTask{
		ID:               "pt-no-stuck",
		GenerationTaskID: "gt-1",
		Status:           "queued",
	})

	w.recoverStuckTasks(context.Background())

	got, err := repo.GetProcessingTask("pt-no-stuck")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued (unchanged)", got.Status)
	}
}

func TestProcessActionLoadSourceFramesMissing(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	taskID := "gt-missing-frames"
	seedWorkerGenerationTask(t, db, taskID, "user-1", "succeeded")

	actionKey := "idle_normal"
	actionID := "gta-missing"
	seedWorkerAction(t, db, actionID, taskID, actionKey, "succeeded", 2)

	source, err := w.validator.ValidateSources(taskID, "user-1")
	if err == nil {
		t.Fatalf("expected validation error for missing frames, got nil")
	}
	if source != nil {
		t.Fatalf("expected nil source, got non-nil")
	}
}

func TestFinalizeTaskCleansTempDir(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	taskID := "gt-cleanup"
	pt := &processing.ProcessingTask{
		ID:               "pt-cleanup",
		GenerationTaskID: taskID,
		Status:           "processing",
	}
	createProcessingTask(t, repo, db, pt)

	tmpDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", ".tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	tmpFile := filepath.Join(tmpDir, "temp.txt")
	if err := os.WriteFile(tmpFile, []byte("temp"), 0644); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	w.finalizeTask(pt.ID, fmt.Errorf("processing failed"))

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed, err: %v", err)
	}
}

func TestUpdateStageAndProgress(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	pt := &processing.ProcessingTask{
		ID:               "pt-stage",
		GenerationTaskID: "gt-1",
		Status:           "processing",
	}
	createProcessingTask(t, repo, db, pt)

	w.updateStage(pt.ID, StageBackgroundRemoval, ProgressBackgroundRemoval)

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.CurrentStage != StageBackgroundRemoval {
		t.Fatalf("CurrentStage = %s, want %s", got.CurrentStage, StageBackgroundRemoval)
	}
	if got.Progress != ProgressBackgroundRemoval {
		t.Fatalf("Progress = %d, want %d", got.Progress, ProgressBackgroundRemoval)
	}

	w.updateProgress(pt.ID, ProgressQuality)

	got, err = repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task after updateProgress: %v", err)
	}
	if got.Progress != ProgressQuality {
		t.Fatalf("Progress = %d, want %d", got.Progress, ProgressQuality)
	}
}
