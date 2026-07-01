package decision

import "testing"

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

func TestScoreWithAffectNeedFusionRanksByFusedScore(t *testing.T) {
	candidates := []BehaviorCandidate{
		{ID: "high-fuse", BaseScore: 0.5, NeedScore: 0.1, PersonalityScore: 0.1, RelationshipScore: 0.1, RiskScore: 0.1},
		{ID: "low-fuse", BaseScore: 0.2, NeedScore: 0.1, PersonalityScore: 0.1, RelationshipScore: 0.1, RiskScore: 0.1},
	}
	affect := AffectSignalInput{Positive: 0.9, Negative: 0.05, Stress: 0.05}
	needs := []NeedScoringInput{{Kind: "connection", Deviation: 0.1}}
	result := ScoreWithAffectNeedFusion(candidates, DefaultBehaviorScoringOptions(), affect, needs)
	if result[0].ID != "high-fuse" || result[1].ID != "low-fuse" {
		t.Fatalf("expected high-fuse candidate first: %#v", result)
	}
	if len(result[0].Reasons) == 0 {
		t.Fatal("expected reasons after fusion scoring")
	}
}

func TestScoreWithAffectNeedFusionEmptyCandidates(t *testing.T) {
	result := ScoreWithAffectNeedFusion(nil, DefaultBehaviorScoringOptions(), AffectSignalInput{}, nil)
	if result != nil {
		t.Fatal("expected nil for nil candidates")
	}
}
