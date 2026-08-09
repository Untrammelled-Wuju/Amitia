package decision

import (
	"testing"
	"time"
)

func TestHardConstraintFilterBlocksBlockedID(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"proactive_greet"}
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now, BehaviorHistory{})
	if result.Allowed {
		t.Fatal("被屏蔽 ID 应不允许通过")
	}
	if result.Code != "blocked_id" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
}

func TestHardConstraintFilterCooldownWithHistory(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("proactive_greet", now.Add(-1*time.Minute), 0.0)
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now, history)
	if result.Allowed {
		t.Fatal("需要 5 分钟冷却, 只过去 1 分钟应被阻止")
	}
	if result.Code != "cooldown" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
}

func TestHardConstraintFilterCooldownExpired(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("proactive_greet", now.Add(-10*time.Minute), 0.0)
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now, history)
	if !result.Allowed {
		t.Fatal("超过 5 分钟冷却期应允许通过")
	}
}

func TestHardConstraintFilterCooldownNoHistory(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now, BehaviorHistory{})
	if !result.Allowed {
		t.Fatal("无历史时应允许通过")
	}
}

func TestHardConstraintFilterHardConstraint(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidate := BehaviorCandidate{
		ID: "chat_reply",
		Constraints: []BehaviorConstraint{
			{Kind: "safety", Hard: true, Limit: 0.2, Observed: 0.8},
		},
	}
	result := filter.Check(candidate, now, BehaviorHistory{})
	if result.Allowed {
		t.Fatal("硬约束被突破应阻止")
	}
	if result.Code != "hard_constraint_failed" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
}

func TestHardConstraintFilterAllowsClean(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidate := BehaviorCandidate{ID: "chat_reply", BaseScore: 0.5}
	result := filter.Check(candidate, now, BehaviorHistory{})
	if !result.Allowed {
		t.Fatalf("干净候选应允许通过: %s", result.Reason)
	}
}

func TestHardConstraintFilterFilterSplit(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	config.BlockedIDs = []string{"proactive_greet"}
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "chat_reply"},
		{ID: "proactive_greet"},
		{ID: "ask_clarify"},
	}
	allowed, blocked := filter.Filter(candidates, now, BehaviorHistory{})
	if len(allowed) != 2 {
		t.Fatalf("应允许 2 个, 实际 %d", len(allowed))
	}
	if len(blocked) != 1 || blocked[0].ID != "proactive_greet" {
		t.Fatalf("应阻止 proactive_greet, 实际 %#v", blocked)
	}
}

func TestHardConstraintFilterPreservesInput(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidates := []BehaviorCandidate{
		{ID: "chat_reply", FinalScore: 0.5, BaseScore: 0.5},
	}
	origFinal := candidates[0].FinalScore
	origBase := candidates[0].BaseScore
	_, _ = filter.Filter(candidates, now, BehaviorHistory{})
	if candidates[0].FinalScore != origFinal {
		t.Fatal("FinalScore 不应被修改")
	}
	if candidates[0].BaseScore != origBase {
		t.Fatal("BaseScore 不应被修改")
	}
}
