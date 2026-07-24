// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartTask_PendingTransitionsToQueued(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-pending", []string{"idle_normal"})

	result, err := svc.StartTask(summary.ID)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("status = %s, want queued", result.Status)
	}
	if result.CurrentStage != "queued" {
		t.Fatalf("currentStage = %s, want queued", result.CurrentStage)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.ExecutionID == "" {
		t.Fatal("execution_id should be set")
	}
	if task.StartedAt == "" {
		t.Fatal("started_at should be set")
	}
	if task.Status != "queued" {
		t.Fatalf("status = %s, want queued", task.Status)
	}

	var actions []GenerationTaskAction
	if err := db.Where("task_id = ?", summary.ID).Find(&actions).Error; err != nil {
		t.Fatalf("query actions: %v", err)
	}
	for _, a := range actions {
		if a.Status != "queued" {
			t.Fatalf("action %s status = %s, want queued", a.ActionKey, a.Status)
		}
		if a.ErrorCode != "" {
			t.Fatalf("action %s error_code should be cleared, got %s", a.ActionKey, a.ErrorCode)
		}
		if a.StartedAt != "" {
			t.Fatalf("action %s started_at should be cleared, got %s", a.ActionKey, a.StartedAt)
		}
	}
}

func TestStartTask_FailedTaskTransitionsToQueued(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-failed", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed', error_code = ?, error_message = ? WHERE id = ?", ErrCodeGenerationWorkerError, "all failed", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', error_code = ?, error_message = ?, attempt_number = 1 WHERE task_id = ?", ErrCodeImageGenerationAuthFailed, "auth", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	result, err := svc.StartTask(summary.ID)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("status = %s, want queued", result.Status)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.ErrorCode != "" {
		t.Fatalf("error_code should be cleared, got %s", task.ErrorCode)
	}
	if task.ErrorMessage != "" {
		t.Fatalf("error_message should be cleared, got %s", task.ErrorMessage)
	}

	var actions []GenerationTaskAction
	if err := db.Where("task_id = ?", summary.ID).Find(&actions).Error; err != nil {
		t.Fatalf("query actions: %v", err)
	}
	for _, a := range actions {
		if a.Status != "queued" {
			t.Fatalf("action %s status = %s, want queued", a.ActionKey, a.Status)
		}
	}
}

func TestStartTask_PartiallySucceededPreservesSucceededActions(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-partial", []string{"idle_normal", "walk_left"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'partially_succeeded' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	var actions []GenerationTaskAction
	if err := db.Where("task_id = ?", summary.ID).Order("sort_order ASC").Find(&actions).Error; err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("actions count = %d, want 2", len(actions))
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'succeeded', attempt_number = 1 WHERE id = ?", actions[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', attempt_number = 1 WHERE id = ?", actions[1].ID).Error; err != nil {
		t.Fatal(err)
	}

	result, err := svc.StartTask(summary.ID)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("status = %s, want queued", result.Status)
	}

	var succeededAction GenerationTaskAction
	if err := db.Where("id = ?", actions[0].ID).First(&succeededAction).Error; err != nil {
		t.Fatal(err)
	}
	if succeededAction.Status != "succeeded" {
		t.Fatalf("succeeded action should be preserved, got %s", succeededAction.Status)
	}

	var failedAction GenerationTaskAction
	if err := db.Where("id = ?", actions[1].ID).First(&failedAction).Error; err != nil {
		t.Fatal(err)
	}
	if failedAction.Status != "queued" {
		t.Fatalf("failed action should be reset to queued, got %s", failedAction.Status)
	}
}

func TestStartTask_DuplicateCallFromQueuedReturnsConflict(t *testing.T) {
	svc, _, _ := setupTestService(t)
	summary := createValidTask(t, svc, "dup-start", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("first StartTask: %v", err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestStartTask_ProcessingReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-processing", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestStartTask_SucceededReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-succeeded", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'succeeded' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'succeeded' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestStartTask_CancelledReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-cancelled", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'cancelled' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestStartTask_NonExistentReturnsNotFound(t *testing.T) {
	svc, _, _ := setupTestService(t)

	_, err := svc.StartTask("nonexistent-task-id")
	assertBusinessError(t, err, ErrCodeGenerationTaskNotFound)
}

func TestStartTask_ModelDisabledReturnsModelUnavailable(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-model-disabled", []string{"idle_normal"})

	if err := db.Exec("UPDATE image_gen_configs SET enabled = 0 WHERE id = 1").Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeImageModelUnavailable)
}

func TestStartTask_MissingReferenceImageReturnsInvalid(t *testing.T) {
	svc, _, dataDir := setupTestService(t)
	summary := createValidTask(t, svc, "start-no-ref", []string{"idle_normal"})

	taskDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID)
	if err := os.RemoveAll(taskDir); err != nil {
		t.Fatalf("remove task dir: %v", err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeReferenceImageInvalid)
}

func TestStartTask_AllActionsCompletedReturnsConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-all-done", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'partially_succeeded' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'succeeded' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestCancelTask_QueuedTaskSetsCancelRequested(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-queued", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	err := svc.CancelTask(summary.ID)
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.CancelRequestedAt == "" {
		t.Fatal("cancel_requested_at should be set")
	}
	if task.Status != "queued" {
		t.Fatalf("status = %s, want queued (cancel sets flag only)", task.Status)
	}
}

func TestCancelTask_ProcessingTaskSetsCancelRequested(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-processing", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.CancelTask(summary.ID)
	if err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.CancelRequestedAt == "" {
		t.Fatal("cancel_requested_at should be set")
	}
}

func TestCancelTask_PendingReturnsStateConflict(t *testing.T) {
	svc, _, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-pending", []string{"idle_normal"})

	err := svc.CancelTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestCancelTask_SucceededReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-succeeded", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'succeeded' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.CancelTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestCancelTask_FailedReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-failed", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.CancelTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestCancelTask_NonExistentReturnsNotFound(t *testing.T) {
	svc, _, _ := setupTestService(t)

	err := svc.CancelTask("nonexistent-task-id")
	assertBusinessError(t, err, ErrCodeGenerationTaskNotFound)
}

func TestCancelTask_CancelledTaskReturnsConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-twice", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if err := svc.CancelTask(summary.ID); err != nil {
		t.Fatalf("first CancelTask: %v", err)
	}

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'cancelled' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.CancelTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestCancelTask_DoesNotDeleteSuccessfulMaterials(t *testing.T) {
	svc, db, dataDir := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-keep-materials", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	attemptDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID, "generated", "idle_normal", "attempt-1", "raw")
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	imagePath := filepath.Join(attemptDir, "frame-0000.png")
	if err := os.WriteFile(imagePath, []byte("fake-png"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := svc.CancelTask(summary.ID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("successful material should not be deleted: %v", err)
	}
	_ = db
}

func TestRetryAction_FailedActionResetsAndQueues(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-failed", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', error_code = ?, error_message = ?, attempt_number = 1 WHERE task_id = ?", ErrCodeImageGenerationAuthFailed, "auth", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("status = %s, want queued", resp.Status)
	}
	if resp.ErrorCode != "" {
		t.Fatalf("error_code should be cleared, got %s", resp.ErrorCode)
	}
	if resp.AttemptNumber != 2 {
		t.Fatalf("attempt_number = %d, want 2", resp.AttemptNumber)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("task status = %s, want queued", task.Status)
	}
}

func TestRetryAction_SucceededActionResetsAndQueues(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-succeeded", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'succeeded' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'succeeded', attempt_number = 1 WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("status = %s, want queued", resp.Status)
	}
	if resp.AttemptNumber != 2 {
		t.Fatalf("attempt_number = %d, want 2", resp.AttemptNumber)
	}
}

func TestRetryAction_CancelledActionResetsAndQueues(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-cancelled", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'cancelled' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'cancelled' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("status = %s, want queued", resp.Status)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("task status = %s, want queued (retry on cancelled task should re-queue)", task.Status)
	}
}

func TestRetryAction_PendingReturnsStateConflict(t *testing.T) {
	svc, _, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-pending", []string{"idle_normal"})

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestRetryAction_QueuedReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-queued", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'queued' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestRetryAction_ProcessingReturnsStateConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-processing", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'processing' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestRetryAction_NonExistentActionReturnsNotFound(t *testing.T) {
	svc, _, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-no-action", []string{"idle_normal"})

	_, err := svc.RetryAction(summary.ID, "nonexistent_action")
	assertBusinessError(t, err, ErrCodeActionNotFound)
}

func TestRetryAction_NonExistentTaskReturnsNotFound(t *testing.T) {
	svc, _, _ := setupTestService(t)

	_, err := svc.RetryAction("nonexistent-task", "idle_normal")
	assertBusinessError(t, err, ErrCodeGenerationTaskNotFound)
}

func TestRetryAction_DoesNotChangeTaskStatusWhenProcessing(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-in-processing", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'processing' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', attempt_number = 1 WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != "processing" {
		t.Fatalf("task status = %s, want processing (should not change)", task.Status)
	}
}

func TestRetryAction_PreservesOldAttemptMaterials(t *testing.T) {
	svc, db, dataDir := setupTestService(t)
	summary := createValidTask(t, svc, "retry-keep-materials", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', attempt_number = 1 WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	attemptDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", summary.ID, "generated", "idle_normal", "attempt-1", "raw")
	if err := os.MkdirAll(attemptDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldImage := filepath.Join(attemptDir, "frame-0000.png")
	if err := os.WriteFile(oldImage, []byte("old-png"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}

	if _, err := os.Stat(oldImage); err != nil {
		t.Fatalf("old attempt material should be preserved: %v", err)
	}

	var action GenerationTaskAction
	if err := db.Where("task_id = ? AND action_key = ?", summary.ID, "idle_normal").First(&action).Error; err != nil {
		t.Fatalf("query action: %v", err)
	}
	if action.AttemptNumber != 2 {
		t.Fatalf("attempt_number = %d, want 2", action.AttemptNumber)
	}
}

func TestRetryAction_QueuesTaskWhenNotProcessing(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "retry-queue-task", []string{"idle_normal"})

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed', attempt_number = 1 WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction: %v", err)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("task status = %s, want queued (non-processing task should be re-queued)", task.Status)
	}
	if task.CurrentStage != "queued" {
		t.Fatalf("current_stage = %s, want queued", task.CurrentStage)
	}
}

func TestStateConflict_StartAfterCancelReturnsConflict(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "start-after-cancel", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("first StartTask: %v", err)
	}
	if err := svc.CancelTask(summary.ID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'cancelled' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.StartTask(summary.ID)
	assertBusinessError(t, err, ErrCodeGenerationStateConflict)
}

func TestStateConflict_CancelThenRetryPreservesCancelFlag(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "cancel-then-retry", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if err := svc.CancelTask(summary.ID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'cancelled' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'cancelled' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := svc.RetryAction(summary.ID, "idle_normal")
	if err != nil {
		t.Fatalf("RetryAction after cancel should succeed (cancelled action is retryable): %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("action status = %s, want queued", resp.Status)
	}

	var task GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("task status = %s, want queued (retry should re-queue cancelled task)", task.Status)
	}
}

func TestStartTask_GeneratesNewExecutionIdEachCall(t *testing.T) {
	svc, db, _ := setupTestService(t)
	summary := createValidTask(t, svc, "exec-id-regen", []string{"idle_normal"})

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("first StartTask: %v", err)
	}
	var task1 GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task1).Error; err != nil {
		t.Fatal(err)
	}
	firstExecID := task1.ExecutionID
	if firstExecID == "" {
		t.Fatal("first execution_id should not be empty")
	}

	if err := db.Exec("UPDATE desktop_pet_generation_tasks SET status = 'failed' WHERE id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE desktop_pet_generation_task_actions SET status = 'failed' WHERE task_id = ?", summary.ID).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.StartTask(summary.ID); err != nil {
		t.Fatalf("second StartTask: %v", err)
	}
	var task2 GenerationTask
	if err := db.Where("id = ?", summary.ID).First(&task2).Error; err != nil {
		t.Fatal(err)
	}
	if task2.ExecutionID == "" {
		t.Fatal("second execution_id should not be empty")
	}
	if task2.ExecutionID == firstExecID {
		t.Fatalf("second start should generate new execution_id, got same: %s", task2.ExecutionID)
	}
}
