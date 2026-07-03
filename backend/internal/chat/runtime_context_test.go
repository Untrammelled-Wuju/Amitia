package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

func TestBuildRuntimeContextPromptIncludesPipelineDecisions(t *testing.T) {
	prompt := buildRuntimeContextPrompt(&interaction.RuntimeAssembly{
		Version: "orchestrator-runtime-v1",
		Path:    interaction.PathTypeDeep,
		Safety: interaction.RuntimeSafetyDecision{
			Level:   "conservative",
			Reasons: []string{"high_stress"},
		},
		Delivery: interaction.RuntimeDeliveryIntent{
			Channel:      "web",
			RequiresText: true,
		},
		Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
		Budget: []interaction.TokenBudgetPlan{
			{Module: interaction.TokenBudgetModule{Name: "memories", Tokens: 420, Priority: interaction.BudgetPriorityLowAuthority}, Allocated: 200, Trimmed: true},
		},
		Context: interaction.ContextSnapshot{
			Version:     "context-snapshot-v1",
			AssembledAt: time.Now(),
			Psyche: interaction.FieldReady(interaction.PsycheState{
				Stress: 0.9,
			}, "psyche", "v1"),
			Channel: interaction.FieldReady(interaction.ChannelCapabilities{
				Channel:      "web",
				SupportsText: true,
			}, "channel", "v1"),
		},
	})
	if !strings.Contains(prompt, "运行时编排上下文") {
		t.Fatalf("missing runtime prompt prefix: %s", prompt)
	}
	if !strings.Contains(prompt, `"path":"deep"`) {
		t.Fatalf("missing path: %s", prompt)
	}
	if !strings.Contains(prompt, `"transaction":"all"`) {
		t.Fatalf("missing transaction boundary: %s", prompt)
	}
	if !strings.Contains(prompt, `"budget"`) || !strings.Contains(prompt, `"Allocated":200`) {
		t.Fatalf("missing runtime budget: %s", prompt)
	}
	if !strings.Contains(prompt, `"level":"conservative"`) {
		t.Fatalf("missing safety level: %s", prompt)
	}
}
