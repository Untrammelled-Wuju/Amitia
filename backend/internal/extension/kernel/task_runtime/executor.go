package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type TaskExecutor struct {
	mu            sync.RWMutex
	tasks         map[string]*Task
	queue         []*Task
	running       map[string]*Task
	maxConcurrent int
	workspaceRoot string
	completed     []*Task
}

func NewTaskExecutor(maxConcurrent int, workspaceRoot string) *TaskExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &TaskExecutor{
		tasks:         make(map[string]*Task),
		running:       make(map[string]*Task),
		maxConcurrent: maxConcurrent,
		workspaceRoot: workspaceRoot,
	}
}

type TaskExecuteHandler func(ctx context.Context, task *Task) (output []byte, outputHash string, artifactRef string, err error)

type EnqueueResult struct {
	TaskID      string
	OperationID string
	Queued      bool
	Position    int
	Reason      string
}

func (e *TaskExecutor) Enqueue(task *Task) EnqueueResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.tasks[task.definition.TaskID]; exists {
		return EnqueueResult{TaskID: task.definition.TaskID, Reason: "task already exists"}
	}
	activeCount := 0
	for _, existing := range e.tasks {
		switch existing.State() {
		case TaskStateCreated, TaskStateStarting, TaskStateRunning:
			activeCount++
		}
	}
	e.tasks[task.definition.TaskID] = task
	if activeCount >= e.maxConcurrent {
		task.SetState(TaskStateQueued)
		e.queue = append(e.queue, task)
		sort.Slice(e.queue, func(i, j int) bool {
			return e.queue[i].Priority() > e.queue[j].Priority()
		})
		return EnqueueResult{
			TaskID:      task.definition.TaskID,
			OperationID: task.operationID,
			Queued:      true,
			Position:    len(e.queue),
		}
	}
	return EnqueueResult{
		TaskID:      task.definition.TaskID,
		OperationID: task.operationID,
		Queued:      false,
	}
}

type ExecuteRequest struct {
	TaskID  string
	Handler TaskExecuteHandler
}

type ExecuteResult struct {
	TaskID   string
	Status   TaskState
	Output   []byte
	Error    string
	Duration time.Duration
}

func (e *TaskExecutor) Execute(ctx context.Context, req ExecuteRequest) ExecuteResult {
	e.mu.Lock()
	task, exists := e.tasks[req.TaskID]
	if !exists {
		e.mu.Unlock()
		return ExecuteResult{TaskID: req.TaskID, Status: TaskStateFailed, Error: "task not found"}
	}
	if _, running := e.running[req.TaskID]; running {
		e.mu.Unlock()
		return ExecuteResult{TaskID: req.TaskID, Status: TaskStateFailed, Error: "task already running"}
	}
	e.running[req.TaskID] = task
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.running, req.TaskID)
		if task.State().IsTerminal() {
			e.completed = append(e.completed, task)
		}
		e.mu.Unlock()
		e.dispatchNext(ctx)
	}()

	task.SetState(TaskStateStarting)
	task.MarkStarted()

	maxDuration := task.definition.MaxDuration
	if maxDuration <= 0 {
		maxDuration = 30 * time.Minute
	}

	taskCtx, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	go func() {
		select {
		case <-taskCtx.Done():
			if taskCtx.Err() == context.DeadlineExceeded {
				task.TimedOut()
			}
		case <-task.CancelSignal().Done():
			task.Cancelled(task.CancelSignal().Reason())
		}
	}()

	if task.CancelSignal().IsCancelled() {
		return ExecuteResult{
			TaskID: req.TaskID,
			Status: TaskStateCancelled,
			Error:  task.CancelSignal().Reason(),
		}
	}

	output, outputHash, artifactRef, err := req.Handler(taskCtx, task)
	if err != nil {
		task.Fail(err.Error())
		return ExecuteResult{
			TaskID:   req.TaskID,
			Status:   TaskStateFailed,
			Error:    err.Error(),
			Duration: time.Since(*task.StartedAt()),
		}
	}

	if task.State() == TaskStateCancelled || task.State() == TaskStateTimedOut {
		finalState := task.State()
		return ExecuteResult{
			TaskID:   req.TaskID,
			Status:   finalState,
			Duration: time.Since(*task.StartedAt()),
		}
	}

	task.Succeed(output, outputHash, artifactRef)
	result := task.Result()
	return ExecuteResult{
		TaskID:   req.TaskID,
		Status:   TaskStateSucceeded,
		Output:   result.Output,
		Duration: result.Duration,
	}
}

func (e *TaskExecutor) dispatchNext(ctx context.Context) {
	e.mu.Lock()
	if len(e.queue) == 0 || len(e.running) >= e.maxConcurrent {
		e.mu.Unlock()
		return
	}
	e.queue = e.queue[1:]
	e.mu.Unlock()
}

func (e *TaskExecutor) Cancel(taskID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task_runtime: task %s not found", taskID)
	}
	task.Cancel(reason)
	if task.State() == TaskStateQueued {
		task.Cancelled(reason)
		for i, q := range e.queue {
			if q.definition.TaskID == taskID {
				e.queue = append(e.queue[:i], e.queue[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (e *TaskExecutor) Get(taskID string) (*Task, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	task, exists := e.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task_runtime: task %s not found", taskID)
	}
	return task, nil
}

func (e *TaskExecutor) List() []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Task, 0, len(e.tasks))
	for _, t := range e.tasks {
		result = append(result, t)
	}
	return result
}

func (e *TaskExecutor) ListByExtension(extensionID string) []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Task
	for _, t := range e.tasks {
		if t.definition.ExtensionID == extensionID {
			result = append(result, t)
		}
	}
	return result
}

func (e *TaskExecutor) RunningCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.running)
}

func (e *TaskExecutor) QueuedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.queue)
}

func (e *TaskExecutor) CompletedCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.completed)
}

func (e *TaskExecutor) Cleanup(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task_runtime: task %s not found", taskID)
	}
	if !task.State().IsTerminal() {
		return errors.New("task_runtime: cannot cleanup non-terminal task")
	}
	delete(e.tasks, taskID)
	for i, c := range e.completed {
		if c.definition.TaskID == taskID {
			e.completed = append(e.completed[:i], e.completed[i+1:]...)
			break
		}
	}
	return nil
}

func (e *TaskExecutor) Recover(ctx context.Context, taskID string, handler TaskExecuteHandler) ExecuteResult {
	e.mu.Lock()
	task, exists := e.tasks[taskID]
	if !exists {
		e.mu.Unlock()
		return ExecuteResult{TaskID: taskID, Status: TaskStateFailed, Error: "task not found"}
	}
	if !task.definition.Recoverable {
		e.mu.Unlock()
		return ExecuteResult{TaskID: taskID, Status: TaskStateFailed, Error: "task not recoverable"}
	}
	cp := task.LatestCheckpoint()
	if cp == nil {
		e.mu.Unlock()
		return ExecuteResult{TaskID: taskID, Status: TaskStateFailed, Error: "no checkpoint to recover from"}
	}
	e.mu.Unlock()

	task.SetState(TaskStateStarting)
	return e.Execute(ctx, ExecuteRequest{TaskID: taskID, Handler: handler})
}
