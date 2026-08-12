package screenshot

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	toolID        = "android.screen.screenshot"
	toolModelName = "android.screen.screenshot"
	handlerName   = "media.screenshot.capture"
	toolVersion   = "0.1.0"
	permissionID  = "android.media.screen_capture"
)

func BuildToolDefinition() (capability.ToolDefinition, error) {
	handler := NewCaptureHandler()

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "displayId": {
      "type": "integer",
      "minimum": 0,
      "default": 0,
      "description": "Target display identifier. 0 is the primary display."
    },
    "format": {
      "type": "string",
      "enum": ["png", "jpeg", "webp"],
      "default": "png",
      "description": "Output image format."
    },
    "quality": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100,
      "description": "Lossy quality for jpeg/webp. Ignored for png."
    },
    "maxWidth": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum output width. Aspect ratio preserved. No upscaling."
    },
    "maxHeight": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum output height. Aspect ratio preserved. No upscaling."
    }
  },
  "required": [],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "resourceUri": {
      "type": "string",
      "description": "amitia:// URI of the screenshot artifact."
    },
    "mimeType": {
      "type": "string",
      "description": "image/png, image/jpeg, or image/webp"
    },
    "width": {
      "type": "integer",
      "description": "Actual pixel width of the captured image."
    },
    "height": {
      "type": "integer",
      "description": "Actual pixel height of the captured image."
    },
    "displayId": {
      "type": "integer",
      "description": "Display identifier that was captured."
    },
    "timestampMs": {
      "type": "integer",
      "description": "Capture timestamp in milliseconds."
    },
    "sizeBytes": {
      "type": "integer",
      "description": "Encoded file size in bytes."
    },
    "contentHash": {
      "type": "string",
      "description": "SHA-256 hex digest of the encoded file (sha256:...)."
    }
  },
  "required": ["resourceUri", "mimeType", "width", "height"]
}`)

	return capability.ToolDefinition{
		ID:            toolID,
		ModelName:     toolModelName,
		Source:        capability.ToolSourceBuiltin,
		Name:          "Android Screen Screenshot",
		Description:   "Capture a single static screenshot from an Android device via the native accessibility bridge. Returns an amitia:// resource reference, never raw pixels.",
		Version:       toolVersion,
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		Permissions:   []capability.PermissionRequirement{
			{
				Capability:  permissionID,
				Description: "grants the agent permission to read the device screen via Android accessibility screenshot",
				Risk:        "high",
			},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b34-screen-shot-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerName,
			"capabilityId":           string(handler.capabilityID),
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
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeAndroid_Native,
			HandlerName: handlerName,
			Endpoint:    "",
		},
	}, nil
}

type PermissionDefinition struct {
	ID          string
	Name        string
	Description string
	Risk        string
}

func BuildPermissionDefinition() PermissionDefinition {
	return PermissionDefinition{
		ID:          permissionID,
		Name:        "android.media.screen_capture",
		Description: "Allows the agent to capture device screenshots via Android AccessibilityService. Kernel-level gate distinct from the Android system accessibility authorization.",
		Risk:        "high",
	}
}
