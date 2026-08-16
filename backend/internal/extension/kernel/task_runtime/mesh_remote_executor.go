package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type meshSessionLookup interface {
	GetActiveSession(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (deviceruntime.RuntimeSession, error)
}

type MeshRemoteTaskExecutor struct {
	hub                *server.ConnectionHub
	sessionLookup      meshSessionLookup
	PendingTasks       *PendingTaskManager
	HeartbeatInterval  time.Duration
	LeaseDuration      time.Duration
}

func NewMeshRemoteTaskExecutor(hub *server.ConnectionHub, lookup meshSessionLookup) *MeshRemoteTaskExecutor {
	return &MeshRemoteTaskExecutor{
		hub:               hub,
		sessionLookup:     lookup,
		PendingTasks:      NewPendingTaskManager(),
		HeartbeatInterval: 30 * time.Second,
		LeaseDuration:     5 * time.Minute,
	}
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

	if e.PendingTasks != nil {
		_, err := e.PendingTasks.Register(request, sessionID.String(), generation, deadline)
		if err != nil {
			return TaskExecutionOutcome{
				Status:       RunStatusFailed,
				ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
				ErrorMessage: fmt.Sprintf("register pending task: %v", err),
			}, err
		}
	}

	taskCmd := MeshTaskCommand{
		TaskRunID:        request.Run.TaskRunID,
		TaskDefinitionID: request.Run.TaskDefinitionID,
		AttemptID:        request.AttemptID.String(),
		Input:            request.Run.Input,
		DeadlineAt:       request.Run.DeadlineAt,
		MaxAttempts:      request.Run.MaxAttempts,
	}

	inputBytes, err := json.Marshal(taskCmd)
	if err != nil {
		if e.PendingTasks != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "marshal failed")
		}
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: "marshal task command: " + err.Error(),
		}, err
	}

	cmdPayload := protocol.CommandPayload{
		CommandID:       uuid.New().String(),
		CommandName:     "task.execute",
		CommandSequence: time.Now().UnixNano(),
		Payload:         inputBytes,
	}

	payloadBytes, err := json.Marshal(cmdPayload)
	if err != nil {
		if e.PendingTasks != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "marshal failed")
		}
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: "marshal command payload: " + err.Error(),
		}, err
	}

	env := protocol.Envelope{
		EnvelopeVersion:      1,
		Protocol:             "amitia.device-mesh",
		MessageType:          protocol.MessageTypeCommand,
		MessageID:            uuid.New().String(),
		UserID:               target.UserID,
		DeviceID:             target.DeviceID,
		RuntimeID:            target.RuntimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		Sequence:             1,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		if e.PendingTasks != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "marshal failed")
		}
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: "marshal envelope: " + err.Error(),
		}, err
	}

	if !e.hub.Send(sessionID, generation, envBytes) {
		if e.PendingTasks != nil {
			e.PendingTasks.Cancel(request.Run.TaskRunID, "send failed")
		}
		return TaskExecutionOutcome{
			Status:       RunStatusRecoveryRequired,
			ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
			ErrorMessage: "failed to send task to device",
		}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "failed to send task to device")
	}

	if e.PendingTasks == nil {
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
			ErrorMessage: "pending task manager not configured",
		}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "pending task manager not configured")
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

	cancelCmd := MeshTaskCancelCommand{
		TaskRunID: run.TaskRunID,
		AttemptID: run.ExecutionAttemptID.String(),
	}

	payloadBytes, err := json.Marshal(cancelCmd)
	if err != nil {
		return fmt.Errorf("marshal cancel command: %w", err)
	}

	cmdPayload := protocol.CommandPayload{
		CommandID:       uuid.New().String(),
		CommandName:     "task.cancel",
		CommandSequence: time.Now().UnixNano(),
		Payload:         payloadBytes,
	}

	cmdBytes, err := json.Marshal(cmdPayload)
	if err != nil {
		return fmt.Errorf("marshal command payload: %w", err)
	}

	env := protocol.Envelope{
		EnvelopeVersion:      1,
		Protocol:             "amitia.device-mesh",
		MessageType:          protocol.MessageTypeCommand,
		MessageID:            uuid.New().String(),
		UserID:               target.UserID,
		DeviceID:             target.DeviceID,
		RuntimeID:            target.RuntimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		Sequence:             1,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(cmdBytes),
		SentAt:               time.Now().UTC(),
		Payload:              cmdBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if !e.hub.Send(sessionID, generation, envBytes) {
		return NewTaskError(ErrRemoteTaskExecutorUnavailable, "failed to send cancel to device")
	}
	return nil
}

type MeshTaskCommand struct {
	TaskRunID        string          `json:"taskRunId"`
	TaskDefinitionID string          `json:"taskDefinitionId"`
	AttemptID        string          `json:"attemptId"`
	Input            json.RawMessage `json:"input,omitempty"`
	DeadlineAt       *time.Time      `json:"deadlineAt,omitempty"`
	MaxAttempts      int             `json:"maxAttempts"`
}

type MeshTaskCancelCommand struct {
	TaskRunID string `json:"taskRunId"`
	AttemptID string `json:"attemptId"`
}
