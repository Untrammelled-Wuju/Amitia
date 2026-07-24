// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet"
	"gorm.io/gorm"
)

func setProcessingExpired(t *testing.T, db *gorm.DB, taskID string) {
	t.Helper()
	pastTime := time.Now().Add(-10 * time.Minute).Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing', current_stage = 'generating', worker_id = ?, execution_id = ?, lease_expires_at = ?, last_heartbeat_at = ?, started_at = ?, updated_at = ? WHERE id = ?",
		"old-worker", "old-exec", pastTime, pastTime, pastTime, now, taskID).Error; err != nil {
		t.Fatalf("set processing expired: %v", err)
	}
}

func setProcessingValid(t *testing.T, db *gorm.DB, taskID string) {
	t.Helper()
	futureTime := time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing', current_stage = 'generating', worker_id = ?, execution_id = ?, lease_expires_at = ?, last_heartbeat_at = ?, started_at = ?, updated_at = ? WHERE id = ?",
		"live-worker", "live-exec", futureTime, futureTime, futureTime, now, taskID).Error; err != nil {
		t.Fatalf("set processing valid: %v", err)
	}
}

func setTaskStatus(t *testing.T, db *gorm.DB, taskID, status string) {
	t.Helper()
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = ? WHERE id = ?", status, taskID).Error; err != nil {
		t.Fatalf("set task status %s: %v", status, err)
	}
}

func insertPollingFrame(t *testing.T, db *gorm.DB, frameID, taskID, actionID, operationID string, frameIndex int) {
	t.Helper()
	now := time.Now().Format("2006-01-02 15:04:05")
	f := &desktoppet.GenerationFrame{
		ID:                  frameID,
		TaskID:              taskID,
		TaskActionID:        actionID,
		ExecutionID:         "old-exec",
		FrameIndex:          frameIndex,
		FramePhase:          "submitted",
		Status:              "submitted",
		AttemptNumber:       0,
		PromptSnapshot:      "prompt",
		Provider:            testProviderName,
		Model:               "doubao-seedream-5-0",
		ProviderOperationID: operationID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatalf("create polling frame: %v", err)
	}
}

func TestRecoverOnStartup_ExpiredLeaseResetsToQueued(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-expired"
	task := insertTask(t, db, taskID, 1, "")
	setProcessingExpired(t, db, taskID)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.Status != "queued" {
		t.Fatalf("status = %s, want queued (recovered)", stored.Status)
	}
	if stored.CurrentStage != "queued" {
		t.Fatalf("current_stage = %s, want queued", stored.CurrentStage)
	}
	if stored.WorkerID != "" {
		t.Fatalf("worker_id should be cleared, got %s", stored.WorkerID)
	}
	if stored.ExecutionID != "" {
		t.Fatalf("execution_id should be cleared, got %s", stored.ExecutionID)
	}
	if stored.LeaseExpiresAt != "" {
		t.Fatalf("lease_expires_at should be cleared, got %s", stored.LeaseExpiresAt)
	}
	if stored.LastHeartbeatAt != "" {
		t.Fatalf("last_heartbeat_at should be cleared, got %s", stored.LastHeartbeatAt)
	}
	_ = task
}

func TestRecoverOnStartup_ValidLeaseNotReset(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-valid"
	insertTask(t, db, taskID, 1, "")
	setProcessingValid(t, db, taskID)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.Status != "processing" {
		t.Fatalf("status = %s, want processing (lease still valid, not recovered)", stored.Status)
	}
	if stored.WorkerID != "live-worker" {
		t.Fatalf("worker_id = %s, want live-worker (should not be cleared)", stored.WorkerID)
	}
	if stored.ExecutionID != "live-exec" {
		t.Fatalf("execution_id = %s, want live-exec (should not be cleared)", stored.ExecutionID)
	}
}

func TestRecoverOnStartup_QueuedTasksNotAffected(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-queued"
	insertTask(t, db, taskID, 1, "")

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.Status != "queued" {
		t.Fatalf("status = %s, want queued (queued tasks not affected)", stored.Status)
	}
}

func TestRecoverOnStartup_TerminalTasksNotAffected(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	terminalStatuses := []string{"succeeded", "failed", "cancelled", "partially_succeeded"}
	for _, status := range terminalStatuses {
		taskID := "task-recover-" + status
		insertTask(t, db, taskID, 1, "")
		setTaskStatus(t, db, taskID, status)
	}

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	for _, status := range terminalStatuses {
		taskID := "task-recover-" + status
		var stored desktoppet.GenerationTask
		if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
			t.Fatalf("query task %s: %v", taskID, err)
		}
		if stored.Status != status {
			t.Fatalf("task %s status = %s, want %s (terminal tasks not affected)", taskID, stored.Status, status)
		}
	}
}

func TestRecoverOnStartup_NoRecoverableTasksReturnsClean(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var count int64
	if err := db.Table("desktop_pet_generation_tasks").Where("status = 'queued'").Count(&count).Error; err != nil {
		t.Fatalf("count queued: %v", err)
	}
	if count != 0 {
		t.Fatalf("no tasks should be queued, got %d", count)
	}
}

func TestRecoverOnStartup_MultipleExpiredTasksAllRecovered(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	for i := 0; i < 3; i++ {
		taskID := "task-multi-recover-" + string(rune('A'+i))
		insertTask(t, db, taskID, 1, "")
		setProcessingExpired(t, db, taskID)
	}

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var recoveredCount int64
	if err := db.Table("desktop_pet_generation_tasks").Where("status = 'queued'").Count(&recoveredCount).Error; err != nil {
		t.Fatalf("count queued: %v", err)
	}
	if recoveredCount != 3 {
		t.Fatalf("recovered count = %d, want 3", recoveredCount)
	}

	var stillProcessing int64
	if err := db.Table("desktop_pet_generation_tasks").Where("status = 'processing'").Count(&stillProcessing).Error; err != nil {
		t.Fatalf("count processing: %v", err)
	}
	if stillProcessing != 0 {
		t.Fatalf("still processing = %d, want 0 (all recovered)", stillProcessing)
	}
}

func TestRecoverOnStartup_PreservesPollingFrames(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-polling"
	insertTask(t, db, taskID, 1, "")
	setProcessingExpired(t, db, taskID)
	action := insertAction(t, db, taskID, "idle_normal", "act-poll-1", 1)

	insertPollingFrame(t, db, "frame-poll-1", taskID, action.ID, "op-abc-1", 0)
	insertPollingFrame(t, db, "frame-poll-2", taskID, action.ID, "op-abc-2", 1)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var frames []desktoppet.GenerationFrame
	if err := db.Where("task_id = ?", taskID).Order("frame_index ASC").Find(&frames).Error; err != nil {
		t.Fatalf("query frames: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("frames count = %d, want 2 (polling frames should be preserved)", len(frames))
	}
	for _, f := range frames {
		if f.ProviderOperationID == "" {
			t.Fatalf("frame %d provider_operation_id should be preserved", f.FrameIndex)
		}
		if f.Status != "submitted" {
			t.Fatalf("frame %d status = %s, want submitted (preserved)", f.FrameIndex, f.Status)
		}
	}
}

func TestRecoverOnStartup_RecoveredTaskCanBeClaimedAgain(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-reclaim"
	insertTask(t, db, taskID, 1, "")
	setProcessingExpired(t, db, taskID)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	claimed, err := repo.ClaimTask(taskID, "new-worker", "new-exec", time.Now().Add(5*time.Minute).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("ClaimTask after recovery: %v", err)
	}
	if !claimed {
		t.Fatal("recovered task should be claimable by new worker")
	}

	var stored desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&stored).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if stored.WorkerID != "new-worker" {
		t.Fatalf("worker_id = %s, want new-worker", stored.WorkerID)
	}
	if stored.ExecutionID != "new-exec" {
		t.Fatalf("execution_id = %s, want new-exec", stored.ExecutionID)
	}
	if stored.Status != "processing" {
		t.Fatalf("status = %s, want processing (newly claimed)", stored.Status)
	}
}

func TestRecoverOnStartup_MixedScenarioRecoversOnlyExpired(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	expiredID := "task-mix-expired"
	validID := "task-mix-valid"
	queuedID := "task-mix-queued"
	failedID := "task-mix-failed"

	insertTask(t, db, expiredID, 1, "")
	insertTask(t, db, validID, 1, "")
	insertTask(t, db, queuedID, 1, "")
	insertTask(t, db, failedID, 1, "")

	setProcessingExpired(t, db, expiredID)
	setProcessingValid(t, db, validID)
	setTaskStatus(t, db, failedID, "failed")

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var expired desktoppet.GenerationTask
	if err := db.Where("id = ?", expiredID).First(&expired).Error; err != nil {
		t.Fatal(err)
	}
	if expired.Status != "queued" {
		t.Fatalf("expired task status = %s, want queued", expired.Status)
	}

	var valid desktoppet.GenerationTask
	if err := db.Where("id = ?", validID).First(&valid).Error; err != nil {
		t.Fatal(err)
	}
	if valid.Status != "processing" {
		t.Fatalf("valid lease task status = %s, want processing (not recovered)", valid.Status)
	}

	var queued desktoppet.GenerationTask
	if err := db.Where("id = ?", queuedID).First(&queued).Error; err != nil {
		t.Fatal(err)
	}
	if queued.Status != "queued" {
		t.Fatalf("queued task status = %s, want queued (untouched)", queued.Status)
	}

	var failed desktoppet.GenerationTask
	if err := db.Where("id = ?", failedID).First(&failed).Error; err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" {
		t.Fatalf("failed task status = %s, want failed (untouched)", failed.Status)
	}
}

func TestListRecoverableTasks_OnlyReturnsExpiredProcessing(t *testing.T) {
	db, repo, _, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	expiredID := "task-list-expired"
	validID := "task-list-valid"
	queuedID := "task-list-queued"

	insertTask(t, db, expiredID, 1, "")
	insertTask(t, db, validID, 1, "")
	insertTask(t, db, queuedID, 1, "")

	setProcessingExpired(t, db, expiredID)
	setProcessingValid(t, db, validID)

	tasks, err := repo.ListRecoverableTasks()
	if err != nil {
		t.Fatalf("ListRecoverableTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("recoverable count = %d, want 1 (only expired processing)", len(tasks))
	}
	if tasks[0].ID != expiredID {
		t.Fatalf("recoverable task id = %s, want %s", tasks[0].ID, expiredID)
	}
}

func TestListRecoverableTasks_EmptyResult(t *testing.T) {
	db, repo, _, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	tasks, err := repo.ListRecoverableTasks()
	if err != nil {
		t.Fatalf("ListRecoverableTasks: %v", err)
	}
	if tasks == nil {
		t.Fatal("should return non-nil empty slice")
	}
	if len(tasks) != 0 {
		t.Fatalf("recoverable count = %d, want 0", len(tasks))
	}
}

func TestListPollingFrames_OnlyReturnsSubmittedOrPollingWithOperationId(t *testing.T) {
	db, repo, _, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-polling-filter"
	insertTask(t, db, taskID, 1, "")
	action := insertAction(t, db, taskID, "idle_normal", "act-poll-filter", 1)

	now := time.Now().Format("2006-01-02 15:04:05")

	submitted := &desktoppet.GenerationFrame{
		ID: "frame-submitted", TaskID: taskID, TaskActionID: action.ID, FrameIndex: 0,
		Status: "submitted", ProviderOperationID: "op-1", PromptSnapshot: "p", Provider: testProviderName,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(submitted).Error; err != nil {
		t.Fatalf("create submitted: %v", err)
	}

	polling := &desktoppet.GenerationFrame{
		ID: "frame-polling", TaskID: taskID, TaskActionID: action.ID, FrameIndex: 1,
		Status: "polling", ProviderOperationID: "op-2", PromptSnapshot: "p", Provider: testProviderName,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(polling).Error; err != nil {
		t.Fatalf("create polling: %v", err)
	}

	pending := &desktoppet.GenerationFrame{
		ID: "frame-pending", TaskID: taskID, TaskActionID: action.ID, FrameIndex: 2,
		Status: "pending", ProviderOperationID: "op-3", PromptSnapshot: "p", Provider: testProviderName,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(pending).Error; err != nil {
		t.Fatalf("create pending: %v", err)
	}

	submittedNoOp := &desktoppet.GenerationFrame{
		ID: "frame-no-op", TaskID: taskID, TaskActionID: action.ID, FrameIndex: 3,
		Status: "submitted", ProviderOperationID: "", PromptSnapshot: "p", Provider: testProviderName,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(submittedNoOp).Error; err != nil {
		t.Fatalf("create submitted no op: %v", err)
	}

	frames, err := repo.ListPollingFrames(taskID)
	if err != nil {
		t.Fatalf("ListPollingFrames: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("polling frames count = %d, want 2 (only submitted/polling with operation_id)", len(frames))
	}

	ids := map[string]bool{}
	for _, f := range frames {
		ids[f.ID] = true
	}
	if !ids["frame-submitted"] || !ids["frame-polling"] {
		t.Fatalf("expected frame-submitted and frame-polling, got %v", ids)
	}
	if ids["frame-pending"] {
		t.Fatal("frame-pending should not be returned (status=pending)")
	}
	if ids["frame-no-op"] {
		t.Fatal("frame-no-op should not be returned (empty operation_id)")
	}
}

func TestListPollingFrames_EmptyResult(t *testing.T) {
	db, repo, _, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-polling-empty"
	insertTask(t, db, taskID, 1, "")

	frames, err := repo.ListPollingFrames(taskID)
	if err != nil {
		t.Fatalf("ListPollingFrames: %v", err)
	}
	if frames == nil {
		t.Fatal("should return non-nil empty slice")
	}
	if len(frames) != 0 {
		t.Fatalf("polling frames count = %d, want 0", len(frames))
	}
}

func TestRecoverOnStartup_FullFlowRecoveryAndExecution(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	pngBytes := makeWorkerPNG(t)
	mp := &mockProvider{submission: successfulSubmission(pngBytes)}
	registry.Register(testProviderName, mp)

	taskID := "task-full-recovery"
	sourceRel := writeWorkerReferenceImage(t, dataDir, taskID)
	task := insertTask(t, db, taskID, 1, sourceRel)
	action := insertAction(t, db, taskID, "idle_blink", "act-full-recovery", 1)
	setProcessingExpired(t, db, taskID)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var recovered desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&recovered).Error; err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "queued" {
		t.Fatalf("status = %s, want queued after recovery", recovered.Status)
	}

	status := w.runAction(context.Background(), &recovered, action)
	if status != "succeeded" {
		t.Fatalf("runAction after recovery: status = %s, want succeeded", status)
	}

	var frames []desktoppet.GenerationFrame
	if err := db.Where("task_action_id = ?", action.ID).Order("frame_index ASC").Find(&frames).Error; err != nil {
		t.Fatalf("query frames: %v", err)
	}
	if len(frames) == 0 {
		t.Fatal("expected frames to be created after recovery and execution")
	}
	for _, f := range frames {
		if f.Status != "succeeded" {
			t.Fatalf("frame %d status = %s, want succeeded", f.FrameIndex, f.Status)
		}
		if f.ResultImagePath == "" {
			t.Fatalf("frame %d missing result image path", f.FrameIndex)
		}
	}
	_ = task
}

func TestRecoverOnStartup_PreservesCallLogs(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-calllogs"
	insertTask(t, db, taskID, 1, "")
	action := insertAction(t, db, taskID, "idle_normal", "act-calllogs", 1)
	setProcessingExpired(t, db, taskID)

	now := time.Now().Format("2006-01-02 15:04:05")
	callLog := &desktoppet.GenerationCallLog{
		ID:               "log-preserve-1",
		TaskID:           taskID,
		TaskActionID:     action.ID,
		FrameID:          "frame-old-1",
		ExecutionID:      "old-exec",
		Provider:         testProviderName,
		Model:            "doubao-seedream-5-0",
		RequestStartedAt: now,
		RequestStatus:    "submitted",
		AttemptNumber:    1,
		Usage:            "unknown",
		CreatedAt:        now,
	}
	if err := db.Create(callLog).Error; err != nil {
		t.Fatalf("create call log: %v", err)
	}

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var logs []desktoppet.GenerationCallLog
	if err := db.Where("task_id = ?", taskID).Find(&logs).Error; err != nil {
		t.Fatalf("query call logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("call logs count = %d, want 1 (preserved across recovery)", len(logs))
	}
	if logs[0].ID != "log-preserve-1" {
		t.Fatalf("call log id = %s, want log-preserve-1", logs[0].ID)
	}
	if logs[0].ExecutionID != "old-exec" {
		t.Fatalf("call log execution_id = %s, want old-exec (preserved)", logs[0].ExecutionID)
	}
}

func TestRecoverOnStartup_DoesNotDuplicateRecovery(t *testing.T) {
	db, repo, registry, _ := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-no-dup"
	insertTask(t, db, taskID, 1, "")
	setProcessingExpired(t, db, taskID)

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	var afterFirst desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&afterFirst).Error; err != nil {
		t.Fatal(err)
	}
	if afterFirst.Status != "queued" {
		t.Fatalf("after first recovery: status = %s, want queued", afterFirst.Status)
	}
	firstUpdatedAt := afterFirst.UpdatedAt

	time.Sleep(1 * time.Second)

	w.RecoverOnStartup(context.Background())

	var afterSecond desktoppet.GenerationTask
	if err := db.Where("id = ?", taskID).First(&afterSecond).Error; err != nil {
		t.Fatal(err)
	}
	if afterSecond.Status != "queued" {
		t.Fatalf("after second recovery: status = %s, want queued (no duplicate recovery)", afterSecond.Status)
	}
	if afterSecond.UpdatedAt != firstUpdatedAt {
		t.Fatalf("updated_at changed after second recovery: first=%s second=%s (should not be touched)", firstUpdatedAt, afterSecond.UpdatedAt)
	}
}

func TestRecoverOnStartup_PreservesSuccessfulFrameMaterials(t *testing.T) {
	db, repo, registry, dataDir := setupWorkerTestEnv(t)
	seedWorkerCharacter(t, db)
	seedSimpleModelConfig(t, db, 1, "sk-test", 1)

	taskID := "task-recover-materials"
	insertTask(t, db, taskID, 1, "")
	action := insertAction(t, db, taskID, "idle_normal", "act-materials", 1)
	setProcessingExpired(t, db, taskID)

	attemptDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "generated", "idle_normal", "attempt-1", "raw")
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	imagePath := filepath.Join(attemptDir, "frame-0000.png")
	if err := os.WriteFile(imagePath, []byte("saved-before-crash"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	succeededFrame := &desktoppet.GenerationFrame{
		ID: "frame-saved-1", TaskID: taskID, TaskActionID: action.ID, FrameIndex: 0,
		Status: "succeeded", ResultImagePath: filepath.ToSlash(filepath.Join(
			"desktop-pets", "generation-tasks", taskID, "generated", "idle_normal", "attempt-1", "raw", "frame-0000.png")),
		PromptSnapshot: "p", Provider: testProviderName, Model: "m",
		CreatedAt: now, UpdatedAt: now, CompletedAt: now,
	}
	if err := db.Create(succeededFrame).Error; err != nil {
		t.Fatalf("create succeeded frame: %v", err)
	}

	w := NewWorker(db, repo, registry)
	w.RecoverOnStartup(context.Background())

	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("successful material should be preserved across recovery: %v", err)
	}

	var frame desktoppet.GenerationFrame
	if err := db.Where("id = ?", "frame-saved-1").First(&frame).Error; err != nil {
		t.Fatalf("query frame: %v", err)
	}
	if frame.Status != "succeeded" {
		t.Fatalf("frame status = %s, want succeeded (preserved)", frame.Status)
	}
	if frame.ResultImagePath == "" {
		t.Fatal("frame result_image_path should be preserved")
	}
}
