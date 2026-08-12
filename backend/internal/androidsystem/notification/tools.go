package notification

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type PermissionDefinition struct {
	ID          string
	Name        string
	Description string
	Risk        string
}

func BuildPermissionDefinitions() []PermissionDefinition {
	return []PermissionDefinition{
		{
			ID:          androidsystem.PermissionNotificationRead,
			Name:        "android.notification.read",
			Description: "允许Agent通过Android NotificationListenerService读取系统通知。与Kernel权限独立于Android系统Listener授权。",
			Risk:        "high",
		},
		{
			ID:          androidsystem.PermissionNotificationPost,
			Name:        "android.notification.post",
			Description: "允许Agent以Amitia名义发送系统通知。Android 13+需要POST_NOTIFICATIONS运行时权限。",
			Risk:        "medium",
		},
		{
			ID:          androidsystem.PermissionNotificationControl,
			Name:        "android.notification.control",
			Description: "允许Agent执行第三方通知控制操作：dismiss、open、invoke action。受Kernel权限与Android Listener授权双重管控。",
			Risk:        "high",
		},
	}
}

type ToolRegistrar interface {
	RegisterNotificationTools(registry *capability.ToolRegistry) error
}

type notificationToolRegistrar struct{}

func NewToolRegistrar() ToolRegistrar {
	return &notificationToolRegistrar{}
}

func (r *notificationToolRegistrar) RegisterNotificationTools(registry *capability.ToolRegistry) error {
	tools := BuildNotificationTools()

	for _, tool := range tools {
		if err := registry.Register(nil, tool); err != nil {
			if err := registry.Replace(nil, tool); err != nil {
				return fmt.Errorf("register notification tool %s: %w", tool.ID, err)
			}
		}
	}

	return nil
}

func BuildNotificationTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_notification",
	}

	return []capability.ToolDefinition{
		buildStatusTool(runtime),
		buildListTool(runtime),
		buildGetTool(runtime),
		buildPostTool(runtime),
		buildCancelOwnTool(runtime),
		buildDismissTool(runtime),
		buildOpenTool(runtime),
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
			"listenerDeclared": {"type": "boolean"},
			"listenerGranted": {"type": "boolean"},
			"listenerConnected": {"type": "boolean"},
			"postPermissionRequired": {"type": "boolean"},
			"postPermissionGranted": {"type": "boolean"},
			"notificationsEnabled": {"type": "boolean"},
			"canRead": {"type": "boolean"},
			"canDismiss": {"type": "boolean"},
			"canPost": {"type": "boolean"},
			"userActionRequired": {"type": "boolean"},
			"state": {"type": "string"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDStatus,
		ModelName:   "android.notification.status",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Notification Status",
		Description: "查询Android Notification Provider的授权与连接状态，包含Listener访问权限和发送权限。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationRead, Risk: "medium"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
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
		"properties": {
			"limit": {"type": "integer", "minimum": 1, "maximum": 100, "default": 50},
			"packageName": {"type": "string"},
			"includeOngoing": {"type": "boolean", "default": false}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"notifications": {"type": "array"},
			"count": {"type": "integer"},
			"filteredCount": {"type": "integer"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDList,
		ModelName:   "android.notification.list",
		Source:      capability.ToolSourceBuiltin,
		Name:        "List Notifications",
		Description: "读取当前活动系统通知列表。需要Notification Listener访问权限。返回安全投影，不含原始Android对象。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationRead, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationList,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   3,
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
			HandlerName: OperationList,
		},
		Enabled: true,
	}
}

func buildGetTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["notificationRef"],
		"properties": {
			"notificationRef": {"type": "string", "pattern": "^(ntf_|own_)"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"notification": {"type": "object"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDGet,
		ModelName:   "android.notification.get",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Get Notification",
		Description: "根据opaque notificationRef读取单条通知的安全投影。需Listener权限。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationRead, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationGet,
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

func buildPostTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"title": {"type": "string", "maxLength": 256},
			"body": {"type": "string", "maxLength": 4096},
			"channel": {"type": "string", "enum": ["amitia_agent", "amitia_task"], "default": "amitia_agent"},
			"silent": {"type": "boolean", "default": false}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"notificationRef": {"type": "string"},
			"posted": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDPost,
		ModelName:   "android.notification.post",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Post Notification",
		Description: "以Amitia名义发送系统通知。使用独立Channel，不修改Runtime FGS通知。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationPost, Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationPost,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
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
			HandlerName: OperationPost,
		},
		Enabled: true,
	}
}

func buildCancelOwnTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["notificationRef"],
		"properties": {
			"notificationRef": {"type": "string", "pattern": "^own_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"cancelled": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDCancelOwn,
		ModelName:   "android.notification.cancel_own",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Cancel Own Notification",
		Description: "取消Amitia自己发送的通知。只能取消own_ namespace的opaque ref。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationPost, Risk: "medium"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationCancelOwn,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 1024,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationCancelOwn,
		},
		Enabled: true,
	}
}

func buildDismissTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["notificationRef"],
		"properties": {
			"notificationRef": {"type": "string", "pattern": "^ntf_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"requested": {"type": "boolean"},
			"dismissed": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDDismiss,
		ModelName:   "android.notification.dismiss",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Dismiss Third-Party Notification",
		Description: "通过NotificationListenerService取消单条第三方通知。需要Listener权限。只操作clearable通知。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationControl, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationDismiss,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       true,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationDismiss,
		},
		Enabled: true,
	}
}

func buildOpenTool(runtime capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"required": ["notificationRef"],
		"properties": {
			"notificationRef": {"type": "string", "pattern": "^ntf_"}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"invoked": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDOpen,
		ModelName:   "android.notification.open",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Open Notification",
		Description: "触发第三方通知的contentIntent。需要control权限和Approval。",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: androidsystem.PermissionNotificationControl, Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b39-notification-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationOpen,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       true,
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
			RuntimeType: runtime.RuntimeType,
			RuntimeID:   runtime.RuntimeID,
			HandlerName: OperationOpen,
		},
		Enabled: true,
	}
}
