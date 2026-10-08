package agent

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type TaskRuntimeExecutor interface {
	Execute(ctx context.Context, taskType string, input map[string]interface{}) (json.RawMessage, error)
}

type OwnedTaskDispatchExecutor interface {
	ExecuteOwnedDispatch(context.Context, protocol.TaskDispatchPayload) (json.RawMessage, error)
}

type OwnedTaskDispatchOutcomeExecutor interface {
	ExecuteOwnedDispatchOutcome(context.Context, protocol.TaskDispatchPayload) (protocol.OwnedTaskExecutionOutcome, error)
}

type OwnedTaskPauseExecutor interface {
	PauseOwnedDispatch(context.Context, protocol.TaskPausePayload) error
}

type defaultTaskWorker struct {
	client       *MeshClient
	taskRuntime  TaskRuntimeExecutor
	mu           sync.Mutex
	cancelFns    map[string]taskCancellation
	progressSeq  map[string]int64
	heartbeatSeq map[string]int64
}

type taskCancellation struct {
	attempt, lease string
	cancel         context.CancelFunc
}

func NewTaskWorker(client *MeshClient) *defaultTaskWorker {
	return &defaultTaskWorker{
		client:       client,
		cancelFns:    make(map[string]taskCancellation),
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
	if err := w.validateTaskAuthority(ctx, dispatch); err != nil {
		return err
	}
	taskCtx, cancel := context.WithCancel(ctx)
	if dispatch.DeadlineAt != nil {
		cancel()
		taskCtx, cancel = context.WithDeadline(ctx, *dispatch.DeadlineAt)
	}
	w.mu.Lock()
	if _, exists := w.cancelFns[dispatch.TaskRunID]; exists {
		w.mu.Unlock()
		cancel()
		return fmt.Errorf("任务已在当前设备执行，请勿重复派发")
	}
	w.cancelFns[dispatch.TaskRunID] = taskCancellation{dispatch.AttemptID, dispatch.LeaseID, cancel}
	w.mu.Unlock()
	lease := &taskLeaseTimer{cancel: cancel}
	if err := w.client.awaitTaskLease(taskCtx, dispatch, 0, lease.Confirm); err != nil {
		w.mu.Lock()
		if entry, exists := w.cancelFns[dispatch.TaskRunID]; exists && entry.attempt == dispatch.AttemptID && entry.lease == dispatch.LeaseID {
			delete(w.cancelFns, dispatch.TaskRunID)
		}
		w.mu.Unlock()
		cancel()
		lease.Close()
		return err
	}

	go w.runHeartbeat(taskCtx, dispatch, lease)
	go func() {
		defer cancel()
		defer lease.Close()
		w.runTask(taskCtx, dispatch)
	}()

	return nil
}

func (w *defaultTaskWorker) CancelTask(ctx context.Context, taskRunID, attemptID, leaseID string) error {
	log.Printf("devicemesh: agent: task cancel received: taskRunId=%s attemptId=%s", taskRunID, attemptID)

	w.mu.Lock()
	entry, ok := w.cancelFns[taskRunID]
	if ok && (entry.attempt != attemptID || entry.lease != leaseID) {
		w.mu.Unlock()
		return fmt.Errorf("任务取消请求不属于当前执行租约")
	}
	w.mu.Unlock()

	if ok && entry.cancel != nil {
		entry.cancel()
		return nil
	}
	return fmt.Errorf("设备缺少可确认的当前任务执行，不能确认任务已停止")
}

func (w *defaultTaskWorker) runTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) {
	defer func() {
		w.mu.Lock()
		if entry, exists := w.cancelFns[dispatch.TaskRunID]; exists && entry.attempt == dispatch.AttemptID && entry.lease == dispatch.LeaseID {
			delete(w.cancelFns, dispatch.TaskRunID)
			delete(w.progressSeq, dispatch.TaskRunID)
			delete(w.heartbeatSeq, dispatch.TaskRunID)
		}
		w.mu.Unlock()
	}()

	if _, full := w.taskRuntime.(OwnedTaskDispatchOutcomeExecutor); !full || len(dispatch.OwnedExecutionScope) == 0 {
		w.reportProgress(ctx, dispatch, 0, nil, nil, nil, "starting", "task execution started")
	}

	result, err := w.executeAuthorizedTask(ctx, dispatch)

	select {
	case <-ctx.Done():
		if _, full := w.taskRuntime.(OwnedTaskDispatchOutcomeExecutor); full && len(dispatch.OwnedExecutionScope) > 0 {
			w.client.sendOwnedTaskUnknown(dispatch)
			return
		}
		w.client.sendTaskComplete(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, false, nil, "context cancelled", dispatch)
		return
	default:
	}
	if err != nil {
		log.Printf("devicemesh: agent: task execution failed: taskRunId=%s err=%v", dispatch.TaskRunID, err)
		if _, full := w.taskRuntime.(OwnedTaskDispatchOutcomeExecutor); full && len(dispatch.OwnedExecutionScope) > 0 {
			w.client.sendOwnedTaskUnknown(dispatch)
			return
		}
		w.client.sendTaskComplete(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, false, nil, err.Error(), dispatch)
		return
	}
	if _, full := w.taskRuntime.(OwnedTaskDispatchOutcomeExecutor); full && len(dispatch.OwnedExecutionScope) > 0 {
		var outcome protocol.OwnedTaskExecutionOutcome
		if json.Unmarshal(result, &outcome) != nil || outcome.PausedCheckpointVersion < 0 || outcome.PausedCheckpointVersion > 9007199254740991 || outcome.PausedCheckpointVersion > 0 && (len(outcome.Result) != 0 || outcome.ResultArtifactID != "") || outcome.PausedCheckpointVersion == 0 && (outcome.ResultArtifactID == "" && !json.Valid(outcome.Result) || outcome.ResultArtifactID != "" && len(outcome.Result) != 0) {
			w.client.sendOwnedTaskUnknown(dispatch)
			return
		}
		if outcome.PausedCheckpointVersion > 0 {
			w.mu.Lock()
			if entry, exists := w.cancelFns[dispatch.TaskRunID]; exists && entry.attempt == dispatch.AttemptID && entry.lease == dispatch.LeaseID {
				delete(w.cancelFns, dispatch.TaskRunID)
				delete(w.progressSeq, dispatch.TaskRunID)
				delete(w.heartbeatSeq, dispatch.TaskRunID)
			}
			w.mu.Unlock()
		}
		w.client.sendOwnedTaskComplete(dispatch, outcome)
		return
	}
	w.client.sendTaskComplete(dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID, true, result, "", dispatch)
}

func (w *defaultTaskWorker) PauseTask(ctx context.Context, request protocol.TaskPausePayload) error {
	w.mu.Lock()
	entry, exists := w.cancelFns[request.TaskRunID]
	w.mu.Unlock()
	if !exists || entry.attempt != request.AttemptID || entry.lease != request.LeaseID {
		return fmt.Errorf("任务暂停请求不属于当前设备执行")
	}
	executor, supported := w.taskRuntime.(OwnedTaskPauseExecutor)
	if !supported {
		return fmt.Errorf("设备任务执行器不支持已确认暂停")
	}
	return executor.PauseOwnedDispatch(ctx, request)
}

func (w *defaultTaskWorker) executeAuthorizedTask(ctx context.Context, dispatch protocol.TaskDispatchPayload) (json.RawMessage, error) {
	if err := w.validateTaskAuthority(ctx, dispatch); err != nil {
		return nil, err
	}
	var err error
	invoke := w.taskAuthorityInvocation(dispatch)
	execute := func(current context.Context) (*protocol.RuntimeResultPayload, error) {
		run := func() (*protocol.RuntimeResultPayload, error) {
			if err := w.validateTaskConnection(dispatch); err != nil {
				return nil, err
			}
			current = context.WithValue(current, taskOwnerRPCKey{}, taskOwnerRPC(func(ctx context.Context, id, method string, params json.RawMessage) (json.RawMessage, error) {
				return w.client.callTaskOwner(ctx, dispatch, id, method, params)
			}))
			output, runErr := w.executeTaskByType(current, dispatch)
			if runErr != nil {
				return nil, runErr
			}
			return &protocol.RuntimeResultPayload{Status: "completed", Result: output}, nil
		}
		if journal := w.client.conf.ExecutionJournal; journal != nil {
			if err := w.validateTaskConnection(dispatch); err != nil {
				return nil, err
			}
			return journal.Execute(current, invoke, run)
		}
		return run()
	}
	var saved *protocol.RuntimeResultPayload
	if guard := w.client.conf.ExecutionGuard; guard != nil {
		saved, err = guard(ctx, invoke, execute)
	} else if len(dispatch.OwnedExecutionScope) != 0 || dispatch.AuthorityCallID != "" {
		err = fmt.Errorf("设备缺少任务授权校验端口")
	} else {
		saved, err = execute(ctx)
	}
	if saved != nil {
		if err := w.validateTaskConnection(dispatch); err != nil {
			return nil, err
		}
		return saved.Result, err
	}
	return nil, err
}

func (w *defaultTaskWorker) taskAuthorityInvocation(dispatch protocol.TaskDispatchPayload) protocol.RuntimeInvokePayload {
	input := dispatch.Input
	if len(dispatch.TargetDefinitionPin) > 0 {
		input, _ = json.Marshal(struct {
			Input      json.RawMessage `json:"input"`
			Definition json.RawMessage `json:"targetDefinitionPin"`
			Root       json.RawMessage `json:"rootTaskMetadata,omitempty"`
			Checkpoint json.RawMessage `json:"resumeCheckpoint,omitempty"`
			Generation int64           `json:"taskGeneration,omitempty"`
		}{dispatch.Input, dispatch.TargetDefinitionPin, dispatch.RootTaskMetadata, dispatch.ResumeCheckpoint, dispatch.TaskGeneration})
	}
	key := "task/" + dispatch.TaskRunID
	if len(dispatch.OwnedExecutionScope) > 0 && dispatch.TaskGeneration > 0 {
		key += fmt.Sprintf("/%d", dispatch.TaskGeneration)
	}
	return protocol.RuntimeInvokePayload{InvocationID: dispatch.AttemptID, IdempotencyKey: key, SpaceID: w.client.conf.SpaceID, DeviceID: w.client.conf.Identity.DeviceID, RuntimeID: w.client.conf.Identity.RuntimeID, RuntimeSessionID: dispatch.RuntimeSessionID, ConnectionGeneration: dispatch.ConnectionGeneration, RuntimeType: "task", Handler: dispatch.TaskDefinitionID, Input: input, AuthorityCallID: dispatch.AuthorityCallID, OwnedExecutionScope: dispatch.OwnedExecutionScope}
}

func (w *defaultTaskWorker) validateTaskAuthority(ctx context.Context, dispatch protocol.TaskDispatchPayload) error {
	if err := w.validateTaskConnection(dispatch); err != nil {
		return err
	}
	owned := len(dispatch.OwnedExecutionScope) != 0 || dispatch.AuthorityCallID != ""
	if owned {
		var pin protocol.TargetTaskDefinitionPin
		if len(dispatch.TargetDefinitionPin) > 8192 || json.Unmarshal(dispatch.TargetDefinitionPin, &pin) != nil || pin.DeviceID != dispatch.DeviceID.String() || pin.TaskID != dispatch.TaskDefinitionID || pin.ExtensionID == "" || pin.ModuleID == "" || pin.InstalledGeneration < 1 {
			return fmt.Errorf("设备任务缺少一致的目标插件版本")
		}
		for _, value := range []string{pin.DefinitionFingerprint, pin.PortableFingerprint, strings.TrimPrefix(pin.EntryHash, "sha256:")} {
			decoded, err := hex.DecodeString(value)
			if err != nil || len(decoded) != 32 {
				return fmt.Errorf("目标设备任务指纹无效")
			}
		}
	}
	if owned && w.client.conf.ExecutionJournal == nil {
		return fmt.Errorf("设备缺少持久化任务执行记录")
	}
	guard := w.client.conf.ExecutionGuard
	if guard == nil {
		if owned {
			return fmt.Errorf("设备缺少任务授权校验端口")
		}
		return nil
	}
	_, err := guard(ctx, w.taskAuthorityInvocation(dispatch), func(context.Context) (*protocol.RuntimeResultPayload, error) { return nil, nil })
	return err
}

func (w *defaultTaskWorker) validateTaskConnection(dispatch protocol.TaskDispatchPayload) error {
	if w.client.conf.ExecutionGuard == nil && len(dispatch.OwnedExecutionScope) == 0 && dispatch.AuthorityCallID == "" {
		return nil
	}
	if w.client.conf.Identity == nil || w.client.State() != StateReady || dispatch.DeviceID != w.client.conf.Identity.DeviceID || dispatch.RuntimeID != w.client.conf.Identity.RuntimeID || dispatch.RuntimeSessionID != w.client.sessionIdentity() || dispatch.ConnectionGeneration != w.client.sessionGeneration() || dispatch.RuntimeSessionID == "" || dispatch.ConnectionGeneration < 1 {
		return fmt.Errorf("任务执行前或执行后设备会话已失效")
	}
	return nil
}

func (w *defaultTaskWorker) executeTaskByType(ctx context.Context, dispatch protocol.TaskDispatchPayload) (json.RawMessage, error) {
	if len(dispatch.Input) > 1<<20 {
		return nil, fmt.Errorf("设备任务输入超过限制")
	}
	var input map[string]interface{}
	if len(dispatch.Input) > 0 {
		if err := json.Unmarshal(dispatch.Input, &input); err != nil {
			return nil, fmt.Errorf("invalid task input: %w", err)
		}
	}

	_, scoped := coordination.FromContext(ctx)
	owned := scoped || len(dispatch.OwnedExecutionScope) != 0 || dispatch.AuthorityCallID != ""
	taskType := dispatch.TaskDefinitionID
	if value, present := input["taskType"]; present {
		requested, valid := value.(string)
		if owned && (!valid || requested != dispatch.TaskDefinitionID) {
			return nil, fmt.Errorf("任务输入不能覆盖已授权的任务定义")
		}
		if !owned && requested != "" {
			taskType = requested
		}
	}
	log.Printf("devicemesh: agent: executing task: taskRunId=%s taskType=%s", dispatch.TaskRunID, taskType)

	if taskType == "" {
		return nil, fmt.Errorf("missing task definition id")
	}
	if owned {
		if executor, ok := w.taskRuntime.(OwnedTaskDispatchOutcomeExecutor); ok {
			outcome, err := executor.ExecuteOwnedDispatchOutcome(ctx, dispatch)
			if err != nil {
				return nil, err
			}
			return json.Marshal(outcome)
		}
	}

	w.reportProgress(ctx, dispatch, 1, float64Ptr(0), float64Ptr(100), float64Ptr(0), "executing", fmt.Sprintf("executing task type: %s", taskType))

	var result json.RawMessage
	var err error
	if owned {
		executor, ok := w.taskRuntime.(OwnedTaskDispatchExecutor)
		if !ok {
			return nil, fmt.Errorf("设备任务执行端尚未接入完整派发授权，拒绝丢失执行身份")
		}
		result, err = executor.ExecuteOwnedDispatch(ctx, dispatch)
	} else {
		result, err = w.dispatchTaskExecution(ctx, taskType, input)
	}
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

func (w *defaultTaskWorker) runHeartbeat(ctx context.Context, dispatch protocol.TaskDispatchPayload, lease *taskLeaseTimer) {
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
				if err := w.client.awaitTaskLease(ctx, dispatch, seq, lease.Confirm); err != nil {
					_ = w.CancelTask(ctx, dispatch.TaskRunID, dispatch.AttemptID, dispatch.LeaseID)
					return
				}
			}
		}
	}
}

func (w *defaultTaskWorker) CancelAllTasks() {
	w.mu.Lock()
	entries := make([]taskCancellation, 0, len(w.cancelFns))
	for _, entry := range w.cancelFns {
		entries = append(entries, entry)
	}
	w.mu.Unlock()
	for _, entry := range entries {
		entry.cancel()
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
		dispatch,
	)
}

func float64Ptr(v float64) *float64 {
	return &v
}
