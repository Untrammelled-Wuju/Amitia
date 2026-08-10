package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type RuntimeReplanResult struct {
	BehaviorPlan   *decision.BehaviorPlan
	ExpressionPlan *decision.ExpressionPlan
	Goals          RuntimeGoalContext
}

type Replanner interface {
	Replan(
		ctx context.Context,
		scope InteractionScope,
		previous RuntimeAssembly,
		req *ProcessRequest,
		continuation decision.ContinuationDecision,
	) (RuntimeReplanResult, error)
}

func (p *RuntimePipeline) Replan(
	ctx context.Context,
	scope InteractionScope,
	previous RuntimeAssembly,
	req *ProcessRequest,
	continuation decision.ContinuationDecision,
) (RuntimeReplanResult, error) {
	if p == nil {
		return RuntimeReplanResult{}, fmt.Errorf("runtime pipeline is nil")
	}

	now := time.Now().UTC()
	goalContext := p.buildGoalContext(scope, req, previous.Appraisal, now)

	if p.hasAchievedGoalsOnly(goalContext) && continuation.ReplanCount >= continuation.Iteration {
		return RuntimeReplanResult{Goals: goalContext}, nil
	}

	safetyDecision := previous.Safety

	var newBehaviorPlan *decision.BehaviorPlan
	var newExpressionPlan *decision.ExpressionPlan

	if p.candidateRegistry != nil {
		bp, ep, err := p.runDecision(ctx, scope, previous.Context, previous.Appraisal, previous.Personality, goalContext, now, safetyDecision)
		if err != nil {
			return RuntimeReplanResult{Goals: goalContext}, fmt.Errorf("replan decision failed: %w", err)
		}
		newBehaviorPlan = bp
		newExpressionPlan = ep
	}

	if newBehaviorPlan == nil {
		return RuntimeReplanResult{Goals: goalContext}, nil
	}

	return RuntimeReplanResult{
		BehaviorPlan:   newBehaviorPlan,
		ExpressionPlan: newExpressionPlan,
		Goals:          goalContext,
	}, nil
}

func (p *RuntimePipeline) hasAchievedGoalsOnly(ctx RuntimeGoalContext) bool {
	if len(ctx.Active) == 0 {
		return true
	}
	for _, goal := range ctx.Active {
		if goal.Status != decision.GoalStatusAchieved && goal.Status != decision.GoalStatusAbandoned {
			return false
		}
	}
	return true
}