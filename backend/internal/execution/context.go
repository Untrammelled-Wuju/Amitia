package execution

import (
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ExecutionContext struct {
	ExecutionID string `json:"executionId"`

	RootExecutionID   string `json:"rootExecutionId,omitempty"`
	ParentExecutionID string `json:"parentExecutionId,omitempty"`

	UserID runtimeidentity.UserID `json:"userId"`

	ConversationID string `json:"conversationId,omitempty"`
	TaskID         string `json:"taskId,omitempty"`

	InvocationID string `json:"invocationId,omitempty"`
	TraceID      string `json:"traceId,omitempty"`

	RuntimeTarget *capability.DeploymentTarget `json:"runtimeTarget,omitempty"`

	ScopeSnapshotID      string `json:"scopeSnapshotId,omitempty"`
	PermissionSnapshotID string `json:"permissionSnapshotId,omitempty"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`

	Resume *ResumeContext `json:"resume,omitempty"`

	Budget ExecutionBudget `json:"budget,omitempty"`

	Source string `json:"source,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

func NewExecutionContext(rootID, userID string) ExecutionContext {
	return ExecutionContext{
		ExecutionID:      NewExecutionID(),
		RootExecutionID:   rootID,
		UserID:           runtimeidentity.UserID(userID),
		ScopeSnapshotID:  "",
		Budget:           DefaultExecutionBudget(),
		CreatedAt:        time.Now().UTC(),
	}
}

func NewChildExecution(parent ExecutionContext, source string) ExecutionContext {
	return ExecutionContext{
		ExecutionID:        NewExecutionID(),
		RootExecutionID:     parent.RootExecutionID,
		ParentExecutionID:   parent.ExecutionID,
		UserID:             parent.UserID,
		ConversationID:     parent.ConversationID,
		TaskID:             parent.TaskID,
		TraceID:            parent.TraceID,
		RuntimeTarget:      parent.RuntimeTarget,
		ScopeSnapshotID:    parent.ScopeSnapshotID,
		PermissionSnapshotID: parent.PermissionSnapshotID,
		Source:             source,
		Budget:             parent.Budget,
		Metadata:           make(map[string]any),
		CreatedAt:          time.Now().UTC(),
	}
}

func NewExecutionID() string {
	return "exec_" + uuid.NewString()
}

func (c ExecutionContext) HasRoot() bool {
	return c.RootExecutionID != ""
}

func (c ExecutionContext) HasParent() bool {
	return c.ParentExecutionID != ""
}

func (c ExecutionContext) CapabilityAcquisitions() int {
	return c.Budget.CapabilityAcquisitions
}

func (c ExecutionContext) CanAcquire() bool {
	return c.Budget.CapabilityAcquisitions < MaxCapabilityAcquisitions
}

func (c ExecutionContext) IncrementAcquisitions() {
	c.Budget.CapabilityAcquisitions++
}
