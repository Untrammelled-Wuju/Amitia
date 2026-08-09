package execution

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func NewScopeGate() *ScopeGate {
	return &ScopeGate{}
}

type ScopeGate struct {
	ScopeManager scope.ScopeManager
}

func (g *ScopeGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) error {
	if g.ScopeManager == nil {
		return fmt.Errorf("scope denied: scope manager not configured")
	}

	decision := g.ScopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType:    scope.SubjectTool,
		SubjectID:      tool.ID,
		CharacterID:    inv.CharacterID,
		ConversationID: inv.ConversationID,
		ExtensionID:    inv.ExtensionID,
		ModuleID:       inv.ModuleID,
		InvocationID:   inv.InvocationID,
		Generation:     inv.Generation,
	})
	if !decision.Allowed {
		return fmt.Errorf("scope denied: %v", decision.Reasons)
	}
	return nil
}
