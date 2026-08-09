package execution

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewInvocationValidator() *InvocationValidator {
	return &InvocationValidator{}
}

type InvocationValidator struct{}

func (v *InvocationValidator) Validate(ctx context.Context, request ToolExecutionRequest) error {
	inv := request.Invocation
	if inv.InvocationID == "" {
		return fmt.Errorf("invocation_id is required")
	}
	if request.ToolID == "" {
		return fmt.Errorf("tool_id is required")
	}
	if inv.UserID == "" && inv.Source != capability.InvocationSourceScheduledTask {
		return fmt.Errorf("user_id is required")
	}
	if !inv.Source.Valid() {
		return fmt.Errorf("invalid invocation source")
	}
	if inv.ApprovalMode != "" && !inv.ApprovalMode.Valid() {
		return fmt.Errorf("invalid approval mode")
	}
	if inv.RootID == "" {
		return fmt.Errorf("invocation root_id is required")
	}
	if inv.TraceID == "" {
		return fmt.Errorf("invocation trace_id is required")
	}
	if inv.OperationID == "" {
		return fmt.Errorf("invocation operation_id is required")
	}
	if inv.InvocationID == inv.ParentID {
		return fmt.Errorf("invocation_id must not equal parent_id")
	}
	if inv.ParentID != "" && inv.RootID == inv.InvocationID {
		return fmt.Errorf("root_id must not equal invocation_id when parent_id is set")
	}
	return nil
}
