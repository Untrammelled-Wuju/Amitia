package display

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildDisplayTools() []capability.ToolDefinition {
	runtime := RuntimeBinding
	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildListTool(runtime),
		buildGetTool(runtime),
	}
}

func buildStatusTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"includeTopology": {
				"type": "boolean",
				"description": "Whether to include topology information if available."
			}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"supported": {"type": "boolean"},
			"displayCount": {"type": "integer"},
			"defaultDisplayId": {"type": "integer"},
			"secondaryDisplaySupported": {"type": "boolean"},
			"presentationDisplayCount": {"type": "integer"},
			"managedVirtualDisplayCount": {"type": "integer"},
			"uiTreeMultiDisplaySupported": {"type": "boolean"},
			"gestureMultiDisplaySupported": {"type": "boolean"},
			"screenshotMultiDisplaySupported": {"type": "boolean"},
			"screenFrameMultiDisplaySupported": {"type": "boolean"},
			"topologySupported": {"type": "boolean"},
			"generation": {"type": "integer"},
			"state": {"type": "string"},
			"reason": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.display.status",
		ModelName:   "android.display.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Display Status",
		Description: "Get the current Android multi-display support status including display counts, capabilities, and topology support.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "android.display.inspect", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
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

func buildListTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"includeDefault": {
				"type": "boolean",
				"description": "Include the default display in results."
			},
			"includeSecondary": {
				"type": "boolean",
				"description": "Include secondary displays in results."
			},
			"type": {
				"type": "string",
				"description": "Filter by display type (default, built_in, external, wireless, presentation, virtual_amitia)."
			},
			"presentationOnly": {
				"type": "boolean",
				"description": "Only return presentation displays."
			},
			"managedOnly": {
				"type": "boolean",
				"description": "Only return Amitia-managed virtual displays."
			},
			"interactiveOnly": {
				"type": "boolean",
				"description": "Only return displays that support interaction (UI tree, gesture, or screenshot)."
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"generation": {"type": "integer"},
			"defaultDisplayId": {"type": "integer"},
			"displays": {"type": "array", "items": {"type": "object"}},
			"capturedAt": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.display.list",
		ModelName:   "android.display.list",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Display List",
		Description: "List all current Android logical displays with their capabilities. Returns a consistent snapshot of all displays.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "android.display.inspect", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		Metadata: map[string]any{
			"androidNativeOperation": OperationList,
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
			MaxOutputBytes: 8192,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationList,
		},
		Enabled: true,
	}
}

func buildGetTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"displayId": {
				"type": "integer",
				"description": "Android display ID to look up."
			},
			"ref": {
				"type": "string",
				"description": "Display ref identifier (display:<id>:<generation>)."
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
		ID:          "android.display.get",
		ModelName:   "android.display.get",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Display Get",
		Description: "Get detailed information about a specific Android display by ID or ref.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "android.display.inspect", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
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
			MaxOutputBytes: 4096,
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
