package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestToolFacadeExposesUIAgentAppearanceTool(t *testing.T) {
	registry := capability.NewToolRegistry()
	if err := builtin.RegisterUIAgentAppearanceTool(context.Background(), registry); err != nil {
		t.Fatalf("register appearance tool: %v", err)
	}
	facade := NewToolFacade(registry, nil)
	tools, err := facade.ModelTools(context.Background(), InvocationScope{})
	if err != nil {
		t.Fatalf("list model tools: %v", err)
	}
	for _, modelTool := range tools {
		if modelTool.Function.Name == builtin.UIAgentAppearanceToolName {
			return
		}
	}
	t.Fatalf("expected model tool %s", builtin.UIAgentAppearanceToolName)
}
