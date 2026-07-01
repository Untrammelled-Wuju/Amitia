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
	result := filter.Check(candidate, now)
	if result.Allowed {
		t.Fatal("被屏蔽 ID 应不允许通过")
	}
}

func TestHardConstraintFilterCooldownActive(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	filter.Cooldown.MarkSent("proactive_greet", now)
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now.Add(1*time.Minute))
	if result.Allowed {
		t.Fatal("需要 5 分钟冷却, 只过去 1 分钟应被阻止")
	}
}

func TestHardConstraintFilterCooldownExpired(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	filter.Cooldown.MarkSent("proactive_greet", now)
	candidate := BehaviorCandidate{ID: "proactive_greet"}
	result := filter.Check(candidate, now.Add(10*time.Minute))
	if !result.Allowed {
		t.Fatal("超过 5 分钟冷却期应允许通过")
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
	result := filter.Check(candidate, now)
	if result.Allowed {
		t.Fatal("硬约束被突破应阻止")
	}
}

func TestHardConstraintFilterAllowsClean(t *testing.T) {
	config := DefaultHardConstraintFilterConfig()
	filter := NewHardConstraintFilter(config)
	now := time.Now().UTC()
	candidate := BehaviorCandidate{ID: "chat_reply", BaseScore: 0.5}
	result := filter.Check(candidate, now)
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
	allowed, blocked := filter.Filter(candidates, now)
	if len(allowed) != 2 {
		t.Fatalf("应允许 2 个, 实际 %d", len(allowed))
	}
	if len(blocked) != 1 || blocked[0].ID != "proactive_greet" {
		t.Fatalf("应阻止 proactive_greet, 实际 %#v", blocked)
	}
}
