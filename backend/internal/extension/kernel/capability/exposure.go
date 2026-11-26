package capability

import (
	"context"
	"encoding/json"
	"fmt"
)

type ToolExposureManager struct {
	registry         *ToolRegistry
	availabilityEval AvailabilityEvaluator
	taskSelector     *TaskSelector
	schemaIndex      *DescriptorIndex
	schemaStore      *SchemaStore
}

func NewToolExposureManager(registry *ToolRegistry, availabilityEval AvailabilityEvaluator) *ToolExposureManager {
	return &ToolExposureManager{
		registry:         registry,
		availabilityEval: availabilityEval,
	}
}

func (m *ToolExposureManager) WithTaskSelector(selector *TaskSelector) *ToolExposureManager {
	m.taskSelector = selector
	return m
}

func (m *ToolExposureManager) TaskSelector() *TaskSelector {
	return m.taskSelector
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

		estimated := estimateToolTokens(tool)
		if tokenBudget > 0 && totalTokens+estimated > tokenBudget {
			continue
		}

		views = append(views, tool.ModelToolView())
		totalTokens += estimated
	}

	return views
}

func (m *ToolExposureManager) GetModelToolsForTask(
	ctx context.Context,
	sessionCtx ToolInvocationContext,
	query string,
	tokenBudget int,
	limit int,
) ([]ModelToolView, error) {
	if m.taskSelector == nil {
		return m.GetModelTools(ctx, sessionCtx, tokenBudget), nil
	}
	report, err := m.taskSelector.Select(ctx, sessionCtx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("select tools for task: %w", err)
	}
	views := make([]ModelToolView, 0, len(report.Selected))
	totalTokens := 0
	for _, selected := range report.Selected {
		estimated := estimateToolTokens(selected.Tool)
		if tokenBudget > 0 && totalTokens+estimated > tokenBudget {
			continue
		}
		views = append(views, selected.Tool.ModelToolView())
		totalTokens += estimated
	}
	return views, nil
}

func (m *ToolExposureManager) SelectForTask(
	ctx context.Context,
	sessionCtx ToolInvocationContext,
	query string,
	limit int,
) (SelectionReport, error) {
	if m.taskSelector == nil {
		return SelectionReport{}, fmt.Errorf("%w: exposure manager has no task selector", ErrTaskSelectorInvalid)
	}
	return m.taskSelector.Select(ctx, sessionCtx, query, limit)
}

type ProviderToolFormat string

const (
	ProviderFormatOpenAI ProviderToolFormat = "openai"
	ProviderFormatClaude ProviderToolFormat = "claude"
	ProviderFormatGemini ProviderToolFormat = "gemini"
)

type ProviderToolAdapter func(tool ModelToolView, format ProviderToolFormat) (json.RawMessage, error)
