package capability

import (
	"context"
	"testing"
	"time"
)

func TestB19NewToolInvocationContextGeneratesInvocationID(t *testing.T) {
	inv := NewToolInvocationContext(ToolInvocationOptions{
		UserID: "user1",
	})

	if inv.InvocationID == "" {
		t.Fatal("expected invocation_id to be generated")
	}
	if inv.TraceID == "" {
		t.Fatal("expected trace_id to be generated")
	}
	if inv.OperationID == "" {
		t.Fatal("expected operation_id to be generated")
	}
	if inv.RootID != inv.InvocationID {
		t.Fatalf("expected root_id to equal invocation_id for root invocation, got: %s vs %s", inv.RootID, inv.InvocationID)
	}
}

func TestB19NewToolInvocationContextInheritsFromParent(t *testing.T) {
	parent := NewToolInvocationContext(ToolInvocationOptions{
		UserID: "user1",
	})

	child := NewToolInvocationContext(ToolInvocationOptions{
		Parent: &parent,
		UserID: "user1",
	})

	if child.ParentID != parent.InvocationID {
		t.Fatalf("expected parent_id to be %s, got: %s", parent.InvocationID, child.ParentID)
	}
	if child.RootID != parent.RootID {
		t.Fatalf("expected root_id to inherit parent's root_id %s, got: %s", parent.RootID, child.RootID)
	}
	if child.TraceID != parent.TraceID {
		t.Fatalf("expected trace_id to inherit parent's trace_id %s, got: %s", parent.TraceID, child.TraceID)
	}
	if child.OperationID != parent.OperationID {
		t.Fatalf("expected operation_id to inherit parent's operation_id %s, got: %s", parent.OperationID, child.OperationID)
	}
}

func TestB19NewToolInvocationContextWithCustomTraceAndOperation(t *testing.T) {
	parent := NewToolInvocationContext(ToolInvocationOptions{
		UserID: "user1",
	})

	child := NewToolInvocationContext(ToolInvocationOptions{
		Parent:      &parent,
		UserID:      "user1",
		TraceID:     "custom-trace",
		OperationID: "custom-operation",
	})

	if child.TraceID != "custom-trace" {
		t.Fatalf("expected custom trace_id, got: %s", child.TraceID)
	}
	if child.OperationID != "custom-operation" {
		t.Fatalf("expected custom operation_id, got: %s", child.OperationID)
	}
	if child.RootID != parent.RootID {
		t.Fatalf("expected root_id to still inherit from parent, got: %s", child.RootID)
	}
}

func TestB19NewToolInvocationContextMetadataDefensiveCopy(t *testing.T) {
	metadata := map[string]any{"key": "value"}
	inv := NewToolInvocationContext(ToolInvocationOptions{
		UserID:   "user1",
		Metadata: metadata,
	})

	metadata["key"] = "mutated"

	if inv.Metadata["key"] != "value" {
		t.Fatalf("metadata should be defensive copy, got: %v", inv.Metadata["key"])
	}
}

func TestB19NewToolSuccessResultHasCorrectStatus(t *testing.T) {
	result := NewToolSuccessResult("inv-1", "tool-1")
	if result.Status != ToolResultStatusSuccess {
		t.Fatalf("expected success status, got: %s", result.Status)
	}
	if result.InvocationID != "inv-1" {
		t.Fatalf("expected invocation_id inv-1, got: %s", result.InvocationID)
	}
	if result.ToolID != "tool-1" {
		t.Fatalf("expected tool_id tool-1, got: %s", result.ToolID)
	}
	if result.Error != nil {
		t.Fatalf("expected nil error for success result, got: %v", result.Error)
	}
}

func TestB19NewToolFailureResultNormalizesError(t *testing.T) {
	result := NewToolFailureResult("inv-1", "tool-1", &ToolError{
		Code: "",
	})
	if result.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed status, got: %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be set")
	}
	if result.Error.Code == "" {
		t.Fatal("expected error code to be normalized")
	}
	if result.Error.Category == "" {
		t.Fatal("expected error category to be normalized")
	}
}

func TestB19NewToolFailureResultHandlesNilError(t *testing.T) {
	result := NewToolFailureResult("inv-1", "tool-1", nil)
	if result.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed status, got: %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected default error when nil passed")
	}
}

func TestB19NewToolCancelledResultHasCorrectCode(t *testing.T) {
	result := NewToolCancelledResult("inv-1", "tool-1")
	if result.Status != ToolResultStatusCancelled {
		t.Fatalf("expected cancelled status, got: %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error for cancelled result")
	}
	if result.Error.Code != ErrorCodeCancelled {
		t.Fatalf("expected cancelled code, got: %s", result.Error.Code)
	}
	if result.Error.Category != ToolErrorCategoryCancellation {
		t.Fatalf("expected cancellation category, got: %s", result.Error.Category)
	}
	if result.Error.Retryable {
		t.Fatal("cancelled should not be retryable")
	}
}

func TestB19NewToolTimedOutResultHasCorrectCode(t *testing.T) {
	result := NewToolTimedOutResult("inv-1", "tool-1")
	if result.Status != ToolResultStatusTimedOut {
		t.Fatalf("expected timed_out status, got: %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error for timed_out result")
	}
	if result.Error.Code != ErrorCodeTimeout {
		t.Fatalf("expected timeout code, got: %s", result.Error.Code)
	}
	if result.Error.Category != ToolErrorCategoryTimeout {
		t.Fatalf("expected timeout category, got: %s", result.Error.Category)
	}
}

func TestB19ResultFromContextError(t *testing.T) {
	t.Run("deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(5 * time.Millisecond)
		result := ResultFromContextError("inv-1", ctx.Err())
		if result.Status != ToolResultStatusTimedOut {
			t.Fatalf("expected timed_out status, got: %s", result.Status)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := ResultFromContextError("inv-1", ctx.Err())
		if result.Status != ToolResultStatusCancelled {
			t.Fatalf("expected cancelled status, got: %s", result.Status)
		}
	})
}

func TestB19ResultCloneDeepCopiesAllFields(t *testing.T) {
	original := UnifiedToolResult{
		InvocationID: "inv-1",
		ToolID:       "tool-1",
		Status:       ToolResultStatusSuccess,
		Content: []ToolContent{
			{Type: ToolContentText, Text: "hello"},
		},
		Structured: []byte(`{"key":"value"}`),
		SideEffects: []RecordedSideEffect{
			{Type: "write", Target: "/tmp/file"},
		},
		DurationMS: 100,
		Metadata:   map[string]any{"key": "value"},
		Error: &ToolError{
			Code:    ErrorCodeExecutionFailed,
			Message: "fail",
			Details: map[string]any{"inner": "data"},
		},
	}

	clone := original.Clone()

	clone.Metadata["key"] = "mutated"
	clone.Content[0].Text = "mutated"
	clone.Structured[0] = 'X'
	clone.SideEffects[0].Target = "/mutated"
	clone.Error.Details["inner"] = "mutated"

	if original.Metadata["key"] != "value" {
		t.Fatal("metadata not deep copied")
	}
	if original.Content[0].Text != "hello" {
		t.Fatal("content not deep copied")
	}
	if original.Structured[0] != '{' {
		t.Fatal("structured not deep copied")
	}
	if original.SideEffects[0].Target != "/tmp/file" {
		t.Fatal("side effects not deep copied")
	}
	if original.Error.Details["inner"] != "data" {
		t.Fatal("error details not deep copied")
	}
}

func TestB19ToolResultStatusValidRejectsInvalidStatus(t *testing.T) {
	validStatuses := []ToolResultStatus{
		ToolResultStatusSuccess,
		ToolResultStatusFailed,
		ToolResultStatusCancelled,
		ToolResultStatusTimedOut,
	}
	for _, s := range validStatuses {
		if !s.Valid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}

	invalidStatuses := []ToolResultStatus{"", "unknown", "error", "pending"}
	for _, s := range invalidStatuses {
		if s.Valid() {
			t.Fatalf("expected %s to be invalid", s)
		}
	}
}

func TestB19NormalizeToolErrorHandlesNil(t *testing.T) {
	if NormalizeToolError(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestB19NormalizeToolErrorFillsCategory(t *testing.T) {
	err := &ToolError{
		Code:    ErrorCodeInvalidInput,
		Message: "bad input",
	}
	normalized := NormalizeToolError(err)
	if normalized.Category != ToolErrorCategoryValidation {
		t.Fatalf("expected validation category, got: %s", normalized.Category)
	}
}

func TestB19ErrorCategoryForCode(t *testing.T) {
	tests := []struct {
		code     string
		expected ToolErrorCategory
	}{
		{ErrorCodeInvalidInput, ToolErrorCategoryValidation},
		{ErrorCodeInvalidResult, ToolErrorCategoryValidation},
		{ErrorCodePermissionDenied, ToolErrorCategoryPermission},
		{ErrorCodeScopeDenied, ToolErrorCategoryPermission},
		{ErrorCodeNotAvailable, ToolErrorCategoryAvailability},
		{ErrorCodeTimeout, ToolErrorCategoryTimeout},
		{ErrorCodeCancelled, ToolErrorCategoryCancellation},
		{ErrorCodeRateLimited, ToolErrorCategoryRateLimit},
		{"unknown", ToolErrorCategoryInternal},
	}
	for _, tt := range tests {
		if got := ErrorCategoryForCode(tt.code); got != tt.expected {
			t.Fatalf("code %s: expected %s, got: %s", tt.code, tt.expected, got)
		}
	}
}

func TestB19InvocationSourceValid(t *testing.T) {
	validSources := []InvocationSource{
		InvocationSourceModel,
		InvocationSourceUser,
		InvocationSourceWorkflow,
		InvocationSourcePlugin,
		InvocationSourceSystem,
		InvocationSourceScheduledTask,
		InvocationSourceComputerUse,
	}
	for _, s := range validSources {
		if !s.Valid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}

	invalidSources := []InvocationSource{"", "unknown", "test"}
	for _, s := range invalidSources {
		if s.Valid() {
			t.Fatalf("expected %s to be invalid", s)
		}
	}
}

func TestB19DeepNestedInvocationChainPreservesRoot(t *testing.T) {
	root := NewToolInvocationContext(ToolInvocationOptions{
		UserID: "user1",
	})
	level1 := NewToolInvocationContext(ToolInvocationOptions{
		Parent: &root,
		UserID: "user1",
	})
	level2 := NewToolInvocationContext(ToolInvocationOptions{
		Parent: &level1,
		UserID: "user1",
	})
	level3 := NewToolInvocationContext(ToolInvocationOptions{
		Parent: &level2,
		UserID: "user1",
	})

	if level3.RootID != root.RootID {
		t.Fatalf("deep nested chain should preserve root_id, got: %s vs %s", level3.RootID, root.RootID)
	}
	if level3.TraceID != root.TraceID {
		t.Fatalf("deep nested chain should preserve trace_id, got: %s vs %s", level3.TraceID, root.TraceID)
	}
	if level3.OperationID != root.OperationID {
		t.Fatalf("deep nested chain should preserve operation_id, got: %s vs %s", level3.OperationID, root.OperationID)
	}
}
