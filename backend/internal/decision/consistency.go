package decision

type ConsistencyResult struct {
	Consistent     bool
	Score          float64
	Violations     []string
	FallbackAction string
}

type ConsistencyChecker struct {
	MinScore float64
}

func DefaultConsistencyChecker() ConsistencyChecker {
	return ConsistencyChecker{
		MinScore: 0.40,
	}
}

func (c *ConsistencyChecker) Verify(plan BehaviorPlan, goals []Goal, intentions []Intention) ConsistencyResult {
	result := ConsistencyResult{
		Consistent:     true,
		Score:          1.0,
		Violations:     make([]string, 0),
		FallbackAction: "wait_observe",
	}
	goalScore := checkGoalConsistency(plan.Selected, goals)
	if goalScore < 0.5 {
		result.Violations = append(result.Violations, "goal_mismatch:"+plan.Selected.ID)
	}
	intentScore := checkIntentionConsistency(plan.Selected, intentions)
	if intentScore < 0.5 {
		result.Violations = append(result.Violations, "intention_mismatch:"+plan.Selected.ID)
	}
	safetyScore := checkSafetyConsistency(plan)
	if safetyScore < 0.5 {
		result.Violations = append(result.Violations, "safety_violation:"+plan.Selected.ID)
	}
	result.Score = (goalScore + intentScore + safetyScore) / 3.0
	if result.Score < c.MinScore {
		result.Consistent = false
	}
	return result
}

func (c *ConsistencyChecker) VerifyWithFallback(plan BehaviorPlan, goals []Goal, intentions []Intention) BehaviorPlan {
	result := c.Verify(plan, goals, intentions)
	if result.Consistent {
		return plan
	}
	fallback := BehaviorPlan{
		Version:         PlanVersionV1,
		ID:              "plan-fallback-" + plan.CreatedAt.Format("20060102150405"),
		CreatedAt:       plan.CreatedAt,
		Selected:        buildFallbackCandidate(),
		Priority:        BehaviorPriorityLow,
		SafetyLevel:     BehaviorSafetyLevelConservative,
		DoNotSend:       false,
		Intent:          "正常回复",
		Strategy:        "保持自然沟通",
		AllowedTopics:   []string{"日常对话"},
		ForbiddenTopics: []string{"不适当内容"},
		ResponseGoal:    "完成本轮交互",
		ToneHint:        "中性",
		Audit: BehaviorAudit{
			FormulaVersion: string(BehaviorFormulaVersionV1),
			Diagnostics:    result.Violations,
			SnapshotID:     "consistency-fallback",
		},
	}
	return fallback
}

func checkGoalConsistency(candidate BehaviorCandidate, goals []Goal) float64 {
	if len(goals) == 0 {
		return 0.5
	}
	matched := 0
	for _, goal := range goals {
		if goal.Status == GoalStatusActive || goal.Status == GoalStatusPending {
			boost := mapGoalToBoost(goal.Type, candidate.ID)
			if boost > 0 {
				matched++
			}
		}
	}
	activeGoals := 0
	for _, goal := range goals {
		if goal.Status == GoalStatusActive || goal.Status == GoalStatusPending {
			activeGoals++
		}
	}
	if activeGoals == 0 {
		return 0.5
	}
	return float64(matched) / float64(activeGoals)
}

func checkIntentionConsistency(candidate BehaviorCandidate, intentions []Intention) float64 {
	if len(intentions) == 0 {
		return 0.5
	}
	matched := 0
	active := 0
	for _, intent := range intentions {
		if intent.Status == IntentionStatusFormed || intent.Status == IntentionStatusExecuting {
			active++
			boost := mapIntentionToBoost(intent, candidate.ID)
			if boost > 0 {
				matched++
			}
		}
	}
	if active == 0 {
		return 0.5
	}
	return float64(matched) / float64(active)
}

func checkSafetyConsistency(plan BehaviorPlan) float64 {
	if plan.DoNotSend {
		return 1.0
	}
	if plan.SafetyLevel == BehaviorSafetyLevelBlocked {
		return 0.0
	}
	if plan.SafetyLevel == BehaviorSafetyLevelConservative && plan.Selected.RiskScore > 0.7 {
		return 0.3
	}
	if plan.Selected.RiskScore > 0.9 {
		return 0.1
	}
	if plan.Selected.RiskScore > 0.6 {
		return 0.6
	}
	return 1.0
}
