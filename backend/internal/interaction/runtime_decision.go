package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/personality"
)

func (p *RuntimePipeline) runDecision(ctx context.Context, scope InteractionScope, snapshot ContextSnapshot, appraisal *AppraisalResult, compiledPersonality *personality.CompiledPersonality, goalContext RuntimeGoalContext, now time.Time, safetyDecision RuntimeSafetyDecision) (*decision.BehaviorPlan, *decision.ExpressionPlan, error) {
	if p.candidateRegistry == nil {
		return nil, nil, nil
	}
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

	decisionCtx := decision.CandidateGenerationContext{
		UserID:             scope.UserID,
		CharacterID:        scope.CharacterID,
		Goals:              goalsForDecision(goalContext),
		Intentions:         append([]decision.Intention(nil), goalContext.Intentions...),
		Psyche:             psycheSignals,
		Relationship:       relSnapshot,
		Life:               lifeSnapshot,
		PersonalityWeights: personalityWeights,
		Trigger:            goalContext.Current.Trigger,
		Now:                now,
	}
	candidates := decision.GenerateCandidates(decisionCtx, p.candidateRegistry)

	scoringContext := decision.CandidateScoringContext{
		Goals:              goalsForDecision(goalContext),
		Intentions:         append([]decision.Intention(nil), goalContext.Intentions...),
		Psyche:             psycheSignals,
		Relationship:       relSnapshot,
		Life:               lifeSnapshot,
		PersonalityWeights: personalityWeights,
		Now:                now,
	}
	scoredCandidates, err := decision.ScoreCandidates(candidates, scoringContext, decision.DefaultBehaviorScoringOptions())
	if err != nil {
		return nil, nil, fmt.Errorf("scoring failed: %w", err)
	}

	arbitrationInput := decision.ArbitrationInput{
		Candidates: scoredCandidates,
		Goals:      goalsForDecision(goalContext),
		Intentions: append([]decision.Intention(nil), goalContext.Intentions...),
		Psyche:     psycheSignals,
		Life:       lifeSnapshot,
		Filter:     decision.DefaultHardConstraintFilter(),
		Now:        now,
	}
	arbitrationResult, arbErr := p.arbitrationLayer.Arbitrate(arbitrationInput)
	if arbErr != nil {
		return nil, nil, fmt.Errorf("arbitration failed: %w", arbErr)
	}
	if !arbitrationResult.HasSelection {
		return nil, nil, nil
	}

	safetyLevel := decision.BehaviorSafetyLevelNormal
	if safetyDecision.Blocked {
		safetyLevel = decision.BehaviorSafetyLevelBlocked
	}
	planSafetyCtx := decision.PlanSafetyContext{
		Level:   safetyLevel,
		Blocked: safetyDecision.Blocked,
		Reasons: safetyDecision.Reasons,
	}

	var personalityRef decision.CompiledPersonalityRef
	if compiledPersonality != nil {
		personalityRef = decision.CompiledPersonalityRef{
			Version:           compiledPersonality.Version,
			SourceCharacterID: compiledPersonality.CharacterID,
			BehaviorWeights:   personalityWeights,
			RawConfig:         compiledPersonality.RawConfig,
			ExpressionPolicyKey: compiledPersonality.ExpressionPolicyKey,
		}
	}

	buildInput := decision.BehaviorPlanBuildInput{
		UserID:          scope.UserID,
		CharacterID:     scope.CharacterID,
		ConversationID:  scope.ConversationID,
		InteractionID:   scope.InteractionID,
		RequestID:       scope.RequestID,
		Arbitration:     arbitrationResult,
		Goals:           goalsForDecision(goalContext),
		Intentions:      append([]decision.Intention(nil), goalContext.Intentions...),
		Psyche:          psycheSignals,
		Relationship:    relSnapshot,
		Life:            lifeSnapshot,
		Personality:     personalityRef,
		Safety:          planSafetyCtx,
		Now:             now,
	}

	builder := decision.NewBehaviorPlanBuilder()
	plan, buildErr := builder.Build(buildInput)
	if buildErr != nil {
		return nil, nil, fmt.Errorf("plan build failed: %w", buildErr)
	}
	if plan == nil {
		return nil, nil, nil
	}

	if !plan.NeedsExpression || plan.DoNotSend {
		return plan, nil, nil
	}

	emotionIntensity := 0.5
	if compiledPersonality != nil {
		if v, ok := compiledPersonality.ExpressionStyle["emotionalExpression"]; ok {
			emotionIntensity = v
		}
	}
	exprCtrl := decision.ExpressionControlInput{
		EmotionIntensity:   emotionIntensity,
		RiskScore:          plan.Selected.RiskScore,
		StressLevel:        psycheSignals.Stress.Value,
		RelationshipSafety: 0.5,
	}
	if relSnapshot.Dimensions != nil {
		if v, ok := relSnapshot.Dimensions[decision.RelationshipSafety]; ok {
			exprCtrl.RelationshipSafety = v.Value
		}
	}

	safetyResult := decision.SafetyCheckResult{Passed: !safetyDecision.Blocked, Blocked: safetyDecision.Blocked}
	if safetyDecision.Blocked {
		safetyResult.Reason = "runtime_safety_blocked"
	}
	for _, r := range safetyDecision.Reasons {
		if safetyResult.Reason != "" {
			safetyResult.Reason += "; "
		}
		safetyResult.Reason += r
	}
	safetyResult.ConfidenceScore = 0.9

	exprInput := decision.ExpressionPlanInput{
		BehaviorPlan:               *plan,
		Psyche:                     psycheSignals,
		ExpressionCtrl:             exprCtrl,
		SafetyResult:               safetyResult,
		PersonalityExpressionStyle: personalityStyle,
		Now:                        now,
	}
	exprPlan, exprErr := decision.GenerateExpressionPlan(exprInput)
	if exprErr != nil {
		return nil, nil, fmt.Errorf("expression plan generation failed: %w", exprErr)
	}
	return plan, &exprPlan, nil
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
