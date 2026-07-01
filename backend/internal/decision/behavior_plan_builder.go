package decision

import "time"

type BehaviorPlanBuilder struct {
	Now time.Time
}

func NewBehaviorPlanBuilder(now time.Time) BehaviorPlanBuilder {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return BehaviorPlanBuilder{Now: now}
}

func (b *BehaviorPlanBuilder) Build(candidate BehaviorCandidate, input ArbitrationInput) BehaviorPlan {
	now := b.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	plan := BehaviorPlan{
		Version:     PlanVersionV1,
		ID:          "plan-" + now.Format("20060102150405"),
		UserID:      "",
		CharacterID: "",
		CreatedAt:   now,
		Selected:    candidate,
		Priority:    derivePlanPriority(candidate),
		SafetyLevel: derivePlanSafety(candidate),
		Psyche:      input.Psyche,
		Relationship: input.Relationship,
		Life:        input.Life,
		Audit: BehaviorAudit{
			FormulaVersion: string(BehaviorFormulaVersionV1),
		},
	}
	if plan.Priority == BehaviorPriorityCritical || plan.Priority == BehaviorPriorityHigh {
		plan.NeedsExpression = true
	}
	return plan
}

func derivePlanPriority(candidate BehaviorCandidate) BehaviorPriority {
	if candidate.FinalScore >= 0.8 {
		return BehaviorPriorityCritical
	}
	if candidate.FinalScore >= 0.6 {
		return BehaviorPriorityHigh
	}
	if candidate.FinalScore >= 0.3 {
		return BehaviorPriorityNormal
	}
	return BehaviorPriorityLow
}

func derivePlanSafety(candidate BehaviorCandidate) BehaviorSafetyLevel {
	if candidate.RiskScore >= 0.8 {
		return BehaviorSafetyLevelBlocked
	}
	if candidate.RiskScore >= 0.5 {
		return BehaviorSafetyLevelConservative
	}
	return BehaviorSafetyLevelNormal
}
