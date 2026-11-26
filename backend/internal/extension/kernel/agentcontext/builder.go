//go:build rsi_agentcontext

package agentcontext

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type AgentContextMode string

const (
	AgentContextTask   AgentContextMode = "task"
	AgentContextLegacy AgentContextMode = "legacy"
)

var ErrAgentContextInvalid = errors.New("agent context request is invalid")

type AgentContextRequest struct {
	Session     capability.ToolInvocationContext
	Query       string
	Limit       int
	TokenBudget int
	Detail      bool
	Cursor      string
}

type AgentContext struct {
	Mode    AgentContextMode
	Tools   []capability.ModelToolView
	Tokens  int
	Reads   int64
	Trimmed int
	HasMore bool
	Cursor  string
}

func (c AgentContext) Names() []string {
	names := make([]string, 0, len(c.Tools))
	for _, tool := range c.Tools {
		names = append(names, tool.Name)
	}
	return names
}

type AgentContextBuilder struct {
	exposure *capability.ToolExposureManager
	budget   int
}

func NewAgentContextBuilder(exposure *capability.ToolExposureManager) (*AgentContextBuilder, error) {
	if exposure == nil {
		return nil, fmt.Errorf("%w: exposure manager is required", ErrAgentContextInvalid)
	}
	return &AgentContextBuilder{exposure: exposure, budget: capability.DefaultSchemaTokenBudget}, nil
}

func (b *AgentContextBuilder) WithTokenBudget(budget int) (*AgentContextBuilder, error) {
	if budget < 0 {
		return nil, fmt.Errorf("%w: token budget=%d", ErrAgentContextInvalid, budget)
	}
	if budget > 0 {
		b.budget = budget
	}
	return b, nil
}

func (b *AgentContextBuilder) Build(ctx context.Context, request AgentContextRequest) (AgentContext, error) {
	if b == nil || b.exposure == nil {
		return AgentContext{}, fmt.Errorf("%w: builder is not initialized", ErrAgentContextInvalid)
	}
	if err := ctx.Err(); err != nil {
		return AgentContext{}, err
	}
	if request.Limit < 0 {
		return AgentContext{}, fmt.Errorf("%w: limit=%d", ErrAgentContextInvalid, request.Limit)
	}
	if request.TokenBudget < 0 {
		return AgentContext{}, fmt.Errorf("%w: token budget=%d", ErrAgentContextInvalid, request.TokenBudget)
	}

	budget := request.TokenBudget
	if budget == 0 {
		budget = b.budget
	}

	if b.exposure.TaskSelector() == nil {
		views := b.exposure.GetModelTools(ctx, request.Session, budget)
		return assemble(AgentContextLegacy, views, 0, 0, false, ""), nil
	}

	if strings.TrimSpace(request.Query) == "" {
		return AgentContext{}, fmt.Errorf("%w: task mode requires a query", ErrAgentContextInvalid)
	}

	if !request.Detail {
		views, err := b.exposure.GetModelToolsForTask(ctx, request.Session, request.Query, budget, request.Limit)
		if err != nil {
			return AgentContext{}, fmt.Errorf("build agent context tools: %w", err)
		}
		return assemble(AgentContextTask, views, 0, 0, false, ""), nil
	}

	var (
		page capability.DetailPage
		err  error
	)
	page, err = b.exposure.GetModelToolDetails(ctx, request.Session, capability.DetailRequest{
		Query:  request.Query,
		Limit:  request.Limit,
		Budget: budget,
		Cursor: request.Cursor,
	})
	if err != nil {
		return AgentContext{}, fmt.Errorf("build agent context details: %w", err)
	}
	return assemble(AgentContextTask, page.Views, page.SchemaReads, page.Trimmed, page.HasMore, page.NextCursor), nil
}

func assemble(mode AgentContextMode, views []capability.ModelToolView, reads int64, trimmed int, hasMore bool, cursor string) AgentContext {
	tokens := 0
	for _, view := range views {
		tokens += (len(view.InputSchema)+3)/4 + len(view.Name)/4 + len(view.Description)/4
	}
	return AgentContext{
		Mode:    mode,
		Tools:   views,
		Tokens:  tokens,
		Reads:   reads,
		Trimmed: trimmed,
		HasMore: hasMore,
		Cursor:  cursor,
	}
}
