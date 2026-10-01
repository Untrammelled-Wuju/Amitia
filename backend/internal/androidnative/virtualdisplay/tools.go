package virtualdisplay

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const RuntimeID = "android_native_virtual_display"

func BuildVirtualDisplayTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   RuntimeID,
	}

	tools := []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildCreateTool(runtime),
		buildGetTool(runtime),
		buildListTool(runtime),
		buildResizeTool(runtime),
		buildReleaseTool(runtime),
		buildLaunchTool(runtime),
		buildCaptureTool(runtime),
		buildTapTool(runtime),
		buildSwipeTool(runtime),
		buildKeyTool(runtime),
		buildTextTool(runtime),
	}
	for index := range tools {
		tools[index].ModelExposure = capability.ModelExposureRule{
			ExposedByDefault: true,
			Categories:       []string{"android", "automation", "virtual_display"},
			Priority:         48,
		}
	}
	return tools
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
			"supported": {"type": "boolean"},
			"featureSecondaryDisplays": {"type": "boolean"},
			"canCreate": {"type": "boolean"},
			"active": {"type": "boolean"},
			"display": {"type": "object"},
			"frameSourceSupported": {"type": "boolean"},
			"uiTreeSupported": {"type": "boolean"},
			"gestureSupported": {"type": "boolean"},
			"thirdPartyLaunchSupported": {"type": "boolean"},
			"uiTreeTool": {"type": "string"},
			"nodeActionTools": {"type": "array", "items": {"type": "string"}},
			"state": {"type": "string"},
			"reason": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.virtual_display.status",
		ModelName:    "android_virtual_display_status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Virtual Display Status",
		Description:  "查询Android虚拟显示能力状态。虚拟屏优先使用android_virtual_display_capture对displayId截图，再用android_interaction_visual_locate或android_interaction_visual_click传displayId定位并操作；uiTreeSupported=true时可使用android_ui_tree_snapshot增强。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionVirtualDisplayInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-vd-v1"},
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

func buildCreateTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"width": {
				"type": "integer",
				"minimum": 320,
				"maximum": 2560
			},
			"height": {
				"type": "integer",
				"minimum": 320,
				"maximum": 2560
			},
			"densityDpi": {
				"type": "integer",
				"minimum": 72,
				"maximum": 640
			},
			"refreshRate": {
				"type": "number",
				"minimum": 0,
				"maximum": 1000
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"display": {"type": "object"},
			"frameSourceReady": {"type": "boolean"},
			"thirdPartyLaunchSupported": {"type": "boolean"},
			"uiTreeSupported": {"type": "boolean"},
			"gestureSupported": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.virtual_display.create",
		ModelName:    "android_virtual_display_create",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Virtual Display Create",
		Description:  "创建Android虚拟显示器。默认1080x1920 @ 420dpi。支持同时存在多个活动虚拟显示器，返回真实 Android system displayId。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionVirtualDisplayManage, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      true,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-vd-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationCreate,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationCreate,
		},
		Enabled: true,
	}
}

func buildGetTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"ref": {"type": "string"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"display": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.virtual_display.get",
		ModelName:    "android_virtual_display_get",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Virtual Display Get",
		Description:  "获取当前活动虚拟显示器的详细信息。可指定ref验证一致性。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionVirtualDisplayInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-vd-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationGet,
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
			HandlerName: OperationGet,
		},
		Enabled: true,
	}
}

func buildListTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	outputSchema := json.RawMessage(`{
		"type":"object",
		"properties":{
			"displays":{"type":"array","items":{"type":"object"}},
			"count":{"type":"integer"}
		}
	}`)
	return capability.ToolDefinition{
		ID: "android.virtual_display.list", ModelName: "android_virtual_display_list",
		Source: capability.ToolSourceBuiltin, Name: "Virtual Display List",
		Description: "列出当前由 AMITIA 管理的全部真实 Android VirtualDisplay 与 system displayId。",
		InputSchema: inputSchema, OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{{Capability: PermissionVirtualDisplayInspect, Risk: "low"}},
		RiskLevel:   capability.RiskLow, SideEffect: capability.SideEffectReadOnly,
		HasSideEffects: false, Idempotent: true, Retryable: true, TimeoutMS: 5000,
		ToolVersion:     capability.ToolVersion{SchemaVersion: 1, Revision: "workflow-final-vd-v2"},
		Metadata:        map[string]any{"androidNativeOperation": OperationList, "bridgeProtocol": "android_native"},
		ExecutionPolicy: capability.ToolExecutionPolicy{Timeout: 5 * time.Second, MaxConcurrency: 5, Idempotent: true, AllowBackground: true},
		ResultPolicy:    capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 8192, Streaming: capability.ToolStreamingPolicy{Enabled: false}},
		Runtime:         capability.RuntimeBinding{RuntimeType: runtime.RuntimeType, RuntimeID: runtime.RuntimeID, HandlerName: OperationList},
		Enabled:         true,
	}
}

func buildResizeTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"ref": {"type": "string"},
			"width": {
				"type": "integer",
				"minimum": 320,
				"maximum": 2560
			},
			"height": {
				"type": "integer",
				"minimum": 320,
				"maximum": 2560
			},
			"densityDpi": {
				"type": "integer",
				"minimum": 72,
				"maximum": 640
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"display": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.virtual_display.resize",
		ModelName:    "android_virtual_display_resize",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Virtual Display Resize",
		Description:  "调整虚拟显示器尺寸和密度。必须指定ref。Generation自增。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionVirtualDisplayManage, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-vd-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationResize,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationResize,
		},
		Enabled: true,
	}
}

func buildReleaseTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"ref": {"type": "string"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"released": {"type": "boolean"},
			"wasActive": {"type": "boolean"},
			"state": {"type": "string"},
			"status": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.virtual_display.release",
		ModelName:    "android_virtual_display_release",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Virtual Display Release",
		Description:  "释放虚拟显示器资源。幂等操作：重复释放不会报错。必须指定ref。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionVirtualDisplayManage, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-vd-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationRelease,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   1,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationRelease,
		},
		Enabled: true,
	}
}

func buildLaunchTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.launch",
		"android_virtual_display_launch",
		"Virtual Display Launch",
		"在指定 Android VirtualDisplay 上启动第三方应用。",
		OperationLaunch,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref","packageName"],
			"properties":{
				"ref":{"type":"string"},
				"packageName":{"type":"string","minLength":1,"maxLength":255},
				"component":{"type":"string","maxLength":512}
			}
		}`,
	)
}

func buildCaptureTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.capture",
		"android_virtual_display_capture",
		"Virtual Display Capture",
		"读取指定 Android VirtualDisplay 的最新画面，返回图像资源和元数据。可配合 android_interaction_visual_locate 或 android_interaction_visual_click 使用 displayId 进行截图定位。",
		OperationCapture,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref"],
			"properties":{
				"ref":{"type":"string"},
				"format":{"type":"string","enum":["png","jpeg","webp"]},
				"quality":{"type":"integer","minimum":1,"maximum":100},
				"maxWidth":{"type":"integer","minimum":320,"maximum":2560},
				"maxHeight":{"type":"integer","minimum":320,"maximum":2560}
			}
		}`,
	)
}

func buildTapTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.tap",
		"android_virtual_display_tap",
		"Virtual Display Tap",
		"向指定 Android VirtualDisplay 注入点击坐标。",
		OperationTap,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref","x","y"],
			"properties":{
				"ref":{"type":"string"},
				"x":{"type":"number"},
				"y":{"type":"number"}
			}
		}`,
	)
}

func buildSwipeTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.swipe",
		"android_virtual_display_swipe",
		"Virtual Display Swipe",
		"向指定 Android VirtualDisplay 注入滑动手势。",
		OperationSwipe,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref","startX","startY","endX","endY"],
			"properties":{
				"ref":{"type":"string"},
				"startX":{"type":"number"},
				"startY":{"type":"number"},
				"endX":{"type":"number"},
				"endY":{"type":"number"},
				"durationMs":{"type":"integer","minimum":1,"maximum":5000}
			}
		}`,
	)
}

func buildKeyTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.key",
		"android_virtual_display_key",
		"Virtual Display Key",
		"向指定 Android VirtualDisplay 注入按键事件。",
		OperationKey,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref","keyCode"],
			"properties":{
				"ref":{"type":"string"},
				"keyCode":{"type":"integer","minimum":1,"maximum":100000},
				"metaState":{"type":"integer","minimum":0,"maximum":2147483647}
			}
		}`,
	)
}

func buildTextTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildDisplayOperationTool(
		runtime,
		"android.virtual_display.text",
		"android_virtual_display_text",
		"Virtual Display Text",
		"向指定 Android VirtualDisplay 输入文本。",
		OperationText,
		`{
			"type":"object",
			"additionalProperties":false,
			"required":["ref","text"],
			"properties":{
				"ref":{"type":"string"},
				"text":{"type":"string","minLength":1,"maxLength":4096}
			}
		}`,
	)
}

func buildDisplayOperationTool(
	runtime capability.RuntimeBinding,
	id string,
	modelName string,
	name string,
	description string,
	operation string,
	inputSchema string,
) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:             id,
		ModelName:      modelName,
		Source:         capability.ToolSourceBuiltin,
		Name:           name,
		Description:    description,
		InputSchema:    json.RawMessage(inputSchema),
		OutputSchema:   json.RawMessage(`{"type":"object","additionalProperties":true}`),
		Permissions:    []capability.PermissionRequirement{{Capability: PermissionVirtualDisplayManage, Risk: "medium"}},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      true,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "app-process-host-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": operation,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 16 * 1024 * 1024,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: operation,
		},
		Enabled: true,
	}
}
