package proactive

import (
	"testing"
	"time"
)

func TestScoreMotivationIdleMonotonic(t *testing.T) {
	short := ScoreMotivation(MotivationInput{
		IdleDuration:    1 * time.Hour,
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	mid := ScoreMotivation(MotivationInput{
		IdleDuration:    12 * time.Hour,
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	lg := ScoreMotivation(MotivationInput{
		IdleDuration:    48 * time.Hour,
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	if !(short <= mid && mid <= lg) {
		t.Fatalf("expected idle motivation to be monotonic, got %d %d %d", short, mid, lg)
	}
}

func TestScoreMotivationIntimacyBounded(t *testing.T) {
	low := ScoreMotivation(MotivationInput{
		IntimacyScore:   0.1,
		InitiativeScore: 0.5,
	})
	high := ScoreMotivation(MotivationInput{
		IntimacyScore:   0.9,
		InitiativeScore: 0.5,
	})
	if low >= high {
		t.Fatalf("expected higher intimacy to produce higher motivation, got %d %d", low, high)
	}
}

func TestScoreMotivationProspectiveSat(t *testing.T) {
	none := ScoreMotivation(MotivationInput{
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	some := ScoreMotivation(MotivationInput{
		PendingItems:    5,
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	many := ScoreMotivation(MotivationInput{
		PendingItems:    20,
		IntimacyScore:   0.5,
		InitiativeScore: 0.5,
	})
	if !(none <= some && some <= many) {
		t.Fatalf("expected prospective to be monotonic, got %d %d %d", none, some, many)
	}
	gain1 := some - none
	gain2 := many - some
	if gain2 >= gain1 {
		t.Fatalf("expected saturation: gain 0->5 (%d) > gain 5->20 (%d)", gain1, gain2)
	}
}

func TestScoreMotivationInitiativeRange(t *testing.T) {
	low := ScoreMotivation(MotivationInput{
		IntimacyScore:   0.5,
		InitiativeScore: 0.1,
	})
	high := ScoreMotivation(MotivationInput{
		IntimacyScore:   0.5,
		InitiativeScore: 0.9,
	})
	if low >= high {
		t.Fatalf("expected higher initiative to produce higher motivation, got %d %d", low, high)
	}
}

func TestScoreMotivationAllComponentsBounded(t *testing.T) {
	full := ScoreMotivation(MotivationInput{
		IdleDuration:    72 * time.Hour,
		IntimacyScore:   1.0,
		PendingItems:    100,
		InitiativeScore: 1.0,
	})
	if full > 100 {
		t.Fatalf("expected max motivation to cap at 100, got %d", full)
	}
	zero := ScoreMotivation(MotivationInput{})
	if zero < 0 {
		t.Fatalf("expected min motivation to be >=0, got %d", zero)
	}
}

func TestScoreSuppressionFatigueBusy(t *testing.T) {
	rested := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 1.0, Fatigue: 0.0, Busy: false},
		UserActiveNow: true,
	})
	fatigued := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 0.3, Fatigue: 0.8, Busy: false},
		UserActiveNow: true,
	})
	busy := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 0.5, Fatigue: 0.5, Busy: true},
		UserActiveNow: true,
	})
	if rested >= fatigued {
		t.Fatalf("expected fatigued to have higher suppression than rested, got %d %d", rested, fatigued)
	}
	if busy < 20 {
		t.Fatalf("expected busy state to have substantial suppression, got %d", busy)
	}
}

func TestScoreSuppressionLastReplyDecay(t *testing.T) {
	now := time.Now()
	recent := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		LastUserReplyAt: now.Add(-5 * time.Minute),
		UserActiveNow:   true,
	})
	older := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		LastUserReplyAt: now.Add(-45 * time.Minute),
		UserActiveNow:   true,
	})
	stale := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		LastUserReplyAt: now.Add(-2 * time.Hour),
		UserActiveNow:   true,
	})
	if !(recent > older && older >= stale) {
		t.Fatalf("expected suppression to decrease with time since last reply, got %d %d %d", recent, older, stale)
	}
}

func TestScoreSuppressionSentCount(t *testing.T) {
	none := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		UserActiveNow: true,
	})
	several := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		RecentSentCount: 5,
		UserActiveNow:   true,
	})
	many := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		RecentSentCount: 15,
		UserActiveNow:   true,
	})
	if !(none <= several && several <= many) {
		t.Fatalf("expected sent count to increase suppression, got %d %d %d", none, several, many)
	}
}

func TestScoreSuppressionUserInactive(t *testing.T) {
	active := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		UserActiveNow: true,
	})
	inactive := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 1.0, Fatigue: 0, Busy: false},
		UserActiveNow: false,
	})
	if active >= inactive {
		t.Fatalf("expected inactive user to produce higher suppression, got %d %d", active, inactive)
	}
}

func TestScoreSuppressionCombinedCap(t *testing.T) {
	full := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 0.1, Fatigue: 1.0, Busy: true},
		LastUserReplyAt: time.Now(),
		RecentSentCount: 100,
		UserActiveNow:   false,
	})
	if full > 100 {
		t.Fatalf("expected max suppression to cap at 100, got %d", full)
	}
}

func TestBudgetTrackerDailyCap(t *testing.T) {
	bt := NewBudgetTracker(3, 10*time.Minute, 30*time.Minute)
	now := time.Now()
	if !bt.CanSend(now) {
		t.Fatal("expected to be able to send initially")
	}
	bt.MarkSent(now)
	if bt.SentToday != 1 {
		t.Fatalf("expected sent count 1, got %d", bt.SentToday)
	}
	bt.SentToday = 3
	if bt.CanSend(now) {
		t.Fatal("expected budget exhausted to block sending")
	}
	if !IsBudgetExhausted(bt.SentToday, bt.DailyLimit) {
		t.Fatal("expected budget exhausted")
	}
	bt.ResetDaily()
	if bt.SentToday != 0 {
		t.Fatalf("expected reset to zero sent count, got %d", bt.SentToday)
	}
}

func TestBudgetTrackerCooldown(t *testing.T) {
	bt := NewBudgetTracker(10, 10*time.Minute, 30*time.Minute)
	now := time.Now()
	bt.MarkSent(now)
	if bt.CanSend(now) {
		t.Fatal("expected cooldown to block immediate re-send")
	}
	if bt.CanSend(now.Add(5 * time.Minute)) {
		t.Fatal("expected cooldown to block re-send within cooldown duration")
	}
	if !bt.CanSend(now.Add(11 * time.Minute)) {
		t.Fatal("expected send allowed after cooldown expires")
	}
}

func TestBudgetTrackerConsecutiveUnansweredIncreasesCooldown(t *testing.T) {
	bt := NewBudgetTracker(10, 10*time.Minute, 30*time.Minute)
	now := time.Now()
	bt.MarkSent(now)
	if bt.CanSend(now.Add(11 * time.Minute)) {
		bt.MarkSent(now.Add(11 * time.Minute))
	}
	bt.MarkUnanswered()
	delayAfterOne := DecayNextDelay(bt.ConsecutiveUnanswered, bt.CooldownDuration)
	bt.MarkUnanswered()
	delayAfterTwo := DecayNextDelay(bt.ConsecutiveUnanswered, bt.CooldownDuration)
	if delayAfterTwo <= delayAfterOne {
		t.Fatalf("expected delay to increase with consecutive unanswered, got %v %v", delayAfterOne, delayAfterTwo)
	}
}

func TestDecayNextDelayMax(t *testing.T) {
	base := 10 * time.Minute
	delay0 := DecayNextDelay(0, base)
	if delay0 != base {
		t.Fatalf("expected 0 unanswered to return base delay, got %v", delay0)
	}
	delayMany := DecayNextDelay(10000, base)
	if delayMany > 24*time.Hour {
		t.Fatalf("expected delay capped at 24h, got %v", delayMany)
	}
}

func TestBudgetTrackerOnUserReplyResets(t *testing.T) {
	bt := NewBudgetTracker(10, 10*time.Minute, 30*time.Minute)
	bt.ConsecutiveUnanswered = 5
	bt.OnUserReply()
	if bt.ConsecutiveUnanswered != 0 {
		t.Fatalf("expected user reply to reset consecutive unanswered, got %d", bt.ConsecutiveUnanswered)
	}
}

func TestMotivationExceedsSuppressionAllowsSend(t *testing.T) {
	mot := ScoreMotivation(MotivationInput{
		IdleDuration:    12 * time.Hour,
		IntimacyScore:   0.8,
		PendingItems:    3,
		InitiativeScore: 0.7,
	})
	sup := ScoreSuppression(SuppressionInput{
		UnifiedState:  UnifiedState{Energy: 0.9, Fatigue: 0.1, Busy: false},
		UserActiveNow: true,
	})
	if mot <= sup {
		t.Fatalf("expected motivation (%d) to exceed suppression (%d) for send decision", mot, sup)
	}
}

func TestSuppressionExceedsMotivationBlocksSend(t *testing.T) {
	mot := ScoreMotivation(MotivationInput{
		IdleDuration:    30 * time.Minute,
		IntimacyScore:   0.2,
		InitiativeScore: 0.2,
	})
	sup := ScoreSuppression(SuppressionInput{
		UnifiedState:    UnifiedState{Energy: 0.2, Fatigue: 0.9, Busy: true},
		LastUserReplyAt: time.Now(),
		RecentSentCount: 10,
		UserActiveNow:   false,
	})
	if mot > sup {
		t.Fatalf("expected suppression (%d) to exceed motivation (%d) for block decision", sup, mot)
	}
}
