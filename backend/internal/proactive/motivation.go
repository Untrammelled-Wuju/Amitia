package proactive

import (
	"math"
	"time"
)

type MotivationInput struct {
	IdleDuration           time.Duration
	IntimacyScore          float64
	PendingItems           int
	InitiativeScore        float64
	UnresolvedThreadCount  int
	ProspectiveDueCount    int
	QueueBackpressureLevel BackpressureLevel
}

func ScoreMotivation(input MotivationInput) int {
	idleComponent := scoreIdleMotivation(input.IdleDuration)
	intimacyComponent := scoreIntimacyMotivation(input.IntimacyScore)
	prospectiveComponent := scoreProspectiveMotivation(input.PendingItems)
	initiativeComponent := scoreInitiativeMotivation(input.InitiativeScore)

	unresolvedComponent := scoreUnresolvedThreadsMotivation(input.UnresolvedThreadCount)
	prospectiveDueComponent := scoreProspectiveDueMotivation(input.ProspectiveDueCount)
	backpressureComponent := scoreBackpressureMotivation(input.QueueBackpressureLevel)

	total := idleComponent + intimacyComponent + prospectiveComponent + initiativeComponent + unresolvedComponent + prospectiveDueComponent + backpressureComponent
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}
	return total
}

func scoreIdleMotivation(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	hours := d.Hours()
	raw := 25.0 * math.Log1p(hours) / math.Log1p(72)
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreIntimacyMotivation(intimacy float64) int {
	if intimacy <= 0 {
		return 0
	}
	raw := 25.0 * clamp01(intimacy)
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreProspectiveMotivation(count int) int {
	if count <= 0 {
		return 0
	}
	f := float64(count)
	raw := 25.0 * (1.0 - math.Exp(-f*0.5))
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}

func scoreUnresolvedThreadsMotivation(count int) int {
	if count <= 0 {
		return 0
	}
	f := float64(count)
	raw := 15.0 * (1.0 - math.Exp(-f*0.3))
	if raw < 0 {
		raw = 0
	}
	if raw > 15 {
		raw = 15
	}
	return int(math.Round(raw))
}

func scoreProspectiveDueMotivation(count int) int {
	if count <= 0 {
		return 0
	}
	f := float64(count)
	raw := 20.0 * (1.0 - math.Exp(-f*0.25))
	if raw < 0 {
		raw = 0
	}
	if raw > 20 {
		raw = 20
	}
	return int(math.Round(raw))
}

func scoreBackpressureMotivation(level BackpressureLevel) int {
	switch level {
	case BackpressureFull:
		return -40
	case BackpressureHigh:
		return -25
	case BackpressureMed:
		return -10
	case BackpressureLow:
		return -5
	default:
		return 0
	}
}

func scoreInitiativeMotivation(initiative float64) int {
	if initiative <= 0 {
		return 0
	}
	raw := 25.0 * clamp01(initiative)
	if raw < 0 {
		raw = 0
	}
	if raw > 25 {
		raw = 25
	}
	return int(math.Round(raw))
}
