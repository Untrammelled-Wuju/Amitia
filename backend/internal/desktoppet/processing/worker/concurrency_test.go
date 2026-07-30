// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/taskstate"
)

func TestConcurrency_ConcurrentTaskClaiming(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)

	pt := &processing.ProcessingTask{
		ID:                "pt-concurrent-claim",
		GenerationTaskID:  "gt-cc",
		ProcessingVersion: 1,
		Status:            "queued",
	}
	createProcessingTask(t, repo, db, pt)

	var successCount int32
	var wg sync.WaitGroup
	workers := 5

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			leaseExpires := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
			execID := "exec-cc-" + string(rune('A'+workerID))
			ok, err := repo.ClaimProcessingTask(pt.ID, "worker-"+string(rune('A'+workerID)), execID, leaseExpires)
			if err != nil {
				return
			}
			if ok {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful claim, got %d", successCount)
	}

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "processing" {
		t.Fatalf("Status = %s, want processing", got.Status)
	}
	if got.ExecutionID == "" {
		t.Fatal("ExecutionID should not be empty")
	}
}

func TestConcurrency_ConcurrentStateTransition(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir, nil, nil)

	pt := &processing.ProcessingTask{
		ID:                "pt-concurrent-transition",
		GenerationTaskID:  "gt-ct",
		ProcessingVersion: 1,
		Status:            "queued",
	}
	createProcessingTask(t, repo, db, pt)

	var successCount int32
	var wg sync.WaitGroup
	workers := 5

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			execID := "exec-ct-" + string(rune('A'+workerID))
			leaseExpires := time.Now().Add(30 * time.Second).Format("2006-01-02 15:04:05")
			now := time.Now().Format("2006-01-02 15:04:05")
			progress := 10

			_, err := w.stateEngine.Transition(context.Background(), taskstate.TransitionRequest{
				EntityType:      contracts.EntityProcessingTask,
				EntityID:        pt.ID,
				From:            []contracts.LifecycleStatus{contracts.StatusQueued},
				To:              contracts.StatusProcessing,
				Stage:           contracts.StageValidatingSources,
				Reason:          contracts.ReasonProcessingTaskClaim,
				ActorType:       contracts.ActorWorker,
				ActorID:         "worker-" + string(rune('A'+workerID)),
				ExecutionID:     execID,
				WorkerID:        "worker-" + string(rune('A'+workerID)),
				LeaseExpiresAt:  leaseExpires,
				LastHeartbeatAt: now,
				Progress:        &progress,
				NeedOwnership:   false,
			})
			if err != nil {
				if taskstate.IsConflictError(err) {
					return
				}
				return
			}
			atomic.AddInt32(&successCount, 1)
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful transition, got %d", successCount)
	}

	got, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "processing" {
		t.Fatalf("Status = %s, want processing", got.Status)
	}
}

func TestConcurrency_ActionRowVersionOptimisticLock(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)

	pt := &processing.ProcessingTask{
		ID:                "pt-rv-test",
		GenerationTaskID:  "gt-rv",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-rv",
	}
	createProcessingTask(t, repo, db, pt)

	if err := repo.CreateProcessingActions(db, []processing.ProcessingAction{
		{
			ID:                     "pa-rv-1",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-rv-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "idle_normal",
			SourceAttemptNumber:    1,
			Status:                 "pending",
			SourceFrameCount:       4,
			RowVersion:             1,
		},
	}); err != nil {
		t.Fatalf("create action: %v", err)
	}

	ok1, err := repo.UpdateProcessingActionWithRowVersion(db, "pa-rv-1", 1, map[string]interface{}{
		"status": "processing",
	})
	if err != nil {
		t.Fatalf("first update with correct row_version: %v", err)
	}
	if !ok1 {
		t.Fatal("expected ok=true with correct row_version=1")
	}

	action, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.RowVersion != 2 {
		t.Fatalf("RowVersion = %d, want 2 after first update", action.RowVersion)
	}

	ok2, err := repo.UpdateProcessingActionWithRowVersion(db, "pa-rv-1", 1, map[string]interface{}{
		"status": "succeeded",
	})
	if err != nil {
		t.Fatalf("second update with stale row_version: %v", err)
	}
	if ok2 {
		t.Fatal("expected ok=false with stale row_version=1")
	}

	action2, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action after stale update: %v", err)
	}
	if action2.Status != "processing" {
		t.Fatalf("Status = %s, want processing (stale update should not apply)", action2.Status)
	}

	ok3, err := repo.UpdateProcessingActionWithRowVersion(db, "pa-rv-1", 2, map[string]interface{}{
		"status": "succeeded",
	})
	if err != nil {
		t.Fatalf("third update with correct row_version=2: %v", err)
	}
	if !ok3 {
		t.Fatal("expected ok=true with correct row_version=2")
	}

	action3, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action after correct update: %v", err)
	}
	if action3.Status != "succeeded" {
		t.Fatalf("Status = %s, want succeeded", action3.Status)
	}
	if action3.RowVersion != 3 {
		t.Fatalf("RowVersion = %d, want 3", action3.RowVersion)
	}
}

func TestConcurrency_ConcurrentActionUpdates(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)

	pt := &processing.ProcessingTask{
		ID:                "pt-cau",
		GenerationTaskID:  "gt-cau",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-cau",
	}
	createProcessingTask(t, repo, db, pt)

	if err := repo.CreateProcessingActions(db, []processing.ProcessingAction{
		{
			ID:                     "pa-cau-1",
			ProcessingTaskID:       pt.ID,
			GenerationTaskActionID: "gta-cau-1",
			ActionKey:              "idle_normal",
			ActionNameSnapshot:     "idle_normal",
			SourceAttemptNumber:    1,
			Status:                 "pending",
			SourceFrameCount:       4,
			RowVersion:             1,
		},
	}); err != nil {
		t.Fatalf("create action: %v", err)
	}

	var successCount int32
	var wg sync.WaitGroup
	workers := 5

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.UpdateProcessingActionWithRowVersion(db, "pa-cau-1", 1, map[string]interface{}{
				"status": "processing",
			})
			if err != nil {
				return
			}
			if ok {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful update, got %d", successCount)
	}

	action, err := repo.GetProcessingActionByActionKey(pt.ID, "idle_normal")
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.Status != "processing" {
		t.Fatalf("Status = %s, want processing", action.Status)
	}
	if action.RowVersion != 2 {
		t.Fatalf("RowVersion = %d, want 2", action.RowVersion)
	}
}

func TestConcurrency_OwnershipBasedUpdate(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)

	pt := &processing.ProcessingTask{
		ID:                "pt-own",
		GenerationTaskID:  "gt-own",
		ProcessingVersion: 1,
		Status:            "processing",
		ExecutionID:       "exec-owner",
	}
	createProcessingTask(t, repo, db, pt)

	ok1, err := repo.UpdateProcessingTaskOwned(pt.ID, "exec-owner", map[string]interface{}{
		"progress": 50,
	})
	if err != nil {
		t.Fatalf("update with correct ownership: %v", err)
	}
	if !ok1 {
		t.Fatal("expected ok=true with correct execution_id")
	}

	got, _ := repo.GetProcessingTask(pt.ID)
	if got.Progress != 50 {
		t.Fatalf("Progress = %d, want 50", got.Progress)
	}

	ok2, err := repo.UpdateProcessingTaskOwned(pt.ID, "exec-wrong", map[string]interface{}{
		"progress": 99,
	})
	if err != nil {
		t.Fatalf("update with wrong ownership: %v", err)
	}
	if ok2 {
		t.Fatal("expected ok=false with wrong execution_id")
	}

	got2, _ := repo.GetProcessingTask(pt.ID)
	if got2.Progress != 50 {
		t.Fatalf("Progress = %d, want 50 (unchanged after failed ownership)", got2.Progress)
	}
}
