package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
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
	GetActiveSession(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (deviceruntime.RuntimeSession, error)
}

type MeshRemoteTaskExecutor struct {
	hub               *server.ConnectionHub
	sessionLookup     meshSessionLookup
	PendingTasks      *PendingTaskManager
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
			ErrorMessage: "device execution requires userId and deviceId",
		}, NewTaskError(ErrTaskDeviceBindingInvalid, "device execution requires userId and deviceId")
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
		TaskRunID:            request.Run.TaskRunID,
		TaskDefinitionID:     request.Run.TaskDefinitionID,
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
		session, err := e.sessionLookup.GetActiveSession(ctx, target.UserID, target.DeviceID, target.RuntimeID)
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

	conn, ok := e.hub.GetByRuntime(target.UserID, target.DeviceID, target.RuntimeID)
	if !ok {
		return "", 0, fmt.Errorf("no active connection for target device")
	}
	return conn.SessionID, conn.Generation, nil
}

func (e *MeshRemoteTaskExecutor) CancelAndWait(ctx context.Context, run *TaskRun, timeout time.Duration) error {
	if err := e.Cancel(ctx, run); err != nil {
		return err
	}

	if e.PendingTasks == nil {
		return nil
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !e.PendingTasks.WaitForCancelAck(cancelCtx, run.TaskRunID) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "cancel ack timed out")
	}
	return nil
}

func (e *MeshRemoteTaskExecutor) ValidateTarget(ctx context.Context, target TaskExecutionTarget) error {
	normalized := target.Normalize()
	if normalized.IsZero() {
		return NewTaskError(ErrTaskExecutionTargetInvalid, "empty execution target")
	}

	if normalized.HasDevice() && (normalized.UserID == "" || normalized.RuntimeID == "") {
		return NewTaskError(ErrTaskDeviceBindingInvalid, "device target requires userId and runtimeId")
	}

	if normalized.HasRuntimeSession() && normalized.ConnectionGeneration < 1 {
		return NewTaskError(ErrTaskRuntimeSessionBindingInvalid, "runtime session requires connectionGeneration >= 1")
	}
	return nil
}

func (e *MeshRemoteTaskExecutor) Cancel(ctx context.Context, run *TaskRun) error {
	if e.hub == nil {
		return NewTaskError(ErrRemoteTaskExecutorUnavailable, "mesh remote executor not configured")
	}

	target := run.ExecutionTarget.Normalize()
	if !target.HasDevice() {
		return NewTaskError(ErrTaskDeviceBindingInvalid, "cancel requires device target")
	}

	sessionID := target.RuntimeSessionID
	generation := target.ConnectionGeneration

	if (!target.HasRuntimeSession() || generation < 1) && e.sessionLookup != nil {
		session, err := e.sessionLookup.GetActiveSession(ctx, target.UserID, target.DeviceID, target.RuntimeID)
		if err != nil {
			return fmt.Errorf("resolve session for cancel: %w", err)
		}
		sessionID = session.ID
		generation = session.ConnectionGeneration
	}

	if sessionID == "" || generation < 1 {
		return NewTaskError(ErrTaskRuntimeSessionBindingInvalid, "cancel requires active session")
	}

	leaseID := run.LeaseID
	if e.PendingTasks != nil {
		if pending, ok := e.PendingTasks.Get(run.TaskRunID); ok && pending != nil && pending.LeaseID != "" {
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
