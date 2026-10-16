package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/interaction"
)

func (a *chatToolRuntimeAdapter) ConfigureMultiAgent(coordinator *interaction.MultiAgentCoordinator, tracker interaction.InteractionTracker, goals *decision.GoalRegistry) {
	a.multiAgent = &chatMultiAgentRuntime{coordinator: coordinator, tracker: tracker, goals: goals}
}

func (a *chatToolRuntimeAdapter) multiAgentAvailable(ctx context.Context, scope chat.SkillScope) bool {
	return a != nil && a.multiAgent.available(ctx, scope)
}

func (a *chatToolRuntimeAdapter) appendMultiAgentTools(ctx context.Context, existing []tool.Tool, scope chat.SkillScope) []tool.Tool {
	if a == nil {
		return existing
	}
	for _, def := range existing {
		if isChatMultiAgentTool(def.Function.Name) {
			return existing
		}
	}
	return append(existing, a.multiAgent.ModelTools(ctx, scope)...)
}

func (a *chatToolRuntimeAdapter) executeMultiAgentTool(ctx context.Context, name string, input json.RawMessage, scope chat.SkillScope) chat.ToolResult {
	if a == nil {
		return multiAgentToolFailure("MULTI_AGENT_NOT_AUTHORIZED", fmt.Errorf("multi-agent runtime is not available"))
	}
	return a.multiAgent.Execute(ctx, name, input, scope)
}
