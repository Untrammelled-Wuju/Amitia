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

type defaultTaskWorker struct {
	client     *MeshClient
	mu         sync.Mutex
	cancelFns  map[string]context.CancelFunc
	progressSeq map[string]int64
}

func NewTaskWorker(client *MeshClient) *defaultTaskWorker {
	return &defaultTaskWorker{
		client:      client,
		cancelFns:   make(map[string]context.CancelFunc),
		progressSeq: make(map[string]int64),
	}
}

func (w *defaultTaskWorker) ExecuteTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) error {
	log.Printf("devicemesh: agent: task dispatch received: taskRunId=%s attemptId=%s",
		dispatch.TaskRunID, dispatch.AttemptID)

	w.client.sendTaskClaim(
		dispatch.TaskRunID,
		dispatch.AttemptID,
		dispatch.LeaseID,
		w.client.conf.Identity.RuntimeID.String(),
		60000,
	)

	taskCtx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancelFns[dispatch.TaskRunID] = cancel
	w.mu.Unlock()

	go w.runTask(taskCtx, dispatch)

	return nil
}

func (w *defaultTaskWorker) CancelTask(ctx context.Context, taskRunID, attemptID string) error {
	log.Printf("devicemesh: agent: task cancel received: taskRunId=%s attemptId=%s", taskRunID, attemptID)

	w.mu.Lock()
	cancel, ok := w.cancelFns[taskRunID]
	delete(w.cancelFns, taskRunID)
	w.mu.Unlock()

	if ok && cancel != nil {
		cancel()
	}

	w.client.sendTaskComplete(taskRunID, attemptID, "", false, nil, "task cancelled by server")
	return nil
}

func (w *defaultTaskWorker) runTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) {
	defer func() {
		w.mu.Lock()
		delete(w.cancelFns, dispatch.TaskRunID)
		delete(w.progressSeq, dispatch.TaskRunID)
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
	log.Printf("devicemesh: agent: executing task: taskRunId=%s taskType=%s", dispatch.TaskRunID, taskType)

	w.reportProgress(ctx, dispatch, 1, float64Ptr(0), float64Ptr(100), float64Ptr(0), "executing", fmt.Sprintf("executing task type: %s", taskType))

	w.reportProgress(ctx, dispatch, 2, float64Ptr(50), float64Ptr(100), float64Ptr(50), "executing", "task in progress")

	w.reportProgress(ctx, dispatch, 3, float64Ptr(100), float64Ptr(100), float64Ptr(100), "completing", "task completing")

	result := map[string]interface{}{
		"taskType":  taskType,
		"completed": true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	resultBytes, _ := json.Marshal(result)
	return resultBytes, nil
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
