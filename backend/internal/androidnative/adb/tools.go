package adb

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildADBTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_adb",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildDevicesTool(runtime),
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
			"supported": {"type": "boolean"},
			"backend": {"type": "string"},
			"serverAvailable": {"type": "boolean"},
			"deviceCount": {"type": "integer"},
			"authorizedDeviceCount": {"type": "integer"},
			"defaultDeviceReady": {"type": "boolean"},
			"state": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.adb.status",
		ModelName:   "android.adb.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "ADB Status",
		Description: "查询ADB后端可用性与设备连接状态。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionADBInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b28-adb-v1"},
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

func buildDevicesTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"devices": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"serial": {"type": "string"},
						"state": {"type": "string"},
						"transport": {"type": "string"},
						"product": {"type": "string"},
						"model": {"type": "string"},
						"device": {"type": "string"},
						"isDefault": {"type": "boolean"}
					}
				}
			},
			"count": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.adb.devices",
		ModelName:   "android.adb.devices",
		Source:      capability.ToolSourceBuiltin,
		Name:        "ADB Devices",
		Description: "枚举当前ADB连接的设备列表与授权状态。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionADBInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b28-adb-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationDevices,
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
			HandlerName: OperationDevices,
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
			"deviceSerial": {
				"type": "string",
				"maxLength": 256
			},
			"executable": {
				"type": "string",
				"enum": ["getprop", "id", "uname"]
			},
			"args": {
				"type": "array",
				"items": {"type": "string"},
				"maxItems": 64
			},
			"stdin": {
				"type": "string",
				"maxLength": 65536
			},
			"timeoutMs": {
				"type": "integer",
				"minimum": 1,
				"maximum": 30000
			}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"deviceSerial": {"type": "string"},
			"exitCode": {"type": "integer"},
			"stdout": {"type": "string"},
			"stderr": {"type": "string"},
			"durationMs": {"type": "integer"},
			"timedOut": {"type": "boolean"},
			"exitCodeAvailable": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          "android.adb.execute",
		ModelName:   "android.adb.execute",
		Source:      capability.ToolSourceBuiltin,
		Name:        "ADB Execute",
		Description: "在已授权的ADB设备上执行受控shell命令（仅只读诊断命令：getprop/id/uname）。",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionADBExecute, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b28-adb-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationExecute,
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
			MaxOutputBytes: 1572864,
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

func BuildADBPermissionDefinitions() []PermissionDefinition {
	return []PermissionDefinition{
		{
			ID:          PermissionADBInspect,
			Name:        "android.adb.inspect",
			Description: "允许Agent读取ADB后端可用性与设备连接状态。",
			Risk:        "low",
		},
		{
			ID:          PermissionADBExecute,
			Name:        "android.adb.execute",
			Description: "允许Agent在已授权的ADB设备上执行受控shell命令。",
			Risk:        "medium",
		},
	}
}

type PermissionDefinition struct {
	ID          string
	Name        string
	Description string
	Risk        string
}
