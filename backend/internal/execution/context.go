package execution

import (
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

const (
	MetadataOriginPrincipalType    = "originPrincipalType"
	MetadataOriginDeviceID         = "originDeviceId"
	MetadataOriginRuntimeID        = "originRuntimeId"
	MetadataOriginSessionID        = "originSessionId"
	MetadataOriginRuntimeSessionID = "originRuntimeSessionId"
)

type RuntimeTarget struct {
	Placement        string                    `json:"placement"`
	SpaceID          runtimeidentity.SpaceID   `json:"spaceId,omitempty"`
	DeviceID         runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`
	RuntimeSessionID string                    `json:"runtimeSessionId,omitempty"`
}

type WorkspaceBinding struct {
	ID       string `json:"id"`
	DeviceID string `json:"deviceId,omitempty"`
	Name     string `json:"name,omitempty"`
	Kind     string `json:"kind,omitempty"`
	RootURI  string `json:"rootUri,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

// SandboxConstraint marks an execution as confined to an OS sandbox root. It is
// set only for conversations that entered through a channel whose plugin runs
// inside an enforced sandbox; every other execution leaves it nil and keeps the
// unrestricted host behaviour.
type SandboxConstraint struct {
	Root         string `json:"root"`
	AllowNetwork bool   `json:"allowNetwork"`
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

	ExtensionID        string             `json:"extensionId,omitempty"`
	ModuleID           string             `json:"moduleId,omitempty"`
	WorkspaceID        string             `json:"workspaceId,omitempty"`
	PrimaryWorkspaceID string             `json:"primaryWorkspaceId,omitempty"`
	WorkspaceBindings  []WorkspaceBinding `json:"workspaceBindings,omitempty"`
	Sandbox            *SandboxConstraint `json:"sandbox,omitempty"`

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
	workspaces := append([]WorkspaceBinding(nil), parent.WorkspaceBindings...)
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
		PrimaryWorkspaceID:   parent.PrimaryWorkspaceID,
		WorkspaceBindings:    workspaces,
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
