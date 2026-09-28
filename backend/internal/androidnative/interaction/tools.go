package interaction

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildInteractionTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_interaction",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildClickTool(runtime),
		buildLongClickTool(runtime),
		buildInputTextTool(runtime),
		buildClearTextTool(runtime),
		buildScrollTool(runtime),
		buildSwipeTool(runtime),
		buildNodeActionTool(runtime),
		buildGlobalActionTool(runtime),
		buildGestureTool(runtime),
		buildScreenshotTool(runtime),
		buildVisualLocateTool(runtime),
		buildVisualClickTool(runtime),
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
			"available": {"type": "boolean"},
			"connected": {"type": "boolean"},
			"gestureAvailable": {"type": "boolean"},
			"accessibilityAction": {"type": "boolean"},
			"accessibilityGesture": {"type": "boolean"},
			"coordinateTap": {"type": "boolean"},
			"textInput": {"type": "boolean"},
			"scroll": {"type": "boolean"},
			"visualLocate": {"type": "boolean"},
			"ocrAvailable": {"type": "boolean"},
			"imageUnderstandAvailable": {"type": "boolean"},
			"shizuku": {"type": "boolean"},
			"rootFallback": {"type": "boolean"},
			"adbFallback": {"type": "boolean"},
			"providers": {"type": "object"},
			"state": {"type": "string"},
			"reason": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.status",
		ModelName:    "android_interaction_status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Status",
		Description:  "查询Android Interaction能力状态。检测Accessibility Action/Gesture、Coordinate、Visual Locate等能力可用性。不触发任何副作用。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionReadVisual, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationStatus,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.status",
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

func buildClickTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"target": {
				"type": "object",
				"properties": {
					"snapshotId": {"type": "string"},
					"nodeId": {"type": "string"},
					"x": {"type": "integer"},
					"y": {"type": "integer"},
					"text": {"type": "string"},
					"resourceId": {"type": "string"},
					"role": {"type": "string"},
					"description": {"type": "string"}
				}
			},
			"allowCoordinateFallback": {"type": "boolean"},
			"allowShizukuFallback": {"type": "boolean"},
			"allowVisualFallback": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"},
			"allowAdbFallback": {"type": "boolean"},
			"verify": {"type": "boolean"}
		},
		"required": ["target"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"displayId": {"type": "integer"},
			"verified": {"type": "boolean"},
			"verification": {"type": "string"},
			"durationMs": {"type": "integer"},
			"warning": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.click",
		ModelName:    "android_interaction_click",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Click",
		Description:  "点击Android UI目标。优先使用Accessibility ACTION_CLICK，失败时按策略降级到坐标、Shizuku、Root或ADB fallback。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionClick, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultNodeClickTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationClick,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.click",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultNodeClickTimeoutMS * time.Millisecond,
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
			HandlerName: OperationClick,
		},
		Enabled: true,
	}
}

func buildLongClickTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"target": {
				"type": "object",
				"properties": {
					"snapshotId": {"type": "string"},
					"nodeId": {"type": "string"},
					"x": {"type": "integer"},
					"y": {"type": "integer"}
				}
			},
			"durationMs": {
				"type": "integer",
				"minimum": 300,
				"maximum": 3000
			},
			"allowCoordinateFallback": {"type": "boolean"},
			"allowShizukuFallback": {"type": "boolean"},
			"allowVisualFallback": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"},
			"allowAdbFallback": {"type": "boolean"},
			"verify": {"type": "boolean"}
		},
		"required": ["target"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"verified": {"type": "boolean"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.long_click",
		ModelName:    "android_interaction_long_click",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Long Click",
		Description:  "长按Android UI目标。优先使用Accessibility ACTION_LONG_CLICK，失败时按策略降级到Gesture long press或Shizuku。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionClick, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultGestureTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationLongClick,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.long_click",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultGestureTimeoutMS * time.Millisecond,
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
			HandlerName: OperationLongClick,
		},
		Enabled: true,
	}
}

func buildInputTextTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"target": {
				"type": "object",
				"properties": {
					"snapshotId": {"type": "string"},
					"nodeId": {"type": "string"}
				}
			},
			"text": {"type": "string", "maxLength": 10000},
			"allowShizukuFallback": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"},
			"allowAdbFallback": {"type": "boolean"},
			"verify": {"type": "boolean"}
		},
		"required": ["target", "text"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"},
			"verified": {"type": "boolean"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.input_text",
		ModelName:    "android_interaction_input_text",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Input Text",
		Description:  "向Android UI输入文本。优先使用Accessibility ACTION_SET_TEXT，失败时按策略降级到Shizuku、Root或ADB。Password字段默认拒绝。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionInput, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultInputTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationInputText,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.input_text",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultInputTimeoutMS * time.Millisecond,
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
			HandlerName: OperationInputText,
		},
		Enabled: true,
	}
}

func buildClearTextTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"target": {
				"type": "object",
				"properties": {
					"snapshotId": {"type": "string"},
					"nodeId": {"type": "string"}
				}
			},
			"verify": {"type": "boolean"}
		},
		"required": ["target"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"},
			"verified": {"type": "boolean"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.clear_text",
		ModelName:    "android_interaction_clear_text",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Clear Text",
		Description:  "清空Android UI文本字段。使用Accessibility ACTION_SET_TEXT(\"\")。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionInput, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultInputTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationClearText,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.clear_text",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultInputTimeoutMS * time.Millisecond,
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
			HandlerName: OperationClearText,
		},
		Enabled: true,
	}
}

func buildScrollTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"target": {
				"type": "object",
				"properties": {
					"snapshotId": {"type": "string"},
					"nodeId": {"type": "string"}
				}
			},
			"direction": {
				"type": "string",
				"enum": ["forward", "backward", "up", "down", "left", "right"]
			},
			"amount": {
				"type": "string",
				"enum": ["small", "medium", "large"]
			},
			"allowCoordinateFallback": {"type": "boolean"},
			"allowShizukuFallback": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"},
			"allowAdbFallback": {"type": "boolean"},
			"verify": {"type": "boolean"}
		},
		"required": ["target", "direction"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"verified": {"type": "boolean"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.scroll",
		ModelName:    "android_interaction_scroll",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Scroll",
		Description:  "滚动Android UI。优先使用Accessibility ACTION_SCROLL_FORWARD/BACKWARD，失败时降级到swipe手势。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionGesture, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultGestureTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationScroll,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.scroll",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultGestureTimeoutMS * time.Millisecond,
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
			HandlerName: OperationScroll,
		},
		Enabled: true,
	}
}

func buildSwipeTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"displayId": {"type": "integer"},
			"startX": {"type": "integer"},
			"startY": {"type": "integer"},
			"endX": {"type": "integer"},
			"endY": {"type": "integer"},
			"durationMs": {
				"type": "integer",
				"minimum": 100,
				"maximum": 3000
			},
			"allowCoordinateFallback": {"type": "boolean"},
			"allowShizukuFallback": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"},
			"allowAdbFallback": {"type": "boolean"}
		},
		"required": ["startX", "startY", "endX", "endY"]
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"displayId": {"type": "integer"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.swipe",
		ModelName:    "android_interaction_swipe",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Swipe",
		Description:  "执行Android滑动手势。优先使用Accessibility Gesture或Coordinate executor，失败时按策略降级到Shizuku、Root或ADB。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionGesture, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultGestureTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationSwipe,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.swipe",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultGestureTimeoutMS * time.Millisecond,
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
			HandlerName: OperationSwipe,
		},
		Enabled: true,
	}
}

func buildVisualLocateTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"description": {"type": "string"},
			"text": {"type": "string"},
			"role": {"type": "string"},
			"expectedPackage": {"type": "string"},
			"ocrFirst": {"type": "boolean"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"candidates": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"source": {"type": "string"},
						"text": {"type": "string"},
						"description": {"type": "string"},
						"bounds": {"type": "object"},
						"centerX": {"type": "integer"},
						"centerY": {"type": "integer"},
						"confidence": {"type": "number"},
						"ocrLineId": {"type": "string"}
					}
				}
			},
			"count": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.visual_locate",
		ModelName:    "android_interaction_visual_locate",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Visual Locate",
		Description:  "通过截图和Image Intelligence定位Android UI目标。返回候选列表，不执行点击。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionReadVisual, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      int64(DefaultVisualLocateTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationVisualLocate,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.visual_locate",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultVisualLocateTimeoutMS * time.Millisecond,
			MaxConcurrency:   2,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 65536,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationVisualLocate,
		},
		Enabled: true,
	}
}

func buildVisualClickTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"description": {"type": "string"},
			"text": {"type": "string"},
			"role": {"type": "string"},
			"expectedPackage": {"type": "string"},
			"ocrFirst": {"type": "boolean"},
			"verify": {"type": "boolean"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"success": {"type": "boolean"},
			"operation": {"type": "string"},
			"strategy": {"type": "string"},
			"x": {"type": "integer"},
			"y": {"type": "integer"},
			"verified": {"type": "boolean"},
			"durationMs": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:           "android.interaction.visual_click",
		ModelName:    "android_interaction_visual_click",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Visual Click",
		Description:  "通过截图和Image Intelligence定位并点击Android UI目标。先visual_locate再coordinate click。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionClick, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      int64(DefaultVisualClickTimeoutMS),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-interaction-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationVisualClick,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.visual_click",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          DefaultVisualClickTimeoutMS * time.Millisecond,
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
			HandlerName: OperationVisualClick,
		},
		Enabled: true,
	}
}

func buildNodeActionTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type":"object",
		"required":["nativeRef","action"],
		"additionalProperties":false,
		"properties":{
			"nativeRef":{"type":"string","minLength":1,"maxLength":4096},
			"action":{"type":"string","enum":["click","long_click","focus","clear_focus","select","clear_selection","set_text","clear_text","set_selection","copy","cut","paste","scroll_forward","scroll_backward","scroll_up","scroll_down","scroll_left","scroll_right","scroll_to_position","expand","collapse","dismiss","show_on_screen","context_click","accessibility_focus","clear_accessibility_focus","show_tooltip","hide_tooltip","ime_enter","press_and_hold","next_at_movement_granularity","previous_at_movement_granularity","next_html_element","previous_html_element","set_progress"]},
			"args":{"type":"object","additionalProperties":true}
		}
	}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"},"action":{"type":"string"}}}`)
	return capability.ToolDefinition{
		ID:           "android.interaction.node_action",
		ModelName:    "android_interaction_node_action",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Node Action",
		Description:  "Execute a typed accessibility action on a resolved Android UI node.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionNodeAction, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationNodeAction,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.node_action",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 65536},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationNodeAction,
		},
		Enabled: true,
	}
}

func buildGlobalActionTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type":"object",
		"required":["action"],
		"additionalProperties":false,
		"properties":{
			"action":{"type":"string","enum":["back","home","recents","notifications","quick_settings","power_dialog","toggle_split_screen","lock_screen","take_screenshot","keycode_headset_hook","accessibility_shortcut","accessibility_all_apps","accessibility_button","accessibility_button_chooser","dismiss_notification_shade","dpad_up","dpad_down","dpad_left","dpad_right","dpad_center","media_play_pause","menu"]}
		}
	}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"performed":{"type":"boolean"},"success":{"type":"boolean"},"action":{"type":"string"},"generation":{"type":"integer"}}}`)
	return capability.ToolDefinition{
		ID:           "android.interaction.global_action",
		ModelName:    "android_interaction_global_action",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Global Action",
		Description:  "Perform a typed Android global navigation or system action through AccessibilityService.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "android.interaction.global", Risk: "medium"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 47},
		Metadata: map[string]any{
			"androidNativeOperation": OperationGlobalAction,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.global_action",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 8192},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationGlobalAction,
		},
		Enabled: true,
	}
}

func buildGestureTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type":"object",
		"required":["strokes"],
		"additionalProperties":false,
		"properties":{
			"displayId":{"type":"integer","minimum":0,"maximum":16},
			"strokes":{
				"type":"array",
				"minItems":1,
				"maxItems":16,
				"items":{
					"type":"object",
					"required":["points"],
					"additionalProperties":false,
					"properties":{
						"points":{
							"type":"array",
							"minItems":1,
							"maxItems":512,
							"items":{
								"oneOf":[
									{"type":"object","required":["x","y"],"additionalProperties":false,"properties":{"x":{"type":"number"},"y":{"type":"number"}}},
									{"type":"array","minItems":2,"maxItems":2,"items":{"type":"number"}}
								]
							}
						},
						"startTimeMs":{"type":"integer","minimum":0,"maximum":60000},
						"durationMs":{"type":"integer","minimum":1,"maximum":60000},
						"willContinue":{"type":"boolean"}
					}
				}
			}
		}
	}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"performed":{"type":"boolean"},"success":{"type":"boolean"},"action":{"type":"string"},"generation":{"type":"integer"}}}`)
	return capability.ToolDefinition{
		ID:           "android.interaction.gesture",
		ModelName:    "android_interaction_gesture",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Gesture",
		Description:  "Dispatch one or more bounded AccessibilityService gesture strokes.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInteractionGesture, Risk: "medium"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      70000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationGesture,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.gesture",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          70 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 8192},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationGesture,
		},
		Enabled: true,
	}
}

func buildScreenshotTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"displayId":{"type":"integer","minimum":0,"maximum":16}}}`)
	outputSchema := json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"},"imageBase64":{"type":"string"},"mimeType":{"type":"string"},"width":{"type":"integer"},"height":{"type":"integer"}}}`)
	return capability.ToolDefinition{
		ID:           "android.interaction.screenshot",
		ModelName:    "android_interaction_screenshot",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Interaction Screenshot",
		Description:  "Capture one bounded screenshot through AccessibilityService.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "android.media.screen_capture", Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-accessibility-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "accessibility"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationScreenshot,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "android.interaction.screenshot",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       true,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 12 * 1024 * 1024},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationScreenshot,
		},
		Enabled: true,
	}
}
