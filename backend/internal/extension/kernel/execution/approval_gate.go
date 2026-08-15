package execution

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func NewApprovalGate() *ApprovalGate {
	return &ApprovalGate{}
}

type ApprovalGate struct {
	OnEvaluate func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision) (bool, error)
}

func (g *ApprovalGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision) (bool, error) {
	if decision != PermissionRequireApproval {
		return true, nil
	}

	if g.OnEvaluate != nil {
		return g.OnEvaluate(ctx, tool, inv, decision)
	}

	return false, nil
}
