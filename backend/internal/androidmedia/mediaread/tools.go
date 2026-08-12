package mediaread

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	ToolIDInfo  = "android.media.read.info"
	ToolIDImage = "android.media.read.image"

	HandlerInfo  = OperationInfo
	HandlerImage = OperationImage

	toolVersion     = "0.1.0"
	permissionRead  = "android.media.read"
)

func BuildInfoToolDefinition() (capability.ToolDefinition, error) {
	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "resourceUri": {
      "type": "string",
      "description": "Canonical amitia:// resource URI of the image to inspect."
    }
  },
  "required": ["resourceUri"],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "resourceUri": { "type": "string" },
    "mimeType": { "type": "string" },
    "format": { "type": "string" },
    "sizeBytes": { "type": "integer" },
    "width": { "type": "integer" },
    "height": { "type": "integer" },
    "orientation": { "type": "integer" },
    "hasAlpha": { "type": "boolean" },
    "animated": { "type": "boolean" },
    "source": { "type": "string" }
  },
  "required": ["resourceUri", "mimeType", "format", "sizeBytes", "width", "height"]
}`)

	return capability.ToolDefinition{
		ID:          ToolIDInfo,
		ModelName:   ToolIDInfo,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Media Read Info",
		Description: "Read image resource metadata without running OCR. Returns format, dimensions, size, alpha, and detected source. Supports amitia:// URIs from Camera, Screenshot, Workspace, Attachments, and Temp.",
		Version:     toolVersion,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  permissionRead,
				Description: "read image resource metadata via Media Read adapter",
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
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-mediaread-info-v1"},
		Metadata: map[string]any{
			"operation":      HandlerInfo,
			"bridgeProtocol": "mediaread",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   2,
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
			HandlerName: HandlerInfo,
		},
	}, nil
}

func BuildImageToolDefinition() (capability.ToolDefinition, error) {
	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "resourceUri": {
      "type": "string",
      "description": "Canonical amitia:// resource URI of the image to normalize."
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
    "maxPixels": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum pixel count."
    },
    "normalizeOrientation": {
      "type": "boolean",
      "description": "Normalize EXIF orientation to visual orientation."
    },
    "stripMetadata": {
      "type": "boolean",
      "description": "Strip sensitive EXIF metadata (GPS, device info, serial)."
    }
  },
  "required": ["resourceUri"],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "resourceUri": {
      "type": "string",
      "description": "amitia:// URI of the normalized image resource."
    },
    "mimeType": { "type": "string" },
    "width": { "type": "integer" },
    "height": { "type": "integer" },
    "sizeBytes": { "type": "integer" },
    "normalized": {
      "type": "boolean",
      "description": "Whether a new artifact was generated."
    },
    "sourceUri": {
      "type": "string",
      "description": "Original resource URI if a new artifact was generated."
    }
  },
  "required": ["resourceUri", "mimeType", "width", "height", "sizeBytes", "normalized"]
}`)

	return capability.ToolDefinition{
		ID:          ToolIDImage,
		ModelName:   ToolIDImage,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Media Read Image",
		Description: "Normalize an Android media image (decode safely, fix orientation, strip sensitive metadata). Returns a canonical resource URI ready for downstream OCR or Image Understand. Reuses the original artifact when no change is needed.",
		Version:     toolVersion,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  permissionRead,
				Description: "normalize image resources via Media Read adapter for downstream OCR/Understand",
				Risk:        "low",
			},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectWrite,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: true,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b36-mediaread-image-v1"},
		Metadata: map[string]any{
			"operation":      HandlerImage,
			"bridgeProtocol": "mediaread",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   2,
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
			HandlerName: HandlerImage,
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
		ID:          permissionRead,
		Name:        "android.media.read",
		Description: "Allows reading and normalizing Android media resources (images from Camera, Screenshot, Workspace, Attachments) via the Media Read adapter. Output is consumed by existing Image Intelligence OCR and Understand tools, not a separate OCR backend.",
		Risk:        "low",
	}
}
