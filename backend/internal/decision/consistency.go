package decision

import "fmt"

type ConsistencyResult struct {
	Consistent bool     `json:"consistent"`
	Violations []string `json:"violations,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type ConsistencyChecker struct{}

func DefaultConsistencyChecker() ConsistencyChecker {
	return ConsistencyChecker{}
}

func (c *ConsistencyChecker) Verify(plan BehaviorPlan) ConsistencyResult {
	result := ConsistencyResult{
		Consistent: true,
		Violations: make([]string, 0),
		Warnings:   make([]string, 0),
	}

	if plan.Version != PlanVersionV2 {
		result.Violations = append(result.Violations, fmt.Sprintf("invalid plan version: %s", plan.Version))
	}
	if plan.ID == "" {
		result.Violations = append(result.Violations, "empty plan ID")
	}
	if plan.Selected.ID == "" {
		result.Violations = append(result.Violations, "empty selected candidate ID")
	}
	if plan.Selected.ScoringVersion == "" {
		result.Violations = append(result.Violations, "missing selected scoring version")
	}
	if plan.Audit.FormulaVersion != "" && plan.Selected.ScoringVersion != plan.Audit.FormulaVersion {
		result.Violations = append(result.Violations,
			fmt.Sprintf("scoring version mismatch: selected=%s audit=%s",
				plan.Selected.ScoringVersion, plan.Audit.FormulaVersion))
	}
	if plan.Selected.FinalScore == 0 && plan.Selected.BaseScore > 0 {
		result.Warnings = append(result.Warnings, "final score is zero but base score > 0")
	}

	if plan.DoNotSend && plan.NeedsExpression {
		result.Violations = append(result.Violations, "do_not_send and needs_expression cannot both be true")
	}
	if plan.NeedsExpression && plan.ExpressionPlanID == "" {
		result.Violations = append(result.Violations, "needs_expression=true requires expression_plan_id")
	}
	if !plan.NeedsExpression && plan.ExpressionPlanID != "" {
		result.Violations = append(result.Violations, "needs_expression=false requires empty expression_plan_id")
	}

	if plan.SafetyLevel == BehaviorSafetyLevelBlocked {
		if !plan.DoNotSend {
			result.Violations = append(result.Violations, "safety_level=blocked requires do_not_send=true")
		}
		if plan.NeedsExpression {
			result.Violations = append(result.Violations, "safety_level=blocked requires needs_expression=false")
		}
	}

	if plan.Selected.Tag == BehaviorTagDelay && !plan.DoNotSend {
		result.Violations = append(result.Violations, "delay/observe candidate requires do_not_send=true")
	}
	if plan.Selected.ActionType == CandidateActionToolCall && plan.NeedsExpression {
		result.Violations = append(result.Violations, "tool_call candidate should not need expression")
	}

	if len(result.Violations) > 0 {
		result.Consistent = false
	}
	return result
}

func (c *ConsistencyChecker) VerifyAndReport(plan BehaviorPlan) (ConsistencyResult, error) {
	result := c.Verify(plan)
	if !result.Consistent {
		return result, fmt.Errorf("plan consistency check failed: %v", result.Violations)
	}
	return result, nil
}
