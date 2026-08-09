package decision

import (
	"testing"
	"time"
)

func TestArbitrationSelectsHighestScored(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.6, PersonalityScore: 0.2, NeedScore: 0.1, FinalScore: 0.9},
		{ID: "wait_observe", BaseScore: 0.1, PersonalityScore: 0.1, FinalScore: 0.2},
	}
	input := ArbitrationInput{
		Candidates: candidates,
		Filter:     filter,
		Now:        now,
	}
	result := layer.Arbitrate(input)
	if result.Selected.ID != "chat_reply" {
		t.Fatalf("应选择 chat_reply, 实际 %s", result.Selected.ID)
	}
	if result.FallbackUsed {
		t.Fatal("不应使用回退")
	}
}

func TestArbitrationFallbackOnEmptyAllowed(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"chat_reply", "offer_support", "ask_clarify", "set_boundary", "express_emotion", "wait_observe", "tool_search", "proactive_greet"}
	filter := NewHardConstraintFilter(config)
	input := ArbitrationInput{
		Candidates: []BehaviorCandidate{{ID: "chat_reply"}},
		Filter:     filter,
		Now:        now,
	}
	result := layer.Arbitrate(input)
	if !result.FallbackUsed {
		t.Fatal("全部被过滤应使用回退")
	}
	if result.Selected.ID != "wait_observe" {
		t.Fatalf("回退应为 wait_observe, 实际 %s", result.Selected.ID)
	}
}

func TestArbitrationBelowThresholdFallback(t *testing.T) {
	now := time.Now().UTC()
	layer := DefaultArbitrationLayer()
	layer.Config.MinScoreThreshold = 1.0
	filter := DefaultHardConstraintFilter()
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", BaseScore: 0.1, FinalScore: 0.05},
	}
	input := ArbitrationInput{
		Candidates: candidates,
		Filter:     filter,
		Now:        now,
	}
	result := layer.Arbitrate(input)
	if !result.FallbackUsed {
		t.Fatal("低于阈值应使用回退")
	}
}

func TestApplyBehaviorCostSignals(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("chat_reply", now.Add(-5*time.Minute), 0.5)
	history.Record("chat_reply", now.Add(-3*time.Minute), 0.3)
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", FinalScore: 0.8},
	}
	result := ApplyBehaviorCostSignals(candidates, history, now)
	if result[0].RepeatPenalty == 0 && result[0].FatiguePenalty == 0 {
		t.Fatalf("expected repeat or fatigue penalty > 0, got repeat=%f fatigue=%f", result[0].RepeatPenalty, result[0].FatiguePenalty)
	}
	if result[0].FinalScore != 0.8 {
		t.Fatalf("ApplyBehaviorCostSignals should not modify FinalScore, got %f", result[0].FinalScore)
	}
}

func TestBuildFallbackCandidate(t *testing.T) {
	fallback := buildFallbackCandidate()
	if fallback.ID != "wait_observe" {
		t.Fatalf("回退候选应为 wait_observe, 实际 %s", fallback.ID)
	}
	if fallback.Tag != BehaviorTagDelay {
		t.Fatalf("回退 Tag 应为 delay, 实际 %s", fallback.Tag)
	}
}
