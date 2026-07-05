package decision

import (
	"time"
)

type CandidateGenerationContext struct {
	UserID            string
	CharacterID       string
	Goals             []Goal
	Intentions        []Intention
	Psyche            PsycheSignalSet
	Relationship      RelationshipSnapshot
	Life              LifeSnapshot
	Beliefs           BeliefSnapshot
	PersonalityWeights map[BehaviorTag]float64
	Now               time.Time
}

func GenerateCandidates(ctx CandidateGenerationContext, registry *CandidateRegistry) []BehaviorCandidate {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	defs := registry.All()
	candidates := make([]BehaviorCandidate, 0, len(defs))
	for _, def := range defs {
		candidate := behaviorFromActionDef(def)
		candidate = enrichFromGoals(candidate, ctx.Goals, ctx.Intentions)
		candidate = enrichFromRelationship(candidate, ctx.Relationship)
		candidate = enrichFromPsyche(candidate, ctx.Psyche)
		candidate = enrichFromLife(candidate, ctx.Life)
		candidate = enrichFromPersonality(candidate, ctx.PersonalityWeights)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func GenerateCandidatesWithExcludes(ctx CandidateGenerationContext, registry *CandidateRegistry, excludes []string) []BehaviorCandidate {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	defs := registry.AllExcept(excludes)
	candidates := make([]BehaviorCandidate, 0, len(defs))
	for _, def := range defs {
		candidate := behaviorFromActionDef(def)
		candidate = enrichFromGoals(candidate, ctx.Goals, ctx.Intentions)
		candidate = enrichFromRelationship(candidate, ctx.Relationship)
		candidate = enrichFromPsyche(candidate, ctx.Psyche)
		candidate = enrichFromLife(candidate, ctx.Life)
		candidate = enrichFromPersonality(candidate, ctx.PersonalityWeights)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func behaviorFromActionDef(def CandidateActionDef) BehaviorCandidate {
	return BehaviorCandidate{
		ID:        def.ID,
		BaseScore: def.BaseScore,
	}
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
	candidate.NeedScore = round4(candidate.NeedScore + goalBoost + intentionBoost)
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
	candidate.RelationshipScore = round4(candidate.RelationshipScore + (trustVal-0.5)*0.2)
	return candidate
}

func enrichFromPsyche(candidate BehaviorCandidate, psyche PsycheSignalSet) BehaviorCandidate {
	stressVal := psyche.Stress.Value
	if stressVal > 0.7 {
		candidate.Constraints = append(candidate.Constraints, BehaviorConstraint{
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
		candidate.BaseScore = round4(candidate.BaseScore + stressVal*0.15)
	}
	return candidate
}

func enrichFromLife(candidate BehaviorCandidate, life LifeSnapshot) BehaviorCandidate {
	if candidate.ID == "proactive_greet" && life.Busy > 0.7 {
		candidate.RiskScore = round4(candidate.RiskScore + life.Busy*0.4)
		if life.Busy >= 0.9 {
			candidate.Constraints = append(candidate.Constraints, BehaviorConstraint{
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
		candidate.PersonalityScore = round4(candidate.PersonalityScore + w)
		candidate.Reasons = append(candidate.Reasons, BehaviorReason{
			Source: "personality",
			Key:    string(tag),
			Delta:  round4(w),
		})
	}
	return candidate
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
