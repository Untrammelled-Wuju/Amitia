package proactive

import (
	"math"
	"strings"
	"time"
)

type InterruptionAvailability string

const (
	InterruptionAvailabilityUnknown InterruptionAvailability = "unknown"
	InterruptionAvailabilityFree    InterruptionAvailability = "free"
	InterruptionAvailabilityBusy    InterruptionAvailability = "busy"
)

type InterruptionRiskInput struct {
	Now                    time.Time
	Channel                string
	Availability           InterruptionAvailability
	IdleDuration           time.Duration
	AvailabilityConfidence float64
	ConsecutiveUnanswered  int
	LastOutputAt           time.Time
	QuietStart             string
	QuietEnd               string
	SentCountToday         int
	MaxPerDay              int
	DoNotDisturbUntil      time.Time
}

type InterruptionRiskResult struct {
	Score     float64
	HardBlock bool
	Reasons   []string
}

func ScoreInterruptionRisk(input InterruptionRiskInput) InterruptionRiskResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	confidence := clamp01(input.AvailabilityConfidence)
	score := 0.10
	reasons := make([]string, 0, 6)

	if !input.DoNotDisturbUntil.IsZero() && now.Before(input.DoNotDisturbUntil) {
		return InterruptionRiskResult{
			Score:     1,
			HardBlock: true,
			Reasons:   []string{"do_not_disturb"},
		}
	}

	if input.QuietStart != "" && input.QuietEnd != "" && !quietHoursAllow(input.QuietStart, input.QuietEnd, now.Format("15:04")) {
		score += 0.45
		reasons = append(reasons, "quiet_hours")
	}

	switch input.Availability {
	case InterruptionAvailabilityBusy:
		score += 0.35 * availabilityWeight(confidence)
		reasons = append(reasons, "availability_busy")
	case InterruptionAvailabilityFree:
		score -= 0.18 * availabilityWeight(confidence)
		reasons = append(reasons, "availability_free")
	}

	if input.ConsecutiveUnanswered > 0 {
		score += unansweredRisk(input.ConsecutiveUnanswered)
		reasons = append(reasons, "recent_unanswered")
	}

	if !input.LastOutputAt.IsZero() {
		elapsed := now.Sub(input.LastOutputAt)
		if elapsed >= 0 && elapsed < 30*time.Minute {
			score += (1 - elapsed.Minutes()/30) * 0.22
			reasons = append(reasons, "recent_output")
		}
	}

	if input.IdleDuration > 0 {
		idleRisk := IdleDurationRisk(input.IdleDuration)
		score += idleRisk
		reasons = append(reasons, "idle_duration")
	}

	if input.MaxPerDay > 0 && input.SentCountToday >= input.MaxPerDay {
		score += 0.40
		reasons = append(reasons, "daily_budget_exhausted")
	} else if input.MaxPerDay > 0 && input.SentCountToday == input.MaxPerDay-1 {
		score += 0.14
		reasons = append(reasons, "daily_budget_low")
	}

	switch primaryConversationChannel(input.Channel) {
	case "wechat", "qq":
		score += 0.08
		reasons = append(reasons, "external_channel")
	}

	score = roundRiskScore(clamp01(score))
	return InterruptionRiskResult{Score: score, HardBlock: false, Reasons: reasons}
}

func availabilityWeight(confidence float64) float64 {
	return 0.25 + clamp01(confidence)*0.75
}

func unansweredRisk(count int) float64 {
	if count <= 0 {
		return 0
	}
	risk := 0.10 * math.Log2(float64(count)+1)
	if risk > 0.36 {
		return 0.36
	}
	return risk
}

func roundRiskScore(score float64) float64 {
	return math.Round(score*1000) / 1000
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	if math.IsNaN(value) {
		return 0
	}
	return value
}

func InterruptionRiskAllowsSend(result InterruptionRiskResult, threshold float64) bool {
	if result.HardBlock {
		return false
	}
	if threshold <= 0 {
		threshold = 0.65
	}
	return result.Score < threshold
}

func HasInterruptionRiskReason(result InterruptionRiskResult, reason string) bool {
	reason = strings.TrimSpace(reason)
	for _, item := range result.Reasons {
		if item == reason {
			return true
		}
	}
	return false
}