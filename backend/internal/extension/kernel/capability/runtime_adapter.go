package capability

import (
	"context"
	"encoding/json"
)

type RuntimeType string

const (
	RuntimeTypeBuiltin        RuntimeType = "builtin"
	RuntimeTypePluginJS       RuntimeType = "plugin_js"
	RuntimeTypePluginService  RuntimeType = "plugin_service"
	RuntimeTypeMCP            RuntimeType = "mcp"
	RuntimeTypeWorkflow       RuntimeType = "workflow"
	RuntimeTypeInternal       RuntimeType = "internal"
	RuntimeTypeLegacy         RuntimeType = "legacy"
	RuntimeTypeJavaScript     RuntimeType = "javascript"
	RuntimeTypeWASM           RuntimeType = "wasm"
	RuntimeTypeTrustedService RuntimeType = "trusted_service"
	RuntimeTypeTask           RuntimeType = "task"
	RuntimeTypeBrowser        RuntimeType = "browser"
	RuntimeTypeAndroid_Native     RuntimeType = "android_native"
	RuntimeTypeIOS_Native         RuntimeType = "ios_native"
	RuntimeTypeDesktop_Extension  RuntimeType = "desktop_extension"
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

type StreamingRuntimeAdapter interface {
	RuntimeAdapter

	ExecuteStream(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		input json.RawMessage,
		emitter ToolStreamEmitter,
	) UnifiedToolResult
}

type CancellableRuntimeAdapter interface {
	RuntimeAdapter

	Cancel(
		ctx context.Context,
		binding RuntimeBinding,
		invocation ToolInvocationContext,
		reason ToolCancellationReason,
	) error
}

type ErrRuntimeCancellationUnsupported struct{}

func (e ErrRuntimeCancellationUnsupported) Error() string {
	return "runtime does not support explicit cancellation"
}
