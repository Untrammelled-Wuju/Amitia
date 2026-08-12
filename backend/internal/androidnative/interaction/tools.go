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
			"accessibilityAction": {"type": "boolean"},
			"accessibilityGesture": {"type": "boolean"},
			"coordinateTap": {"type": "boolean"},
			"textInput": {"type": "boolean"},
			"scroll": {"type": "boolean"},
			"visualLocate": {"type": "boolean"},
			"ocrAvailable": {"type": "boolean"},
			"imageUnderstandAvailable": {"type": "boolean"},
			"rootFallback": {"type": "boolean"},
			"adbFallback": {"type": "boolean"},
			"state": {"type": "string"},
			"reason": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.interaction.status",
		ModelName:   "android.interaction.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Status",
		Description: "查询Android Interaction能力状态。检测Accessibility Action/Gesture、Coordinate、Visual Locate等能力可用性。不触发任何副作用。",
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
		ID:          "android.interaction.click",
		ModelName:   "android.interaction.click",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Click",
		Description: "点击Android UI目标。优先使用Accessibility ACTION_CLICK，失败时按策略降级到坐标点击、Root/ADB fallback。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationClick,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.long_click",
		ModelName:   "android.interaction.long_click",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Long Click",
		Description: "长按Android UI目标。优先使用Accessibility ACTION_LONG_CLICK，失败时降级到Gesture long press。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationLongClick,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.input_text",
		ModelName:   "android.interaction.input_text",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Input Text",
		Description: "向Android UI输入文本。优先使用Accessibility ACTION_SET_TEXT。Password字段默认拒绝。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationInputText,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.clear_text",
		ModelName:   "android.interaction.clear_text",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Clear Text",
		Description: "清空Android UI文本字段。使用Accessibility ACTION_SET_TEXT(\"\")。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationClearText,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.scroll",
		ModelName:   "android.interaction.scroll",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Scroll",
		Description: "滚动Android UI。优先使用Accessibility ACTION_SCROLL_FORWARD/BACKWARD，失败时降级到swipe手势。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationScroll,
			"bridgeProtocol":         "android_native",
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
			}
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
		ID:          "android.interaction.swipe",
		ModelName:   "android.interaction.swipe",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Swipe",
		Description: "执行Android滑动手势。使用Accessibility Gesture或Coordinate executor。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationSwipe,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.visual_locate",
		ModelName:   "android.interaction.visual_locate",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Visual Locate",
		Description: "通过截图和Image Intelligence定位Android UI目标。返回候选列表，不执行点击。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationVisualLocate,
			"bridgeProtocol":         "android_native",
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
		ID:          "android.interaction.visual_click",
		ModelName:   "android.interaction.visual_click",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Interaction Visual Click",
		Description: "通过截图和Image Intelligence定位并点击Android UI目标。先visual_locate再coordinate click。",
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
		Metadata: map[string]any{
			"androidNativeOperation": OperationVisualClick,
			"bridgeProtocol":         "android_native",
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
