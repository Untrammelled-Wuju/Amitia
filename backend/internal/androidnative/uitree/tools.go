package uitree

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildUITreeTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_ui_tree",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildSnapshotTool(runtime),
		buildFindTool(runtime),
		buildGetTool(runtime),
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
			"preferredSource": {"type": "string"},
			"availableSources": {"type": "array", "items": {"type": "string"}},
			"accessibilityConnected": {"type": "boolean"},
			"rootAvailable": {"type": "boolean"},
			"adbAvailable": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.ui_tree.status",
		ModelName:   "android.ui_tree.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "UI Tree Status",
		Description: "查询Android UI Tree能力状态。检测Accessibility、Root、ADB来源可用性。不触发授权或采集。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionUITreeRead, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-uitree-v1"},
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

func buildSnapshotTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"source": {
				"type": "string",
				"enum": ["auto", "accessibility", "root", "adb"]
			},
			"includeAllWindows": {"type": "boolean"},
			"includeInvisible": {"type": "boolean"},
			"maxDepth": {
				"type": "integer",
				"minimum": 1,
				"maximum": 64
			},
			"excludeOwnPackage": {"type": "boolean"},
			"allowRootFallback": {"type": "boolean"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"snapshotId": {"type": "string"},
			"generation": {"type": "integer"},
			"source": {"type": "string"},
			"capturedAt": {"type": "integer"},
			"activeWindowId": {"type": "string"},
			"windows": {"type": "array"},
			"nodes": {"type": "array"},
			"nodeCount": {"type": "integer"},
			"truncated": {"type": "boolean"},
			"capability": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.ui_tree.snapshot",
		ModelName:   "android.ui_tree.snapshot",
		Source:      capability.ToolSourceBuiltin,
		Name:        "UI Tree Snapshot",
		Description: "获取当前Android UI窗口和节点结构。默认使用Accessibility，可选Root/ADB fallback。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionUITreeRead, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     false,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-uitree-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationSnapshot,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       false,
			ApprovalRequired: false,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: DefaultMaxOutputBytes,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationSnapshot,
		},
		Enabled: true,
	}
}

func buildFindTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"snapshotId": {"type": "string"},
			"text": {"type": "string", "maxLength": 1024},
			"resourceId": {"type": "string", "maxLength": 1024},
			"className": {"type": "string", "maxLength": 512},
			"role": {"type": "string"},
			"clickable": {"type": "boolean"},
			"editable": {"type": "boolean"},
			"scrollable": {"type": "boolean"},
			"visible": {"type": "boolean"},
			"matchMode": {
				"type": "string",
				"enum": ["exact", "contains", "contains_ci"]
			},
			"limit": {
				"type": "integer",
				"minimum": 1,
				"maximum": 100
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"snapshotId": {"type": "string"},
			"nodeIds": {"type": "array", "items": {"type": "string"}},
			"count": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.ui_tree.find",
		ModelName:   "android.ui_tree.find",
		Source:      capability.ToolSourceBuiltin,
		Name:        "UI Tree Find",
		Description: "基于结构化条件过滤当前Snapshot节点。支持text/resourceId/className/role等条件。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionUITreeRead, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      3000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-uitree-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationFind,
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
			MaxOutputBytes: 65536,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationFind,
		},
		Enabled: true,
	}
}

func buildGetTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["snapshotId", "nodeId"],
		"properties": {
			"snapshotId": {"type": "string"},
			"nodeId": {"type": "string"}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"snapshotId": {"type": "string"},
			"generation": {"type": "integer"},
			"source": {"type": "string"},
			"node": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.ui_tree.get",
		ModelName:   "android.ui_tree.get",
		Source:      capability.ToolSourceBuiltin,
		Name:        "UI Tree Get",
		Description: "根据snapshotId和nodeId获取节点详情。验证Snapshot有效性和Generation未失效。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionUITreeRead, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      3000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-uitree-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationGet,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationGet,
		},
		Enabled: true,
	}
}
