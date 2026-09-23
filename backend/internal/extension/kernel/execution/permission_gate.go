package execution

import (
	"context"
	"log"
	"strings"

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

type PermissionGateEvaluation struct {
	Decision PermissionDecision
	Subject  permission.PermissionSubject
	Reasons  []permission.PermissionReason
}

func (g *PermissionGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionDecision {
	return g.EvaluateDetailed(ctx, tool, inv).Decision
}

func (g *PermissionGate) EvaluateDetailed(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionGateEvaluation {
	if g.OnEvaluate != nil {
		return PermissionGateEvaluation{Decision: g.OnEvaluate(ctx, tool, inv)}
	}
	if g.Broker == nil {
		return PermissionGateEvaluation{Decision: PermissionDeny}
	}
	return g.evaluateWithBroker(ctx, tool, inv)
}

func (g *PermissionGate) evaluateWithBroker(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) PermissionGateEvaluation {
	subject := permission.SubjectForToolDefinition(tool)

	scope := permission.ScopeGlobalOnly()
	if tool.ExtensionID != "" && inv.CharacterID != "" {
		scope = permission.ScopeForCharacter(inv.CharacterID)
	} else if tool.ExtensionID != "" && inv.ConversationID != "" {
		scope = permission.ScopeForConversation(inv.ConversationID)
	}

	requirements := permission.BuildRequirements(tool, scope)
	if len(requirements) == 0 {
		return PermissionGateEvaluation{Decision: PermissionAllow, Subject: subject}
	}

	request := permission.BuildEvaluationRequestFromInvocation(subject, requirements, inv, string(tool.RiskLevel))

	result := g.Broker.Evaluate(ctx, request)
	if result.Decision != permission.DecisionAllow {
		reasons := make([]string, 0, len(result.Reasons))
		for _, reason := range result.Reasons {
			value := reason.Code
			if reason.Permission != "" {
				value += ":" + reason.Permission
			}
			reasons = append(reasons, value)
		}
		log.Printf("[permission-gate] tool=%s subject=%s/%s approval_mode=%s scope_snapshot=%s decision=%s reasons=%s", tool.ID, subject.Type, subject.ID, inv.ApprovalMode, inv.ScopeSnapshotID, result.Decision, strings.Join(reasons, ","))
	}

	switch result.Decision {
	case permission.DecisionAllow:
		return PermissionGateEvaluation{Decision: PermissionAllow, Subject: subject, Reasons: result.Reasons}
	case permission.DecisionDeny:
		return PermissionGateEvaluation{Decision: PermissionDeny, Subject: subject, Reasons: result.Reasons}
	case permission.DecisionRequireApproval:
		return PermissionGateEvaluation{Decision: PermissionRequireApproval, Subject: subject, Reasons: result.Reasons}
	default:
		return PermissionGateEvaluation{Decision: PermissionDeny, Subject: subject, Reasons: result.Reasons}
	}
}
