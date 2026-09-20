package execution

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewApprovalGate() *ApprovalGate {
	return &ApprovalGate{}
}

type ApprovalGate struct {
	OnEvaluate func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision, input json.RawMessage) (bool, error)
}

func (g *ApprovalGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision, input json.RawMessage) (bool, error) {
	if decision != PermissionRequireApproval {
		return true, nil
	}

	if g.OnEvaluate != nil {
		return g.OnEvaluate(ctx, tool, inv, decision, input)
	}

	return false, nil
}
