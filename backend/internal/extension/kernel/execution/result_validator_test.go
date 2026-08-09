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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/output-valid",
		Status:       capability.ToolResultStatusSuccess,
		Structured:   json.RawMessage(`{"count": 3}`),
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/output-invalid",
		Status:       capability.ToolResultStatusSuccess,
		Structured:   json.RawMessage(`{"count": "three"}`),
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/json-invalid",
		Status:       capability.ToolResultStatusSuccess,
		Structured:   json.RawMessage(`{"count":`),
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/text-only",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentText,
				Text: "This is a text-only response",
			},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/content-structured",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentStructured,
				Data: json.RawMessage(`{"data": "valid"}`),
			},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/content-invalid",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentStructured,
				Data: json.RawMessage(`{"data": 123}`),
			},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/no-leak",
		Status:       capability.ToolResultStatusSuccess,
		Structured:   json.RawMessage(`{"secret": "value_with_underscores_123"}`),
	}

	validated := v.Validate(context.Background(), tool, inv, result)
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

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/no-output-schema",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{
				Type: capability.ToolContentText,
				Text: "plain text result",
			},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success for text-only result without output schema, got: %s (err: %v)", validated.Status, validated.Error)
	}
}

func TestB19ResultValidatorRejectsInvocationIDMismatch(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/mismatch",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: "wrong-id",
		ToolID:       "test/mismatch",
		Status:       capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "hello"},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for invocation_id mismatch, got: %s", validated.Status)
	}
	if validated.Error == nil || validated.Error.Code != capability.ErrorCodeInvalidResult {
		t.Fatalf("expected invalid_result error, got: %v", validated.Error)
	}
	if validated.InvocationID != inv.InvocationID {
		t.Fatalf("expected invocation_id to be preserved, got: %s", validated.InvocationID)
	}
}

func TestB19ResultValidatorFillsMissingInvocationID(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/missing-id",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "hello"},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.InvocationID != inv.InvocationID {
		t.Fatalf("expected invocation_id to be filled from invocation, got: %s", validated.InvocationID)
	}
}

func TestB19ResultValidatorSuccessWithErrorBecomesFailed(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/success-error",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/success-error",
		Status:       capability.ToolResultStatusSuccess,
		Error: &capability.ToolError{
			Code:    capability.ErrorCodeExecutionFailed,
			Message: "something went wrong",
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status when success has error, got: %s", validated.Status)
	}
}

func TestB19ResultValidatorFailedWithoutErrorGetsDefault(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/failed-no-error",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/failed-no-error",
		Status:       capability.ToolResultStatusFailed,
		Content: []capability.ToolContent{
			{Type: capability.ToolContentText, Text: "some content"},
		},
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status, got: %s", validated.Status)
	}
	if validated.Error == nil {
		t.Fatalf("expected error to be auto-populated")
	}
	if validated.Error.Code != capability.ErrorCodeExecutionFailed {
		t.Fatalf("expected execution_failed code, got: %s", validated.Error.Code)
	}
}

func TestB19ResultValidatorCancelledGetsCorrectCode(t *testing.T) {
	v := NewResultValidator()

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		Status:       capability.ToolResultStatusCancelled,
	}

	validated := v.Validate(context.Background(), capability.ToolDefinition{}, inv, result)
	if validated.Status != capability.ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status, got: %s", validated.Status)
	}
	if validated.Error == nil {
		t.Fatalf("expected auto-generated error for cancelled")
	}
	if validated.Error.Code != capability.ErrorCodeCancelled {
		t.Fatalf("expected cancelled code, got: %s", validated.Error.Code)
	}
	if validated.Error.Retryable {
		t.Fatalf("cancelled should not be retryable")
	}
}

func TestB19ResultValidatorCloneDoesNotMutateOriginal(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/clone",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	originalContent := []capability.ToolContent{
		{Type: capability.ToolContentText, Text: "original"},
	}
	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/clone",
		Status:       capability.ToolResultStatusSuccess,
		Content:      originalContent,
	}

	_ = v.Validate(context.Background(), tool, inv, result)

	if len(originalContent) != 1 || originalContent[0].Text != "original" {
		t.Fatalf("original result content was mutated")
	}
}

func TestB19ResultValidatorUnknownStatusBecomesInvalidResult(t *testing.T) {
	v := NewResultValidator()

	tool := capability.ToolDefinition{
		ID:     "test/unknown-status",
		Source: capability.ToolSourceBuiltin,
	}

	inv := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		UserID: "user1",
	})

	result := capability.UnifiedToolResult{
		InvocationID: inv.InvocationID,
		ToolID:       "test/unknown-status",
		Status:       capabilityToolResultStatus("unknown"),
	}

	validated := v.Validate(context.Background(), tool, inv, result)
	if validated.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed status for unknown status, got: %s", validated.Status)
	}
}

// Helper to avoid import cycle
func capabilityToolResultStatus(s string) capability.ToolResultStatus {
	return capability.ToolResultStatus(s)
}
