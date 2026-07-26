package execution

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewAvailabilityGate(evaluator capability.AvailabilityEvaluator) *AvailabilityGate {
	return &AvailabilityGate{evaluator: evaluator}
}

type AvailabilityGate struct {
	evaluator capability.AvailabilityEvaluator
}

func (g *AvailabilityGate) Evaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) capability.AvailabilityResult {
	if g.evaluator != nil {
		return g.evaluator.Evaluate(ctx, tool, inv)
	}
	return capability.AvailabilityResult{Visible: true, Executable: true}
}
