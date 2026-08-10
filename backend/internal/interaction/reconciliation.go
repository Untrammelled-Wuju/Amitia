package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type AgentTaskRef struct {
	TaskRunID   string `json:"taskRunID"`
	InvocationID string `json:"invocationID"`
	Status      string `json:"status"`
	Generation  int64  `json:"generation"`
	Completed   bool   `json:"completed"`
}

type AgentWorkflowRef struct {
	ExecutionID string `json:"executionID"`
	WorkflowID  string `json:"workflowID"`
	Status      string `json:"status"`
	Completed   bool   `json:"completed"`
	Attempts    int    `json:"attempts"`
}

type AgentInvocationRef struct {
	InvocationID string `json:"invocationID"`
	CapabilityID string `json:"capabilityID"`
	Status       string `json:"status"`
	Completed    bool   `json:"completed"`
}

type AgentReconciliationSnapshot struct {
	UserID         string                          `json:"userId"`
	CharacterID    string                          `json:"characterId"`
	ConversationID string                          `json:"conversationId"`
	InteractionID  string                          `json:"interactionId"`
	Goals          []decision.Goal                 `json:"goals"`
	Observations   []decision.Observation           `json:"observations"`
	Tasks          []AgentTaskRef                  `json:"tasks"`
	Workflows      []AgentWorkflowRef              `json:"workflows"`
	Invocations    []AgentInvocationRef            `json:"invocations"`
	CapturedAt     time.Time                       `json:"capturedAt"`
}

type GoalReconciliationReader interface {
	GetGoal(ctx context.Context, goalID string) (decision.Goal, bool)
	ActiveForScope(ctx context.Context, userID, characterID, conversationID string) []decision.Goal
}

type TaskReconciliationReader interface {
	GetTaskRun(ctx context.Context, taskRunID string) (AgentTaskRef, bool)
	ListTaskRunsByInteraction(ctx context.Context, invocationID string) []AgentTaskRef
}

type WorkflowReconciliationReader interface {
	GetWorkflowRun(ctx context.Context, executionID string) (AgentWorkflowRef, bool)
	ListWorkflowRunsByInteraction(ctx context.Context, invocationID string) []AgentWorkflowRef
}

type InvocationReconciliationReader interface {
	GetInvocation(ctx context.Context, invocationID string) (AgentInvocationRef, bool)
	ListInvocationsByInteraction(ctx context.Context, invocationID string) []AgentInvocationRef
}

type AgentObservationReader interface {
	ListObservationsByInteraction(ctx context.Context, interactionID string) []decision.Observation
}

type AgentReconciliationProcessor interface {
	Capture(ctx context.Context, scope ReconciliationCaptureScope) (*AgentReconciliationSnapshot, error)
}

type ReconciliationCaptureScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	InteractionID  string
	RequestID      string
	GoalIDs        []string
	ObservationIDs []string
	TaskRunIDs     []string
	ExecutionIDs   []string
	InvocationIDs  []string
}
