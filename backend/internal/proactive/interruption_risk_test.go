package proactive

import (
	"testing"
	"time"
)

func TestScoreInterruptionRiskUnansweredMonotonic(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)
	base := InterruptionRiskInput{
		Now:                    now,
		Channel:                "web",
		Availability:           InterruptionAvailabilityUnknown,
		AvailabilityConfidence: 0.5,
		MaxPerDay:              5,
	}

	none := ScoreInterruptionRisk(base)
	base.ConsecutiveUnanswered = 1
	one := ScoreInterruptionRisk(base)
	base.ConsecutiveUnanswered = 3
	three := ScoreInterruptionRisk(base)
	base.ConsecutiveUnanswered = 8
	eight := ScoreInterruptionRisk(base)

	if !(none.Score < one.Score && one.Score < three.Score && three.Score < eight.Score) {
		t.Fatalf("expected unanswered risk to increase monotonically, got %.3f %.3f %.3f %.3f", none.Score, one.Score, three.Score, eight.Score)
	}
	if !HasInterruptionRiskReason(eight, "recent_unanswered") {
		t.Fatalf("expected recent_unanswered reason, got %#v", eight.Reasons)
	}
}

func TestScoreInterruptionRiskLowConfidenceBusyIsLimited(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)
	low := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:                    now,
		Channel:                "web",
		Availability:           InterruptionAvailabilityBusy,
		AvailabilityConfidence: 0.1,
	})
	high := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:                    now,
		Channel:                "web",
		Availability:           InterruptionAvailabilityBusy,
		AvailabilityConfidence: 0.95,
	})

	if low.Score >= high.Score {
		t.Fatalf("expected high confidence busy risk to be higher, got low=%.3f high=%.3f", low.Score, high.Score)
	}
	if high.Score-low.Score < 0.20 {
		t.Fatalf("expected confidence to materially affect busy risk, got low=%.3f high=%.3f", low.Score, high.Score)
	}
	if low.Score > 0.25 {
		t.Fatalf("expected low confidence busy signal to stay limited, got %.3f", low.Score)
	}
}

func TestScoreInterruptionRiskDoNotDisturbHardBlocks(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)
	result := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:               now,
		Channel:           "web",
		DoNotDisturbUntil: now.Add(time.Hour),
	})

	if !result.HardBlock {
		t.Fatalf("expected hard block, got %#v", result)
	}
	if result.Score != 1 {
		t.Fatalf("expected max risk score, got %.3f", result.Score)
	}
	if InterruptionRiskAllowsSend(result, 0.95) {
		t.Fatal("expected hard block to prevent sending")
	}
	if !HasInterruptionRiskReason(result, "do_not_disturb") {
		t.Fatalf("expected do_not_disturb reason, got %#v", result.Reasons)
	}
}

func TestScoreInterruptionRiskRecoveryCanLowerRisk(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)
	busy := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:                    now,
		Channel:                "wechat",
		Availability:           InterruptionAvailabilityBusy,
		AvailabilityConfidence: 0.9,
		ConsecutiveUnanswered:  4,
		LastOutputAt:           now.Add(-5 * time.Minute),
		SentCountToday:         2,
		MaxPerDay:              3,
	})
	recovered := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:                    now.Add(3 * time.Hour),
		Channel:                "web",
		Availability:           InterruptionAvailabilityFree,
		AvailabilityConfidence: 0.9,
		ConsecutiveUnanswered:  0,
		SentCountToday:         0,
		MaxPerDay:              3,
	})

	if recovered.Score >= busy.Score {
		t.Fatalf("expected recovery to lower risk, got busy=%.3f recovered=%.3f", busy.Score, recovered.Score)
	}
	if !InterruptionRiskAllowsSend(recovered, 0.65) {
		t.Fatalf("expected recovered state to allow send, got %#v", recovered)
	}
}

func TestScoreInterruptionRiskQuietHoursAndBudget(t *testing.T) {
	now := time.Date(2026, 7, 1, 23, 30, 0, 0, time.UTC)
	result := ScoreInterruptionRisk(InterruptionRiskInput{
		Now:            now,
		Channel:        "qq",
		QuietStart:     "22:00",
		QuietEnd:       "08:00",
		SentCountToday: 1,
		MaxPerDay:      1,
	})

	if result.Score < 0.90 {
		t.Fatalf("expected high risk during quiet hours with exhausted budget, got %.3f", result.Score)
	}
	if !HasInterruptionRiskReason(result, "quiet_hours") || !HasInterruptionRiskReason(result, "daily_budget_exhausted") {
		t.Fatalf("expected quiet and budget reasons, got %#v", result.Reasons)
	}
}
