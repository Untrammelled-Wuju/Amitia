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
	if g.ScopeManager != nil {
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

	if tool.Scope.Type == "" {
		return nil
	}
	if tool.Scope.Type == "global" {
		return nil
	}
	if tool.Scope.Type == "character" && inv.CharacterID != "" && tool.Scope.ID == inv.CharacterID {
		return nil
	}
	if tool.Scope.Type == "conversation" && tool.Scope.ID == inv.ConversationID {
		return nil
	}
	if tool.Internal {
		return nil
	}
	return fmt.Errorf("scope denied: tool requires scope %s/%s", tool.Scope.Type, tool.Scope.ID)
}
