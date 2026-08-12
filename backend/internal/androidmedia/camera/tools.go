package camera

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	toolIDStatus  = "android.camera.status"
	toolIDList    = "android.camera.list"
	toolIDCapture = "android.camera.capture"

	handlerStatus  = OperationCameraStatus
	handlerList    = OperationCameraList
	handlerCapture = OperationCameraCapture

	toolVersion  = "0.1.0"
	permissionID = "android.media.camera"
)

func BuildStatusToolDefinition() (capability.ToolDefinition, error) {
	handler := NewStatusHandler(NewHandler(nil, DefaultPolicy()))

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {},
  "required": [],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "supported": { "type": "boolean" },
    "permissionState": { "type": "string" },
    "userActionRequired": { "type": "boolean" },
    "cameraCount": { "type": "integer" },
    "defaultLens": { "type": "string" },
    "captureAvailable": { "type": "boolean" },
    "reason": { "type": "string" }
  },
  "required": ["supported"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDStatus,
		ModelName:   toolIDStatus,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Camera Status",
		Description: "Query Android camera capability and permission state. Does not trigger capture.",
		Version:     toolVersion,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  permissionID,
				Description: "read camera capability state",
				Risk:        "low",
			},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectNone,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-camera-status-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerStatus,
			"capabilityId":           string(handler.CapabilityID()),
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
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
			RuntimeType: capability.RuntimeTypeAndroid_Native,
			HandlerName: handlerStatus,
		},
	}, nil
}

func BuildListToolDefinition() (capability.ToolDefinition, error) {
	handler := NewListHandler(NewHandler(nil, DefaultPolicy()))

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {},
  "required": [],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "cameras": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "cameraId": { "type": "string" },
          "lensFacing": { "type": "string" },
          "sensorOrientation": { "type": "integer" },
          "flashAvailable": { "type": "boolean" },
          "supportsAutoFocus": { "type": "boolean" },
          "supportsZoom": { "type": "boolean" },
          "maxWidth": { "type": "integer" },
          "maxHeight": { "type": "integer" }
        }
      }
    },
    "count": { "type": "integer" }
  },
  "required": ["cameras"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDList,
		ModelName:   toolIDList,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Camera List",
		Description: "List available physical cameras on the Android device with their capabilities.",
		Version:     toolVersion,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  permissionID,
				Description: "list available cameras and their capabilities",
				Risk:        "low",
			},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectNone,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-camera-list-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerList,
			"capabilityId":           string(handler.CapabilityID()),
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
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
			HandlerName: handlerList,
		},
	}, nil
}

func BuildCaptureToolDefinition() (capability.ToolDefinition, error) {
	handler := NewCaptureHandler(NewHandler(nil, DefaultPolicy()))

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "cameraId": {
      "type": "string",
      "description": "Specific camera device identifier. Mutually exclusive with lens."
    },
    "lens": {
      "type": "string",
      "enum": ["front", "back", "external"],
      "description": "Select camera by lens facing. Default: back."
    },
    "format": {
      "type": "string",
      "enum": ["jpeg", "png", "webp"],
      "default": "jpeg",
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
    },
    "flashMode": {
      "type": "string",
      "enum": ["off", "on", "auto"],
      "description": "Flash mode. Default: off."
    },
    "focusMode": {
      "type": "string",
      "enum": ["auto", "continuous"],
      "description": "Focus mode. Default: auto."
    },
    "rotation": {
      "type": "integer",
      "description": "Additional rotation in degrees to apply to output."
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
      "description": "amitia:// URI of the captured photo artifact."
    },
    "mimeType": {
      "type": "string",
      "description": "image/jpeg, image/png, or image/webp"
    },
    "width": {
      "type": "integer",
      "description": "Actual pixel width of the captured image."
    },
    "height": {
      "type": "integer",
      "description": "Actual pixel height of the captured image."
    },
    "cameraId": {
      "type": "string",
      "description": "Camera device identifier that was used."
    },
    "lensFacing": {
      "type": "string",
      "description": "front, back, or external"
    },
    "rotation": {
      "type": "integer",
      "description": "Applied rotation in degrees."
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
    },
    "exifStripped": {
      "type": "boolean",
      "description": "Whether sensitive EXIF metadata was stripped."
    }
  },
  "required": ["resourceUri", "mimeType", "width", "height"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDCapture,
		ModelName:   toolIDCapture,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Camera Capture",
		Description: "Capture a single still photo from an Android device camera via CameraX. Returns an amitia:// resource reference, never raw pixels. Requires explicit user approval.",
		Version:     toolVersion,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  permissionID,
				Description: "grants the agent permission to capture photos via Android camera",
				Risk:        "high",
			},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-camera-capture-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerCapture,
			"capabilityId":           string(handler.CapabilityID()),
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
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeAndroid_Native,
			HandlerName: handlerCapture,
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
		Name:        "android.media.camera",
		Description: "Allows the agent to query camera capabilities and capture photos via Android CameraX. Kernel-level gate distinct from the Android system CAMERA permission.",
		Risk:        "high",
	}
}
