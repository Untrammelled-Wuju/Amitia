package execution

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestB19InvocationValidatorRejectsMissingInvocationID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			RootID:      "inv-1",
			TraceID:     "trace-1",
			OperationID: "op-1",
			UserID:      "user1",
			Source:      capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing invocation_id")
	}
}

func TestB19InvocationValidatorRejectsMissingToolID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "inv-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing tool_id")
	}
}

func TestB19InvocationValidatorRejectsInvalidSource(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "root-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSource("invalid"),
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestB19InvocationValidatorRejectsMissingUserID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "root-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestB19InvocationValidatorAllowsScheduledTaskWithoutUserID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "root-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			Source:       capability.InvocationSourceScheduledTask,
		},
	})
	if err != nil {
		t.Fatalf("scheduled task should not require user_id, got: %v", err)
	}
}

func TestB19InvocationValidatorRejectsMissingRootID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing root_id")
	}
}

func TestB19InvocationValidatorRejectsMissingTraceID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "root-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing trace_id")
	}
}

func TestB19InvocationValidatorRejectsMissingOperationID(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "root-1",
			TraceID:      "trace-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error for missing operation_id")
	}
}

func TestB19InvocationValidatorRejectsSelfParent(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			ParentID:     "inv-1",
			RootID:       "inv-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error when invocation_id equals parent_id")
	}
}

func TestB19InvocationValidatorRejectsParentWithSameRootAsInvocation(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-2",
			ParentID:     "inv-1",
			RootID:       "inv-2",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err == nil {
		t.Fatal("expected error when root_id equals invocation_id but parent_id is set")
	}
}

func TestB19InvocationValidatorPassesWithValidRootInvocation(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-1",
			RootID:       "inv-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err != nil {
		t.Fatalf("expected no error for valid root invocation, got: %v", err)
	}
}

func TestB19InvocationValidatorPassesWithValidChildInvocation(t *testing.T) {
	v := NewInvocationValidator()
	err := v.Validate(context.Background(), ToolExecutionRequest{
		ToolID: "test-tool",
		Invocation: capability.ToolInvocationContext{
			InvocationID: "inv-2",
			ParentID:     "inv-1",
			RootID:       "inv-1",
			TraceID:      "trace-1",
			OperationID:  "op-1",
			UserID:       "user1",
			Source:       capability.InvocationSourceModel,
		},
	})
	if err != nil {
		t.Fatalf("expected no error for valid child invocation, got: %v", err)
	}
}
