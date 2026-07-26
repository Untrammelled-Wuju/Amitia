package execution

import (
	"context"
	"fmt"
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
	if inv.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if inv.Source == "" {
		return fmt.Errorf("invocation source is required")
	}
	return nil
}
