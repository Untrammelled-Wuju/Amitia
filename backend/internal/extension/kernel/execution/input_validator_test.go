package execution

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestB18InputValidatorRejectsMissingRequired(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/required",
		ModelName:   "test_required",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
}

func TestB18InputValidatorRejectsWrongType(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/type",
		ModelName:   "test_type",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"query": 123}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for wrong type")
	}
}

func TestB18InputValidatorRejectsEnumMismatch(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"mode": {
				"type": "string",
				"enum": ["fast", "deep"]
			}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/enum",
		ModelName:   "test_enum",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"mode": "slow"}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for enum mismatch")
	}
}

func TestB18InputValidatorRejectsNestedArrayItems(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/array",
		ModelName:   "test_array",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"items": [1, 2, 3]}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for nested array items mismatch")
	}
}

func TestB18InputValidatorRejectsAdditionalProperties(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"properties": {
			"query": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/additional",
		ModelName:   "test_additional",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"query": "test", "extra": "field"}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for additional properties")
	}
}

func TestB18InputValidatorRejectsInvalidJSON(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string"}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/json",
		ModelName:   "test_json",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"query":`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for invalid JSON input")
	}
}

func TestB18InputValidatorRespectsMaxInputBytesBeforeSchema(t *testing.T) {
	v := NewInputValidator()
	v.MaxInputBytes = 5

	tool := capability.ToolDefinition{
		ID:        "test/size",
		ModelName: "test_size",
		Source:    capability.ToolSourceBuiltin,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"query": {"type": "string"}}
		}`),
	}

	input := json.RawMessage(`{"query": "this is a very long value"}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected size limit error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size limit error, got: %v", err)
	}
}

func TestB18InputValidatorPassesValidComplexInput(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["query", "mode"],
		"properties": {
			"query": {
				"type": "string",
				"minLength": 1
			},
			"mode": {
				"type": "string",
				"enum": ["fast", "deep"]
			},
			"filters": {
				"type": "array",
				"items": {
					"type": "object",
					"required": ["key"],
					"properties": {
						"key": {"type": "string"}
					}
				}
			}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/complex",
		ModelName:   "test_complex",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{
		"query": "search term",
		"mode": "fast",
		"filters": [
			{"key": "date"},
			{"key": "user"}
		]
	}`)

	err := v.Validate(context.Background(), tool, input)
	if err != nil {
		t.Fatalf("expected valid complex input to pass, got: %v", err)
	}
}

func TestB18InputValidatorRejectsMinLengthViolation(t *testing.T) {
	v := NewInputValidator()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"minLength": 1
			}
		}
	}`)

	tool := capability.ToolDefinition{
		ID:          "test/minlen",
		ModelName:   "test_minlen",
		InputSchema: schema,
		Source:      capability.ToolSourceBuiltin,
	}

	input := json.RawMessage(`{"query": ""}`)

	err := v.Validate(context.Background(), tool, input)
	if err == nil {
		t.Fatal("expected validation error for minLength violation")
	}
}
