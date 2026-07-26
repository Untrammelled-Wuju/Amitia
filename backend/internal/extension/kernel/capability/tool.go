package capability

import (
	"context"
	"encoding/json"
)

type ToolSource string

const (
	ToolSourceBuiltin  ToolSource = "builtin"
	ToolSourcePlugin   ToolSource = "plugin"
	ToolSourceMCP      ToolSource = "mcp"
	ToolSourceWorkflow ToolSource = "workflow"
	ToolSourceInternal ToolSource = "internal"
	ToolSourceLegacy   ToolSource = "legacy_tool"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type SideEffectLevel string

const (
	SideEffectNone        SideEffectLevel = "none"
	SideEffectReadOnly    SideEffectLevel = "read_only"
	SideEffectWrite       SideEffectLevel = "write"
	SideEffectExternal    SideEffectLevel = "external"
	SideEffectSystem      SideEffectLevel = "system"
	SideEffectFinancial   SideEffectLevel = "financial"
	SideEffectDestructive SideEffectLevel = "destructive"
)

type PermissionRequirement struct {
	Capability  string `json:"capability"`
	Description string `json:"description,omitempty"`
	Risk        string `json:"risk,omitempty"`
}

type ScopeRule struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

type LegacyToolExecutor func(ctx context.Context, input json.RawMessage, scope map[string]string) (json.RawMessage, error)

type ToolInvocation struct {
	ToolID         string          `json:"toolId"`
	Input          json.RawMessage `json:"input"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	TraceID        string          `json:"traceId,omitempty"`
}

type ToolResult struct {
	InvocationID string          `json:"invocationId"`
	Status       string          `json:"status"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	VisibleText  string          `json:"visibleText,omitempty"`
	DurationMS   int64           `json:"durationMs"`
}

func BuildToolID(source ToolSource, namespace, name string) string {
	return BuildCapabilityID(CapabilitySource(source), namespace, name)
}
