package execution

import (
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimeTarget struct {
	Placement        string                    `json:"placement"`
	SpaceID          runtimeidentity.SpaceID   `json:"spaceId,omitempty"`
	DeviceID         runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`
	RuntimeSessionID string                    `json:"runtimeSessionId,omitempty"`
}

type ExecutionContext struct {
	ExecutionID string `json:"executionId"`

	RootExecutionID   string `json:"rootExecutionId,omitempty"`
	ParentExecutionID string `json:"parentExecutionId,omitempty"`

	SpaceID runtimeidentity.SpaceID `json:"spaceId"`

	ConversationID string `json:"conversationId,omitempty"`
	TaskID         string `json:"taskId,omitempty"`

	InvocationID string `json:"invocationId,omitempty"`
	TraceID      string `json:"traceId,omitempty"`

	RuntimeTarget *RuntimeTarget `json:"runtimeTarget,omitempty"`

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

func NewExecutionContext(rootID, spaceID string) ExecutionContext {
	return ExecutionContext{
		ExecutionID:     NewExecutionID(),
		RootExecutionID: rootID,
		SpaceID:         runtimeidentity.SpaceID(spaceID),
		ScopeSnapshotID: "",
		Budget:          DefaultExecutionBudget(),
		CreatedAt:       time.Now().UTC(),
	}
}

func NewChildExecution(parent ExecutionContext, source string) ExecutionContext {
	metadata := make(map[string]any, len(parent.Metadata))
	for key, value := range parent.Metadata {
		metadata[key] = value
	}
	return ExecutionContext{
		ExecutionID:          NewExecutionID(),
		RootExecutionID:      parent.RootExecutionID,
		ParentExecutionID:    parent.ExecutionID,
		SpaceID:              parent.SpaceID,
		ConversationID:       parent.ConversationID,
		TaskID:               parent.TaskID,
		InvocationID:         parent.InvocationID,
		TraceID:              parent.TraceID,
		RuntimeTarget:        parent.RuntimeTarget,
		ScopeSnapshotID:      parent.ScopeSnapshotID,
		PermissionSnapshotID: parent.PermissionSnapshotID,
		ExtensionID:          parent.ExtensionID,
		ModuleID:             parent.ModuleID,
		WorkspaceID:          parent.WorkspaceID,
		Source:               source,
		Budget:               parent.Budget,
		Metadata:             metadata,
		CreatedAt:            time.Now().UTC(),
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

func (c ExecutionContext) ToInvocationSource() string {
	return c.Source
}
