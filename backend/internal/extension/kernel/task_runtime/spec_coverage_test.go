package task_runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestTaskRunStatusTransitions(t *testing.T) {
	tests := []struct {
		from     TaskRunStatus
		to       TaskRunStatus
		expected bool
	}{
		{RunStatusCreated, RunStatusQueued, true},
		{RunStatusQueued, RunStatusStarting, true},
		{RunStatusStarting, RunStatusRunning, true},
		{RunStatusRunning, RunStatusSucceeded, true},
		{RunStatusRunning, RunStatusFailed, true},
		{RunStatusRunning, RunStatusCancelling, true},
		{RunStatusCancelling, RunStatusCancelled, true},
		{RunStatusCancelling, RunStatusFailed, true},
		{RunStatusRecoveryRequired, RunStatusStarting, true},
		{RunStatusRecoveryRequired, RunStatusManualIntervention, true},
		{RunStatusPaused, RunStatusResuming, true},
		{RunStatusResuming, RunStatusRunning, true},
	}
	for _, tt := range tests {
		if IsValidTransition(tt.from, tt.to) != tt.expected {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, !tt.expected, tt.expected)
		}
	}
}

func TestTaskRunStatusIllegalTransitions(t *testing.T) {
	illegal := []struct {
		from TaskRunStatus
		to   TaskRunStatus
	}{
		{RunStatusSucceeded, RunStatusRunning},
		{RunStatusFailed, RunStatusRunning},
		{RunStatusCancelled, RunStatusRunning},
		{RunStatusManualIntervention, RunStatusRunning},
		{RunStatusSucceeded, RunStatusFailed},
		{RunStatusFailed, RunStatusSucceeded},
		{RunStatusCancelled, RunStatusSucceeded},
		{RunStatusTimedOut, RunStatusRunning},
	}
	for _, tt := range illegal {
		if IsValidTransition(tt.from, tt.to) {
			t.Errorf("expected transition from %s to %s to be invalid", tt.from, tt.to)
		}
	}
}

func TestTaskRunStatusTerminal(t *testing.T) {
	terminal := []TaskRunStatus{
		RunStatusSucceeded, RunStatusFailed, RunStatusCancelled,
		RunStatusTimedOut, RunStatusManualIntervention,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	nonTerminal := []TaskRunStatus{
		RunStatusCreated, RunStatusQueued, RunStatusStarting, RunStatusRunning,
		RunStatusCheckpointing, RunStatusPausing, RunStatusPaused,
		RunStatusResuming, RunStatusCancelling, RunStatusRecoveryRequired,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

func TestTaskIdempotencyValues(t *testing.T) {
	if Idempotent != "idempotent" {
		t.Errorf("expected idempotent, got %s", Idempotent)
	}
	if ConditionallyIdempotent != "conditionally_idempotent" {
		t.Errorf("expected conditionally_idempotent, got %s", ConditionallyIdempotent)
	}
	if NonIdempotent != "non_idempotent" {
		t.Errorf("expected non_idempotent, got %s", NonIdempotent)
	}
}

func TestTaskRecoverabilityValues(t *testing.T) {
	if NotRecoverable != "not_recoverable" {
		t.Errorf("expected not_recoverable, got %s", NotRecoverable)
	}
	if CheckpointRecoverable != "checkpoint_recoverable" {
		t.Errorf("expected checkpoint_recoverable, got %s", CheckpointRecoverable)
	}
	if RestartableFromBeginning != "restartable_from_beginning" {
		t.Errorf("expected restartable_from_beginning, got %s", RestartableFromBeginning)
	}
	if ManualRecovery != "manual_recovery" {
		t.Errorf("expected manual_recovery, got %s", ManualRecovery)
	}
}

func TestTaskRetryOnSuccess(t *testing.T) {
	executor := NewTaskExecutor(4, "/tmp/tasks")
	task := makeTestTask(t)
	executor.Enqueue(task)
	result := executor.Execute(context.Background(), ExecuteRequest{
		TaskID: "task-1",
		Handler: func(ctx context.Context, task *Task) ([]byte, string, string, error) {
			return []byte(`{"result":"ok"}`), "sha256:out", "", nil
		},
	})
	if result.Status != TaskStateSucceeded {
		t.Fatalf("expected succeeded, got %s", result.Status)
	}
	if result.Output == nil {
		t.Fatal("expected output on success")
	}
}

func TestTaskRetryIdempotent(t *testing.T) {
	def := TaskDefinition{
		TaskID:      "task-retry",
		ExtensionID: "com.example",
		Entry:       "entry.js",
		EntryHash:   "sha256:hash",
		Idempotency: Idempotent,
		RetryPolicy: TaskRetryPolicy{MaxAttempts: 3},
	}
	task, err := NewTask(def, TaskInput{TaskID: "task-retry", InputHash: "h1"}, "op-1", 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.definition.Idempotency != Idempotent {
		t.Fatalf("expected idempotent, got %s", task.definition.Idempotency)
	}
}

func TestTaskRetryNonIdempotent(t *testing.T) {
	def := TaskDefinition{
		TaskID:      "task-no-retry",
		ExtensionID: "com.example",
		Entry:       "entry.js",
		EntryHash:   "sha256:hash",
		Idempotency: NonIdempotent,
	}
	task, err := NewTask(def, TaskInput{TaskID: "task-no-retry", InputHash: "h1"}, "op-1", 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.definition.Idempotency != NonIdempotent {
		t.Fatalf("expected non_idempotent, got %s", task.definition.Idempotency)
	}
}

func TestTaskCheckpointPayloadHash(t *testing.T) {
	task := makeTestTask(t)
	payload := json.RawMessage(`{"cursor":100}`)
	expectedHash := hashBytes(payload)
	cp := Checkpoint{
		CheckpointID: "cp-hash",
		Data:         payload,
		Hash:         expectedHash,
	}
	if err := task.SaveCheckpoint(cp); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	latest := task.LatestCheckpoint()
	if latest == nil {
		t.Fatal("expected latest checkpoint")
	}
	actualHash := hashBytes(latest.Data)
	if actualHash != expectedHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, actualHash)
	}
}

func TestTaskCancelSignal(t *testing.T) {
	task := makeTestTask(t)
	if task.CancelSignal().IsCancelled() {
		t.Fatal("task should not be cancelled initially")
	}
	task.Cancel("test cancel")
	if !task.CancelSignal().IsCancelled() {
		t.Fatal("task should be cancelled after Cancel")
	}
	if task.CancelSignal().Reason() != "test cancel" {
		t.Fatalf("expected reason 'test cancel', got %s", task.CancelSignal().Reason())
	}
}

func TestTaskResourceLimits(t *testing.T) {
	limits := DefaultTaskResourceLimits()
	if limits.MaxMemoryMB != 512 {
		t.Fatalf("expected 512 MB, got %d", limits.MaxMemoryMB)
	}
	if limits.MaxCPUPercent != 50 {
		t.Fatalf("expected 50%% CPU, got %d", limits.MaxCPUPercent)
	}
	if limits.MaxConcurrentTasks != 4 {
		t.Fatalf("expected 4 concurrent tasks, got %d", limits.MaxConcurrentTasks)
	}
}

func TestTaskTimeoutPolicy(t *testing.T) {
	policy := DefaultTimeoutPolicy()
	if policy.DefaultTimeout != 30*time.Minute {
		t.Fatalf("expected 30 min default timeout, got %v", policy.DefaultTimeout)
	}
	if policy.MaxTimeout != 24*time.Hour {
		t.Fatalf("expected 24h max timeout, got %v", policy.MaxTimeout)
	}
	if policy.HardKillAfter != 30*time.Second {
		t.Fatalf("expected 30s hard kill, got %v", policy.HardKillAfter)
	}
}

func TestMustTransitionRejectsInvalid(t *testing.T) {
	err := MustTransition(RunStatusSucceeded, RunStatusRunning)
	if err == nil {
		t.Fatal("expected error for succeeded -> running")
	}
}

func TestMustTransitionAcceptsValid(t *testing.T) {
	err := MustTransition(RunStatusCreated, RunStatusQueued)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskLegacyBooleans(t *testing.T) {
	def := TaskDefinition{
		TaskID:      "task-legacy",
		ExtensionID: "com.example",
		Entry:       "entry.js",
		Checkpoint:  true,
		Idempotent:  true,
		Recoverable: true,
	}
	if !def.Checkpoint {
		t.Fatal("checkpoint should be true")
	}
	if !def.Idempotent {
		t.Fatal("idempotent should be true")
	}
	if !def.Recoverable {
		t.Fatal("recoverable should be true")
	}
}

func TestResultPolicyValues(t *testing.T) {
	if ResultInlineJSON != "inline_json" {
		t.Errorf("expected inline_json, got %s", ResultInlineJSON)
	}
	if ResultArtifact != "artifact" {
		t.Errorf("expected artifact, got %s", ResultArtifact)
	}
	if ResultAuto != "auto" {
		t.Errorf("expected auto, got %s", ResultAuto)
	}
}

func TestCleanupPolicyValues(t *testing.T) {
	if CleanupAlways != "always" {
		t.Errorf("expected always, got %s", CleanupAlways)
	}
	if CleanupOnSuccess != "on_success" {
		t.Errorf("expected on_success, got %s", CleanupOnSuccess)
	}
	if CleanupOnFailure != "on_failure" {
		t.Errorf("expected on_failure, got %s", CleanupOnFailure)
	}
	if CleanupRetainForDebug != "retain_for_debug" {
		t.Errorf("expected retain_for_debug, got %s", CleanupRetainForDebug)
	}
}

func TestDefaultTaskRuntimeConfig(t *testing.T) {
	cfg := DefaultTaskRuntimeConfig()
	if cfg.GlobalMaxConcurrent != 4 {
		t.Fatalf("expected 4 global max, got %d", cfg.GlobalMaxConcurrent)
	}
	if cfg.PerExtensionMaxConcurrent != 2 {
		t.Fatalf("expected 2 per extension, got %d", cfg.PerExtensionMaxConcurrent)
	}
	if cfg.PerDefinitionMaxConcurrent != 1 {
		t.Fatalf("expected 1 per definition, got %d", cfg.PerDefinitionMaxConcurrent)
	}
	if cfg.MaxRetryAttempts != 3 {
		t.Fatalf("expected 3 max retry attempts, got %d", cfg.MaxRetryAttempts)
	}
}
