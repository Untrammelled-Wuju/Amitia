package proactive

import (
	"math"
	"time"
)

type SuppressionInput struct {
	UnifiedState    UnifiedState
	LastUserReplyAt time.Time
	RecentSentCount int
	UserActiveNow   bool
}

func ScoreSuppression(input SuppressionInput) int {
	fatigueComponent := scoreFatigueSuppression(input.UnifiedState)
	lastReplyComponent := scoreLastReplySuppression(input.LastUserReplyAt)
	sentCountComponent := scoreSentCountSuppression(input.RecentSentCount)
	userActiveComponent := scoreUserActiveSuppression(input.UserActiveNow)

	total := fatigueComponent + lastReplyComponent + sentCountComponent + userActiveComponent
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}
	return total
}

func scoreFatigueSuppression(us UnifiedState) int {
	if us.Busy {
		return 25
	}
	raw := 25.0 * us.Fatigue
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreLastReplySuppression(lastUserReplyAt time.Time) int {
	if lastUserReplyAt.IsZero() {
		return 0
	}
	elapsed := time.Since(lastUserReplyAt)
	if elapsed < 0 {
		return 0
	}
	minutes := elapsed.Minutes()
	if minutes >= 60 {
		return 0
	}
	raw := 25.0 * (1.0 - minutes/60.0)
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreSentCountSuppression(recentSentCount int) int {
	if recentSentCount <= 0 {
		return 0
	}
	f := float64(recentSentCount)
	raw := 25.0 * (1.0 - math.Exp(-f*0.4))
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreUserActiveSuppression(userActiveNow bool) int {
	if userActiveNow {
		return 0
	}
	return 25
}
