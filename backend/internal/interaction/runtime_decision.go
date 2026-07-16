package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/personality"
)

func (p *RuntimePipeline) runDecision(ctx context.Context, scope InteractionScope, snapshot ContextSnapshot, appraisal *AppraisalResult, compiledPersonality *personality.CompiledPersonality) (*decision.BehaviorPlan, *decision.ExpressionPlan) {
	if p.candidateRegistry == nil {
		return nil, nil
	}
	now := time.Now()
	var psycheSignals decision.PsycheSignalSet
	if snapshot.Psyche.Status == LoadStatusReady {
		psycheSignals = decision.PsycheSignalSet{Mood: decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodPressure}, Stress: decision.ScalarSignal{Value: snapshot.Psyche.Value.Stress}, CognitiveLoad: decision.ScalarSignal{Value: snapshot.Psyche.Value.Fatigue}, Valence: decision.ScalarSignal{Value: snapshot.Psyche.Value.Valence}, Arousal: decision.ScalarSignal{Value: snapshot.Psyche.Value.Arousal}, Dominance: decision.ScalarSignal{Value: snapshot.Psyche.Value.Dominance}, MoodValence: decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodValence}, MoodArousal: decision.ScalarSignal{Value: snapshot.Psyche.Value.MoodArousal}}
	} else {
		psycheSignals = decision.PsycheSignalSet{Mood: decision.ScalarSignal{Value: 0.5}, Stress: decision.ScalarSignal{Value: 0.0}}
	}
	personalityWeights, personalityStyle := derivePersonalityWeights(compiledPersonality), derivePersonalityStyle(compiledPersonality)
	var relSnapshot decision.RelationshipSnapshot
	if snapshot.Relationship.Status == LoadStatusReady {
		relSnapshot = decision.RelationshipSnapshot{UserID: scope.UserID, CharacterID: scope.CharacterID, Dimensions: map[decision.RelationshipDimension]decision.RelationshipDimensionValue{decision.RelationshipTrust: {Value: snapshot.Relationship.Value.Trust}, decision.RelationshipFamiliarity: {Value: snapshot.Relationship.Value.Familiarity}, decision.RelationshipSafety: {Value: snapshot.Relationship.Value.Security}}}
	}
	var lifeSnapshot decision.LifeSnapshot
	if snapshot.Life.Status == LoadStatusReady {
		lifeSnapshot = decision.LifeSnapshot{Energy: snapshot.Life.Value.Energy, Busy: 0}
		if snapshot.Life.Value.Busy {
			lifeSnapshot.Busy = 0.8
		}
	} else {
		lifeSnapshot = decision.LifeSnapshot{Energy: 0.7}
	}
	ctx_ := decision.CandidateGenerationContext{UserID: scope.UserID, CharacterID: scope.CharacterID, Psyche: psycheSignals, Relationship: relSnapshot, Life: lifeSnapshot, PersonalityWeights: personalityWeights, Now: now}
	candidates := decision.GenerateCandidates(ctx_, p.candidateRegistry)
	arbitrationInput := decision.ArbitrationInput{Candidates: candidates, Relationship: relSnapshot, Psyche: psycheSignals, Life: lifeSnapshot, Filter: decision.DefaultHardConstraintFilter(), Now: now}
	arbitrationResult := p.arbitrationLayer.Arbitrate(arbitrationInput)
	builder := decision.NewBehaviorPlanBuilder(now)
	plan := builder.Build(arbitrationResult.Selected, arbitrationInput)
	plan.CharacterID, plan.UserID = scope.CharacterID, scope.UserID
	if compiledPersonality != nil {
		plan.Personality = decision.CompiledPersonalityRef{Version: compiledPersonality.Version, SourceCharacterID: compiledPersonality.CharacterID, BehaviorWeights: personalityWeights}
	}
	emotionIntensity := 0.5
	if compiledPersonality != nil {
		if v, ok := compiledPersonality.ExpressionStyle["emotionalExpression"]; ok {
			emotionIntensity = v
		}
	}
	exprCtrl := decision.ExpressionControlInput{EmotionIntensity: emotionIntensity, StressLevel: psycheSignals.Stress.Value, RelationshipSafety: 0.5}
	if relSnapshot.Dimensions != nil {
		if v, ok := relSnapshot.Dimensions[decision.RelationshipSafety]; ok {
			exprCtrl.RelationshipSafety = v.Value
		}
	}
	exprInput := decision.ExpressionPlanInput{BehaviorPlan: plan, Psyche: psycheSignals, ExpressionCtrl: exprCtrl, PersonalityExpressionStyle: personalityStyle, Now: now}
	exprPlan := decision.GenerateExpressionPlan(exprInput)
	return &plan, &exprPlan
}

func derivePersonalityWeights(cp *personality.CompiledPersonality) map[decision.BehaviorTag]float64 {
	if cp == nil {
		return nil
	}
	b := cp.BehaviorBias
	weights := make(map[decision.BehaviorTag]float64)
	mapBiasToTag := func(tag decision.BehaviorTag, key string, factor float64) {
		if value, ok := b[key]; ok {
			weights[tag] = (value - 0.5) * factor
		}
	}
	mapBiasToTag(decision.BehaviorTagReply, "warmth", 0.3)
	mapBiasToTag(decision.BehaviorTagReply, "initiative", 0.2)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "warmth", 0.4)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "companionship", 0.3)
	mapBiasToTag(decision.BehaviorTagOfferSupport, "affection", 0.2)
	mapBiasToTag(decision.BehaviorTagAskClarify, "clarification", 0.6)
	mapBiasToTag(decision.BehaviorTagAskClarify, "directness", 0.2)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "boundary", 0.5)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "conflictAvoidance", -0.3)
	mapBiasToTag(decision.BehaviorTagSetBoundary, "directness", 0.3)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "initiative", 0.5)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "warmth", 0.3)
	mapBiasToTag(decision.BehaviorTagProactiveCheck, "companionship", 0.2)
	mapBiasToTag(decision.BehaviorTagDelay, "initiative", -0.4)
	mapBiasToTag(decision.BehaviorTagDelay, "patience", 0.3)
	mapBiasToTag(decision.BehaviorTagRepair, "affection", 0.4)
	mapBiasToTag(decision.BehaviorTagRepair, "warmth", 0.3)
	return weights
}

func derivePersonalityStyle(cp *personality.CompiledPersonality) map[string]float64 {
	if cp == nil || cp.ExpressionStyle == nil {
		return nil
	}
	result := make(map[string]float64, len(cp.ExpressionStyle))
	for key, value := range cp.ExpressionStyle {
		result[key] = value
	}
	return result
}
