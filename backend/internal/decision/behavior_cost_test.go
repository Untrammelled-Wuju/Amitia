package decision

import (
	"testing"
	"time"
)

func TestBehaviorHistoryRecordAndCount(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("chat_reply", now.Add(-10*time.Minute), 0.5)
	history.Record("chat_reply", now.Add(-5*time.Minute), 0.3)
	history.Record("proactive_greet", now.Add(-3*time.Minute), 0.2)
	count := history.CountRecent("chat_reply", now.Add(-15*time.Minute))
	if count != 2 {
		t.Fatalf("chat_reply 最近应有 2 次, 实际 %d", count)
	}
	totalCost := history.TotalCost("chat_reply", now.Add(-15*time.Minute))
	if totalCost != 0.8 {
		t.Fatalf("总成本应为 0.8, 实际 %f", totalCost)
	}
}

func TestBehaviorHistoryMaxLimit(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(5)
	for i := 0; i < 10; i++ {
		history.Record("chat_reply", now.Add(time.Duration(i)*time.Minute), 0.1)
	}
	if len(history.Executions) != 5 {
		t.Fatalf("超出 MaxHistory 应只保留 5 条, 实际 %d", len(history.Executions))
	}
}

func TestComputeRepeatPenaltyZeroHistory(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	penalty := ComputeRepeatPenalty(history, "chat_reply", now, DefaultRepeatPenaltyConfig())
	if penalty != 0 {
		t.Fatalf("无历史记录时重复惩罚应为 0, 实际 %f", penalty)
	}
}

func TestComputeRepeatPenaltyAccumulates(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("chat_reply", now.Add(-20*time.Minute), 0.5)
	history.Record("chat_reply", now.Add(-10*time.Minute), 0.3)
	history.Record("chat_reply", now.Add(-5*time.Minute), 0.2)
	penalty := ComputeRepeatPenalty(history, "chat_reply", now, DefaultRepeatPenaltyConfig())
	if penalty <= 0 {
		t.Fatalf("多次重复应有惩罚, 实际 %f", penalty)
	}
}

func TestComputeFatiguePenaltyBelowThreshold(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("chat_reply", now.Add(-30*time.Minute), 0.5)
	config := DefaultFatiguePenaltyConfig()
	config.Threshold = 2
	penalty := ComputeFatiguePenalty(history, "chat_reply", now, config)
	if penalty != 0 {
		t.Fatalf("低于阈值不应有疲劳惩罚, 实际 %f", penalty)
	}
}

func TestComputeFatiguePenaltyExceedsThreshold(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	config := DefaultFatiguePenaltyConfig()
	config.Threshold = 1
	history.Record("chat_reply", now.Add(-30*time.Minute), 0.5)
	history.Record("chat_reply", now.Add(-20*time.Minute), 0.3)
	history.Record("chat_reply", now.Add(-10*time.Minute), 0.2)
	penalty := ComputeFatiguePenalty(history, "chat_reply", now, config)
	if penalty <= 0 {
		t.Fatalf("超出阈值应有疲劳惩罚, 实际 %f", penalty)
	}
	if penalty > 0.5 {
		t.Fatalf("疲劳惩罚不应超过 0.5, 实际 %f", penalty)
	}
}

func TestComputeCumulativeCost(t *testing.T) {
	now := time.Now().UTC()
	history := NewBehaviorHistory(20)
	history.Record("chat_reply", now.Add(-10*time.Hour), 0.5)
	history.Record("chat_reply", now.Add(-5*time.Hour), 0.3)
	cost := ComputeCumulativeCost(history, "chat_reply", now, 12*time.Hour)
	if cost != 0.8 {
		t.Fatalf("累积成本应为 0.8, 实际 %f", cost)
	}
}
