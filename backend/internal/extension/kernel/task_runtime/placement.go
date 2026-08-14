package task_runtime

import (
	"context"
	"strings"
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

type TaskExecutionTarget struct {
	ProviderID string `json:"providerId,omitempty"`
	DeviceID   string `json:"deviceId,omitempty"`
	RuntimeID  string `json:"runtimeId,omitempty"`
}

func (t TaskExecutionTarget) IsZero() bool {
	return strings.TrimSpace(t.ProviderID) == "" &&
		strings.TrimSpace(t.DeviceID) == "" &&
		strings.TrimSpace(t.RuntimeID) == ""
}

func (t TaskExecutionTarget) HasProvider() bool {
	return strings.TrimSpace(t.ProviderID) != ""
}

func (t TaskExecutionTarget) HasDevice() bool {
	return strings.TrimSpace(t.DeviceID) != ""
}

func (t TaskExecutionTarget) HasRuntime() bool {
	return strings.TrimSpace(t.RuntimeID) != ""
}

func (t TaskExecutionTarget) Normalize() TaskExecutionTarget {
	return TaskExecutionTarget{
		ProviderID: strings.TrimSpace(t.ProviderID),
		DeviceID:   strings.TrimSpace(t.DeviceID),
		RuntimeID:  strings.TrimSpace(t.RuntimeID),
	}
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
		Reason:    "remote execution placement acknowledged but not wired in stage G4",
	}, nil
}

type TaskExecutionRequest struct {
	Run        *TaskRun
	Definition *TaskDefinition
}

type TaskExecutionOutcome struct {
	Status       TaskRunStatus
	Result       *TaskRunResult
	ErrorCode    string
	ErrorMessage string
}

type TaskExecutorPort interface {
	Placement() TaskExecutionPlacement

	Execute(
		ctx context.Context,
		request TaskExecutionRequest,
	) (TaskExecutionOutcome, error)
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

func ValidateTaskExecutionTarget(
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
	case TaskExecutionPlacementCloud:
		if target.HasDevice() {
			return NewTaskError(
				ErrTaskExecutionTargetInvalid,
				"cloud execution placement should not bind a device",
			)
		}
	}

	return nil
}
