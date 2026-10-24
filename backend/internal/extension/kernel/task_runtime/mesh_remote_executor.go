package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TaskProgressHandler interface {
	HandleProgress(ctx context.Context, taskRunID, attemptID, leaseID string, seq int64, current, total, percentage *float64, stage, message string) error
}

type TaskCheckpointHandler interface {
	HandleCheckpoint(ctx context.Context, taskRunID, attemptID, leaseID, checkpointID string, version int64, payload json.RawMessage, payloadHash string) error
}

type TaskCompletionHandler interface {
	HandleCompletion(ctx context.Context, taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string) error
}

type meshSessionLookup interface {
	GetActiveSession(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (deviceruntime.RuntimeSession, error)
}

type MeshRemoteTaskExecutor struct {
	hub               *server.ConnectionHub
	sessionLookup     meshSessionLookup
	PendingTasks      *PendingTaskManager
	inspectionPort    capability.DeviceRuntimeInvocationPort
	HeartbeatInterval time.Duration
	LeaseDuration     time.Duration
	progressHandler   TaskProgressHandler
	checkpointHandler TaskCheckpointHandler
	completionHandler TaskCompletionHandler
}

func NewMeshRemoteTaskExecutor(hub *server.ConnectionHub, lookup meshSessionLookup, shared ...*PendingTaskManager) *MeshRemoteTaskExecutor {
	pending := NewPendingTaskManager()
	if len(shared) > 0 && shared[0] != nil {
		pending = shared[0]
	}
	return &MeshRemoteTaskExecutor{
		hub:               hub,
		sessionLookup:     lookup,
		PendingTasks:      pending,
		HeartbeatInterval: 30 * time.Second,
		LeaseDuration:     5 * time.Minute,
	}
}

func (e *MeshRemoteTaskExecutor) SetInspectionPort(p capability.DeviceRuntimeInvocationPort) {
	e.inspectionPort = p
}

func (e *MeshRemoteTaskExecutor) InspectExecutionJournal(ctx context.Context, run *TaskRun) (executionjournal.Inspection, error) {
	var empty executionjournal.Inspection
	if e == nil || e.inspectionPort == nil || run == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice || run.TaskRunID == "" || run.Generation < 1 || run.ExecutionAttemptID == "" || run.LeaseID == "" || run.ScopeSnapshotID == "" || !run.ExecutionTarget.HasRuntime() || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 || run.TaskDefinitionID == "" {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备账本检查缺少完整任务身份或执行入口")
	}
	target := run.ExecutionTarget.Normalize()
	if scope, owned := coordination.FromContext(ctx); !owned || scope.SpaceID != target.SpaceID.String() || scope.TargetDeviceID != target.DeviceID.String() || !scope.Coordinated {
		return empty, NewTaskError(ErrTaskScopeDenied, "设备账本检查需要原设备所有者授权")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return empty, err
	}
	taskDefinitionID := run.TaskDefinitionID
	if run.ExecutionTarget.SourceTaskDefinitionID != "" {

		taskDefinitionID = run.ExecutionTarget.SourceTaskDefinitionID
	}
	input, err := json.Marshal(executionjournal.TaskInspectionRequest{
		TaskRunID: run.TaskRunID, TaskGeneration: run.Generation,
		TaskExecutionBinding: executionjournal.TaskExecutionBinding{
			AttemptID: run.ExecutionAttemptID.String(), LeaseID: run.LeaseID,
			RuntimeID: target.RuntimeID.String(), SessionID: target.RuntimeSessionID.String(),
			ConnectionGeneration: target.ConnectionGeneration, TaskDefinitionID: taskDefinitionID,
		},
	})
	if err != nil {
		return empty, err
	}
	route := capability.RuntimeExecutionRoute{
		Binding:   capability.RuntimeBinding{RuntimeType: capability.RuntimeType("task_recovery"), HandlerName: executionjournal.TaskInspectionHandler},
		Placement: capability.ProviderPlacementDevice,
		SpaceID:   target.SpaceID, DeviceID: target.DeviceID, RuntimeID: target.RuntimeID, RemoteDevice: true,
	}
	result := e.inspectionPort.Execute(ctx, capability.DeviceRuntimeInvocationRequest{
		Route: route, Binding: route.Binding,
		Invocation: capability.ToolInvocationContext{InvocationID: uuid.NewString(), SpaceID: target.SpaceID.String(), DeadlineDuration: 10 * time.Second},
		Input:      input,
	})
	if result.Status != capability.ToolResultStatusSuccess {
		return empty, NewTaskError(ErrRemoteTaskExecutorUnavailable, "设备执行日志查询未确认，禁止据此继续执行")
	}
	if result.DeviceID != target.DeviceID.String() || result.RuntimeID != target.RuntimeID.String() || result.RuntimeSessionID == "" || result.Generation < 1 || len(result.Structured) == 0 {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备执行日志回执身份无效")
	}
	var inspected executionjournal.Inspection
	if json.Unmarshal(result.Structured, &inspected) != nil || inspected.Result != nil {
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备执行日志回执格式无效")
	}
	switch inspected.State {
	case "absent", "started", "performed", "completed", "stopped_unknown", "stopped_performed":
	default:
		return empty, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备执行日志回执状态无效")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return empty, err
	}
	return inspected, nil
}

func (e *MeshRemoteTaskExecutor) SetProgressHandler(h TaskProgressHandler) {
	e.progressHandler = h
}

func (e *MeshRemoteTaskExecutor) SetCheckpointHandler(h TaskCheckpointHandler) {
	e.checkpointHandler = h
}

func (e *MeshRemoteTaskExecutor) SetCompletionHandler(h TaskCompletionHandler) {
	e.completionHandler = h
}

func (e *MeshRemoteTaskExecutor) Kind() TaskExecutorKind {
	return TaskExecutorKindRemote
}

func (e *MeshRemoteTaskExecutor) SupportsPlacement(placement TaskExecutionPlacement) bool {
	return placement == TaskExecutionPlacementCloud || placement == TaskExecutionPlacementDevice
}

func (e *MeshRemoteTaskExecutor) Execute(ctx context.Context, request TaskExecutionRequest) (TaskExecutionOutcome, error) {
	if request.Run == nil || request.Definition == nil || request.AttemptID == "" || request.Run.ExecutionAttemptID != request.AttemptID {
		return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, NewTaskError(ErrTaskExecutionAttemptInvalid, "远端任务缺少一致的执行身份")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, err
	}
	if e.hub == nil {
		return TaskExecutionOutcome{
			Status:       RunStatusRecoveryRequired,
			ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
			ErrorMessage: "mesh remote executor not configured",
		}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "mesh remote executor not configured")
	}

	switch request.Placement {
	case TaskExecutionPlacementDevice:
		return e.executeOnDevice(ctx, request)
	case TaskExecutionPlacementCloud:
		return e.executeOnCloud(ctx, request)
	default:
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionPlacementInvalid),
			ErrorMessage: "unsupported placement: " + string(request.Placement),
		}, NewTaskError(ErrTaskExecutionPlacementInvalid, "unsupported placement: "+string(request.Placement))
	}
}

func (e *MeshRemoteTaskExecutor) executeOnDevice(ctx context.Context, request TaskExecutionRequest) (TaskExecutionOutcome, error) {
	if scope, owned := coordination.FromContext(ctx); owned {
		pin := request.TargetDefinitionPin
		if pin == nil || pin.DeviceID != scope.TargetDeviceID || pin.TaskID != SourceTaskDefinitionID(request.Definition) || pin.InstalledGeneration < 1 || !validTaskFingerprint(pin.DefinitionFingerprint) {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, NewTaskError(ErrTaskDefinitionInvalid, "远端任务缺少目标设备已确认的插件版本")
		}
	}
	if e.PendingTasks == nil {
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
			ErrorMessage: "pending task manager not configured",
		}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "pending task manager not configured")
	}

	target := request.Target.Normalize()
	if !target.HasDevice() {
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskDeviceBindingInvalid),
			ErrorMessage: "device execution requires spaceId and deviceId",
		}, NewTaskError(ErrTaskDeviceBindingInvalid, "device execution requires spaceId and deviceId")
	}

	sessionID, generation, err := e.resolveDeviceSession(ctx, target)
	if err != nil {
		return TaskExecutionOutcome{
			Status:       RunStatusRecoveryRequired,
			ErrorCode:    string(ErrTaskRuntimeSessionBindingInvalid),
			ErrorMessage: err.Error(),
		}, err
	}

	deadline := e.LeaseDuration
	if request.Run.DeadlineAt != nil {
		remaining := time.Until(*request.Run.DeadlineAt)
		if remaining > 0 && remaining < deadline {
			deadline = remaining
		}
	}

	pt, err := e.PendingTasks.Register(request, sessionID.String(), generation, deadline)
	if err != nil {
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: fmt.Sprintf("register pending task: %v", err),
		}, err
	}

	var deadlineAt *time.Time
	if request.Run.DeadlineAt != nil {
		d := *request.Run.DeadlineAt
		deadlineAt = &d
	}

	var inputCopy json.RawMessage
	if request.Run.Input != nil {
		inputCopy = append(json.RawMessage(nil), request.Run.Input...)
	}

	dispatch := protocol.TaskDispatchPayload{
		TaskGeneration:       request.Run.Generation,
		ProgressBase:         request.ProgressBase,
		TaskRunID:            request.Run.TaskRunID,
		TaskDefinitionID:     SourceTaskDefinitionID(request.Definition),
		AttemptID:            request.AttemptID.String(),
		LeaseID:              pt.LeaseID,
		Input:                inputCopy,
		DeadlineAt:           deadlineAt,
		MaxAttempts:          request.Run.MaxAttempts,
		Placement:            string(request.Placement),
		DeviceID:             target.DeviceID,
		RuntimeID:            target.RuntimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		SentAt:               time.Now().UTC(),
	}
	if scope, owned := coordination.FromContext(ctx); owned {
		metadata := CloneTaskRun(request.Run)
		metadata.Input = nil
		dispatch.RootTaskMetadata, err = json.Marshal(metadata)
		if err == nil && request.ResumeCheckpoint != nil {
			dispatch.ResumeCheckpoint, err = json.Marshal(request.ResumeCheckpoint)
		}
		if err != nil || len(dispatch.RootTaskMetadata) > 64<<10 || len(dispatch.ResumeCheckpoint) > (1<<20)+(64<<10) {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "root metadata invalid")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, NewTaskError(ErrTaskScopeDenied, "远端任务执行元数据或检查点无效")
		}
		dispatch.TargetDefinitionPin, err = json.Marshal(request.TargetDefinitionPin)
		if err != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "target definition invalid")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, err
		}
		if request.Run.ScopeSnapshotID == "" || target.SpaceID.String() != scope.SpaceID || target.DeviceID.String() != scope.TargetDeviceID {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "task authority mismatch")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, NewTaskError(ErrTaskScopeDenied, "远端任务目标与设备授权不一致")
		}
		encoded, err := json.Marshal(scope)
		if err != nil || len(encoded) > 64<<10 {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "task authority invalid")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, NewTaskError(ErrTaskScopeDenied, "远端任务授权范围无效")
		}
		id, confirm, err := coordination.TrackCurrentRemoteAuthority(ctx, scope.SpaceID, scope.TargetDeviceID, sessionID.String(), generation)
		if err != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "task authority unavailable")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, err
		}
		if err := e.PendingTasks.BindAuthority(request.Run.TaskRunID, request.AttemptID.String(), sessionID.String(), generation, confirm, id); err != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "task authority binding failed")
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, err
		}
		dispatch.AuthorityCallID, dispatch.OwnedExecutionScope = id, encoded
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		e.PendingTasks.Cancel(request.Run.TaskRunID, "task authority expired")
		return TaskExecutionOutcome{Status: RunStatusRecoveryRequired}, err
	}

	if !e.hub.SendEnvelope(sessionID, generation, protocol.MessageTypeTaskDispatch, dispatch) {
		e.PendingTasks.Cancel(request.Run.TaskRunID, "send failed")
		return TaskExecutionOutcome{
			Status:       RunStatusRecoveryRequired,
			ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
			ErrorMessage: "failed to send task to device",
		}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "failed to send task to device")
	}

	claimResult, err := e.PendingTasks.WaitForClaim(ctx, request.Run.TaskRunID)
	if err != nil || !claimResult.Success {
		errMsg := "task claim timed out"
		if err != nil {
			errMsg = err.Error()
		}
		if claimResult.Error != "" {
			errMsg = claimResult.Error
		}
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: "worker claim failed: " + errMsg,
		}, NewTaskError(ErrTaskExecutionAttemptInvalid, "worker claim failed: "+errMsg)
	}

	return TaskExecutionOutcome{
		Status:          RunStatusRunning,
		RemoteReference: request.Run.TaskRunID,
		LeaseID:         claimResult.LeaseID,
		LeaseExpiresAt:  &claimResult.LeaseExp,
	}, nil
}

func (e *MeshRemoteTaskExecutor) executeOnCloud(ctx context.Context, request TaskExecutionRequest) (TaskExecutionOutcome, error) {
	return TaskExecutionOutcome{
		Status:       RunStatusFailed,
		ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
		ErrorMessage: "cloud-to-cloud remote execution not implemented",
	}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "cloud-to-cloud remote execution not implemented")
}

func (e *MeshRemoteTaskExecutor) resolveDeviceSession(ctx context.Context, target TaskExecutionTarget) (runtimeidentity.RuntimeSessionID, int64, error) {
	if e.sessionLookup != nil {
		session, err := e.sessionLookup.GetActiveSession(ctx, target.SpaceID, target.DeviceID, target.RuntimeID)
		if err != nil {
			return "", 0, fmt.Errorf("resolve session: %w", err)
		}
		if !session.IsActive() {
			return "", 0, fmt.Errorf("device session not active")
		}
		return session.ID, session.ConnectionGeneration, nil
	}

	if target.HasRuntimeSession() && target.ConnectionGeneration >= 1 {
		return target.RuntimeSessionID, target.ConnectionGeneration, nil
	}

	conn, ok := e.hub.GetByRuntime(target.SpaceID, target.DeviceID, target.RuntimeID)
	if !ok {
		return "", 0, fmt.Errorf("no active connection for target device")
	}
	return conn.SessionID, conn.Generation, nil
}

func (e *MeshRemoteTaskExecutor) ConfirmStoppedAttempt(ctx context.Context, run *TaskRun) bool {
	if e == nil || e.PendingTasks == nil || run == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice || ctx.Err() != nil {
		return false
	}
	if coordination.ValidateCurrent(ctx) != nil {
		return false
	}
	return e.PendingTasks.ConfirmStoppedAttempt(run)
}

func (e *MeshRemoteTaskExecutor) CancelAndWait(ctx context.Context, run *TaskRun, timeout time.Duration) error {

	if e == nil || e.PendingTasks == nil {
		return NewTaskError(ErrRemoteTaskExecutorUnavailable, "cannot confirm remote cancellation without pending task manager")
	}
	if err := e.Cancel(ctx, run); err != nil {
		return err
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !e.PendingTasks.WaitForCancelAckBound(cancelCtx, run) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "cancel ack timed out")
	}
	return nil
}

func (e *MeshRemoteTaskExecutor) ValidateTarget(ctx context.Context, target TaskExecutionTarget) error {
	normalized := target.Normalize()
	if normalized.IsZero() {
		return NewTaskError(ErrTaskExecutionTargetInvalid, "empty execution target")
	}

	if normalized.HasDevice() && (normalized.SpaceID == "" || normalized.RuntimeID == "") {
		return NewTaskError(ErrTaskDeviceBindingInvalid, "device target requires spaceId and runtimeId")
	}

	if normalized.HasRuntimeSession() && normalized.ConnectionGeneration < 1 {
		return NewTaskError(ErrTaskRuntimeSessionBindingInvalid, "runtime session requires connectionGeneration >= 1")
	}
	return nil
}

func (e *MeshRemoteTaskExecutor) Cancel(ctx context.Context, run *TaskRun) error {
	if e == nil || e.hub == nil {
		return NewTaskError(ErrRemoteTaskExecutorUnavailable, "mesh remote executor not configured")
	}
	if run == nil {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "cancel requires original task identity")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}

	target := run.ExecutionTarget.Normalize()
	if !target.HasRuntime() {
		return NewTaskError(ErrTaskDeviceBindingInvalid, "cancel requires original device and runtime target")
	}

	sessionID := target.RuntimeSessionID
	generation := target.ConnectionGeneration
	if run.TaskRunID == "" || run.ExecutionAttemptID == "" || run.LeaseID == "" || sessionID == "" || generation < 1 {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "cancel requires exact original attempt, lease and session binding")
	}

	leaseID := run.LeaseID
	if e.PendingTasks != nil {
		if pending, ok := e.PendingTasks.Get(run.TaskRunID); ok && pending != nil && pending.LeaseID != "" {
			if pending.AttemptID != run.ExecutionAttemptID.String() || pending.SessionID != sessionID.String() || pending.Generation != generation || pending.SpaceID != target.SpaceID.String() || pending.DeviceID != target.DeviceID.String() || pending.RuntimeID != target.RuntimeID.String() || leaseID != pending.LeaseID {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "取消请求不能使用其他执行代次的租约")
			}
			leaseID = pending.LeaseID
		}
	}
	cancelPayload := protocol.TaskCancelPayload{
		TaskRunID:            run.TaskRunID,
		AttemptID:            run.ExecutionAttemptID.String(),
		LeaseID:              leaseID,
		Reason:               "user_requested",
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		SentAt:               time.Now().UTC(),
	}

	if !e.hub.SendEnvelope(sessionID, generation, protocol.MessageTypeTaskCancel, cancelPayload) {
		return NewTaskError(ErrRemoteTaskExecutorUnavailable, "failed to send cancel to device")
	}

	return nil
}
