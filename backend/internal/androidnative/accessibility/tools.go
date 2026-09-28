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
		{
			ID:          androidnative.PermissionAccessibilityManage,
			Name:        "android.accessibility.manage",
			Description: "允许Agent安装、更新或打开Amitia无障碍Provider的设置页。",
			Risk:        "high",
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
		buildProviderStatusTool(runtime),
		buildProviderInstallTool(runtime),
		buildProviderOpenSettingsTool(runtime),
	}
}

func buildProviderStatusTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.accessibility.provider.status",
		ModelName:    "android_accessibility_provider_status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Accessibility Provider Status",
		Description:  "查询Amitia无障碍Provider的安装、连接和版本状态。",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"installed":{"type":"boolean"},"connected":{"type":"boolean"},"ready":{"type":"boolean"},"versionName":{"type":"string"},"packageName":{"type":"string"},"state":{"type":"string"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: androidnative.PermissionAccessibilityReadState, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 44},
		Metadata: map[string]any{
			"androidNativeOperation": OperationProviderStatus,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.accessibility.provider.status",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   4,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationProviderStatus,
		},
		Enabled: true,
	}
}

func buildProviderInstallTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"opened":{"type":"boolean"},"userActionRequired":{"type":"boolean"}}}`)
	return capability.ToolDefinition{
		ID:           "android.accessibility.provider.install",
		ModelName:    "android_accessibility_provider_install",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Install Accessibility Provider",
		Description:  "打开Amitia无障碍Provider安装或更新确认流程。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidnative.PermissionAccessibilityManage, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 44},
		Metadata: map[string]any{
			"androidNativeOperation": OperationProviderInstall,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.accessibility.provider.install",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationProviderInstall,
		},
		Enabled: true,
	}
}

func buildProviderOpenSettingsTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"opened":{"type":"boolean"},"userActionRequired":{"type":"boolean"}}}`)
	return capability.ToolDefinition{
		ID:           "android.accessibility.provider.open_settings",
		ModelName:    "android_accessibility_provider_open_settings",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Open Accessibility Provider Settings",
		Description:  "打开Amitia无障碍Provider的应用详情或系统设置。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidnative.PermissionAccessibilityManage, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 44},
		Metadata: map[string]any{
			"androidNativeOperation": OperationProviderOpenSettings,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.accessibility.provider.open_settings",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationProviderOpenSettings,
		},
		Enabled: true,
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
			"includeNotImportantViews": {"type": "boolean"},
			"enhancedWebAccessibility": {"type": "boolean"},
			"canPerformGestures": {"type": "boolean"},
			"canTakeScreenshot": {"type": "boolean"},
			"interactionReady": {"type": "boolean"},
			"visualReady": {"type": "boolean"},
			"ready": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"},
			"state": {"type": "string"},
			"generation": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.accessibility.status",
		ModelName:    "android_accessibility_status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Accessibility Status",
		Description:  "查询Android无障碍服务的授权与连接状态。",
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
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationStatus,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.accessibility.status",
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
		ID:           "android.accessibility.open_settings",
		ModelName:    "android_accessibility_open_settings",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Open Accessibility Settings",
		Description:  "打开Android系统无障碍设置页，引导用户手动开启Amitia无障碍服务。",
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
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenSettings,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.accessibility.open_settings",
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
