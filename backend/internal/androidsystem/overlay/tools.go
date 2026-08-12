package overlay

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildOverlayTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   RuntimeIDOverlay,
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildPermissionRequestTool(runtime),
		buildCreateTool(runtime),
		buildUpdateTool(runtime),
		buildShowTool(runtime),
		buildHideTool(runtime),
		buildCloseTool(runtime),
		buildListTool(runtime),
		buildCloseAllTool(runtime),
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
			"permissionRequired": {"type": "boolean"},
			"permissionGranted": {"type": "boolean"},
			"nativeHostReady": {"type": "boolean"},
			"canCreate": {"type": "boolean"},
			"canUpdate": {"type": "boolean"},
			"canInteract": {"type": "boolean"},
			"activeCount": {"type": "integer"},
			"userActionRequired": {"type": "boolean"},
			"state": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDStatus,
		ModelName:   "android.overlay.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Overlay Status",
		Description: "查询Android Overlay capability状态，包含SYSTEM_ALERT_WINDOW权限状态和活跃Overlay数量。不触发授权页面。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
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
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationStatus,
		},
		Enabled: true,
	}
}

func buildPermissionRequestTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"opened": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"},
			"permissionGranted": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDPermissionRequest,
		ModelName:   "android.overlay.permission_request",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Request Overlay Permission",
		Description: "打开Android Overlay授权设置页面（SYSTEM_ALERT_WINDOW）。用户离开系统设置后必须重新查询状态确认授权结果。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationPermissionRequest,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
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
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationPermissionRequest,
		},
		Enabled: true,
	}
}

func buildCreateTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["kind"],
		"properties": {
			"kind": {"type": "string", "enum": ["text", "image", "card", "status"]},
			"content": {"type": "object"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"width": {"type": "integer", "maximum": 1080},
			"height": {"type": "integer", "maximum": 1920},
			"gravity": {"type": "string", "enum": ["top_left", "top_right", "bottom_left", "bottom_right", "center", "top_center", "bottom_center"]},
			"focusable": {"type": "boolean"},
			"touchable": {"type": "boolean"},
			"draggable": {"type": "boolean"},
			"ttlMs": {"type": "integer", "maximum": 86400000}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"overlay": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDCreate,
		ModelName:   "android.overlay.create",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Create Overlay",
		Description: "创建Android系统悬浮层（TYPE_APPLICATION_OVERLAY）。需要SYSTEM_ALERT_WINDOW权限。默认非焦点、可触摸、不可拖动。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationCreate,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationCreate,
		},
		Enabled: true,
	}
}

func buildUpdateTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["overlayId"],
		"properties": {
			"overlayId": {"type": "string", "pattern": "^ovl_"},
			"content": {"type": "object"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"width": {"type": "integer", "maximum": 1080},
			"height": {"type": "integer", "maximum": 1920},
			"gravity": {"type": "string", "enum": ["top_left", "top_right", "bottom_left", "bottom_right", "center", "top_center", "bottom_center"]},
			"focusable": {"type": "boolean"},
			"touchable": {"type": "boolean"},
			"draggable": {"type": "boolean"},
			"ttlMs": {"type": "integer", "maximum": 86400000}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"overlay": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDUpdate,
		ModelName:   "android.overlay.update",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Update Overlay",
		Description: "更新已存在的Overlay内容、位置、尺寸等属性。不会创建新Overlay。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationUpdate,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationUpdate,
		},
		Enabled: true,
	}
}

func buildShowTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["overlayId"],
		"properties": {
			"overlayId": {"type": "string", "pattern": "^ovl_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"overlay": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDShow,
		ModelName:   "android.overlay.show",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Show Overlay",
		Description: "显示已隐藏的Overlay。重复show不会重复addView。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationShow,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       true,
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
			HandlerName: OperationShow,
		},
		Enabled: true,
	}
}

func buildHideTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["overlayId"],
		"properties": {
			"overlayId": {"type": "string", "pattern": "^ovl_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"overlay": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDHide,
		ModelName:   "android.overlay.hide",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Hide Overlay",
		Description: "隐藏Overlay但保留instance。hide不会删除metadata，show可以恢复。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationHide,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       true,
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
			HandlerName: OperationHide,
		},
		Enabled: true,
	}
}

func buildCloseTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["overlayId"],
		"properties": {
			"overlayId": {"type": "string", "pattern": "^ovl_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"closed": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDClose,
		ModelName:   "android.overlay.close",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Close Overlay",
		Description: "关闭并释放Overlay资源。幂等操作，已关闭的Overlay返回alreadyClosed。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationClose,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       true,
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
			HandlerName: OperationClose,
		},
		Enabled: true,
	}
}

func buildListTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"overlays": {"type": "array"},
			"count": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDList,
		ModelName:   "android.overlay.list",
		Source:      capability.ToolSourceBuiltin,
		Name:        "List Overlays",
		Description: "列出Amitia当前持有的所有Overlay instance。不扫描其他App窗口。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationList,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 16384,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationList,
		},
		Enabled: true,
	}
}

func buildCloseAllTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"closedCount": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDCloseAll,
		ModelName:   "android.overlay.close_all",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Close All Overlays",
		Description: "关闭Amitia创建的所有Overlay。不影响其他App的悬浮窗。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionCreate, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b42-overlay-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationCloseAll,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       true,
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
			HandlerName: OperationCloseAll,
		},
		Enabled: true,
	}
}
