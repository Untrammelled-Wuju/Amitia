package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type TaskRuntimeExecutor interface {
	Execute(ctx context.Context, taskType string, input map[string]interface{}) (json.RawMessage, error)
}

type defaultTaskWorker struct {
	client       *MeshClient
	taskRuntime  TaskRuntimeExecutor
	mu           sync.Mutex
	cancelFns    map[string]context.CancelFunc
	progressSeq  map[string]int64
	heartbeatSeq map[string]int64
}

func NewTaskWorker(client *MeshClient) *defaultTaskWorker {
	return &defaultTaskWorker{
		client:       client,
		cancelFns:    make(map[string]context.CancelFunc),
		progressSeq:  make(map[string]int64),
		heartbeatSeq: make(map[string]int64),
	}
}

func (w *defaultTaskWorker) SetTaskRuntime(tr TaskRuntimeExecutor) {
	w.taskRuntime = tr
}

func (w *defaultTaskWorker) ExecuteTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) error {
	log.Printf("devicemesh: agent: task dispatch received: taskRunId=%s attemptId=%s",
		dispatch.TaskRunID, dispatch.AttemptID)

	if w.client == nil {
		return fmt.Errorf("task worker mesh client is not configured")
	}
	const leaseDurationMs = int64(5 * time.Minute / time.Millisecond)
	w.client.sendTaskClaim(
		dispatch.TaskRunID,
		dispatch.AttemptID,
		dispatch.LeaseID,
		w.client.conf.Identity.RuntimeID.String(),
		leaseDurationMs,
	)

	taskCtx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancelFns[dispatch.TaskRunID] = cancel
	w.mu.Unlock()

	go w.runHeartbeat(taskCtx, dispatch)
	go func() {
		defer cancel()
		w.runTask(taskCtx, dispatch)
	}()

	return nil
}

func (w *defaultTaskWorker) CancelTask(ctx context.Context, taskRunID, attemptID, leaseID string) error {
	log.Printf("devicemesh: agent: task cancel received: taskRunId=%s attemptId=%s", taskRunID, attemptID)

	w.mu.Lock()
	cancel, ok := w.cancelFns[taskRunID]
	delete(w.cancelFns, taskRunID)
	w.mu.Unlock()

	if ok && cancel != nil {
		cancel()
		return nil
	}
	if w.client != nil {
		w.client.sendTaskComplete(taskRunID, attemptID, leaseID, false, nil, "task was not running on device")
	}
	return nil
}

func (w *defaultTaskWorker) runTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) {
	defer func() {
		w.mu.Lock()
		delete(w.cancelFns, dispatch.TaskRunID)
		delete(w.progressSeq, dispatch.TaskRunID)
		delete(w.heartbeatSeq, dispatch.TaskRunID)
		w.mu.Unlock()
	}()

	w.reportProgress(ctx, dispatch, 0, nil, nil, nil, "starting", "task execution started")

	result, err := w.executeTaskByType(ctx, dispatch)

	select {
	case <-ctx.Done():
		w.client.sendTaskComplete(
			dispatch.TaskRunID,
			dispatch.AttemptID,
			dispatch.LeaseID,
			false,
			nil,
			"context cancelled",
		)
		return
	default:
	}

	if err != nil {
		log.Printf("devicemesh: agent: task execution failed: taskRunId=%s err=%v", dispatch.TaskRunID, err)
		w.client.sendTaskComplete(
			dispatch.TaskRunID,
			dispatch.AttemptID,
			dispatch.LeaseID,
			false,
			nil,
			err.Error(),
		)
		return
	}

	w.client.sendTaskComplete(
		dispatch.TaskRunID,
		dispatch.AttemptID,
		dispatch.LeaseID,
		true,
		result,
		"",
	)
}

func (w *defaultTaskWorker) executeTaskByType(ctx context.Context, dispatch protocol.TaskDispatchPayload) (json.RawMessage, error) {
	var input map[string]interface{}
	if len(dispatch.Input) > 0 {
		if err := json.Unmarshal(dispatch.Input, &input); err != nil {
			return nil, fmt.Errorf("invalid task input: %w", err)
		}
	}

	taskType, _ := input["taskType"].(string)
	if taskType == "" {
		taskType = dispatch.TaskDefinitionID
	}
	log.Printf("devicemesh: agent: executing task: taskRunId=%s taskType=%s", dispatch.TaskRunID, taskType)

	if taskType == "" {
		return nil, fmt.Errorf("missing task definition id")
	}

	w.reportProgress(ctx, dispatch, 1, float64Ptr(0), float64Ptr(100), float64Ptr(0), "executing", fmt.Sprintf("executing task type: %s", taskType))

	result, err := w.dispatchTaskExecution(ctx, taskType, input)
	if err != nil {
		return nil, err
	}

	w.reportProgress(ctx, dispatch, 3, float64Ptr(100), float64Ptr(100), float64Ptr(100), "completing", "task completing")

	return result, nil
}

func (w *defaultTaskWorker) dispatchTaskExecution(ctx context.Context, taskType string, input map[string]interface{}) (json.RawMessage, error) {
	if w.taskRuntime != nil {
		result, err := w.taskRuntime.Execute(ctx, taskType, input)
		if err == nil {
			return result, nil
		}
		return nil, fmt.Errorf("taskRuntime.Execute failed for type %s: %w", taskType, err)
	}

	switch taskType {
	case "ping":
		result := map[string]interface{}{
			"taskType":  taskType,
			"completed": true,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		return json.Marshal(result)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
}

func (w *defaultTaskWorker) runHeartbeat(ctx context.Context, dispatch protocol.TaskDispatchPayload) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			w.heartbeatSeq[dispatch.TaskRunID]++
			seq := w.heartbeatSeq[dispatch.TaskRunID]
			w.mu.Unlock()
			if w.client != nil {
				w.client.sendTaskHeartbeat(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, seq)
			}
		}
	}
}

func (w *defaultTaskWorker) reportProgress(ctx context.Context, dispatch protocol.TaskDispatchPayload, seq int64, current, total, percentage *float64, stage, message string) {
	w.mu.Lock()
	nextSeq := w.progressSeq[dispatch.TaskRunID] + 1
	if seq > nextSeq {
		nextSeq = seq
	}
	w.progressSeq[dispatch.TaskRunID] = nextSeq
	finalSeq := nextSeq
	w.mu.Unlock()

	select {
	case <-ctx.Done():
		return
	default:
	}

	w.client.sendTaskProgress(
		dispatch.TaskRunID,
		dispatch.AttemptID,
		dispatch.LeaseID,
		finalSeq,
		current,
		total,
		percentage,
		stage,
		message,
	)
}

func float64Ptr(v float64) *float64 {
	return &v
}
