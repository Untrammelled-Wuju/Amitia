package task_runtime

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type TaskState string

const (
	TaskStateCreated           TaskState = "created"
	TaskStateQueued            TaskState = "queued"
	TaskStateStarting          TaskState = "starting"
	TaskStateRunning           TaskState = "running"
	TaskStateCheckpointing     TaskState = "checkpointing"
	TaskStatePaused            TaskState = "paused"
	TaskStateSucceeded         TaskState = "succeeded"
	TaskStateFailed            TaskState = "failed"
	TaskStateCancelled         TaskState = "cancelled"
	TaskStateTimedOut          TaskState = "timed_out"
	TaskStateRecoveryRequired  TaskState = "recovery_required"
)

func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateSucceeded, TaskStateFailed, TaskStateCancelled, TaskStateTimedOut, TaskStateRecoveryRequired:
		return true
	}
	return false
}

type TaskDefinition struct {
	TaskID                string
	ExtensionID           string
	ModuleID              string
	RuntimeType           string
	Entry                 string
	EntryHash             string
	InputSchema           json.RawMessage
	OutputSchema          json.RawMessage
	Checkpoint            bool
	Idempotent            bool
	Recoverable           bool
	ResourceLimits        TaskResourceLimits
	PermissionRequirements []string
	AllowedNamespaces     []string
	DefinitionVersion     int
	MaxDuration           time.Duration
}

type TaskResourceLimits struct {
	MaxMemoryMB       int
	MaxCPUPercent     int
	MaxDiskMB         int
	MaxOutputSizeMB   int
	MaxLogSizeMB      int
	MaxConcurrentTasks int
}

func DefaultTaskResourceLimits() TaskResourceLimits {
	return TaskResourceLimits{
		MaxMemoryMB:        512,
		MaxCPUPercent:      50,
		MaxDiskMB:          256,
		MaxOutputSizeMB:    64,
		MaxLogSizeMB:       16,
		MaxConcurrentTasks: 4,
	}
}

type TaskInput struct {
	TaskID    string
	Input     json.RawMessage
	InputHash string
}

type TaskResult struct {
	TaskID      string
	Status      TaskState
	Output      json.RawMessage
	OutputHash  string
	ArtifactRef string
	Error       string
	StartedAt   time.Time
	FinishedAt  *time.Time
	Duration    time.Duration
	Checkpoints []Checkpoint
}

type Checkpoint struct {
	CheckpointID   string
	TaskID         string
	Version        int
	Data           json.RawMessage
	Hash           string
	SavedAt        time.Time
	DefinitionHash string
}

type Progress struct {
	Current int
	Total   int
	Message string
	Metadata map[string]interface{}
}

type Task struct {
	mu              sync.RWMutex
	definition      TaskDefinition
	input           TaskInput
	state           TaskState
	progress        Progress
	checkpoints     []Checkpoint
	result          *TaskResult
	startedAt       *time.Time
	finishedAt      *time.Time
	cancelSignal    *CancelSignal
	workspacePath   string
	operationID     string
	priority        int
	queueReason     string
	scopeSnapshot   string
	permissionSnap  []string
	dependencySnap  string
}

type CancelSignal struct {
	mu     sync.Mutex
	done   chan struct{}
	reason string
}

func NewCancelSignal() *CancelSignal {
	return &CancelSignal{done: make(chan struct{})}
}

func (c *CancelSignal) Cancel(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
	default:
		c.reason = reason
		close(c.done)
	}
}

func (c *CancelSignal) Done() <-chan struct{} { return c.done }
func (c *CancelSignal) Reason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reason
}
func (c *CancelSignal) IsCancelled() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func NewTask(def TaskDefinition, input TaskInput, operationID string, priority int) (*Task, error) {
	if def.TaskID == "" {
		return nil, errors.New("task_runtime: task id required")
	}
	if def.ExtensionID == "" {
		return nil, errors.New("task_runtime: extension id required")
	}
	if def.Entry == "" {
		return nil, errors.New("task_runtime: entry required")
	}
	if input.TaskID != def.TaskID {
		return nil, errors.New("task_runtime: input task id mismatch")
	}
	return &Task{
		definition:   def,
		input:        input,
		state:        TaskStateCreated,
		cancelSignal: NewCancelSignal(),
		operationID:  operationID,
		priority:     priority,
	}, nil
}

func (t *Task) Definition() TaskDefinition {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.definition
}

func (t *Task) State() TaskState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *Task) SetState(state TaskState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = state
}

func (t *Task) Progress() Progress {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.progress
}

func (t *Task) ReportProgress(p Progress) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress = p
}

func (t *Task) SaveCheckpoint(cp Checkpoint) error {
	if !t.definition.Checkpoint {
		return errors.New("task_runtime: checkpoints not enabled for this task")
	}
	if cp.Data == nil {
		return errors.New("task_runtime: checkpoint data required")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	cp.SavedAt = time.Now().UTC()
	cp.TaskID = t.definition.TaskID
	if cp.Version == 0 {
		cp.Version = len(t.checkpoints) + 1
	}
	t.checkpoints = append(t.checkpoints, cp)
	return nil
}

func (t *Task) LatestCheckpoint() *Checkpoint {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.checkpoints) == 0 {
		return nil
	}
	cp := t.checkpoints[len(t.checkpoints)-1]
	return &cp
}

func (t *Task) Cancel(reason string) {
	t.cancelSignal.Cancel(reason)
}

func (t *Task) CancelSignal() *CancelSignal { return t.cancelSignal }

func (t *Task) Succeed(output json.RawMessage, outputHash, artifactRef string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.state = TaskStateSucceeded
	t.finishedAt = &now
	t.result = &TaskResult{
		TaskID:      t.definition.TaskID,
		Status:      TaskStateSucceeded,
		Output:      output,
		OutputHash:  outputHash,
		ArtifactRef: artifactRef,
		FinishedAt:  &now,
		Checkpoints: t.checkpoints,
	}
	if t.startedAt != nil {
		t.result.Duration = now.Sub(*t.startedAt)
		t.result.StartedAt = *t.startedAt
	}
}

func (t *Task) Fail(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.state = TaskStateFailed
	t.finishedAt = &now
	t.result = &TaskResult{
		TaskID:     t.definition.TaskID,
		Status:     TaskStateFailed,
		Error:      err,
		FinishedAt: &now,
		Checkpoints: t.checkpoints,
	}
	if t.startedAt != nil {
		t.result.Duration = now.Sub(*t.startedAt)
		t.result.StartedAt = *t.startedAt
	}
}

func (t *Task) Cancelled(reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.state = TaskStateCancelled
	t.finishedAt = &now
	t.result = &TaskResult{
		TaskID:     t.definition.TaskID,
		Status:     TaskStateCancelled,
		Error:      reason,
		FinishedAt: &now,
		Checkpoints: t.checkpoints,
	}
	if t.startedAt != nil {
		t.result.Duration = now.Sub(*t.startedAt)
		t.result.StartedAt = *t.startedAt
	}
}

func (t *Task) TimedOut() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.state = TaskStateTimedOut
	t.finishedAt = &now
	t.result = &TaskResult{
		TaskID:     t.definition.TaskID,
		Status:     TaskStateTimedOut,
		Error:      "task exceeded max duration",
		FinishedAt: &now,
		Checkpoints: t.checkpoints,
	}
	if t.startedAt != nil {
		t.result.Duration = now.Sub(*t.startedAt)
		t.result.StartedAt = *t.startedAt
	}
}

func (t *Task) Result() *TaskResult {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.result
}

func (t *Task) OperationID() string { return t.operationID }
func (t *Task) Priority() int       { return t.priority }

func (t *Task) SetWorkspace(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.workspacePath = path
}

func (t *Task) Workspace() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.workspacePath
}

func (t *Task) MarkStarted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.startedAt = &now
	t.state = TaskStateRunning
}

func (t *Task) StartedAt() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.startedAt
}

func (t *Task) FinishedAt() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.finishedAt
}

func (t *Task) SetScopeSnapshot(snap string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scopeSnapshot = snap
}

func (t *Task) SetPermissionSnapshot(snap []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.permissionSnap = snap
}

func (t *Task) SetDependencySnapshot(snap string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dependencySnap = snap
}

func (t *Task) ValidateRecovery(other *Task) error {
	if t.definition.TaskID != other.definition.TaskID {
		return errors.New("task_runtime: task id mismatch")
	}
	if t.definition.EntryHash != other.definition.EntryHash {
		return errors.New("task_runtime: entry hash mismatch")
	}
	if t.input.InputHash != other.input.InputHash {
		return errors.New("task_runtime: input hash mismatch")
	}
	if !t.definition.Recoverable {
		return errors.New("task_runtime: task not recoverable")
	}
	return nil
}
