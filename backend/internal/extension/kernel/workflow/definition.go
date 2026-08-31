package workflow

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
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
	MaxConcurrency         int   `json:"maxConcurrency"`
}

type WorkflowStepInput struct {
	Input   json.RawMessage  `json:"input"`
	When    *json.RawMessage `json:"when,omitempty"`
	OnError WorkflowOnError  `json:"onError,omitempty"`
}

type WorkflowOnError struct {
	Mode    string          `json:"mode"`
	Default json.RawMessage `json:"default,omitempty"`
}

type WorkflowPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type WorkflowEdge struct {
	ID           string          `json:"id"`
	Source       string          `json:"source"`
	Target       string          `json:"target"`
	SourceHandle string          `json:"sourceHandle,omitempty"`
	TargetHandle string          `json:"targetHandle,omitempty"`
	Label        string          `json:"label,omitempty"`
	Condition    json.RawMessage `json:"condition,omitempty"`
}

type WorkflowTriggerDefinition struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	EventType string          `json:"eventType,omitempty"`
	Schedule  json.RawMessage `json:"schedule,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Enabled   bool            `json:"enabled"`
}

// WorkflowAgentToolConfig controls how a user workflow is exposed to the model
// tool catalog. The runtime still uses CallableByAgent as the hard enable gate;
// this object only defines the stable model-facing name and description.
type WorkflowAgentToolConfig struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type WorkflowNode struct {
	ID          string                    `json:"id"`
	Type        string                    `json:"type"`
	DependsOn   []string                  `json:"dependsOn,omitempty"`
	TargetID    string                    `json:"targetId,omitempty"`
	Runtime     capability.RuntimeBinding `json:"runtime,omitempty"`
	Permissions []string                  `json:"permissions,omitempty"`
	Scope       string                    `json:"scope,omitempty"`
	Position    WorkflowPosition          `json:"position,omitempty"`
	Label       string                    `json:"label,omitempty"`
	Step        WorkflowStepInput         `json:"step"`
}

type WorkflowDefinition struct {
	SchemaVersion   string                      `json:"schemaVersion"`
	ID              string                      `json:"id"`
	ExtensionID     string                      `json:"extensionId,omitempty"`
	ModuleID        string                      `json:"moduleId,omitempty"`
	Name            string                      `json:"name"`
	Description     string                      `json:"description"`
	InputSchema     json.RawMessage             `json:"inputSchema"`
	OutputSchema    json.RawMessage             `json:"outputSchema"`
	Nodes           []WorkflowNode              `json:"nodes"`
	Edges           []WorkflowEdge              `json:"edges,omitempty"`
	Triggers        []WorkflowTriggerDefinition `json:"triggers,omitempty"`
	Permissions     []string                    `json:"permissions,omitempty"`
	Scope           string                      `json:"scope,omitempty"`
	CallableByAgent bool                        `json:"callableByAgent"`
	AgentTool       WorkflowAgentToolConfig     `json:"agentTool,omitempty"`
	Enabled         bool                        `json:"enabled"`
	HasSideEffects  bool                        `json:"hasSideEffects,omitempty"`
	Idempotent      bool                        `json:"idempotent,omitempty"`
	Limits          WorkflowLimits              `json:"limits,omitempty"`
	Version         string                      `json:"version,omitempty"`
	Source          string                      `json:"source,omitempty"`
	Metadata        map[string]any              `json:"metadata,omitempty"`
	DefinitionHash  string                      `json:"definitionHash,omitempty"`
}
