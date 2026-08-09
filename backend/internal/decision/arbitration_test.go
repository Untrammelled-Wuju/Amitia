package decision

import (
	"math"
	"testing"
	"time"
)

func TestArbitrationSelectsHighestScored(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.6, FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV2},
		{ID: "wait_observe", BaseScore: 0.1, FinalScore: 0.2, ScoringVersion: BehaviorFormulaVersionV2},
	}
	input := ArbitrationInput{
		Candidates: candidates,
		Filter:     filter,
		Now:        now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSelection || result.Selected.ID != "chat_reply" {
		t.Fatalf("应选择 chat_reply, got HasSelection=%v ID=%s", result.HasSelection, result.Selected.ID)
	}
	if result.Disposition != ArbitrationDispositionSelected {
		t.Fatalf("expected disposition=selected, got %s", result.Disposition)
	}
}

func TestArbitrationNoSelectionAllBlocked(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"chat_reply", "wait_observe"}
	filter := NewHardConstraintFilter(config)
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "wait_observe", FinalScore: 0.2, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasSelection {
		t.Fatal("全部 blocked 时应无选择")
	}
	if result.Disposition != ArbitrationDispositionNoSelection {
		t.Fatalf("expected no_selection, got %s", result.Disposition)
	}
	if len(result.Blocked) != 2 {
		t.Fatalf("expected 2 blocked, got %d", len(result.Blocked))
	}
}

func TestArbitrationUsesFallback(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	layer.Config.MinScoreThreshold = 0.10
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.05, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "wait_observe", FinalScore: 0.04, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSelection {
		t.Fatal("应有 fallback 选择")
	}
	if !result.FallbackUsed {
		t.Fatal("应标记为 FallbackUsed")
	}
	if result.Selected.ID != "wait_observe" {
		t.Fatalf("fallback 应为 wait_observe, got %s", result.Selected.ID)
	}
	if result.Disposition != ArbitrationDispositionFallback {
		t.Fatalf("expected fallback disposition, got %s", result.Disposition)
	}
}

func TestArbitrationNoFallbackCandidate(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	layer.Config.MinScoreThreshold = 0.50
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.05, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasSelection {
		t.Fatal("无 wait_observe 时应为 no_selection")
	}
	if result.Disposition != ArbitrationDispositionNoSelection {
		t.Fatalf("expected no_selection, got %s", result.Disposition)
	}
}

func TestArbitrationFallbackBlockedByHardConstraint(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	layer.Config.MinScoreThreshold = 0.50
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"wait_observe"}
	filter := NewHardConstraintFilter(config)
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.30, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "wait_observe", FinalScore: 0.20, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasSelection {
		t.Fatal("fallback 被 hard block 时应为 no_selection")
	}
	if result.Disposition != ArbitrationDispositionNoSelection {
		t.Fatalf("expected no_selection, got %s", result.Disposition)
	}
}

func TestArbitrationRejectsUnscoredCandidate(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.9},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("未打分候选应返回 error")
	}
}

func TestArbitrationRejectsOldScoringVersion(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV1},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("旧版 ScoringVersion 应返回 error")
	}
}

func TestArbitrationDuplicateID(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "chat_reply", FinalScore: 0.6, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("重复 ID 应返回 error")
	}
}

func TestArbitrationEmptyID(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("空 ID 应返回 error")
	}
}

func TestArbitrationNaNScore(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: math.NaN(), ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("NaN FinalScore 应返回 error")
	}
}

func TestArbitrationZeroNow(t *testing.T) {
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "chat_reply", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("Zero Now 应返回 error")
	}
}

func TestArbitrationHardBlockHighScore(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"proactive_greet"}
	filter := NewHardConstraintFilter(config)
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "proactive_greet", FinalScore: 10.0, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "chat_reply", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected.ID != "chat_reply" {
		t.Fatalf("hard block 高分候选后应选 chat_reply, got %s", result.Selected.ID)
	}
}

func TestArbitrationNotModifyInput(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV2, BaseScore: 0.6, RiskScore: 0.1},
	}
	origFinalScore := candidates[0].FinalScore
	origBaseScore := candidates[0].BaseScore
	origRiskScore := candidates[0].RiskScore
	input := ArbitrationInput{
		Candidates: candidates,
		Filter:     filter,
		Now:        now,
	}
	_, _ = layer.Arbitrate(input)
	if candidates[0].FinalScore != origFinalScore {
		t.Fatal("FinalScore 被修改")
	}
	if candidates[0].BaseScore != origBaseScore {
		t.Fatal("BaseScore 被修改")
	}
	if candidates[0].RiskScore != origRiskScore {
		t.Fatal("RiskScore 被修改")
	}
}

func TestArbitrationDeterministic(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	firstID := ""
	for i := 0; i < 100; i++ {
		candidates := []BehaviorCandidate{
			{ID: "set_boundary", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "offer_support", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "wait_observe", FinalScore: 0.1, ScoringVersion: BehaviorFormulaVersionV2},
		}
		input := ArbitrationInput{Candidates: candidates, Filter: filter, Now: now}
		result, err := layer.Arbitrate(input)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstID = result.Selected.ID
		} else if result.Selected.ID != firstID {
			t.Fatalf("不一致 selection: %s vs %s", firstID, result.Selected.ID)
		}
	}
}

func TestArbitrationOverrideWins(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "set_boundary", FinalScore: 0.3, ScoringVersion: BehaviorFormulaVersionV2, Overrides: []string{"offer_support"}},
			{ID: "offer_support", FinalScore: 0.8, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected.ID != "set_boundary" {
		t.Fatalf("override 高分候选后应选 set_boundary, got %s", result.Selected.ID)
	}
	hasOverrideReject := false
	for _, rej := range result.Rejected {
		if rej.Candidate.ID == "offer_support" && rej.Stage == ArbitrationRejectOverride {
			hasOverrideReject = true
			break
		}
	}
	if !hasOverrideReject {
		t.Fatal("offer_support 应出现在 override rejection")
	}
}

func TestArbitrationMutualOverrideError(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "a", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2, Overrides: []string{"b"}},
			{ID: "b", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2, Overrides: []string{"a"}},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("mutual override 应返回 error")
	}
}

func TestArbitrationSelfOverrideError(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "a", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2, Overrides: []string{"a"}},
		},
		Filter: filter,
		Now:    now,
	}
	_, err := layer.Arbitrate(input)
	if err == nil {
		t.Fatal("self override 应返回 error")
	}
}

func TestArbitrationAlternativesOrder(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "a", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "b", FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "c", FinalScore: 0.7, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasSelection || result.Selected.ID != "b" {
		t.Fatalf("应选 b, got %s", result.Selected.ID)
	}
	if len(result.Alternatives) != 2 {
		t.Fatalf("应有两个 alternatives, got %d", len(result.Alternatives))
	}
	if result.Alternatives[0].ID != "c" || result.Alternatives[1].ID != "a" {
		t.Fatalf("alternatives 排序错误: %s, %s", result.Alternatives[0].ID, result.Alternatives[1].ID)
	}
}

func TestArbitrationTieIDOrder(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{
			{ID: "b", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "a", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		},
		Filter: filter,
		Now:    now,
	}
	result, err := layer.Arbitrate(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected.ID != "a" {
		t.Fatalf("tie 时应选 ID 较小的 a, got %s", result.Selected.ID)
	}
	if result.Alternatives[0].ID != "b" {
		t.Fatalf("tie alternative 应为 b, got %s", result.Alternatives[0].ID)
	}
}

func TestInputOrderDoesNotAffectSelection(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	filter := DefaultHardConstraintFilter()
	firstID := ""
	for i := 0; i < 100; i++ {
		candidates := []BehaviorCandidate{
			{ID: "wait_observe", FinalScore: 0.1, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "chat_reply", FinalScore: 0.9, ScoringVersion: BehaviorFormulaVersionV2},
			{ID: "ask_clarify", FinalScore: 0.5, ScoringVersion: BehaviorFormulaVersionV2},
		}
		if i%2 == 0 {
			candidates[0], candidates[1] = candidates[1], candidates[0]
		}
		if i%3 == 0 {
			candidates[1], candidates[2] = candidates[2], candidates[1]
		}
		input := ArbitrationInput{Candidates: candidates, Filter: filter, Now: now}
		result, err := layer.Arbitrate(input)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstID = result.Selected.ID
		} else if result.Selected.ID != firstID {
			t.Fatalf("input order affected selection: first=%s iter%d=%s", firstID, i, result.Selected.ID)
		}
	}
}
