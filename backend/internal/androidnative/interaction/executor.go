package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/androidnative/uitree"
)

type Executor interface {
	Execute(
		ctx context.Context,
		plan InteractionPlan,
	) (InteractionResult, error)
}

type AccessibilityExecutor interface {
	PerformNodeAction(
		ctx context.Context,
		node ResolvedUINode,
		action string,
		args map[string]any,
	) error

	SupportsAction(
		node ResolvedUINode,
		action string,
	) bool
}

type CoordinateExecutor interface {
	Tap(
		ctx context.Context,
		displayID int,
		x int,
		y int,
	) error

	LongPress(
		ctx context.Context,
		displayID int,
		x int,
		y int,
		duration time.Duration,
	) error

	Swipe(
		ctx context.Context,
		request SwipeRequest,
	) error
}

type RootInteractionExecutor interface {
	Tap(
		ctx context.Context,
		x int,
		y int,
	) error

	Swipe(
		ctx context.Context,
		startX, startY, endX, endY int,
		durationMS int,
	) error

	InputText(
		ctx context.Context,
		text string,
	) error
}

type ADBInteractionExecutor interface {
	Tap(
		ctx context.Context,
		x int,
		y int,
	) error

	Swipe(
		ctx context.Context,
		startX, startY, endX, endY int,
		durationMS int,
	) error

	InputText(
		ctx context.Context,
		text string,
	) error
}

type ResolvedUINode = uitree.ResolvedUINode

type ExecutionOutcome int

const (
	OutcomeSuccess ExecutionOutcome = iota
	OutcomeUnsupported
	OutcomeDefinitelyFailed
	OutcomeUnknown
)

type StrategyResult struct {
	Outcome  ExecutionOutcome
	Result   InteractionResult
	Error    error
}

func (s StrategyResult) CanFallback() bool {
	return s.Outcome == OutcomeUnsupported || s.Outcome == OutcomeDefinitelyFailed
}

func (s StrategyResult) IsUnknown() bool {
	return s.Outcome == OutcomeUnknown
}

func NewSuccessResult(result InteractionResult) StrategyResult {
	return StrategyResult{
		Outcome: OutcomeSuccess,
		Result:  result,
	}
}

func NewUnsupportedResult() StrategyResult {
	return StrategyResult{
		Outcome: OutcomeUnsupported,
	}
}

func NewFailureResult(err error) StrategyResult {
	return StrategyResult{
		Outcome: OutcomeDefinitelyFailed,
		Error:   err,
	}
}

func NewUnknownResult() StrategyResult {
	return StrategyResult{
		Outcome: OutcomeUnknown,
	}
}
