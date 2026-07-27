package workflow

import (
	"encoding/json"
)

type WorkflowLimits struct {
	MaxSteps               int   `json:"maxSteps"`
	MaxExecutionDurationMS int64 `json:"maxExecutionDurationMs"`
	MaxStepDurationMS      int64 `json:"maxStepDurationMs"`
	MaxInputBytes          int64 `json:"maxInputBytes"`
	MaxOutputBytes         int64 `json:"maxOutputBytes"`
	MaxIntermediateBytes   int64 `json:"maxIntermediateBytes"`
	MaxHTTPResponseBytes   int64 `json:"maxHttpResponseBytes"`
	MaxHTTPRedirects       int   `json:"maxHttpRedirects"`
	MaxSkillCallDepth      int   `json:"maxSkillCallDepth"`
	MaxSkillCalls          int   `json:"maxSkillCalls"`
	MaxArrayItems          int   `json:"maxArrayItems"`
	MaxExpressionDepth     int   `json:"maxExpressionDepth"`
	MaxTemplateLength      int   `json:"maxTemplateLength"`
	MaxEventsEmitted       int   `json:"maxEventsEmitted"`
	MaxSchedulesCreated    int   `json:"maxSchedulesCreated"`
	MaxSideEffects         int   `json:"maxSideEffects"`
}

type WorkflowStepInput struct {
	Input  json.RawMessage  `json:"input"`
	When   *json.RawMessage `json:"when,omitempty"`
	OnError WorkflowOnError `json:"onError,omitempty"`
}

type WorkflowOnError struct {
	Mode    string          `json:"mode"`
	Default json.RawMessage `json:"default,omitempty"`
}

type WorkflowNode struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	DependsOn []string          `json:"dependsOn,omitempty"`
	Step      WorkflowStepInput `json:"step"`
}

type WorkflowDefinition struct {
	SchemaVersion    string            `json:"schemaVersion"`
	ID               string            `json:"id"`
	ExtensionID      string            `json:"extensionId,omitempty"`
	ModuleID         string            `json:"moduleId,omitempty"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	InputSchema      json.RawMessage   `json:"inputSchema"`
	OutputSchema     json.RawMessage   `json:"outputSchema"`
	Nodes            []WorkflowNode    `json:"nodes"`
	Permissions      []string          `json:"permissions,omitempty"`
	Scope            string            `json:"scope,omitempty"`
	CallableByAgent  bool              `json:"callableByAgent"`
	Enabled          bool              `json:"enabled"`
	HasSideEffects   bool              `json:"hasSideEffects,omitempty"`
	Idempotent       bool              `json:"idempotent,omitempty"`
	Limits           WorkflowLimits    `json:"limits,omitempty"`
	Version          string            `json:"version,omitempty"`
	Source           string            `json:"source,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}
