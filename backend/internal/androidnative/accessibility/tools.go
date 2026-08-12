package accessibility

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type PermissionDefinition struct {
	ID          string
	Name        string
	Description string
	Risk        string
}

func BuildPermissionDefinitions() []PermissionDefinition {
	return []PermissionDefinition{
		{
			ID:          androidnative.PermissionAccessibilityReadState,
			Name:        "android.accessibility.read_state",
			Description: "允许Agent读取Android无障碍服务授权与连接状态。与Kernel权限独立于Android系统Accessibility授权。",
			Risk:        "low",
		},
		{
			ID:          androidnative.PermissionAccessibilityOpenSettings,
			Name:        "android.accessibility.open_settings",
			Description: "允许Agent打开Android系统无障碍设置页，引导用户手动开启Amitia无障碍服务。",
			Risk:        "medium",
		},
	}
}

func BuildAccessibilityTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_accessibility",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildOpenSettingsTool(runtime),
	}
}

func buildStatusTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"platformSupported": {"type": "boolean"},
			"serviceDeclared": {"type": "boolean"},
			"enabledInSettings": {"type": "boolean"},
			"connected": {"type": "boolean"},
			"canRetrieveWindowContent": {"type": "boolean"},
			"canRetrieveInteractiveWindows": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"},
			"state": {"type": "string"},
			"generation": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.accessibility.status",
		ModelName:   "android.accessibility.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Accessibility Status",
		Description: "查询Android无障碍服务的授权与连接状态。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidnative.PermissionAccessibilityReadState, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b27-accessibility-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationStatus,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 2048,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationStatus,
		},
		Enabled: true,
	}
}

func buildOpenSettingsTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"opened": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.accessibility.open_settings",
		ModelName:   "android.accessibility.open_settings",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Open Accessibility Settings",
		Description: "打开Android系统无障碍设置页，引导用户手动开启Amitia无障碍服务。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidnative.PermissionAccessibilityOpenSettings, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b27-accessibility-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenSettings,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 1024,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationOpenSettings,
		},
		Enabled: true,
	}
}
