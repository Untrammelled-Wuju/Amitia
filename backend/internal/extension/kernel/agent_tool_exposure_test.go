package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestAgentModelToolsNeverExposeInternalKernelTool(t *testing.T) {
	definitions := []capability.ToolDefinition{
		{ID: "internal-tool", ModelName: "kernel_internal", Internal: true, Enabled: true, InputSchema: []byte(`{"type":"object","properties":{}}`)},
		{ID: "public-tool", ModelName: "kernel_public", Enabled: true, InputSchema: []byte(`{"type":"object","properties":{}}`)},
	}
	visible := buildModelToolsFromDefinitions(definitions, InvocationScope{})
	if len(visible) != 1 || visible[0].Function.Name != "kernel_public" {
		t.Fatalf("internal kernel tools must never be exposed to a model, got %#v", visible)
	}
}
