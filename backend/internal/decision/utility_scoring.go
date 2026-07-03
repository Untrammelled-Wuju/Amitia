package decision

import (
	"sort"
)

type UtilityObjective string

const (
	UtilityGoalAlignment       UtilityObjective = "goal_alignment"
	UtilityIntentionCommitment UtilityObjective = "intention_commitment"
	UtilityRelationshipHarmony UtilityObjective = "relationship_harmony"
	UtilityEmotionalBalance    UtilityObjective = "emotional_balance"
	UtilitySafetyCompliance    UtilityObjective = "safety_compliance"
	UtilityLifeFit             UtilityObjective = "life_fit"
)

type UtilityWeightConfig struct {
	GoalAlignment       float64 `json:"goalAlignment"`
	IntentionCommitment float64 `json:"intentionCommitment"`
	RelationshipHarmony float64 `json:"relationshipHarmony"`
	EmotionalBalance    float64 `json:"emotionalBalance"`
	SafetyCompliance    float64 `json:"safetyCompliance"`
	LifeFit             float64 `json:"lifeFit"`
}

func DefaultUtilityWeightConfig() UtilityWeightConfig {
	return UtilityWeightConfig{
		GoalAlignment:       1.0,
		IntentionCommitment: 0.8,
		RelationshipHarmony: 0.7,
		EmotionalBalance:    0.6,
		SafetyCompliance:    1.2,
		LifeFit:             0.5,
	}
}

type ObjectiveScore struct {
	Objective UtilityObjective `json:"objective"`
	Score     float64          `json:"score"`
	Weight    float64          `json:"weight"`
	Weighted  float64          `json:"weighted"`
}

type MultiObjectiveResult struct {
	CandidateID string           `json:"candidateId"`
	Scores      []ObjectiveScore `json:"scores"`
	Composite   float64          `json:"composite"`
}

type UtilityScoringContext struct {
	Goals        []Goal
	Intentions   []Intention
	Relationship RelationshipSnapshot
	Psyche       PsycheSignalSet
	Life         LifeSnapshot
}

func ScoreWithMultiObjective(candidates []BehaviorCandidate, ctx UtilityScoringContext, weights UtilityWeightConfig) []MultiObjectiveResult {
	results := make([]MultiObjectiveResult, 0, len(candidates))
	for _, candidate := range candidates {
		result := MultiObjectiveResult{
			CandidateID: candidate.ID,
			Scores:      make([]ObjectiveScore, 0, 6),
		}

		goalScore := computeGoalAlignment(candidate, ctx.Goals)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilityGoalAlignment,
			Score:     goalScore,
			Weight:    weights.GoalAlignment,
			Weighted:  round4(goalScore * weights.GoalAlignment),
		})

		intentScore := computeIntentionCommitment(candidate, ctx.Intentions)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilityIntentionCommitment,
			Score:     intentScore,
			Weight:    weights.IntentionCommitment,
			Weighted:  round4(intentScore * weights.IntentionCommitment),
		})

		relScore := computeRelationshipHarmony(candidate, ctx.Relationship)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilityRelationshipHarmony,
			Score:     relScore,
			Weight:    weights.RelationshipHarmony,
			Weighted:  round4(relScore * weights.RelationshipHarmony),
		})

		emoScore := computeEmotionalBalance(candidate, ctx.Psyche)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilityEmotionalBalance,
			Score:     emoScore,
			Weight:    weights.EmotionalBalance,
			Weighted:  round4(emoScore * weights.EmotionalBalance),
		})

		safetyScore := computeSafetyCompliance(candidate)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilitySafetyCompliance,
			Score:     safetyScore,
			Weight:    weights.SafetyCompliance,
			Weighted:  round4(safetyScore * weights.SafetyCompliance),
		})

		lifeScore := computeLifeFit(candidate, ctx.Life)
		result.Scores = append(result.Scores, ObjectiveScore{
			Objective: UtilityLifeFit,
			Score:     lifeScore,
			Weight:    weights.LifeFit,
			Weighted:  round4(lifeScore * weights.LifeFit),
		})

		composite := 0.0
		for _, s := range result.Scores {
			composite += s.Weighted
		}
		result.Composite = round4(composite)

		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Composite > results[j].Composite
	})

	return results
}

func computeGoalAlignment(candidate BehaviorCandidate, goals []Goal) float64 {
	if len(goals) == 0 {
		return 0.5
	}
	totalAlignment := 0.0
	activeCount := 0
	for _, goal := range goals {
		if goal.Status != GoalStatusActive && goal.Status != GoalStatusPending {
			continue
		}
		activeCount++
		alignment := mapGoalToBoost(goal.Type, candidate.ID)
		priorityMult := priorityMultiplier(goal.Priority)
		totalAlignment += alignment * priorityMult
	}
	if activeCount == 0 {
		return 0.3
	}
	avg := totalAlignment / float64(activeCount)
	return clamp01Val(avg)
}

func computeIntentionCommitment(candidate BehaviorCandidate, intentions []Intention) float64 {
	if len(intentions) == 0 {
		return 0.3
	}
	best := 0.0
	for _, intention := range intentions {
		if intention.Status != IntentionStatusFormed && intention.Status != IntentionStatusExecuting {
			continue
		}
		boost := mapIntentionToBoost(intention, candidate.ID)
		if boost > best {
			best = boost
		}
	}
	return clamp01Val(best)
}

func computeRelationshipHarmony(candidate BehaviorCandidate, rel RelationshipSnapshot) float64 {
	trustVal := 0.5
	if v, ok := rel.Dimensions[RelationshipTrust]; ok {
		trustVal = v.Value
	}
	conflictVal := 0.0
	if v, ok := rel.Dimensions[RelationshipConflict]; ok {
		conflictVal = v.Value
	}
	harmony := trustVal * (1.0 - conflictVal)
	if candidate.ID == "chat_reply" || candidate.ID == "express_emotion" {
		harmony += 0.1
	}
	if candidate.ID == "set_boundary" && trustVal < 0.3 {
		harmony *= 0.5
	}
	return clamp01Val(harmony)
}

func computeEmotionalBalance(candidate BehaviorCandidate, psyche PsycheSignalSet) float64 {
	stressVal := psyche.Stress.Value
	balance := 1.0 - stressVal*0.5
	if candidate.ID == "express_emotion" {
		balance += stressVal * 0.2
	}
	return clamp01Val(balance)
}

func computeSafetyCompliance(candidate BehaviorCandidate) float64 {
	if candidate.ID == "set_boundary" {
		return 0.95
	}
	if candidate.ID == "chat_reply" || candidate.ID == "offer_support" {
		return 0.85
	}
	if candidate.ID == "tool_search" {
		return 0.70
	}
	return 0.80
}

func computeLifeFit(candidate BehaviorCandidate, life LifeSnapshot) float64 {
	fit := 1.0
	if life.Busy > 0.7 && (candidate.ID == "proactive_greet" || candidate.ID == "tool_search") {
		fit -= life.Busy * 0.5
	}
	if life.Energy < 0.3 {
		fit -= (0.3 - life.Energy) * 0.6
	}
	return clamp01Val(fit)
}

func priorityMultiplier(p GoalPriority) float64 {
	switch p {
	case GoalPriorityCritical:
		return 1.0
	case GoalPriorityHigh:
		return 0.8
	case GoalPriorityNormal:
		return 0.5
	case GoalPriorityLow:
		return 0.2
	default:
		return 0.3
	}
}

func clamp01Val(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
