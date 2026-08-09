package decision

func ApplyCandidateContextSignals(candidates []BehaviorCandidate, ctx CandidateGenerationContext) []BehaviorCandidate {
	result := make([]BehaviorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		next := candidate
		next = resetDynamicScores(next)
		next = enrichFromGoals(next, ctx.Goals, ctx.Intentions)
		next = enrichFromRelationship(next, ctx.Relationship)
		next = enrichFromPsyche(next, ctx.Psyche)
		next = enrichFromLife(next, ctx.Life)
		next = enrichFromPersonality(next, ctx.PersonalityWeights)
		result = append(result, next)
	}
	return result
}

func resetDynamicScores(candidate BehaviorCandidate) BehaviorCandidate {
	candidate.PersonalityScore = 0
	candidate.NeedScore = 0
	candidate.RelationshipScore = 0
	candidate.AffectScore = 0
	candidate.RiskScore = 0
	candidate.UserPreferenceScore = 0
	candidate.RepeatPenalty = 0
	candidate.FatiguePenalty = 0
	candidate.FinalScore = 0
	candidate.ScoringVersion = ""
	candidate.Reasons = stripScoringReasons(candidate.Reasons)
	candidate.Constraints = stripScoringConstraints(candidate.Constraints)
	return candidate
}

func stripScoringReasons(reasons []BehaviorReason) []BehaviorReason {
	if len(reasons) == 0 {
		return nil
	}
	result := make([]BehaviorReason, 0, len(reasons))
	for _, r := range reasons {
		if r.Source == "scoring" {
			continue
		}
		result = append(result, r)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func stripScoringConstraints(constraints []BehaviorConstraint) []BehaviorConstraint {
	if len(constraints) == 0 {
		return nil
	}
	result := make([]BehaviorConstraint, 0, len(constraints))
	for _, c := range constraints {
		if c.Kind == "stress_limit" || c.Kind == "busy_block" || c.Kind == "busy_interruption" {
			continue
		}
		result = append(result, c)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func enrichFromGoals(candidate BehaviorCandidate, goals []Goal, intentions []Intention) BehaviorCandidate {
	goalBoost := 0.0
	for _, goal := range goals {
		if goal.Status == GoalStatusActive || goal.Status == GoalStatusPending {
			boost := mapGoalToBoost(goal.Type, candidate.ID)
			if boost > goalBoost {
				goalBoost = boost
			}
		}
	}
	intentionBoost := 0.0
	for _, intention := range intentions {
		if intention.Status == IntentionStatusExecuting || intention.Status == IntentionStatusFormed {
			boost := mapIntentionToBoost(intention, candidate.ID)
			if boost > intentionBoost {
				intentionBoost = boost
			}
		}
	}
	candidate.NeedScore = round4(goalBoost + intentionBoost)
	return candidate
}

func enrichFromRelationship(candidate BehaviorCandidate, rel RelationshipSnapshot) BehaviorCandidate {
	if len(rel.Dimensions) == 0 {
		return candidate
	}
	trustVal := 0.5
	if v, ok := rel.Dimensions[RelationshipTrust]; ok {
		trustVal = v.Value
	}
	candidate.RelationshipScore = round4((trustVal - 0.5) * 0.2)
	return candidate
}

func enrichFromPsyche(candidate BehaviorCandidate, psyche PsycheSignalSet) BehaviorCandidate {
	stressVal := psyche.Stress.Value
	if stressVal > 0.7 {
		candidate.Constraints = appendDedupConstraint(candidate.Constraints, BehaviorConstraint{
			Kind:     "stress_limit",
			Limit:    0.8,
			Observed: stressVal,
			Hard:     stressVal > 0.9,
		})
	}
	if candidate.ID == "proactive_greet" && stressVal > 0.5 {
		candidate.RiskScore = round4(candidate.RiskScore + stressVal*0.3)
	}
	if candidate.ID == "express_emotion" && stressVal > 0.4 {
		candidate.AffectScore = round4(candidate.AffectScore + stressVal*0.15)
	}
	return candidate
}

func enrichFromLife(candidate BehaviorCandidate, life LifeSnapshot) BehaviorCandidate {
	if candidate.ID == "proactive_greet" && life.Busy > 0.7 {
		candidate.RiskScore = round4(candidate.RiskScore + life.Busy*0.4)
		if life.Busy >= 0.9 {
			candidate.Constraints = appendDedupConstraint(candidate.Constraints, BehaviorConstraint{
				Kind:     "busy_block",
				Limit:    0.9,
				Observed: life.Busy,
				Hard:     true,
			})
		}
	}
	if life.Energy < 0.3 {
		candidate.RiskScore = round4(candidate.RiskScore + (0.3-life.Energy)*0.5)
	}
	return candidate
}

func enrichFromPersonality(candidate BehaviorCandidate, weights map[BehaviorTag]float64) BehaviorCandidate {
	if weights == nil {
		return candidate
	}
	tag := candidate.Tag
	if tag == "" {
		return candidate
	}
	if w, ok := weights[tag]; ok {
		candidate.PersonalityScore = round4(w)
		candidate.Reasons = append(candidate.Reasons, BehaviorReason{
			Source: "personality",
			Key:    string(tag),
			Delta:  round4(w),
		})
	}
	return candidate
}

func appendDedupConstraint(constraints []BehaviorConstraint, next BehaviorConstraint) []BehaviorConstraint {
	for _, c := range constraints {
		if c.Kind == next.Kind && c.Hard == next.Hard {
			return constraints
		}
	}
	return append(constraints, next)
}

func mapGoalToBoost(goalType GoalType, candidateID string) float64 {
	switch goalType {
	case GoalTypeConnection:
		if candidateID == "chat_reply" || candidateID == "express_emotion" {
			return 0.25
		}
	case GoalTypeSupport:
		if candidateID == "offer_support" || candidateID == "chat_reply" {
			return 0.30
		}
	case GoalTypeConflictRepair:
		if candidateID == "offer_support" || candidateID == "chat_reply" {
			return 0.35
		}
	case GoalTypeClarification:
		if candidateID == "ask_clarify" {
			return 0.40
		}
	case GoalTypeAutonomy:
		if candidateID == "set_boundary" || candidateID == "wait_observe" {
			return 0.20
		}
	case GoalTypeInformation:
		if candidateID == "tool_search" {
			return 0.30
		}
	}
	return 0
}

func mapIntentionToBoost(intention Intention, candidateID string) float64 {
	baseBoost := mapGoalToBoost(intention.GoalType, candidateID)
	if baseBoost <= 0 {
		return 0
	}
	return round4(baseBoost * intention.CommitmentValue())
}
