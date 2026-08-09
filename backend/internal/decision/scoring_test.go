package decision

import (
	"math"
	"testing"
	"time"
)

func TestScoreBehaviorCandidatesSortsByFinalScore(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "reply", BaseScore: 0.4, PersonalityScore: 0.1, NeedScore: 0.2, RelationshipScore: 0.1, RiskScore: 0.1},
		{ID: "repair", BaseScore: 0.3, PersonalityScore: 0.3, NeedScore: 0.3, RelationshipScore: 0.1, RiskScore: 0.05},
		{ID: "delay", BaseScore: 0.2, PersonalityScore: 0.1, NeedScore: 0.1, RelationshipScore: 0.1, RiskScore: 0.3},
	}

	scored := ScoreBehaviorCandidates(candidates, DefaultBehaviorScoringOptions())

	if len(scored) != 3 {
		t.Fatalf("unexpected candidate count: %d", len(scored))
	}
	if scored[0].ID != "repair" || scored[1].ID != "reply" || scored[2].ID != "delay" {
		t.Fatalf("unexpected sort order: %#v", scored)
	}
	if scored[0].FinalScore <= scored[1].FinalScore {
		t.Fatalf("expected top candidate to have highest score: %#v", scored)
	}
}

func TestSelectBehaviorCandidateBlocksHardConstraint(t *testing.T) {
	candidates := []BehaviorCandidate{
		{
			ID:        "boundary",
			BaseScore: 0.9,
			Constraints: []BehaviorConstraint{
				{Kind: "safety", Hard: true, Limit: 0.2, Observed: 0.8},
			},
		},
		{
			ID:                "reply",
			BaseScore:         0.5,
			NeedScore:         0.2,
			RelationshipScore: 0.2,
			RiskScore:         0.1,
		},
	}

	result := SelectBehaviorCandidate(candidates, DefaultBehaviorScoringOptions())

	if result.Selected.ID != "reply" {
		t.Fatalf("unexpected selected candidate: %#v", result.Selected)
	}
	if len(result.Blocked) != 1 || result.Blocked[0].ID != "boundary" {
		t.Fatalf("expected blocked candidate to be retained in audit: %#v", result.Blocked)
	}
	if len(result.Audit.Diagnostics) < 2 {
		t.Fatalf("expected audit diagnostics: %#v", result.Audit.Diagnostics)
	}
}

func TestScoreBehaviorCandidatesPenalizesHigherRisk(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "low-risk", BaseScore: 0.5, PersonalityScore: 0.2, RiskScore: 0.1},
		{ID: "high-risk", BaseScore: 0.5, PersonalityScore: 0.2, RiskScore: 0.4},
	}

	scored := ScoreBehaviorCandidates(candidates, DefaultBehaviorScoringOptions())

	if scored[0].ID != "low-risk" {
		t.Fatalf("expected lower risk candidate first: %#v", scored)
	}
	if scored[0].FinalScore <= scored[1].FinalScore {
		t.Fatalf("expected risk penalty to reduce score: %#v", scored)
	}
}

func TestScoreBehaviorCandidatesKeepsStableOrderOnTie(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "first", BaseScore: 0.4, PersonalityScore: 0.2, RiskScore: 0.1},
		{ID: "second", BaseScore: 0.4, PersonalityScore: 0.2, RiskScore: 0.1},
		{ID: "third", BaseScore: 0.2, PersonalityScore: 0.1, RiskScore: 0.3},
	}

	scored := ScoreBehaviorCandidates(candidates, DefaultBehaviorScoringOptions())

	if scored[0].ID != "first" || scored[1].ID != "second" {
		t.Fatalf("expected stable order for tied scores: %#v", scored)
	}
}

func TestApplyLifeInterruptionRiskRaisesProactiveRiskWhenBusy(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "proactive", Tag: BehaviorTagProactiveCheck, Channel: BehaviorChannelProactive, RiskScore: 0.1},
		{ID: "reply", Tag: BehaviorTagReply, Channel: BehaviorChannelChat, RiskScore: 0.1},
	}

	adjusted := ApplyLifeInterruptionRisk(candidates, LifeSnapshot{Busy: 0.8})

	if adjusted[0].RiskScore <= candidates[0].RiskScore {
		t.Fatalf("expected proactive candidate risk increase: %#v", adjusted[0])
	}
	if adjusted[1].RiskScore != candidates[1].RiskScore {
		t.Fatalf("expected chat reply risk unchanged: %#v", adjusted[1])
	}
	if len(adjusted[0].Constraints) == 0 || adjusted[0].Constraints[0].Kind != "busy_interruption" {
		t.Fatalf("expected busy interruption constraint: %#v", adjusted[0].Constraints)
	}

}
func TestComputeAffectScoreReturnsHigherForPositiveEmotion(t *testing.T) {
	state := AffectSignalInput{Positive: 0.8, Negative: 0.1, Stress: 0.2}
	score := ComputeAffectScore(state)
	if score <= 0.30 {
		t.Fatalf("expected positive affect to yield higher score, got %f", score)
	}
}

func TestComputeAffectScoreReturnsLowerForHighStress(t *testing.T) {
	state := AffectSignalInput{Positive: 0.5, Negative: 0.3, Stress: 0.8}
	score := ComputeAffectScore(state)
	if score > 0.20 {
		t.Fatalf("expected high stress to suppress affect score, got %f", score)
	}
}

func TestComputeAffectScoreReturnsLowestForNegativeEmotion(t *testing.T) {
	state := AffectSignalInput{Positive: 0.1, Negative: 0.6, Stress: 0.3}
	score := ComputeAffectScore(state)
	if score > 0.15 {
		t.Fatalf("expected strong negative to yield minimal affect score, got %f", score)
	}
}

func TestScoreBehaviorCandidateIncludesAffectScore(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "positive-affect", BaseScore: 0.3, AffectScore: 0.45, RiskScore: 0.1},
		{ID: "negative-affect", BaseScore: 0.3, AffectScore: 0.10, RiskScore: 0.1},
	}
	scored := ScoreBehaviorCandidates(candidates, DefaultBehaviorScoringOptions())
	if scored[0].ID != "positive-affect" {
		t.Fatalf("expected positive affect candidate to rank first: %#v", scored)
	}
	if scored[0].FinalScore <= scored[1].FinalScore {
		t.Fatalf("expected affect score to influence ranking: %#v", scored)
	}
}

func TestAffectWeightCanBeTuned(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "high-affect", BaseScore: 0.2, AffectScore: 0.45, RiskScore: 0.1},
		{ID: "low-affect", BaseScore: 0.4, AffectScore: 0.10, RiskScore: 0.1},
	}
	opts := DefaultBehaviorScoringOptions()
	opts.AffectWeight = 2.0
	scored := ScoreBehaviorCandidates(candidates, opts)
	if scored[0].ID != "high-affect" {
		t.Fatalf("expected boosted affect weight to prioritize high affect: %#v", scored)
	}
}

func TestBusyLifeCanHardBlockProactiveCandidate(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "proactive", Tag: BehaviorTagProactiveCheck, Channel: BehaviorChannelProactive, BaseScore: 0.9},
		{ID: "reply", Tag: BehaviorTagReply, Channel: BehaviorChannelChat, BaseScore: 0.4},
	}

	adjusted := ApplyLifeInterruptionRisk(candidates, LifeSnapshot{Busy: 0.95})
	result := SelectBehaviorCandidate(adjusted, DefaultBehaviorScoringOptions())

	if result.Selected.ID != "reply" {
		t.Fatalf("expected busy state to keep proactive candidate from bypassing risk: %#v", result)
	}
	if len(result.Blocked) != 1 || result.Blocked[0].ID != "proactive" {
		t.Fatalf("expected proactive candidate blocked under extreme busy state: %#v", result.Blocked)
	}
}

func TestComputeAffectNeedFusionPositiveEmotionLowNeed(t *testing.T) {
	affect := AffectSignalInput{Positive: 0.8, Negative: 0.1, Stress: 0.2}
	needs := []NeedScoringInput{
		{Kind: "connection", Deviation: 0.15},
		{Kind: "autonomy", Deviation: 0.10},
	}
	fusion := ComputeAffectNeedFusion(affect, needs)
	if fusion.Multiplier < 1.0 {
		t.Fatalf("expected positive affect + low need to boost multiplier above 1.0, got %f", fusion.Multiplier)
	}
	if fusion.FormulaID != "affect-need-fusion-v1" {
		t.Fatalf("expected formula ID v1, got %s", fusion.FormulaID)
	}
}

func TestComputeAffectNeedFusionNegativeEmotionHighNeed(t *testing.T) {
	affect := AffectSignalInput{Positive: 0.1, Negative: 0.7, Stress: 0.6}
	needs := []NeedScoringInput{
		{Kind: "connection", Deviation: 0.85},
		{Kind: "certainty", Deviation: 0.75},
	}
	fusion := ComputeAffectNeedFusion(affect, needs)
	if fusion.Multiplier > 1.0 {
		t.Fatalf("expected negative affect + high need to suppress multiplier below 1.0, got %f", fusion.Multiplier)
	}
}

func TestComputeAffectNeedFusionEmptyNeeds(t *testing.T) {
	affect := AffectSignalInput{Positive: 0.5, Negative: 0.2, Stress: 0.3}
	fusion := ComputeAffectNeedFusion(affect, nil)
	if fusion.Multiplier <= 0 || fusion.Multiplier > 1.35 {
		t.Fatalf("expected multiplier in (0, 1.35] for empty needs, got %f", fusion.Multiplier)
	}
	if fusion.NeedRaw != 1.0 {
		t.Fatalf("expected needRaw=1.0 for empty needs, got %f", fusion.NeedRaw)
	}
}

func TestComputeAffectNeedFusionClampBoundaries(t *testing.T) {
	affect := AffectSignalInput{Positive: 0.0, Negative: 1.0, Stress: 1.0}
	needs := []NeedScoringInput{{Kind: "connection", Deviation: 0.0}}
	fusion := ComputeAffectNeedFusion(affect, needs)
	if fusion.Multiplier < 0.65 {
		t.Fatalf("expected multiplier clamped at min 0.65, got %f", fusion.Multiplier)
	}
	affect2 := AffectSignalInput{Positive: 1.0, Negative: 0.0, Stress: 0.0}
	needs2 := []NeedScoringInput{{Kind: "connection", Deviation: 1.0}}
	fusion2 := ComputeAffectNeedFusion(affect2, needs2)
	if fusion2.Multiplier > 1.35 {
		t.Fatalf("expected multiplier clamped at max 1.35, got %f", fusion2.Multiplier)
	}
}

func TestScoreCandidates_formulaAccuracy(t *testing.T) {
	// B4 spec section 88: verify canonical formula
	candidates := []BehaviorCandidate{
		{
			ID:                  "test_cand",
			BaseScore:           0.6,
			PersonalityScore:    0.1,
			NeedScore:           0.2,
			RelationshipScore:   0.05,
			AffectScore:         0.03,
			UserPreferenceScore: 0.4,
			RiskScore:           0.1,
			RepeatPenalty:       0.15,
			FatiguePenalty:      0.1,
		},
	}
	options := DefaultBehaviorScoringOptions()

	// Test using compat API which preserves component scores
	result := ScoreBehaviorCandidates(candidates, options)

	// 0.6 + 0.1 + 0.2 + 0.05 + 0.03 + 0.04 - 0.1 - 0.15 - 0.1 = 0.67
	expected := 0.67
	if result[0].FinalScore != expected {
		t.Fatalf("expected FinalScore=%f, got %f", expected, result[0].FinalScore)
	}
	if result[0].ScoringVersion != BehaviorFormulaVersionV2 {
		t.Fatalf("expected ScoringVersion=%s, got %s", BehaviorFormulaVersionV2, result[0].ScoringVersion)
	}
}

func TestScoreCandidatesZeroWeightDisablesFactor(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "test_cand", BaseScore: 0.5, RiskScore: 0.8},
	}
	ctx := CandidateScoringContext{Now: now}
	options := DefaultBehaviorScoringOptions()
	options.RiskWeight = 0

	result, err := ScoreCandidates(candidates, ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	// With RiskWeight=0, FinalScore should equal BaseScore * BaseWeight = 0.5
	if result[0].FinalScore != 0.5 {
		t.Fatalf("expected FinalScore=0.5 with RiskWeight=0, got %f", result[0].FinalScore)
	}
}

func TestScoreCandidatesNegativeWeightReturnsError(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{{ID: "test_cand"}}
	ctx := CandidateScoringContext{Now: now}
	options := DefaultBehaviorScoringOptions()
	options.RiskWeight = -1

	_, err := ScoreCandidates(candidates, ctx, options)
	if err == nil {
		t.Fatal("expected error for negative RiskWeight, got nil")
	}
}

func TestScoreCandidatesNaNBaseReturnsError(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{{ID: "test_cand", BaseScore: math.NaN()}}
	ctx := CandidateScoringContext{Now: now}

	_, err := ScoreCandidates(candidates, ctx, DefaultBehaviorScoringOptions())
	if err == nil {
		t.Fatal("expected error for NaN BaseScore, got nil")
	}
}

func TestScoreCandidatesPreservesInputOrder(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "cand_a", BaseScore: 0.1},
		{ID: "cand_b", BaseScore: 0.9},
		{ID: "cand_c", BaseScore: 0.5},
	}
	ctx := CandidateScoringContext{Now: now}

	result, err := ScoreCandidates(candidates, ctx, DefaultBehaviorScoringOptions())
	if err != nil {
		t.Fatal(err)
	}
	if result[0].ID != "cand_a" || result[1].ID != "cand_b" || result[2].ID != "cand_c" {
		t.Fatalf("ScoreCandidates should preserve input order: %#v", result)
	}
}

func TestScoreCandidatesIdempotent(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "test_cand", BaseScore: 0.5, PersonalityScore: 0.1, RiskScore: 0.2},
	}
	ctx := CandidateScoringContext{Now: now}
	options := DefaultBehaviorScoringOptions()

	first, err := ScoreCandidates(candidates, ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ScoreCandidates(first, ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].FinalScore != second[0].FinalScore {
		t.Fatalf("ScoreCandidates not idempotent: first=%f second=%f", first[0].FinalScore, second[0].FinalScore)
	}
	firstReasons := 0
	for _, r := range first[0].Reasons {
		if r.Source == "scoring" {
			firstReasons++
		}
	}
	secondReasons := 0
	for _, r := range second[0].Reasons {
		if r.Source == "scoring" {
			secondReasons++
		}
	}
	if firstReasons != secondReasons {
		t.Fatalf("ScoreCandidates reasons not idempotent: first=%d second=%d", firstReasons, secondReasons)
	}
}

func TestScoreCandidatesNegativeFinalScore(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "test_cand", BaseScore: 0.0, RiskScore: 0.5, RepeatPenalty: 0.2, FatiguePenalty: 0.15},
	}
	ctx := CandidateScoringContext{Now: now}
	options := DefaultBehaviorScoringOptions()

	result, err := ScoreCandidates(candidates, ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	// 0 - 0.5 - 0.2 - 0.15 = -0.85 (should NOT be clamped to 0)
	if result[0].FinalScore >= 0 {
		t.Fatalf("expected negative FinalScore, got %f", result[0].FinalScore)
	}
}

func TestScoreCandidatesScoringReasonsSum(t *testing.T) {
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{
			ID:                  "test_cand",
			BaseScore:           0.5,
			PersonalityScore:    0.1,
			NeedScore:           0.2,
			RelationshipScore:   0.05,
			AffectScore:         0.05,
			UserPreferenceScore: 0.2,
			RiskScore:           0.1,
			RepeatPenalty:       0.1,
			FatiguePenalty:      0.1,
		},
	}
	ctx := CandidateScoringContext{Now: now}
	options := DefaultBehaviorScoringOptions()

	result, err := ScoreCandidates(candidates, ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	sum := 0.0
	for _, r := range result[0].Reasons {
		if r.Source == "scoring" {
			sum += r.Delta
		}
	}
	diff := sum - result[0].FinalScore
	if diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("scoring reasons sum (%f) should equal FinalScore (%f)", sum, result[0].FinalScore)
	}
}

func TestComputeAffectNeedFusionStillComputesMultiplier(t *testing.T) {
	affect := AffectSignalInput{Positive: 0.8, Negative: 0.1, Stress: 0.2}
	needs := []NeedScoringInput{
		{Kind: "connection", Deviation: 0.15},
		{Kind: "autonomy", Deviation: 0.10},
	}
	fusion := ComputeAffectNeedFusion(affect, needs)
	if fusion.Multiplier < 1.0 {
		t.Fatalf("expected positive affect + low need to boost multiplier above 1.0, got %f", fusion.Multiplier)
	}
	if fusion.FormulaID != "affect-need-fusion-v1" {
		t.Fatalf("expected formula ID v1, got %s", fusion.FormulaID)
	}
}
