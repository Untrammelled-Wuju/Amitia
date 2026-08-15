package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var ErrToolRegistryNil = errors.New("schema: tool registry is nil")

// RegisterSchemaTools registers schema-related tool definitions into the kernel registry.
func RegisterSchemaTools(registry *capability.ToolRegistry, generator *AISchemaGenerator, validator SchemaValidator) error {
	if registry == nil {
		return ErrToolRegistryNil
	}
	ctx := context.Background()

	tools := []*capability.ToolDefinition{
		buildGenerateTool(),
		buildValidateTool(),
		buildCompileTool(),
	}

	for _, def := range tools {
		if err := registry.Register(ctx, *def); err != nil {
			return fmt.Errorf("register schema tool %s: %w", def.ID, err)
		}
	}

	_ = generator
	_ = validator
	return nil
}

func buildGenerateTool() *capability.ToolDefinition {
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Natural language description of the desired UI",
			},
			"components": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional list of allowed component types",
			},
		},
		"required": []string{"description"},
	}

	inputBytes, _ := json.Marshal(inputSchema)

	return &capability.ToolDefinition{
		ID:          capability.BuildToolID(capability.ToolSourceAcquisition, "schema", "generate"),
		Source:      capability.ToolSourceAcquisition,
		Name:        "Generate Schema UI",
		Description: "Generates a SchemaUIDocument from a natural language description using AI or heuristics",
		InputSchema: json.RawMessage(inputBytes),
		Enabled:     true,
	}
}

func buildValidateTool() *capability.ToolDefinition {
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"document": map[string]any{
				"type":        "object",
				"description": "The SchemaUIDocument to validate",
			},
		},
		"required": []string{"document"},
	}

	inputBytes, _ := json.Marshal(inputSchema)

	return &capability.ToolDefinition{
		ID:          capability.BuildToolID(capability.ToolSourceAcquisition, "schema", "validate"),
		Source:      capability.ToolSourceAcquisition,
		Name:        "Validate Schema",
		Description: "Validates a SchemaUIDocument against the component catalog and security rules",
		InputSchema: json.RawMessage(inputBytes),
		Enabled:     true,
	}
}

func buildCompileTool() *capability.ToolDefinition {
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"document": map[string]any{
				"type":        "object",
				"description": "The SchemaUIDocument to compile",
			},
		},
		"required": []string{"document"},
	}

	inputBytes, _ := json.Marshal(inputSchema)

	return &capability.ToolDefinition{
		ID:          capability.BuildToolID(capability.ToolSourceAcquisition, "schema", "compile"),
		Source:      capability.ToolSourceAcquisition,
		Name:        "Compile Schema",
		Description: "Compiles a SchemaUIDocument into a render-ready representation",
		InputSchema: json.RawMessage(inputBytes),
		Enabled:     true,
	}
}
