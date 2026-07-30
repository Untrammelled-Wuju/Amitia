// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/processing"
)

func TestFaultInjection_CrashDuringProcessing(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	pastStr := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")

	pt := &processing.ProcessingTask{
		ID:                "pt-crash-during",
		GenerationTaskID:  "gt-crash",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-crashed",
		WorkerID:          "worker-crashed",
		LeaseExpiresAt:    pastStr,
		LastHeartbeatAt:   pastStr,
		CurrentStage:      "background_removal",
	}
	createProcessingTask(t, repo, db, pt)

	actions := []processing.ProcessingAction{
		{
			ID:                     "pa-crash-1",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-crash-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "idle_normal",
			SourceAttemptNumber:    1,
			Status:                 "processing",
			SourceFrameCount:       4,
			RowVersion:             1,
		},
		{
			ID:                     "pa-crash-2",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-crash-2",
			ActionKey:              "wave",
			ActionNameSnapshot:     "wave",
			SourceAttemptNumber:    1,
			Status:                 "queued",
			SourceFrameCount:       6,
			RowVersion:             1,
		},
	}
	if err := repo.CreateProcessingActions(db, actions); err != nil {
		t.Fatalf("create actions: %v", err)
	}

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

	action1, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action 1: %v", err)
	}
	if action1.Status != "pending" {
		t.Fatalf("action 1 Status = %s, want pending (reset from processing)", action1.Status)
	}

	action2, err := repo.GetProcessingActionByActionKey(pt.ID, "wave")
	if err != nil {
		t.Fatalf("get action 2: %v", err)
	}
	if action2.Status != "pending" {
		t.Fatalf("action 2 Status = %s, want pending (reset from queued)", action2.Status)
	}
}

func TestFaultInjection_HeartbeatLoss(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	staleHeartbeat := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")
	futureLease := time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05")

	pt := &processing.ProcessingTask{
		ID:                "pt-hb-loss",
		GenerationTaskID:  "gt-hb",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-hb",
		WorkerID:          "worker-hb",
		LeaseExpiresAt:    futureLease,
		LastHeartbeatAt:   staleHeartbeat,
		CurrentStage:      "background_removal",
	}
	createProcessingTask(t, repo, db, pt)

	tasks, err := repo.ListRecoverableProcessingTasks()
	if err != nil {
		t.Fatalf("list recoverable: %v", err)
	}
	for _, tk := range tasks {
		if tk.ID == pt.ID {
			t.Fatal("task with active lease should not be recoverable despite stale heartbeat")
		}
	}

	w.recoverStuckTasks(context.Background())

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "processing" {
		t.Fatalf("Status = %s, want processing (lease still valid)", got.Status)
	}

	expiredLease := time.Now().Add(-1 * time.Minute).Format("2006-01-02 15:04:05")
	if err := repo.UpdateProcessingTaskStatusNoTx(pt.ID, map[string]interface{}{
		"lease_expires_at": expiredLease,
	}); err != nil {
		t.Fatalf("update lease: %v", err)
	}

	w.recoverStuckTasks(context.Background())

	got, err = repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task after lease expiry: %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued (lease expired)", got.Status)
	}
	if got.ExecutionID != "" {
		t.Fatalf("ExecutionID = %s, want empty", got.ExecutionID)
	}
}

func TestFaultInjection_RecoveryIdempotency(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	pastStr := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")

	pt := &processing.ProcessingTask{
		ID:                "pt-idempotent",
		GenerationTaskID:  "gt-idem",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-idem",
		WorkerID:          "worker-idem",
		LeaseExpiresAt:    pastStr,
		LastHeartbeatAt:   pastStr,
	}
	createProcessingTask(t, repo, db, pt)

	if err := repo.CreateProcessingActions(db, []processing.ProcessingAction{
		{
			ID:                     "pa-idem-1",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-idem-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "idle_normal",
			SourceAttemptNumber:    1,
			Status:                 "processing",
			SourceFrameCount:       4,
			RowVersion:             1,
		},
	}); err != nil {
		t.Fatalf("create action: %v", err)
	}

	for i := 0; i < 3; i++ {
		w.recoverStuckTasks(context.Background())
	}

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued after multiple recoveries", got.Status)
	}
	if got.RowVersion < 1 {
		t.Fatalf("RowVersion = %d, should have been incremented during recovery", got.RowVersion)
	}

	action, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.Status != "pending" {
		t.Fatalf("action Status = %s, want pending", action.Status)
	}
}

func TestFaultInjection_PartialSuccessPreservedOnRecovery(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	pastStr := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")

	pt := &processing.ProcessingTask{
		ID:                "pt-partial",
		GenerationTaskID:  "gt-partial",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-partial",
		WorkerID:          "worker-partial",
		LeaseExpiresAt:    pastStr,
		LastHeartbeatAt:   pastStr,
	}
	createProcessingTask(t, repo, db, pt)

	actions := []processing.ProcessingAction{
		{
			ID:                     "pa-partial-done",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-partial-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "idle_normal",
			SourceAttemptNumber:    1,
			Status:                 "succeeded",
			SourceFrameCount:       4,
			RowVersion:             1,
		},
		{
			ID:                     "pa-partial-mid",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-partial-2",
			ActionKey:              "wave",
			ActionNameSnapshot:     "wave",
			SourceAttemptNumber:    1,
			Status:                 "processing",
			SourceFrameCount:       6,
			RowVersion:             1,
		},
		{
			ID:                     "pa-partial-fail",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-partial-3",
			ActionKey:              "happy",
			ActionNameSnapshot:     "happy",
			SourceAttemptNumber:    1,
			Status:                 "failed",
			SourceFrameCount:       8,
			RowVersion:             1,
		},
	}
	if err := repo.CreateProcessingActions(db, actions); err != nil {
		t.Fatalf("create actions: %v", err)
	}

	w.recoverStuckTasks(context.Background())

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "queued" {
		t.Fatalf("Status = %s, want queued", got.Status)
	}

	succeededAction, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get succeeded action: %v", err)
	}
	if succeededAction.Status != "succeeded" {
		t.Fatalf("succeeded action Status = %s, want succeeded (preserved)", succeededAction.Status)
	}

	processingAction, err := repo.GetProcessingActionByActionKey(pt.ID, "wave")
	if err != nil {
		t.Fatalf("get processing action: %v", err)
	}
	if processingAction.Status != "pending" {
		t.Fatalf("processing action Status = %s, want pending (reset)", processingAction.Status)
	}

	failedAction, err := repo.GetProcessingActionByActionKey(pt.ID, "happy")
	if err != nil {
		t.Fatalf("get failed action: %v", err)
	}
	if failedAction.Status != "failed" {
		t.Fatalf("failed action Status = %s, want failed (preserved)", failedAction.Status)
	}
}

func TestFaultInjection_MultipleStuckTasksRecovery(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	pastStr := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")
	futureStr := time.Now().Add(10 * time.Minute).Format("2006-01-02 15:04:05")

	stuckTasks := []processing.ProcessingTask{
		{ID: "pt-stuck-1", GenerationTaskID: "gt-s1", ProcessingVersion: 1, Status: "processing", ExecutionID: "exec-s1", WorkerID: "w1", LeaseExpiresAt: pastStr, LastHeartbeatAt: pastStr},
		{ID: "pt-stuck-2", GenerationTaskID: "gt-s2", ProcessingVersion: 1, Status: "processing", ExecutionID: "exec-s2", WorkerID: "w2", LeaseExpiresAt: pastStr, LastHeartbeatAt: pastStr},
		{ID: "pt-stuck-3", GenerationTaskID: "gt-s3", ProcessingVersion: 1, Status: "processing", ExecutionID: "exec-s3", WorkerID: "w3", LeaseExpiresAt: pastStr, LastHeartbeatAt: pastStr},
	}
	for i := range stuckTasks {
		createProcessingTask(t, repo, db, &stuckTasks[i])
	}

	activeTask := processing.ProcessingTask{
		ID: "pt-active-multi", GenerationTaskID: "gt-a", ProcessingVersion: 1, Status: "processing", ExecutionID: "exec-a", WorkerID: "wa", LeaseExpiresAt: futureStr, LastHeartbeatAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	createProcessingTask(t, repo, db, &activeTask)

	queuedTask := processing.ProcessingTask{
		ID: "pt-queued-multi", GenerationTaskID: "gt-q", ProcessingVersion: 1, Status: "queued", LeaseExpiresAt: pastStr,
	}
	createProcessingTask(t, repo, db, &queuedTask)

	w.recoverStuckTasks(context.Background())

	for _, st := range stuckTasks {
		got, err := repo.GetProcessingTask(st.ID)
		if err != nil {
			t.Fatalf("get task %s: %v", st.ID, err)
		}
		if got.Status != "queued" {
			t.Fatalf("task %s Status = %s, want queued", st.ID, got.Status)
		}
		if got.ExecutionID != "" {
			t.Fatalf("task %s ExecutionID = %s, want empty", st.ID, got.ExecutionID)
		}
	}

	activeGot, err := repo.GetProcessingTask(activeTask.ID)
	if err != nil {
		t.Fatalf("get active task: %v", err)
	}
	if activeGot.Status != "processing" {
		t.Fatalf("active task Status = %s, want processing (unchanged)", activeGot.Status)
	}

	queuedGot, err := repo.GetProcessingTask(queuedTask.ID)
	if err != nil {
		t.Fatalf("get queued task: %v", err)
	}
	if queuedGot.Status != "queued" {
		t.Fatalf("queued task Status = %s, want queued (unchanged)", queuedGot.Status)
	}
}
