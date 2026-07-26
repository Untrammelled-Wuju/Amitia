package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func makeTestTask(t *testing.T) *Task {
	t.Helper()
	def := TaskDefinition{
		TaskID:         "task-1",
		ExtensionID:    "com.example/weather",
		ModuleID:       "main",
		RuntimeType:    "task",
		Entry:          "tasks/migrate.js",
		EntryHash:      "sha256:entry",
		Checkpoint:     true,
		Recoverable:    true,
		DefinitionVersion: 1,
		MaxDuration:    10 * time.Second,
		ResourceLimits: DefaultTaskResourceLimits(),
	}
	input := TaskInput{
		TaskID:    "task-1",
		Input:     json.RawMessage(`{"cursor":0}`),
		InputHash: "sha256:input",
	}
	task, err := NewTask(def, input, "op-1", 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func TestNewTaskValidation(t *testing.T) {
	_, err := NewTask(TaskDefinition{}, TaskInput{}, "", 0)
	if err == nil {
		t.Fatal("expected error for empty definition")
	}
}

func TestTaskStateTransitions(t *testing.T) {
	task := makeTestTask(t)
	if task.State() != TaskStateCreated {
		t.Fatalf("expected created, got %s", task.State())
	}
	task.MarkStarted()
	if task.State() != TaskStateRunning {
		t.Fatalf("expected running, got %s", task.State())
	}
	task.Succeed(json.RawMessage(`{"result":"ok"}`), "sha256:out", "")
	if task.State() != TaskStateSucceeded {
		t.Fatalf("expected succeeded, got %s", task.State())
	}
	result := task.Result()
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Status != TaskStateSucceeded {
		t.Fatalf("expected succeeded status, got %s", result.Status)
	}
}

func TestTaskFail(t *testing.T) {
	task := makeTestTask(t)
	task.MarkStarted()
	task.Fail("processing error")
	if task.State() != TaskStateFailed {
		t.Fatalf("expected failed, got %s", task.State())
	}
	if task.Result().Error != "processing error" {
		t.Fatalf("expected error message, got %s", task.Result().Error)
	}
}

func TestTaskCancel(t *testing.T) {
	task := makeTestTask(t)
	task.MarkStarted()
	task.Cancel("user requested")
	if !task.CancelSignal().IsCancelled() {
		t.Fatal("expected cancelled signal")
	}
	task.Cancelled("user requested")
	if task.State() != TaskStateCancelled {
		t.Fatalf("expected cancelled, got %s", task.State())
	}
}

func TestTaskTimedOut(t *testing.T) {
	task := makeTestTask(t)
	task.MarkStarted()
	task.TimedOut()
	if task.State() != TaskStateTimedOut {
		t.Fatalf("expected timed_out, got %s", task.State())
	}
}

func TestTaskCheckpoint(t *testing.T) {
	task := makeTestTask(t)
	cp := Checkpoint{
		CheckpointID: "cp-1",
		Data:         json.RawMessage(`{"cursor":5}`),
		Hash:         "sha256:cp1",
	}
	if err := task.SaveCheckpoint(cp); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	latest := task.LatestCheckpoint()
	if latest == nil {
		t.Fatal("expected latest checkpoint")
	}
	if latest.Version != 1 {
		t.Fatalf("expected version 1, got %d", latest.Version)
	}

	cp2 := Checkpoint{
		CheckpointID: "cp-2",
		Data:         json.RawMessage(`{"cursor":10}`),
		Hash:         "sha256:cp2",
	}
	task.SaveCheckpoint(cp2)
	latest = task.LatestCheckpoint()
	if latest.Version != 2 {
		t.Fatalf("expected version 2, got %d", latest.Version)
	}
}

func TestTaskCheckpointDisabled(t *testing.T) {
	def := TaskDefinition{
		TaskID:      "task-1",
		ExtensionID: "com.example",
		Entry:       "entry.js",
		Checkpoint:  false,
	}
	task, _ := NewTask(def, TaskInput{TaskID: "task-1"}, "op-1", 1)
	err := task.SaveCheckpoint(Checkpoint{Data: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("expected error for disabled checkpoints")
	}
}

func TestTaskValidateRecovery(t *testing.T) {
	task1 := makeTestTask(t)
	task2 := makeTestTask(t)
	if err := task1.ValidateRecovery(task2); err != nil {
		t.Fatalf("expected valid recovery: %v", err)
	}

	task2.definition.EntryHash = "different"
	if err := task1.ValidateRecovery(task2); err == nil {
		t.Fatal("expected entry hash mismatch error")
	}
}

func TestTaskRecoverableRequired(t *testing.T) {
	def := TaskDefinition{
		TaskID:       "task-1",
		ExtensionID:  "com.example",
		Entry:        "entry.js",
		EntryHash:    "sha256:entry",
		Recoverable:  false,
	}
	task1, _ := NewTask(def, TaskInput{TaskID: "task-1", InputHash: "h1"}, "op-1", 1)
	task2, _ := NewTask(def, TaskInput{TaskID: "task-1", InputHash: "h1"}, "op-1", 1)
	if err := task1.ValidateRecovery(task2); err == nil {
		t.Fatal("expected recoverable required error")
	}
}

func TestTaskExecutorEnqueue(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	result := executor.Enqueue(task)
	if result.TaskID != "task-1" {
		t.Fatalf("expected task-1, got %s", result.TaskID)
	}
}

func TestTaskExecutorExecuteSuccess(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	result := executor.Execute(context.Background(), ExecuteRequest{
		TaskID: "task-1",
		Handler: func(ctx context.Context, task *Task) ([]byte, string, string, error) {
			return []byte(`{"migrated":true}`), "sha256:out", "", nil
		},
	})
	if result.Status != TaskStateSucceeded {
		t.Fatalf("expected succeeded, got %s: %s", result.Status, result.Error)
	}
	if string(result.Output) != `{"migrated":true}` {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}

func TestTaskExecutorExecuteFailed(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	result := executor.Execute(context.Background(), ExecuteRequest{
		TaskID: "task-1",
		Handler: func(ctx context.Context, task *Task) ([]byte, string, string, error) {
			return nil, "", "", errors.New("processing failed")
		},
	})
	if result.Status != TaskStateFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestTaskExecutorCancelQueuedTask(t *testing.T) {
	executor := NewTaskExecutor(1, "/tmp/tasks")
	task1 := makeTestTask(t)
	task1.definition.TaskID = "task-1"
	task2 := makeTestTask(t)
	task2.definition.TaskID = "task-2"
	executor.Enqueue(task1)
	executor.Enqueue(task2)
	if executor.QueuedCount() != 1 {
		t.Fatalf("expected 1 queued, got %d", executor.QueuedCount())
	}
	if err := executor.Cancel("task-2", "user requested"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	t2, _ := executor.Get("task-2")
	if t2.State() != TaskStateCancelled {
		t.Fatalf("expected cancelled, got %s", t2.State())
	}
}

func TestTaskExecutorCleanup(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	executor.Execute(context.Background(), ExecuteRequest{
		TaskID: "task-1",
		Handler: func(ctx context.Context, task *Task) ([]byte, string, string, error) {
			return []byte(`{}`), "sha256:out", "", nil
		},
	})
	if err := executor.Cleanup("task-1"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := executor.Get("task-1"); err == nil {
		t.Fatal("expected task removed after cleanup")
	}
}

func TestTaskExecutorCleanupNonTerminal(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	if err := executor.Cleanup("task-1"); err == nil {
		t.Fatal("expected error cleaning up non-terminal task")
	}
}

func TestTaskExecutorRecover(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	task.SaveCheckpoint(Checkpoint{
		Data: json.RawMessage(`{"cursor":5}`),
		Hash: "sha256:cp",
	})
	task.SetState(TaskStateRecoveryRequired)

	result := executor.Recover(context.Background(), "task-1", func(ctx context.Context, task *Task) ([]byte, string, string, error) {
		return []byte(`{"recovered":true}`), "sha256:out", "", nil
	})
	if result.Status != TaskStateSucceeded {
		t.Fatalf("expected succeeded, got %s: %s", result.Status, result.Error)
	}
}

func TestTaskExecutorRecoverNoCheckpoint(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	result := executor.Recover(context.Background(), "task-1", func(ctx context.Context, task *Task) ([]byte, string, string, error) {
		return nil, "", "", nil
	})
	if result.Status != TaskStateFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestTaskExecutorListByExtension(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task1 := makeTestTask(t)
	task1.definition.TaskID = "task-1"
	task2 := makeTestTask(t)
	task2.definition.TaskID = "task-2"
	task2.definition.ExtensionID = "com.other"
	executor.Enqueue(task1)
	executor.Enqueue(task2)
	tasks := executor.ListByExtension("com.example/weather")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task for extension, got %d", len(tasks))
	}
}

func TestProgressReport(t *testing.T) {
	task := makeTestTask(t)
	task.ReportProgress(Progress{Current: 5, Total: 10, Message: "processing"})
	p := task.Progress()
	if p.Current != 5 || p.Total != 10 {
		t.Fatalf("unexpected progress: %+v", p)
	}
	if p.Message != "processing" {
		t.Fatalf("expected message, got %s", p.Message)
	}
}
