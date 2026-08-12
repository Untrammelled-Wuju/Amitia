package share

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func BuildShareTools() []capability.ToolDefinition {
	runtime := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeTypeAndroid_Native,
		RuntimeID:   "android_native_share",
	}

	return []capability.ToolDefinition{
		buildSendTool(runtime),
	}
}

func buildSendTool(rt capability.RuntimeBinding) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"text": {"type": "string", "maxLength": 1048576},
			"subject": {"type": "string", "maxLength": 8192},
			"resources": {
				"type": "array",
				"items": {"type": "string"},
				"maxItems": 10
			},
			"mimeType": {"type": "string"},
			"chooserTitle": {"type": "string", "maxLength": 256}
		},
		"additionalProperties": false
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"status": {"type": "string", "enum": ["chooser_presented"]},
			"resourceCount": {"type": "integer"},
			"mimeType": {"type": "string"},
			"userActionRequired": {"type": "boolean"}
		}
	}`)

	return capability.ToolDefinition{
		ID:          ToolIDSend,
		ModelName:   "android.share.send",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Share Send",
		Description: "Send content via Android Sharesheet. Uses ACTION_SEND/ACTION_SEND_MULTIPLE with createChooser. User must select target app.",
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: PermissionSend, Risk: "high"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectSystem,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b41-share-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": OperationSend,
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
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
			RuntimeType: rt.RuntimeType,
			RuntimeID:   rt.RuntimeID,
			HandlerName: OperationSend,
		},
		Enabled: true,
	}
}
