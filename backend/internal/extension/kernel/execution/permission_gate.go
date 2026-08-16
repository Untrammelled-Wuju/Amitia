package execution

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type PermissionDecision string

const (
	PermissionAllow           PermissionDecision = "allow"
	PermissionDeny            PermissionDecision = "deny"
	PermissionRequireApproval PermissionDecision = "require_approval"
	PermissionAllowOnce       PermissionDecision = "allow_once"
	PermissionAllowPersistent PermissionDecision = "allow_persistent"
)

func NewPermissionGate() *PermissionGate {
	return &PermissionGate{}
}

type PermissionGate struct {
	OnEvaluate func(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision
	Broker     permission.PermissionBroker
}

func (g *PermissionGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
	if g.OnEvaluate != nil {
		return g.OnEvaluate(ctx, tool, inv)
	}
	if g.Broker == nil {
		return PermissionDeny
	}
	return g.evaluateWithBroker(ctx, tool, inv)
}

func (g *PermissionGate) evaluateWithBroker(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
	subject := permission.SubjectForTool(tool.ExtensionID, tool.ID)
	if tool.ExtensionID == "" {
		subject = permission.PermissionSubject{Type: permission.SubjectSystem, ID: "core"}
	}

	scope := permission.PermissionScope{}
	if inv.CharacterID != "" {
		scope = permission.ScopeForCharacter(inv.CharacterID)
	} else if inv.ConversationID != "" {
		scope = permission.ScopeForConversation(inv.ConversationID)
	} else {
		scope = permission.ScopeGlobalOnly()
	}

	requirements := permission.BuildRequirements(tool, scope)
	if len(requirements) == 0 {
		return PermissionAllow
	}

	request := permission.BuildEvaluationRequestFromInvocation(subject, requirements, inv, string(tool.RiskLevel))

	result := g.Broker.Evaluate(ctx, request)

	switch result.Decision {
	case permission.DecisionAllow:
		return PermissionAllow
	case permission.DecisionDeny:
		return PermissionDeny
	case permission.DecisionRequireApproval:
		return PermissionRequireApproval
	default:
		return PermissionDeny
	}
}
