package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/media"
)

const mediaToolDomain = "media"

func registerMediaTools(registry *capability.ToolRegistry, svc *media.Service) error {
	ctx := context.Background()
	defs := buildMediaTools(svc)
	for _, def := range defs {
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register media tool %s: %w", def.ID, err)
		}
	}
	return nil
}

func buildMediaTools(svc *media.Service) []capability.ToolDefinition {
	return []capability.ToolDefinition{
		buildMediaMetadataTool(svc),
		buildMediaConvertTool(svc),
	}
}

func buildMediaMetadataTool(_ *media.Service) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resource": {"type": "string"}
		},
		"required": ["resource"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"metadata": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "media.metadata",
		ModelName:    "media.metadata",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Media Metadata",
		Description:  "Get metadata of a media file.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions:  []capability.PermissionRequirement{{Capability: "media.metadata", Risk: "low"}},
		RiskLevel:    capability.RiskLow,
		SideEffect:   capability.SideEffectReadOnly,
		Idempotent:   true,
		Retryable:    true,
		TimeoutMS:    30000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "media-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:        30 * time.Second,
			MaxConcurrency: 5,
			Idempotent:     true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			MaxOutputBytes: 65536,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeMedia,
			RuntimeID:   "default",
			HandlerName: "media.metadata",
		},
		Enabled: true,
	}
}

func buildMediaConvertTool(_ *media.Service) capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resource": {"type": "string"},
			"target": {"type": "string"},
			"targetContainer": {"type": "string"},
			"videoCodec": {"type": "string"},
			"audioCodec": {"type": "string"}
		},
		"required": ["resource"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resource": {"type": "string"},
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "media.convert",
		ModelName:    "media.convert",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Media Convert",
		Description:  "Convert media to another format.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions:  []capability.PermissionRequirement{{Capability: "media.convert", Risk: "medium"}},
		RiskLevel:    capability.RiskMedium,
		SideEffect:   capability.SideEffectWrite,
		Idempotent:   false,
		Retryable:    false,
		TimeoutMS:    120000,
		ToolVersion:  capability.ToolVersion{SchemaVersion: 1, Revision: "media-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          120 * time.Second,
			MaxConcurrency:   2,
			ApprovalRequired: true,
		},
		ResultPolicy: capability.ToolResultPolicy{
			MaxOutputBytes: 4096,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeMedia,
			RuntimeID:   "default",
			HandlerName: "media.convert",
		},
		Enabled: true,
	}
}
