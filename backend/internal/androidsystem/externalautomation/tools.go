package externalautomation

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildExternalAutomationTools() []capability.ToolDefinition {
	rt := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   RuntimeIDExternalAutomation,
	}

	return []capability.ToolDefinition{
		buildStatusTool(rt),
		buildResolveAppTool(rt),
		buildOpenAppTool(rt),
		buildResolveURITool(rt),
		buildOpenURITool(rt),
		buildOpenSettingsTool(rt),
		buildInvokeIntentTool(rt),
		buildForegroundTool(rt),
		buildWaitForegroundTool(rt),
	}
}

func buildStatusTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"supported": {"type": "boolean"},
			"canResolveApps": {"type": "boolean"},
			"canLaunchApps": {"type": "boolean"},
			"canResolveUri": {"type": "boolean"},
			"canOpenUri": {"type": "boolean"},
			"canOpenSettings": {"type": "boolean"},
			"canInvokeIntent": {"type": "boolean"},
			"canInspectForeground": {"type": "boolean"},
			"canWaitForeground": {"type": "boolean"},
			"state": {"type": "string"},
			"reason": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDStatus,
		ModelName:    "android.external_automation.status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "External Automation Status",
		Description:  "查询Android External Automation capability状态，包含应用解析、启动、URI、Settings、Intent、前台检测等能力。不触发任何外部操作。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      2000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationStatus,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          2 * time.Second,
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
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationStatus,
		},
		Enabled: true,
	}
}

func buildResolveAppTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "minLength": 1, "maxLength": 256}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"apps": {"type": "array"},
			"count": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDResolveApp,
		ModelName:    "android.external_automation.resolve_app",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Resolve App",
		Description:  "解析目标App，支持包名、component或App label查询。只返回有限候选，不暴露全部安装App。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      3000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationResolveApp,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          3 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationResolveApp,
		},
		Enabled: true,
	}
}

func buildOpenAppTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["packageName"],
		"properties": {
			"packageName": {"type": "string", "pattern": "^[a-zA-Z][a-zA-Z0-9_]*(\\.[a-zA-Z][a-zA-Z0-9_]*)+$"},
			"component": {"type": "string"},
			"extras": {"type": "object"},
			"newTask": {"type": "boolean"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"targetPackage": {"type": "string"},
			"targetComponent": {"type": "string"},
			"resolved": {"type": "boolean"},
			"started": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"},
			"timestamp": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDOpenApp,
		ModelName:    "android.external_automation.open_app",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Open App",
		Description:  "启动外部App。使用PackageManager launch intent。若App不可启动返回APP_NOT_LAUNCHABLE。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionLaunch, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenApp,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 2048,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationOpenApp,
		},
		Enabled: true,
	}
}

func buildResolveURITool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["uri"],
		"properties": {
			"uri": {"type": "string", "format": "uri", "maxLength": 8192},
			"action": {"type": "string"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"scheme": {"type": "string"},
			"resolved": {"type": "boolean"},
			"handlers": {"type": "array"},
			"defaultHandler": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDResolveURI,
		ModelName:    "android.external_automation.resolve_uri",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Resolve URI",
		Description:  "解析URI/Deep Link。返回可处理的App候选。只做解析不打开。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      3000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationResolveURI,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          3 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationResolveURI,
		},
		Enabled: true,
	}
}

func buildOpenURITool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["uri"],
		"properties": {
			"uri": {"type": "string", "format": "uri", "maxLength": 8192},
			"packageName": {"type": "string"},
			"preferExternal": {"type": "boolean"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"targetPackage": {"type": "string"},
			"targetComponent": {"type": "string"},
			"userActionRequired": {"type": "boolean"},
			"timestamp": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDOpenURI,
		ModelName:    "android.external_automation.open_uri",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Open URI",
		Description:  "打开URI/Deep Link。通过ACTION_VIEW打开系统默认浏览器或指定可处理App。禁止javascript/file/data/intent scheme。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionOpenURI, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenURI,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 2048,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationOpenURI,
		},
		Enabled: true,
	}
}

func buildOpenSettingsTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["page"],
		"properties": {
			"page": {"type": "string", "enum": ["app_details", "accessibility", "overlay", "notifications", "battery", "unknown_sources", "wireless", "bluetooth", "location", "default_apps"]},
			"packageName": {"type": "string"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"userActionRequired": {"type": "boolean"},
			"timestamp": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDOpenSettings,
		ModelName:    "android.external_automation.open_settings",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Open Settings",
		Description:  "打开系统设置页面。只支持受控Settings Page枚举，不允许任意Settings action。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionSettings, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenSettings,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 1024,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationOpenSettings,
		},
		Enabled: true,
	}
}

func buildInvokeIntentTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["action"],
		"properties": {
			"action": {"type": "string"},
			"data": {"type": "string"},
			"packageName": {"type": "string"},
			"component": {"type": "string"},
			"categories": {"type": "array", "items": {"type": "string"}},
			"extras": {"type": "object"},
			"mode": {"type": "string", "enum": ["activity"]}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"targetPackage": {"type": "string"},
			"targetComponent": {"type": "string"},
			"userActionRequired": {"type": "boolean"},
			"timestamp": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDInvokeIntent,
		ModelName:    "android.external_automation.invoke_intent",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Invoke Intent",
		Description:  "调用受控Android Intent。默认只允许Activity模式，默认允许ACTION_VIEW/MAIN/DIAL/SENDTO。禁止ACTION_SEND重复实现Share。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionIntent, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationInvokeIntent,
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
			MaxOutputBytes: 2048,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationInvokeIntent,
		},
		Enabled: true,
	}
}

func buildForegroundTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"packageName": {"type": "string"},
			"component": {"type": "string"},
			"label": {"type": "string"},
			"displayId": {"type": "integer"},
			"observedAt": {"type": "integer"},
			"source": {"type": "string"},
			"confidence": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDForeground,
		ModelName:    "android.external_automation.foreground",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Foreground State",
		Description:  "查询当前前台App/Activity状态。优先复用Accessibility状态，UsageStats作为fallback。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      2000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationForeground,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          2 * time.Second,
			MaxConcurrency:   3,
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
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationForeground,
		},
		Enabled: true,
	}
}

func buildWaitForegroundTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["packageName"],
		"properties": {
			"packageName": {"type": "string"},
			"component": {"type": "string"},
			"timeoutMs": {"type": "integer", "minimum": 0, "maximum": 30000}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"packageName": {"type": "string"},
			"component": {"type": "string"},
			"label": {"type": "string"},
			"observedAt": {"type": "integer"},
			"source": {"type": "string"},
			"confidence": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           ToolIDWaitForeground,
		ModelName:    "android.external_automation.wait_foreground",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Wait Foreground",
		Description:  "等待目标App/Activity成为前台。单次阻塞Tool，有明确timeout(默认5s，最大30s)。用于open_app/open_uri后的同步。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionLaunch, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      32000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-extauto-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationWaitForeground,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          32 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 2048,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationWaitForeground,
		},
		Enabled: true,
	}
}
