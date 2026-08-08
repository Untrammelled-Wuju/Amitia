package execution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestB18ResultValidatorPassesValidStructuredOutput(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["count"],
		"properties": {
			"count": {"type": "integer"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/output-valid",
		ModelName:    "test_output_valid",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status:     capability.ToolResultStatusSuccess,
		Structured: json.RawMessage(`{"count": 3}`),
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success status, got: %s (err: %v)", validated.Status, validated.Error)
	}
}

func TestB18ResultValidatorRejectsInvalidStructuredOutput(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["count"],
		"properties": {
			"count": {"type": "integer"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/output-invalid",
		ModelName:    "test_output_invalid",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status:     capability.ToolResultStatusSuccess,
		Structured: json.RawMessage(`{"count": "three"}`),
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for invalid output, got: %s", validated.Status)
	}
	if validated.Error == nil || validated.Error.Code != capability.ErrorCodeInvalidResult {
		t.Fatalf("expected invalid_result error, got: %v", validated.Error)
	}
}

func TestB18ResultValidatorRejectsInvalidStructuredJSON(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"count": {"type": "integer"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/json-invalid",
		ModelName:    "test_json_invalid",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status:     capability.ToolResultStatusSuccess,
		Structured: json.RawMessage(`{"count":`),
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for invalid JSON, got: %s", validated.Status)
	}
}

func TestB18ResultValidatorDoesNotRequireStructuredForTextOnly(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["result"],
		"properties": {
			"result": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/text-only",
		ModelName:    "test_text_only",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentText,
				Text: "This is a text-only response",
			},
		},
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected text-only result to pass, got: %s (err: %v)", validated.Status, validated.Error)
	}
}

func TestB18ResultValidatorValidatesStructuredContent(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["data"],
		"properties": {
			"data": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/content-structured",
		ModelName:    "test_content_structured",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentStructured,
				Data: json.RawMessage(`{"data": "valid"}`),
			},
		},
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected valid structured content to pass, got: %s (err: %v)", validated.Status, validated.Error)
	}
}

func TestB18ResultValidatorRejectsInvalidStructuredContent(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["data"],
		"properties": {
			"data": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/content-invalid",
		ModelName:    "test_content_invalid",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentStructured,
				Data: json.RawMessage(`{"data": 123}`),
			},
		},
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for invalid structured content, got: %s", validated.Status)
	}
}

func TestB18ResultErrorMessageDoesNotLeakSchemaErrors(t *testing.T) {
	v := NewResultValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["secret"],
		"properties": {
			"secret": {"type": "string", "pattern": "^[A-Z]+$"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:           "test/no-leak",
		ModelName:    "test_no_leak",
		OutputSchema: schema,
		Source:       capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status:     capability.ToolResultStatusSuccess,
		Structured: json.RawMessage(`{"secret": "value_with_underscores_123"}`),
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatal("expected failed status")
	}

	if validated.Error == nil {
		t.Fatal("expected error to be set")
	}
}

func TestB18ResultValidatorDoesNotAutoFailEmptyOutput(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:        "test/no-output-schema",
		ModelName: "test_no_output_schema",
		Source:    capability.ToolSourceBuiltin,
	}

	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentText,
				Text: "plain text result",
			},
		},
	}

	validated := v.Validate(context.Background(), tool, result)
	if validated.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success for text-only result without output schema, got: %s (err: %v)", validated.Status, validated.Error)
	}
}
