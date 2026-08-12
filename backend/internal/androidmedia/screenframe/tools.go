package screenframe

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	toolIDStatus  = "android.screen_frame.status"
	toolIDStart   = "android.screen_frame.start"
	toolIDLatest  = "android.screen_frame.latest"
	toolIDStop    = "android.screen_frame.stop"

	handlerStatus  = "screen_frame.status"
	handlerStart   = "screen_frame.start"
	handlerLatest  = "screen_frame.latest"
	handlerStop    = "screen_frame.stop"

	toolVersion = "0.1.0"
)

func BuildStartToolDefinition() (capability.ToolDefinition, error) {
	handler := NewStartHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "displayId": {
      "type": "integer",
      "minimum": 0,
      "default": 0,
      "description": "Target display identifier. 0 is the primary display."
    },
    "targetFps": {
      "type": "number",
      "minimum": 0.2,
      "maximum": 10,
      "default": 2,
      "description": "Desired capture rate (frames per second). Throttled by the capture pipeline."
    },
    "maxWidth": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1280,
      "default": 1280,
      "description": "Maximum frame width. Aspect ratio preserved. No upscaling."
    },
    "maxHeight": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1280,
      "default": 1280,
      "description": "Maximum frame height. Aspect ratio preserved. No upscaling."
    }
  },
  "required": [],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "sessionId": { "type": "string" },
    "state": { "type": "string" },
    "displayId": { "type": "integer" },
    "width": { "type": "integer" },
    "height": { "type": "integer" },
    "targetFps": { "type": "number" },
    "generation": { "type": "integer" }
  },
  "required": ["sessionId", "state"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDStart,
		ModelName:   toolIDStart,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Screen Frame Start",
		Description: "Start a short-lived Android screen frame capture session via MediaProjection. Requires user grant and android.media.screen_capture permission.",
		Version:     toolVersion,
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  PermissionContinuousCapture,
				Description: "grants the agent continuous screen reading via MediaProjection capture session",
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
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-frame-start-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerStart,
			"capabilityId":           string(handler.CapabilityID()),
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
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
			HandlerName: handlerStart,
		},
	}, nil
}

func BuildLatestToolDefinition() (capability.ToolDefinition, error) {
	handler := NewLatestHandler(NewBlockedSessionStore(1), DefaultScreenFramePolicy())

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "sessionId": { "type": "string", "description": "Opaque screen frame capture session id returned by start." },
    "afterSequence": { "type": "integer", "minimum": 0, "description": "Wait until a frame newer than this sequence is available." },
    "waitMs": { "type": "integer", "minimum": 0, "maximum": 5000, "description": "Maximum time to wait for a new frame." },
    "format": { "type": "string", "enum": ["png", "jpeg", "webp"], "default": "jpeg" },
    "quality": { "type": "integer", "minimum": 1, "maximum": 100 },
    "maxWidth": { "type": "integer", "minimum": 1 },
    "maxHeight": { "type": "integer", "minimum": 1 }
  },
  "required": ["sessionId"],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "hasFrame": { "type": "boolean" },
    "sessionId": { "type": "string" },
    "sequence": { "type": "integer" },
    "generation": { "type": "integer" },
    "resourceUri": { "type": "string" },
    "mimeType": { "type": "string" },
    "width": { "type": "integer" },
    "height": { "type": "integer" },
    "capturedAt": { "type": "integer" },
    "ageMs": { "type": "integer" },
    "droppedSincePrevious": { "type": "integer" }
  },
  "required": ["hasFrame", "sessionId"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDLatest,
		ModelName:   toolIDLatest,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Screen Frame Latest",
		Description: "On-demand read the newest frame from an active screen frame capture session. Returns an amitia:// resource reference, never raw pixels.",
		Version:     toolVersion,
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  PermissionContinuousCapture,
				Description: "grants the agent continuous screen reading via MediaProjection capture session",
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
		TimeoutMS:      6000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-frame-latest-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerLatest,
			"capabilityId":           string(handler.CapabilityID()),
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          6 * time.Second,
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
			HandlerName: handlerLatest,
		},
	}, nil
}

func BuildStopToolDefinition() (capability.ToolDefinition, error) {
	handler := NewStopHandler(NewBlockedSessionStore(1))

	inputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "sessionId": { "type": "string", "description": "Opaque screen frame capture session id to stop." }
  },
  "required": ["sessionId"],
  "additionalProperties": false
}`)

	outputSchema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "sessionId": { "type": "string" },
    "state": { "type": "string" }
  },
  "required": ["sessionId", "state"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDStop,
		ModelName:   toolIDStop,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Screen Frame Stop",
		Description: "Explicitly stop an active screen frame capture session. Idempotent on repeated calls.",
		Version:     toolVersion,
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  PermissionContinuousCapture,
				Description: "grants the agent continuous screen reading via MediaProjection capture session",
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
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-frame-stop-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerStop,
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
			HandlerName: handlerStop,
		},
	}, nil
}

func BuildStatusToolDefinition() (capability.ToolDefinition, error) {
	handler := NewStatusHandler(NewBlockedSessionStore(1))

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
    "activeSession": { "type": "boolean" },
    "sessionId": { "type": "string" },
    "displayId": { "type": "integer" },
    "width": { "type": "integer" },
    "height": { "type": "integer" },
    "targetFps": { "type": "number" },
    "lastFrameSequence": { "type": "integer" },
    "lastFrameAt": { "type": "integer" },
    "userActionRequired": { "type": "boolean" },
    "state": { "type": "string" }
  },
  "required": ["supported"]
}`)

	return capability.ToolDefinition{
		ID:          toolIDStatus,
		ModelName:   toolIDStatus,
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android Screen Frame Status",
		Description: "Read Android frame capture capability and projection authorization state.",
		Version:     toolVersion,
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{
				Capability:  PermissionContinuousCapture,
				Description: "read screen frame capture capability state",
				Risk:        "medium",
			},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectNone,
		Scope:          capability.ScopeRule{Type: "user"},
		Enabled:        true,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      false,
		TimeoutMS:      3000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b35-frame-status-v1"},
		Metadata: map[string]any{
			"androidNativeOperation": handlerStatus,
			"capabilityId":           string(handler.CapabilityID()),
			"bridgeProtocol":         "android_native",
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          3 * time.Second,
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
			HandlerName: handlerStatus,
		},
	}, nil
}
