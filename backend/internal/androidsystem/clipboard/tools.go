package clipboard

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildClipboardTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_clipboard",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildReadTool(runtime),
		buildWriteTool(runtime),
		buildClearTool(runtime),
	}
}

func buildStatusTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           ToolIDStatus,
		ModelName:    "android.clipboard.status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Clipboard Status",
		Description:  "查询Android Clipboard capability状态，包含前后台/焦点限制。不读取剪贴板正文。",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"supported":{"type":"boolean"},"canWrite":{"type":"boolean"},"canRead":{"type":"boolean"},"appForeground":{"type":"boolean"},"state":{"type":"string"}}}`),
		Permissions:  []capability.PermissionRequirement{{Capability: PermissionRead, Risk: "low"}},
		RiskLevel:    capability.RiskLow,
		SideEffect:   capability.SideEffectReadOnly,
		Idempotent:   true,
		Retryable:    true,
		TimeoutMS:    5000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "b40-clipboard-v1"},
		Metadata:     map[string]any{"androidNativeOperation": OperationStatus, "bridgeProtocol": "android_native"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout: 5 * time.Second, MaxConcurrency: 5, Idempotent: true, AllowBackground: true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 2048},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType, RuntimeID: rt.RuntimeID, HandlerName: OperationStatus,
		},
		Enabled: true,
	}
}

func buildReadTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           ToolIDReadText,
		ModelName:    "android.clipboard.read_text",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Read Clipboard Text",
		Description:  "读取Android剪贴板文本内容。需前台+焦点条件（Android 10+）。",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"hasContent":{"type":"boolean"},"text":{"type":"string"},"mimeType":{"type":"string"}}}`),
		Permissions:  []capability.PermissionRequirement{{Capability: PermissionRead, Risk: "high"}},
		RiskLevel:    capability.RiskHigh,
		SideEffect:   capability.SideEffectReadOnly,
		Idempotent:   false,
		Retryable:    false,
		TimeoutMS:    5000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "b40-clipboard-v1"},
		Metadata:     map[string]any{"androidNativeOperation": OperationReadText, "bridgeProtocol": "android_native"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout: 5 * time.Second, MaxConcurrency: 2, AllowBackground: false,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 65536},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType, RuntimeID: rt.RuntimeID, HandlerName: OperationReadText,
		},
		Enabled: true,
	}
}

func buildWriteTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           ToolIDWriteText,
		ModelName:    "android.clipboard.write_text",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Write Clipboard Text",
		Description:  "写入纯文本到Android剪贴板。固定amitia标签，不影响Runtime语义。",
		InputSchema:  json.RawMessage(`{"type":"object","required":["text"],"properties":{"text":{"type":"string","maxLength":65536},"sensitive":{"type":"boolean"}},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"written":{"type":"boolean"},"bytes":{"type":"integer"},"sensitive":{"type":"boolean"}}}`),
		Permissions:  []capability.PermissionRequirement{{Capability: PermissionWrite, Risk: "medium"}},
		RiskLevel:    capability.RiskMedium,
		SideEffect:   capability.SideEffectWrite,
		Idempotent:   true,
		Retryable:    false,
		TimeoutMS:    5000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "b40-clipboard-v1"},
		Metadata:     map[string]any{"androidNativeOperation": OperationWriteText, "bridgeProtocol": "android_native"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout: 5 * time.Second, MaxConcurrency: 2, Idempotent: true, AllowBackground: true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 1024},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType, RuntimeID: rt.RuntimeID, HandlerName: OperationWriteText,
		},
		Enabled: true,
	}
}

func buildClearTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           ToolIDClear,
		ModelName:    "android.clipboard.clear",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Clear Clipboard",
		Description:  "清除Android当前剪贴板内容。需要write权限。",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"cleared":{"type":"boolean"}}}`),
		Permissions:  []capability.PermissionRequirement{{Capability: PermissionWrite, Risk: "medium"}},
		RiskLevel:    capability.RiskMedium,
		SideEffect:   capability.SideEffectWrite,
		Idempotent:   true,
		Retryable:    false,
		TimeoutMS:    5000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "b40-clipboard-v1"},
		Metadata:     map[string]any{"androidNativeOperation": OperationClear, "bridgeProtocol": "android_native"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout: 5 * time.Second, MaxConcurrency: 2, ApprovalRequired: true, AllowBackground: true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 512},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType, RuntimeID: rt.RuntimeID, HandlerName: OperationClear,
		},
		Enabled: true,
	}
}
