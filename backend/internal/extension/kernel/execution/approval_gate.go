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
	Broker     permission.PermissionBroker
}

func (g *ApprovalGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision) (bool, error) {
	if decision != PermissionRequireApproval {
		return true, nil
	}

	if g.Broker != nil {
		allowed, err := g.evaluateWithBroker(ctx, tool, inv)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}

	if g.OnEvaluate != nil {
		return g.OnEvaluate(ctx, tool, inv, decision)
	}
	return false, nil
}

func (g *ApprovalGate) evaluateWithBroker(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (bool, error) {
	subject := permission.SubjectForTool(tool.ExtensionID, tool.ID)
	if tool.ExtensionID == "" {
		subject = permission.PermissionSubject{Type: permission.SubjectSystem, ID: "core"}
	}

	reqs := make([]permission.PermissionRequirement, 0)
	for _, p := range tool.Permissions {
		reqs = append(reqs, permission.PermissionRequirement{PermissionID: p.Capability})
	}

	explanation := g.Broker.Explain(ctx, permission.PermissionEvaluationRequest{
		Subject:          subject,
		Requirements:     reqs,
		InvocationID:     inv.InvocationID,
		RiskLevel:        string(tool.RiskLevel),
		Generation:       inv.Generation,
		ExecutionContext: permission.ExecutionContextFromInvocation(inv),
	})

	if explanation.RequiredAction == "manual_approval" {
		return false, nil
	}

	return explanation.Decision == permission.DecisionAllow, nil
}
