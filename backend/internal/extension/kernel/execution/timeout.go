package execution

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type TimeoutSource string

const (
	TimeoutSourceCaller        TimeoutSource = "caller_deadline"
	TimeoutSourceInvocation    TimeoutSource = "invocation_expiry"
	TimeoutSourceToolPolicy    TimeoutSource = "tool_policy"
	TimeoutSourceKernelDefault TimeoutSource = "kernel_default"
)

var ErrToolDeadlineExceeded = errors.New("tool invocation deadline exceeded")

type TimeoutBudget struct {
	AcceptedAt        time.Time
	Deadline          time.Time
	Source            TimeoutSource
	ConfiguredTimeout time.Duration
}

func (b TimeoutBudget) Remaining(now time.Time) time.Duration {
	return b.Deadline.Sub(now)
}

func (b TimeoutBudget) Expired(now time.Time) bool {
	return !now.Before(b.Deadline)
}

type AttemptBudget struct {
	Attempt          int
	StartedAt        time.Time
	TotalDeadline    time.Time
	RemainingAtStart time.Duration
}

type TimeoutPhase string

const (
	TimeoutPhasePreDispatch  TimeoutPhase = "pre_dispatch"
	TimeoutPhaseRuntime      TimeoutPhase = "runtime"
	TimeoutPhaseRetryBackoff TimeoutPhase = "retry_backoff"
	TimeoutPhaseStream       TimeoutPhase = "stream"
)

const canonicalTimeoutSourcePriority = 4

func resolveTimeoutSourceRank(source TimeoutSource) int {
	switch source {
	case TimeoutSourceCaller:
		return 0
	case TimeoutSourceInvocation:
		return 1
	case TimeoutSourceToolPolicy:
		return 2
	case TimeoutSourceKernelDefault:
		return 3
	}
	return canonicalTimeoutSourcePriority
}

func NewTimeoutController(defaultTimeout time.Duration) *TimeoutController {
	return &TimeoutController{
		DefaultTimeout: defaultTimeout,
		now:            func() time.Time { return time.Now().UTC() },
	}
}

type TimeoutController struct {
	DefaultTimeout time.Duration
	now            func() time.Time
}

func (c *TimeoutController) Now() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now().UTC()
}

func (c *TimeoutController) ResolveBudget(ctx context.Context, acceptedAt time.Time, inv capability.ToolInvocationContext, tool capability.ToolDefinition) (TimeoutBudget, error) {
	type candidate struct {
		deadline time.Time
		source   TimeoutSource
	}

	var candidates []candidate

	if dl, ok := ctx.Deadline(); ok {
		candidates = append(candidates, candidate{deadline: dl, source: TimeoutSourceCaller})
	}

	if !inv.ExpiresAt.IsZero() {
		candidates = append(candidates, candidate{deadline: inv.ExpiresAt, source: TimeoutSourceInvocation})
	}

	if tool.TimeoutMS > 0 {
		deadline := acceptedAt.Add(time.Duration(tool.TimeoutMS) * time.Millisecond)
		candidates = append(candidates, candidate{deadline: deadline, source: TimeoutSourceToolPolicy})
	} else {
		if c.DefaultTimeout <= 0 {
			return TimeoutBudget{}, errors.New("kernel timeout default must be positive")
		}
		deadline := acceptedAt.Add(c.DefaultTimeout)
		candidates = append(candidates, candidate{deadline: deadline, source: TimeoutSourceKernelDefault})
	}

	if len(candidates) == 0 {
		return TimeoutBudget{}, errors.New("no timeout candidate available")
	}

	bestIdx := 0
	for i := 1; i < len(candidates); i++ {
		if candidates[i].deadline.Before(candidates[bestIdx].deadline) {
			bestIdx = i
		} else if candidates[i].deadline.Equal(candidates[bestIdx].deadline) {
			if resolveTimeoutSourceRank(candidates[i].source) < resolveTimeoutSourceRank(candidates[bestIdx].source) {
				bestIdx = i
			}
		}
	}

	return TimeoutBudget{
		AcceptedAt:        acceptedAt,
		Deadline:          candidates[bestIdx].deadline,
		Source:            candidates[bestIdx].source,
		ConfiguredTimeout: candidates[bestIdx].deadline.Sub(acceptedAt),
	}, nil
}

func (c *TimeoutController) WithTimeout(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (context.Context, context.CancelFunc, error) {
	now := c.Now()
	budget, err := c.ResolveBudget(ctx, now, inv, tool)
	if err != nil {
		return ctx, func() {}, err
	}
	if budget.Expired(now) {
		return ctx, func() {}, ErrToolDeadlineExceeded
	}
	return c.Wrap(ctx, budget)
}

func (c *TimeoutController) Wrap(ctx context.Context, budget TimeoutBudget) (context.Context, context.CancelFunc, error) {
	if budget.Deadline.IsZero() {
		return ctx, func() {}, errors.New("cannot wrap timeout context without deadline")
	}
	wrapped, cancel := context.WithDeadlineCause(ctx, budget.Deadline, ErrToolDeadlineExceeded)
	return wrapped, cancel, nil
}

func (c *TimeoutController) NewAttemptBudget(budget TimeoutBudget, attempt int) AttemptBudget {
	now := c.Now()
	remaining := budget.Remaining(now)
	if remaining < 0 {
		remaining = 0
	}
	return AttemptBudget{
		Attempt:          attempt,
		StartedAt:        now,
		TotalDeadline:    budget.Deadline,
		RemainingAtStart: remaining,
	}
}
