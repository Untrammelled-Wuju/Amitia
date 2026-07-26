package capability

import (
	"context"
	"encoding/json"
)

type ToolExposureManager struct {
	registry         *ToolRegistry
	availabilityEval AvailabilityEvaluator
}

func NewToolExposureManager(registry *ToolRegistry, availabilityEval AvailabilityEvaluator) *ToolExposureManager {
	return &ToolExposureManager{
		registry:         registry,
		availabilityEval: availabilityEval,
	}
}

func (m *ToolExposureManager) GetModelTools(
	ctx context.Context,
	sessionCtx ToolInvocationContext,
	tokenBudget int,
) []ModelToolView {
	all := m.registry.List(ctx, ToolFilter{})
	views := make([]ModelToolView, 0, len(all))
	totalTokens := 0

	for _, tool := range all {
		if tool.Internal {
			continue
		}

		if m.availabilityEval != nil {
			avail := m.availabilityEval.Evaluate(ctx, tool, sessionCtx)
			if !avail.Visible {
				continue
			}
		}

		descTokens := len(tool.Description) / 4
		nameTokens := len(tool.ModelName) / 4
		schemaTokens := 0
		if tool.InputSchema != nil {
			schemaTokens = len(tool.InputSchema) / 4
		}
		estimated := descTokens + nameTokens + schemaTokens

		if tokenBudget > 0 && totalTokens+estimated > tokenBudget {
			continue
		}

		views = append(views, tool.ModelToolView())
		totalTokens += estimated
	}

	return views
}

type ProviderToolFormat string

const (
	ProviderFormatOpenAI ProviderToolFormat = "openai"
	ProviderFormatClaude ProviderToolFormat = "claude"
	ProviderFormatGemini ProviderToolFormat = "gemini"
)

type ProviderToolAdapter func(tool ModelToolView, format ProviderToolFormat) (json.RawMessage, error)
