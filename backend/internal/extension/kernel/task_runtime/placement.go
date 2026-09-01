package task_runtime

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TaskExecutionPlacement string

const (
	TaskExecutionPlacementLocal  TaskExecutionPlacement = "local"
	TaskExecutionPlacementCloud  TaskExecutionPlacement = "cloud"
	TaskExecutionPlacementDevice TaskExecutionPlacement = "device"
)

func (p TaskExecutionPlacement) String() string {
	return string(p)
}

func (p TaskExecutionPlacement) IsValid() bool {
	switch p {
	case TaskExecutionPlacementLocal, TaskExecutionPlacementCloud, TaskExecutionPlacementDevice:
		return true
	}
	return false
}

func (p TaskExecutionPlacement) IsRemote() bool {
	switch p {
	case TaskExecutionPlacementCloud, TaskExecutionPlacementDevice:
		return true
	}
	return false
}

func (p TaskExecutionPlacement) Normalize() TaskExecutionPlacement {
	if p == "" {
		return TaskExecutionPlacementLocal
	}
	return p
}

type TaskExecutionAttemptID string

func NewTaskExecutionAttemptID() TaskExecutionAttemptID {
	return TaskExecutionAttemptID("texec_" + uuid.NewString())
}

func (id TaskExecutionAttemptID) String() string {
	return string(id)
}

type TaskExecutionTarget struct {
	ProviderID         capability.ProviderID         `json:"providerId,omitempty"`
	ProviderInstanceID capability.ProviderInstanceID `json:"providerInstanceId,omitempty"`

	UserID           runtimeidentity.UserID           `json:"userId,omitempty"`
	DeviceID         runtimeidentity.DeviceID         `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID        `json:"runtimeId,omitempty"`
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId,omitempty"`

	ConnectionGeneration int64 `json:"connectionGeneration,omitempty"`

	RuntimeInstanceID string `json:"runtimeInstanceId,omitempty"`
}

func (t TaskExecutionTarget) IsZero() bool {
	return t.ProviderID.IsEmpty() &&
		t.ProviderInstanceID.IsEmpty() &&
		t.UserID == "" &&
		t.DeviceID == "" &&
		t.RuntimeID == "" &&
		t.RuntimeSessionID == "" &&
		t.ConnectionGeneration == 0 &&
		strings.TrimSpace(t.RuntimeInstanceID) == ""
}

func (t TaskExecutionTarget) HasProvider() bool {
	return !t.ProviderID.IsEmpty()
}

func (t TaskExecutionTarget) HasProviderInstance() bool {
	return !t.ProviderInstanceID.IsEmpty()
}

func (t TaskExecutionTarget) HasDevice() bool {
	return t.UserID != "" && t.DeviceID != ""
}

func (t TaskExecutionTarget) HasRuntime() bool {
	return t.UserID != "" && t.DeviceID != "" && t.RuntimeID != ""
}

func (t TaskExecutionTarget) HasRuntimeSession() bool {
	return t.RuntimeSessionID != ""
}

func (t TaskExecutionTarget) HasCurrentConnectionBinding() bool {
	return t.RuntimeSessionID != "" && t.ConnectionGeneration >= 1
}

func (t TaskExecutionTarget) Normalize() TaskExecutionTarget {
	return TaskExecutionTarget{
		ProviderID:           capability.ParseProviderID(string(t.ProviderID)),
		ProviderInstanceID:   capability.ParseProviderInstanceID(string(t.ProviderInstanceID)),
		UserID:               runtimeidentity.ParseUserID(string(t.UserID)),
		DeviceID:             runtimeidentity.ParseDeviceID(string(t.DeviceID)),
		RuntimeID:            runtimeidentity.ParseRuntimeID(string(t.RuntimeID)),
		RuntimeSessionID:     runtimeidentity.ParseRuntimeSessionID(string(t.RuntimeSessionID)),
		ConnectionGeneration: t.ConnectionGeneration,
		RuntimeInstanceID:    strings.TrimSpace(t.RuntimeInstanceID),
	}
}

func (t TaskExecutionTarget) StableEqual(other TaskExecutionTarget) bool {
	return t.ProviderID == other.ProviderID &&
		t.ProviderInstanceID == other.ProviderInstanceID &&
		t.UserID == other.UserID &&
		t.DeviceID == other.DeviceID &&
		t.RuntimeID == other.RuntimeID &&
		t.RuntimeInstanceID == other.RuntimeInstanceID
}

func (t TaskExecutionTarget) ExactEqual(other TaskExecutionTarget) bool {
	return t.StableEqual(other) &&
		t.RuntimeSessionID == other.RuntimeSessionID &&
		t.ConnectionGeneration == other.ConnectionGeneration
}

func (t TaskExecutionTarget) Summary() string {
	parts := []string{}
	if !t.ProviderInstanceID.IsEmpty() {
		parts = append(parts, "providerInstance="+string(t.ProviderInstanceID))
	} else if !t.ProviderID.IsEmpty() {
		parts = append(parts, "provider="+string(t.ProviderID))
	}
	if t.DeviceID != "" {
		parts = append(parts, "device="+string(t.DeviceID))
	}
	if t.RuntimeID != "" {
		parts = append(parts, "runtime="+string(t.RuntimeID))
	}
	if t.RuntimeSessionID != "" {
		parts = append(parts, "session="+string(t.RuntimeSessionID))
	}
	if t.ConnectionGeneration > 0 {
		parts = append(parts, "gen="+string(rune('0'+t.ConnectionGeneration%10)))
	}
	return strings.Join(parts, " ")
}

type TaskPlacementRequest struct {
	TaskRunID        string                 `json:"taskRunId"`
	TaskDefinitionID string                 `json:"taskDefinitionId"`
	ExtensionID      string                 `json:"extensionId"`
	ModuleID         string                 `json:"moduleId"`
	RuntimeType      string                 `json:"runtimeType,omitempty"`
	Requested        TaskExecutionPlacement `json:"requestedPlacement,omitempty"`
}

type TaskPlacementDecision struct {
	Placement TaskExecutionPlacement `json:"placement"`
	Target    TaskExecutionTarget    `json:"target,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Resolved  bool                   `json:"resolved"`
}

type TaskPlacementResolver interface {
	ResolveTaskPlacement(
		ctx context.Context,
		request TaskPlacementRequest,
	) (TaskPlacementDecision, error)
}

type LocalTaskPlacementResolver struct{}

func (LocalTaskPlacementResolver) ResolveTaskPlacement(
	ctx context.Context,
	request TaskPlacementRequest,
) (TaskPlacementDecision, error) {
	placement := request.Requested.Normalize()

	if placement == "" || placement == TaskExecutionPlacementLocal {
		return TaskPlacementDecision{
			Placement: TaskExecutionPlacementLocal,
			Reason:    "default local execution",
			Resolved:  true,
		}, nil
	}

	if !placement.IsValid() {
		return TaskPlacementDecision{}, NewTaskError(
			ErrTaskExecutionPlacementInvalid,
			"invalid execution placement: "+string(placement),
		)
	}

	return TaskPlacementDecision{
		Placement: placement,
		Reason:    "remote execution placement acknowledged but not wired in stage G13",
		Resolved:  false,
	}, nil
}

type TaskExecutionRequest struct {
	Run        *TaskRun
	Definition *TaskDefinition

	AttemptID TaskExecutionAttemptID

	Placement TaskExecutionPlacement
	Target    TaskExecutionTarget
}

type TaskExecutionOutcome struct {
	Status          TaskRunStatus
	Result          *TaskRunResult
	ErrorCode       string
	ErrorMessage    string
	RemoteReference string
	LeaseID         string
	LeaseExpiresAt  *time.Time
}

type TaskExecutorKind string

const (
	TaskExecutorKindLocal  TaskExecutorKind = "local"
	TaskExecutorKindRemote TaskExecutorKind = "remote"
)

type TaskExecutorPort interface {
	Kind() TaskExecutorKind
	SupportsPlacement(TaskExecutionPlacement) bool

	Execute(
		ctx context.Context,
		request TaskExecutionRequest,
	) (TaskExecutionOutcome, error)
}

type RemoteTaskExecutor interface {
	TaskExecutorPort

	ValidateTarget(
		ctx context.Context,
		target TaskExecutionTarget,
	) error

	Cancel(
		ctx context.Context,
		run *TaskRun,
	) error
}

type TaskExecutionTargetSnapshot struct {
	Placement TaskExecutionPlacement
	Target    TaskExecutionTarget
	AttemptID TaskExecutionAttemptID
}

func ResolveRequestedPlacement(
	requestPlacement TaskExecutionPlacement,
	definitionPlacement TaskExecutionPlacement,
) (TaskExecutionPlacement, error) {
	if requestPlacement != "" {
		if !requestPlacement.IsValid() {
			return "", NewTaskError(
				ErrTaskExecutionPlacementInvalid,
				"invalid execution placement: "+string(requestPlacement),
			)
		}
		return requestPlacement, nil
	}

	if definitionPlacement != "" {
		if !definitionPlacement.IsValid() {
			return "", NewTaskError(
				ErrTaskExecutionPlacementInvalid,
				"invalid definition execution placement: "+string(definitionPlacement),
			)
		}
		return definitionPlacement, nil
	}

	return TaskExecutionPlacementLocal, nil
}

func ValidateTaskExecutionTargetShape(
	placement TaskExecutionPlacement,
	target TaskExecutionTarget,
) error {
	normalized := placement.Normalize()

	if !normalized.IsValid() {
		return NewTaskError(
			ErrTaskExecutionPlacementInvalid,
			"invalid execution placement: "+string(placement),
		)
	}

	switch normalized {
	case TaskExecutionPlacementLocal:
		return nil
	case TaskExecutionPlacementCloud:
		target = target.Normalize()
		if target.DeviceID != "" || target.RuntimeSessionID != "" || target.ConnectionGeneration > 0 {
			return NewTaskError(
				ErrTaskExecutionTargetInvalid,
				"cloud execution should not bind device runtime session",
			)
		}
		return nil
	case TaskExecutionPlacementDevice:
		target = target.Normalize()
		if target.DeviceID == "" {
			return nil
		}
		if target.UserID == "" || target.RuntimeID == "" {
			return NewTaskError(
				ErrTaskDeviceBindingInvalid,
				"device target requires userId and runtimeId",
			)
		}
		if target.RuntimeSessionID != "" && target.ConnectionGeneration < 1 {
			return NewTaskError(
				ErrTaskRuntimeSessionBindingInvalid,
				"device target runtime session requires connectionGeneration >= 1",
			)
		}
		if target.ConnectionGeneration > 0 && target.RuntimeSessionID == "" {
			return NewTaskError(
				ErrTaskRuntimeSessionBindingInvalid,
				"device target connection generation requires runtime session",
			)
		}
		return nil
	}

	return nil
}

func ValidateResolvedTaskExecutionTarget(
	placement TaskExecutionPlacement,
	target TaskExecutionTarget,
) error {
	normalized := placement.Normalize()

	if !normalized.IsValid() {
		return NewTaskError(
			ErrTaskExecutionPlacementInvalid,
			"invalid execution placement: "+string(placement),
		)
	}

	switch normalized {
	case TaskExecutionPlacementLocal:
		if target.IsZero() {
			return nil
		}
		if target.HasProvider() && string(target.ProviderID) == "" {
			return NewTaskError(
				ErrTaskProviderBindingInvalid,
				"provider id must be non-empty when specified",
			)
		}
		return nil
	case TaskExecutionPlacementCloud:
		if string(target.ProviderID) == "" || target.ProviderID.IsEmpty() {
			return NewTaskError(
				ErrTaskProviderBindingInvalid,
				"cloud execution requires providerId",
			)
		}
		if string(target.ProviderInstanceID) == "" || target.ProviderInstanceID.IsEmpty() {
			return NewTaskError(
				ErrTaskProviderBindingInvalid,
				"cloud execution requires providerInstanceId",
			)
		}
		if target.DeviceID != "" || target.RuntimeSessionID != "" || target.ConnectionGeneration != 0 {
			return NewTaskError(
				ErrTaskDeviceBindingInvalid,
				"cloud execution must not bind device/runtime session",
			)
		}
		return nil
	case TaskExecutionPlacementDevice:
		if string(target.ProviderID) == "" || target.ProviderID.IsEmpty() {
			return NewTaskError(
				ErrTaskProviderBindingInvalid,
				"device execution requires providerId",
			)
		}
		if string(target.ProviderInstanceID) == "" || target.ProviderInstanceID.IsEmpty() {
			return NewTaskError(
				ErrTaskProviderBindingInvalid,
				"device execution requires providerInstanceId",
			)
		}
		if target.UserID == "" {
			return NewTaskError(
				ErrTaskDeviceBindingInvalid,
				"device execution requires userId",
			)
		}
		if target.DeviceID == "" {
			return NewTaskError(
				ErrTaskDeviceBindingInvalid,
				"device execution requires deviceId",
			)
		}
		if target.RuntimeID == "" {
			return NewTaskError(
				ErrTaskRuntimeBindingInvalid,
				"device execution requires runtimeId",
			)
		}
		// RuntimeSessionID/ConnectionGeneration are transient connection bindings.
		// A durable resolved target only needs the stable provider + device +
		// runtime identity; MeshRemoteTaskExecutor resolves the current session at
		// dispatch time and recovery deliberately clears stale session bindings.
		if target.RuntimeSessionID != "" && target.ConnectionGeneration < 1 {
			return NewTaskError(
				ErrTaskRuntimeSessionBindingInvalid,
				"device runtime session requires connectionGeneration >= 1",
			)
		}
		if target.ConnectionGeneration > 0 && target.RuntimeSessionID == "" {
			return NewTaskError(
				ErrTaskRuntimeSessionBindingInvalid,
				"device connection generation requires runtimeSessionId",
			)
		}
		return nil
	}

	return nil
}

func ValidateTaskExecutionTarget(
	placement TaskExecutionPlacement,
	target TaskExecutionTarget,
) error {
	if err := ValidateTaskExecutionTargetShape(placement, target); err != nil {
		return err
	}
	return nil
}
