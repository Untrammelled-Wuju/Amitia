package decision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type ObservationVersion string

const ObservationVersionV1 ObservationVersion = "agent-observation-v1"

type ObservationKind string

const (
	ObservationKindToolResult             ObservationKind = "tool_result"
	ObservationKindNoAction               ObservationKind = "no_action"
	ObservationKindMaterializationFailure ObservationKind = "materialization_failure"
	ObservationKindDispatchFailure        ObservationKind = "dispatch_failure"
	ObservationKindTaskAccepted           ObservationKind = "task_accepted"
	ObservationKindTaskResult             ObservationKind = "task_result"
)

type ObservationOutcome string

const (
	ObservationOutcomeSucceeded       ObservationOutcome = "succeeded"
	ObservationOutcomeFailed          ObservationOutcome = "failed"
	ObservationOutcomeCancelled       ObservationOutcome = "cancelled"
	ObservationOutcomeTimedOut        ObservationOutcome = "timed_out"
	ObservationOutcomeSkipped         ObservationOutcome = "skipped"
	ObservationOutcomeNotMaterialized ObservationOutcome = "not_materialized"
	ObservationOutcomeNotDispatched   ObservationOutcome = "not_dispatched"
	ObservationOutcomeAccepted        ObservationOutcome = "accepted"
)

type ObservationTargetKind string

const (
	ObservationTargetNone     ObservationTargetKind = "none"
	ObservationTargetTool     ObservationTargetKind = "tool"
	ObservationTargetTask     ObservationTargetKind = "task"
	ObservationTargetWorkflow ObservationTargetKind = "workflow"
)

type ObservationContentKind string

const (
	ObservationContentText       ObservationContentKind = "text"
	ObservationContentStructured ObservationContentKind = "structured"
	ObservationContentResource   ObservationContentKind = "resource"
)

type ObservationContent struct {
	Kind     ObservationContentKind `json:"kind"`
	Text     string                 `json:"text,omitempty"`
	Data     json.RawMessage        `json:"data,omitempty"`
	URI      string                 `json:"uri,omitempty"`
	MimeType string                 `json:"mimeType,omitempty"`
}

type ObservationError struct {
	Code       string `json:"code"`
	Category   string `json:"category,omitempty"`
	DomainCode string `json:"domainCode,omitempty"`
	Message    string `json:"message,omitempty"`
	Retryable  bool   `json:"retryable"`
}

type ObservationSideEffect struct {
	Kind        string `json:"kind"`
	ResourceURI string `json:"resourceUri,omitempty"`
	ExternalID  string `json:"externalId,omitempty"`
	State       string `json:"state,omitempty"`
}

type ObservationResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type ObservationEvidence struct {
	Contents    []ObservationContent    `json:"contents,omitempty"`
	Structured  json.RawMessage         `json:"structured,omitempty"`
	Error       *ObservationError       `json:"error,omitempty"`
	SideEffects []ObservationSideEffect `json:"sideEffects,omitempty"`
	Resources   []ObservationResource   `json:"resources,omitempty"`
	Metadata    map[string]any          `json:"metadata,omitempty"`
}

type Observation struct {
	Version        ObservationVersion    `json:"version"`
	ID             string                `json:"id"`
	PlanID         string                `json:"planId,omitempty"`
	ActionID       string                `json:"actionId,omitempty"`
	InteractionID  string                `json:"interactionId,omitempty"`
	RequestID      string                `json:"requestId,omitempty"`
	UserID         string                `json:"userId,omitempty"`
	CharacterID    string                `json:"characterId,omitempty"`
	ConversationID string                `json:"conversationId,omitempty"`
	CandidateID    string                `json:"candidateId,omitempty"`
	GoalIDs        []string              `json:"goalIds,omitempty"`
	GoalRefs       []GoalRef             `json:"goalRefs,omitempty"`
	IntentionIDs   []string              `json:"intentionIds,omitempty"`
	Kind           ObservationKind       `json:"kind"`
	TargetKind     ObservationTargetKind `json:"targetKind"`
	Outcome        ObservationOutcome    `json:"outcome"`
	InvocationID   string                `json:"invocationId,omitempty"`
	ExternalCallID string                `json:"externalCallId,omitempty"`
	ToolID         string                `json:"toolId,omitempty"`
	TaskRunID      string                `json:"taskRunId,omitempty"`
	TaskDefinitionID string              `json:"taskDefinitionId,omitempty"`
	TaskGeneration int64                 `json:"taskGeneration,omitempty"`
	Evidence       ObservationEvidence   `json:"evidence"`
	ObservedAt     time.Time             `json:"observedAt"`
}

type ObservationBuildErrorCode string

const (
	ErrObservationPlanInvalid          ObservationBuildErrorCode = "OBSERVATION_PLAN_INVALID"
	ErrObservationActionInvalid        ObservationBuildErrorCode = "OBSERVATION_ACTION_INVALID"
	ErrObservationScopeMismatch        ObservationBuildErrorCode = "OBSERVATION_SCOPE_MISMATCH"
	ErrObservationTimeMissing          ObservationBuildErrorCode = "OBSERVATION_TIME_MISSING"
	ErrObservationToolResultMissing    ObservationBuildErrorCode = "OBSERVATION_TOOL_RESULT_MISSING"
	ErrObservationInvocationMismatch   ObservationBuildErrorCode = "OBSERVATION_INVOCATION_MISMATCH"
	ErrObservationToolMismatch         ObservationBuildErrorCode = "OBSERVATION_TOOL_MISMATCH"
	ErrObservationExternalCallMismatch ObservationBuildErrorCode = "OBSERVATION_EXTERNAL_CALL_MISMATCH"
	ErrObservationResultInvalid        ObservationBuildErrorCode = "OBSERVATION_RESULT_INVALID"
)

type ObservationBuildError struct {
	Code ObservationBuildErrorCode
	Err  error
}

func (e ObservationBuildError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
	}
	return string(e.Code)
}

func BuildObservationID(actionID string) string {
	h := sha256.New()
	h.Write([]byte("agent-observation-v1"))
	h.Write([]byte{0x00})
	h.Write([]byte(actionID))
	return "obs:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func BuildTaskAcceptedObservationID(actionID, taskRunID string) string {
	h := sha256.New()
	h.Write([]byte("task-accepted-v1"))
	h.Write([]byte{0x00})
	h.Write([]byte(actionID))
	h.Write([]byte{0x00})
	h.Write([]byte(taskRunID))
	return "obs-ta:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func BuildTaskTerminalObservationID(actionID, taskRunID string, generation int64) string {
	h := sha256.New()
	h.Write([]byte("task-terminal-v1"))
	h.Write([]byte{0x00})
	h.Write([]byte(actionID))
	h.Write([]byte{0x00})
	h.Write([]byte(taskRunID))
	h.Write([]byte{0x00})
	h.Write([]byte(fmt.Sprintf("%d", generation)))
	return "obs-tt:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func ValidateObservation(o Observation) error {
	if o.Version == "" {
		return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("observation version is required")}
	}
	if o.ID == "" {
		return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("observation id is required")}
	}
	if o.ActionID == "" {
		return ObservationBuildError{Code: ErrObservationActionInvalid, Err: fmt.Errorf("observation actionId is required")}
	}
	if o.InteractionID == "" {
		return ObservationBuildError{Code: ErrObservationScopeMismatch, Err: fmt.Errorf("observation interactionId is required")}
	}
	if o.ConversationID == "" {
		return ObservationBuildError{Code: ErrObservationScopeMismatch, Err: fmt.Errorf("observation conversationId is required")}
	}
	if o.ObservedAt.IsZero() {
		return ObservationBuildError{Code: ErrObservationTimeMissing, Err: fmt.Errorf("observation observedAt is required")}
	}
	if !validObservationKind(o.Kind) {
		return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("invalid observation kind: %s", o.Kind)}
	}
	if !validObservationOutcome(o.Outcome) {
		return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("invalid observation outcome: %s", o.Outcome)}
	}
	if !validObservationTargetKind(o.TargetKind) {
		return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("invalid observation targetKind: %s", o.TargetKind)}
	}
	if o.Kind == ObservationKindToolResult {
		if o.ToolID == "" {
			return ObservationBuildError{Code: ErrObservationToolMismatch, Err: fmt.Errorf("tool_result observation requires toolId")}
		}
		if o.InvocationID == "" {
			return ObservationBuildError{Code: ErrObservationInvocationMismatch, Err: fmt.Errorf("tool_result observation requires invocationId")}
		}
		if o.TargetKind == ObservationTargetNone {
			return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("tool_result observation cannot have targetKind=none")}
		}
	}
	if o.Kind == ObservationKindNoAction {
		if o.TargetKind != ObservationTargetNone {
			return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("no_action observation must have targetKind=none")}
		}
		if o.ToolID != "" {
			return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("no_action observation must not have toolId")}
		}
		if o.InvocationID != "" {
			return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("no_action observation must not have invocationId")}
		}
	}
	if o.Kind == ObservationKindMaterializationFailure {
		if o.InvocationID != "" {
			return ObservationBuildError{Code: ErrObservationResultInvalid, Err: fmt.Errorf("materialization_failure observation must not have invocationId")}
		}
	}
	return nil
}

func validObservationKind(k ObservationKind) bool {
	switch k {
	case ObservationKindToolResult, ObservationKindNoAction,
		ObservationKindMaterializationFailure, ObservationKindDispatchFailure,
		ObservationKindTaskAccepted, ObservationKindTaskResult:
		return true
	}
	return false
}

func validObservationOutcome(o ObservationOutcome) bool {
	switch o {
	case ObservationOutcomeSucceeded, ObservationOutcomeFailed, ObservationOutcomeCancelled,
		ObservationOutcomeTimedOut, ObservationOutcomeSkipped,
		ObservationOutcomeNotMaterialized, ObservationOutcomeNotDispatched,
		ObservationOutcomeAccepted:
		return true
	}
	return false
}

func validObservationTargetKind(t ObservationTargetKind) bool {
	switch t {
	case ObservationTargetNone, ObservationTargetTool,
		ObservationTargetTask, ObservationTargetWorkflow:
		return true
	}
	return false
}
