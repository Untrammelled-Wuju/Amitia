package root

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type PermissionDefinition struct {
	ID          string
	Name        string
	Description string
	Risk        string
}

func BuildRootPermissionDefinitions() []PermissionDefinition {
	return []PermissionDefinition{
		{
			ID:          PermissionRootInspect,
			Name:        "android.root.inspect",
			Description: "允许Agent检查Android Host Root可用性与授权状态。",
			Risk:        "low",
		},
		{
			ID:          PermissionRootRequest,
			Name:        "android.root.request",
			Description: "允许Agent显式触发Root Manager授权流程。",
			Risk:        "high",
		},
		{
			ID:          PermissionRootExecute,
			Name:        "android.root.execute",
			Description: "允许Agent在Android Host以root身份执行结构化命令。",
			Risk:        "high",
		},
		{
			ID:          PermissionRootShell,
			Name:        "android.root.shell",
			Description: "允许Agent在Android Host以root身份执行完整shell命令（高风险）。",
			Risk:        "critical",
		},
	}
}

func BuildRootTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_root",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildRequestTool(runtime),
		buildExecuteTool(runtime),
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
			"rootFramework": {"type": "string"},
			"rootManagerDetected": {"type": "boolean"},
			"suBinaryDetected": {"type": "boolean"},
			"authorizationState": {"type": "string"},
			"rootAvailable": {"type": "boolean"},
			"backend": {"type": "string"},
			"state": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.root.status",
		ModelName:   "android.root.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Root Status",
		Description: "查询Android Host Root可用性与授权状态。不会触发Root授权弹窗。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionRootInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-root-v1"},
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

func buildRequestTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"requested": {"type": "boolean"},
			"authorizationState": {"type": "string"},
			"rootAvailable": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.root.request",
		ModelName:   "android.root.request",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Root Request",
		Description: "显式触发Root Manager授权流程。可能弹出Magisk/KernelSU/APatch授权UI。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionRootRequest, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-root-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationRequest,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationRequest,
		},
		Enabled: true,
	}
}

func buildExecuteTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["executable"],
		"properties": {
			"executable": {
				"type": "string",
				"minLength": 1,
				"maxLength": 1024
			},
			"args": {
				"type": "array",
				"items": {"type": "string", "maxLength": 65536},
				"maxItems": 128
			},
			"stdin": {
				"type": "string",
				"maxLength": 1048576
			},
			"env": {
				"type": "object",
				"additionalProperties": {"type": "string"},
				"maxProperties": 32
			},
			"workDir": {
				"type": "string",
				"maxLength": 4096
			},
			"timeoutMs": {
				"type": "integer",
				"minimum": 100,
				"maximum": 60000
			},
			"mode": {
				"type": "string",
				"enum": ["structured", "shell"]
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"exitCode": {"type": "integer"},
			"exitCodeAvailable": {"type": "boolean"},
			"stdout": {"type": "string"},
			"stderr": {"type": "string"},
			"durationMs": {"type": "integer"},
			"timedOut": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.root.execute",
		ModelName:   "android.root.execute",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Root Execute",
		Description: "在Android Host以root身份执行结构化命令。默认使用structured模式（安全），可选shell模式（高风险需额外权限）。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionRootExecute, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b29-root-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationExecute,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          60 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 2097152,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationExecute,
		},
		Enabled: true,
	}
}
