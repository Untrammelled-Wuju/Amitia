package proactive

import (
	"math"
	"time"
)

type BudgetTracker struct {
	DailyLimit            int
	CooldownDuration      time.Duration
	BaseInterval          time.Duration
	SentToday             int
	LastSentAt            time.Time
	ConsecutiveUnanswered int
}

func NewBudgetTracker(dailyLimit int, cooldownDuration time.Duration, baseInterval time.Duration) *BudgetTracker {
	return &BudgetTracker{
		DailyLimit:       dailyLimit,
		CooldownDuration: cooldownDuration,
		BaseInterval:     baseInterval,
	}
}

func (b *BudgetTracker) CanSend(now time.Time) bool {
	if b.SentToday >= b.DailyLimit {
		return false
	}

	if !b.LastSentAt.IsZero() {
		effectiveCooldown := DecayNextDelay(b.ConsecutiveUnanswered, b.CooldownDuration)
		if now.Sub(b.LastSentAt) < effectiveCooldown {
			return false
		}
	}

	return true
}

func (b *BudgetTracker) MarkSent(now time.Time) {
	b.SentToday++
	b.LastSentAt = now
}

func (b *BudgetTracker) MarkUnanswered() {
	b.ConsecutiveUnanswered++
}

func (b *BudgetTracker) OnUserReply() {
	b.ConsecutiveUnanswered = 0
}

func (b *BudgetTracker) ResetDaily() {
	b.SentToday = 0
}

func DecayNextDelay(consecutiveUnanswered int, baseCooldown time.Duration) time.Duration {
	if consecutiveUnanswered <= 0 {
		return baseCooldown
	}

	multiplier := 1.0 + math.Log2(float64(consecutiveUnanswered)+1)*0.8
	if multiplier > 10 {
		multiplier = 10
	}

	delay := time.Duration(float64(baseCooldown) * multiplier)
	if delay > 24*time.Hour {
		delay = 24 * time.Hour
	}
	return delay
}

func IsBudgetExhausted(sentToday, dailyLimit int) bool {
	return sentToday >= dailyLimit
}
