package capability

import (
	"context"
	"encoding/json"
)

type RuntimeType string

const (
	RuntimeTypeBuiltin       RuntimeType = "builtin"
	RuntimeTypePluginJS      RuntimeType = "plugin_js"
	RuntimeTypePluginService RuntimeType = "plugin_service"
	RuntimeTypeMCP           RuntimeType = "mcp"
	RuntimeTypeWorkflow      RuntimeType = "workflow"
	RuntimeTypeInternal      RuntimeType = "internal"
	RuntimeTypeLegacy        RuntimeType = "legacy"
)

type RuntimeBinding struct {
	RuntimeType RuntimeType    `json:"runtimeType"`
	RuntimeID   string         `json:"runtimeId"`
	HandlerName string         `json:"handlerName"`
	Endpoint    string         `json:"endpoint,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RuntimeAdapter interface {
	Supports(binding RuntimeBinding) bool

	Execute(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) UnifiedToolResult

	Health(
		ctx context.Context,
		binding RuntimeBinding,
	) HealthStatus
}
