package shizuku

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	PermissionInspect    = "android.shizuku.inspect"
	PermissionPermission = "android.shizuku.permission"
	PermissionManage     = "android.shizuku.manage"
	PermissionExecute    = "android.shizuku.execute"
	PermissionSystemSvc  = "android.shizuku.system_service"
	PermissionBinder     = "android.shizuku.binder"
)

func BuildShizukuTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_shizuku",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildInfoTool(runtime),
		buildPermissionTool(runtime),
		buildPermissionCheckTool(runtime),
		buildSetEnabledTool(runtime),
		buildOpenManagerTool(runtime),
		buildTestTool(runtime),
		buildUserServiceStatusTool(runtime),
		buildUserServiceBindTool(runtime),
		buildUserServiceUnbindTool(runtime),
		buildProcessStartTool(runtime),
		buildProcessWriteTool(runtime),
		buildProcessReadTool(runtime),
		buildProcessWaitTool(runtime),
		buildProcessKillTool(runtime),
		buildSystemServiceResolveTool(runtime),
		buildBinderTransactTool(runtime),
		buildExecTool(runtime),
	}
}

func buildStatusTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.shizuku.status",
		ModelName:    "shizuku_status",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Status",
		Description:  "Inspect Shizuku, Sui or AXManager availability, permission, enabled state, version and shell UID.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"installed":{"type":"boolean"},"binderAvailable":{"type":"boolean"},"permissionState":{"type":"string"},"authorized":{"type":"boolean"},"enabled":{"type":"boolean"},"provider":{"type":"string"},"state":{"type":"string"},"serviceState":{"type":"string"},"version":{"type":"integer"},"uid":{"type":"integer"},"reason":{"type":"string"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionInspect, Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      8000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationStatus,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.status",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          8 * time.Second,
			MaxConcurrency:   4,
			Idempotent:       true,
			ApprovalRequired: true,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 32768,
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

func buildPermissionTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.shizuku.request_permission",
		ModelName:    "shizuku_request_permission",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Request Permission",
		Description:  "Request Shizuku authorization and bind the privileged user service. This is equivalent to shizuku.set_enabled enabled=true.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"authorized":{"type":"boolean"},"state":{"type":"string"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionPermission, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      65000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationRequestPermission,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.request_permission",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          65 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 16384,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationRequestPermission,
		},
		Enabled: true,
	}
}

func buildSetEnabledTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.shizuku.set_enabled",
		ModelName:    "shizuku_set_enabled",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Set Enabled",
		Description:  "Enable or disable AI access to Shizuku. Enabling also requests authorization and runs a shell identity smoke test.",
		InputSchema:  json.RawMessage(`{"type":"object","required":["enabled"],"properties":{"enabled":{"type":"boolean"}},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"enabled":{"type":"boolean"},"state":{"type":"string"},"provider":{"type":"string"},"version":{"type":"integer"},"uid":{"type":"integer"},"test":{"type":"object"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionManage, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      65000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationSetEnabled,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.set_enabled",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          65 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 32768,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationSetEnabled,
		},
		Enabled: true,
	}
}

func buildOpenManagerTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.shizuku.open_manager",
		ModelName:    "shizuku_open_manager",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Open Manager",
		Description:  "Open the installed Shizuku-compatible manager or its download page when no manager is installed.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"openedManager":{"type":"boolean"},"openedInstallPage":{"type":"boolean"},"packageName":{"type":"string"},"installUrl":{"type":"string"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionManage, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectExternal,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpenManager,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.open_manager",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 16384,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationOpenManager,
		},
		Enabled: true,
	}
}

func buildTestTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           "android.shizuku.test",
		ModelName:    "shizuku_test",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Test",
		Description:  "Run id through the bound Shizuku user service and return the shell identity and exit result.",
		InputSchema:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"exitCode":{"type":"integer"},"exitCodeAvailable":{"type":"boolean"},"stdout":{"type":"string"},"stderr":{"type":"string"},"durationMs":{"type":"integer"},"timedOut":{"type":"boolean"}}}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionExecute, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 45},
		Metadata: map[string]any{
			"androidNativeOperation": OperationTest,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.test",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       true,
			ApprovalRequired: true,
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
			HandlerName: OperationTest,
		},
		Enabled: true,
	}
}

func buildExecTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"anyOf": [
			{"required": ["command"]},
			{"required": ["executable"]}
		],
		"properties": {
			"command": {"type": "string", "minLength": 1, "maxLength": 131072},
			"executable": {"type": "string", "minLength": 1, "maxLength": 4096},
			"args": {"type": "array", "items": {"type": "string", "maxLength": 65536}, "maxItems": 128},
			"stdin": {"type": "string", "maxLength": 1048576},
			"env": {"type": "object", "additionalProperties": {"type": "string", "maxLength": 65536}, "maxProperties": 32},
			"workDir": {"type": "string", "maxLength": 4096},
			"timeoutMs": {"type": "integer", "minimum": 100, "maximum": 120000},
			"maxOutputBytes": {"type": "integer", "minimum": 1024, "maximum": 8388608}
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
		ID:           "android.shizuku.exec",
		ModelName:    "shizuku_exec",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Shizuku Exec",
		Description:  "Execute a command through Shizuku using the canonical shizuku.exec operation. Use command for /system/bin/sh -c execution, or executable plus args for a structured process.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionExecute, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      125000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-shizuku-v1"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": OperationExecute,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     "shizuku.exec",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          125 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8388608,
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

func buildInfoTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.info",
		"shizuku_info",
		"Shizuku Info",
		"Inspect Shizuku binder, versions, UID, SELinux context and protocol compatibility.",
		`{"type":"object","properties":{},"additionalProperties":false}`,
		PermissionInspect,
		8*time.Second,
		OperationInfo,
		true,
	)
}

func buildPermissionCheckTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.permission_check",
		"shizuku_permission_check",
		"Shizuku Permission Check",
		"Check whether a named Android permission is granted to the Shizuku shell UID.",
		`{"type":"object","required":["permission"],"properties":{"permission":{"type":"string","minLength":1,"maxLength":256}},"additionalProperties":false}`,
		PermissionInspect,
		8*time.Second,
		OperationPermissionCheck,
		true,
	)
}

func buildUserServiceStatusTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.user_service.status",
		"shizuku_user_service_status",
		"Shizuku User Service Status",
		"Inspect the bound Shizuku UserService lifecycle state.",
		`{"type":"object","properties":{},"additionalProperties":false}`,
		PermissionInspect,
		8*time.Second,
		OperationUserServiceStatus,
		true,
	)
}

func buildUserServiceBindTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.user_service.bind",
		"shizuku_user_service_bind",
		"Bind Shizuku User Service",
		"Bind the privileged Shizuku UserService used for process and command execution.",
		`{"type":"object","properties":{},"additionalProperties":false}`,
		PermissionManage,
		15*time.Second,
		OperationUserServiceBind,
		false,
	)
}

func buildUserServiceUnbindTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.user_service.unbind",
		"shizuku_user_service_unbind",
		"Unbind Shizuku User Service",
		"Destroy and unbind the privileged Shizuku UserService and its managed processes.",
		`{"type":"object","properties":{},"additionalProperties":false}`,
		PermissionManage,
		8*time.Second,
		OperationUserServiceUnbind,
		false,
	)
}

func buildProcessStartTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.process.start",
		"shizuku_process_start",
		"Shizuku Process Start",
		"Start a long-running process through the Shizuku UserService and return a processId.",
		`{"type":"object","required":["executable"],"properties":{"executable":{"type":"string","minLength":1,"maxLength":4096},"args":{"type":"array","items":{"type":"string","maxLength":65536},"maxItems":128},"env":{"type":"object","additionalProperties":{"type":"string","maxLength":65536},"maxProperties":32},"workDir":{"type":"string","maxLength":4096},"maxBufferBytes":{"type":"integer","minimum":1024,"maximum":67108864}},"additionalProperties":false}`,
		PermissionExecute,
		15*time.Second,
		OperationProcessStart,
		false,
	)
}

func buildProcessWriteTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.process.write",
		"shizuku_process_write",
		"Shizuku Process Write",
		"Write text or base64 data to a managed Shizuku process stdin.",
		`{"type":"object","required":["processId"],"properties":{"processId":{"type":"string","minLength":1,"maxLength":128},"text":{"type":"string","maxLength":1048576},"dataBase64":{"type":"string","maxLength":2097152},"closeStdin":{"type":"boolean"}},"additionalProperties":false}`,
		PermissionExecute,
		15*time.Second,
		OperationProcessWrite,
		false,
	)
}

func buildProcessReadTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.process.read",
		"shizuku_process_read",
		"Shizuku Process Read",
		"Read bounded stdout and stderr from a managed Shizuku process.",
		`{"type":"object","required":["processId"],"properties":{"processId":{"type":"string","minLength":1,"maxLength":128},"maxBytes":{"type":"integer","minimum":1,"maximum":8388608},"waitMs":{"type":"integer","minimum":0,"maximum":30000}},"additionalProperties":false}`,
		PermissionExecute,
		35*time.Second,
		OperationProcessRead,
		false,
	)
}

func buildProcessWaitTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.process.wait",
		"shizuku_process_wait",
		"Shizuku Process Wait",
		"Wait for a managed Shizuku process to finish and return its exit code and buffered output.",
		`{"type":"object","required":["processId"],"properties":{"processId":{"type":"string","minLength":1,"maxLength":128},"timeoutMs":{"type":"integer","minimum":0,"maximum":120000}},"additionalProperties":false}`,
		PermissionExecute,
		125*time.Second,
		OperationProcessWait,
		true,
	)
}

func buildProcessKillTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.process.kill",
		"shizuku_process_kill",
		"Shizuku Process Kill",
		"Terminate or forcibly kill a managed Shizuku process.",
		`{"type":"object","required":["processId"],"properties":{"processId":{"type":"string","minLength":1,"maxLength":128},"force":{"type":"boolean"}},"additionalProperties":false}`,
		PermissionExecute,
		15*time.Second,
		OperationProcessKill,
		false,
	)
}

func buildSystemServiceResolveTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.system_service.resolve",
		"shizuku_system_service_resolve",
		"Resolve Android System Service",
		"Resolve an Android system service binder and inspect its interface descriptor.",
		`{"type":"object","required":["serviceName"],"properties":{"serviceName":{"type":"string","minLength":1,"maxLength":256}},"additionalProperties":false}`,
		PermissionSystemSvc,
		12*time.Second,
		OperationSystemService,
		true,
	)
}

func buildBinderTransactTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	return buildAuxShizukuTool(
		runtime,
		"android.shizuku.binder.transact",
		"shizuku_binder_transact",
		"Shizuku Binder Transaction",
		"Execute a low-level Binder transaction against a resolved Android system service.",
		`{"type":"object","required":["serviceName","transactionCode","dataBase64"],"properties":{"serviceName":{"type":"string","minLength":1,"maxLength":256},"transactionCode":{"type":"integer","minimum":0,"maximum":2147483647},"dataBase64":{"type":"string","minLength":1,"maxLength":8388608},"flags":{"type":"integer","minimum":0,"maximum":16},"maxReplyBytes":{"type":"integer","minimum":1,"maximum":8388608}},"additionalProperties":false}`,
		PermissionBinder,
		30*time.Second,
		OperationBinderTransact,
		false,
	)
}

func buildAuxShizukuTool(
	runtime capability.RuntimeBinding,
	id string,
	modelName string,
	name string,
	description string,
	inputSchema string,
	permissionID string,
	timeout time.Duration,
	operation string,
	idempotent bool,
) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:           id,
		ModelName:    modelName,
		Source:       capability.ToolSourceBuiltin,
		Name:         name,
		Description:  description,
		InputSchema:  json.RawMessage(inputSchema),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: permissionID, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: !idempotent,
		Idempotent:     idempotent,
		Retryable:      idempotent,
		TimeoutMS:      timeout.Milliseconds(),
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-shizuku-v2"},
		ModelExposure:  capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"android", "automation"}, Priority: 46},
		Metadata: map[string]any{
			"androidNativeOperation": operation,
			"bridgeProtocol":         "android_native",
			"canonicalModelName":     strings.TrimPrefix(id, "android."),
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          timeout,
			MaxConcurrency:   1,
			Idempotent:       idempotent,
			ApprovalRequired: true,
			AllowBackground:  true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 12 * 1024 * 1024,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: operation,
		},
		Enabled: true,
	}
}
