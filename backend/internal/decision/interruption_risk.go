package decision

import "time"

type TimePeriod string

const (
	TimePeriodLateNight TimePeriod = "late_night"
	TimePeriodWorkHour  TimePeriod = "work_hour"
	TimePeriodEvening   TimePeriod = "evening"
	TimePeriodWeekend   TimePeriod = "weekend"
	TimePeriodNormal    TimePeriod = "normal"
)

type UserActivity string

const (
	UserActivityActive   UserActivity = "active"
	UserActivityIdle     UserActivity = "idle"
	UserActivityBusy     UserActivity = "busy"
	UserActivitySleeping UserActivity = "sleeping"
	UserActivityUnknown  UserActivity = "unknown"
)

type InterruptionRiskInput struct {
	Now              time.Time
	UserActivity     UserActivity
	IntimacyLevel    float64
	RecentInterrupts int
	UserDoNotDisturb bool
}

func DefaultInterruptionRiskInput() InterruptionRiskInput {
	return InterruptionRiskInput{
		Now:              time.Now().UTC(),
		UserActivity:     UserActivityUnknown,
		IntimacyLevel:    0.5,
		RecentInterrupts: 0,
		UserDoNotDisturb: false,
	}
}

func ComputeInterruptionRisk(input InterruptionRiskInput) float64 {
	risk := 0.0
	risk += computeTimePeriodRisk(input.Now)
	risk += computeActivityRisk(input.UserActivity)
	risk += computeIntimacyRisk(input.IntimacyLevel)
	risk += computeRepeatInterruptRisk(input.RecentInterrupts)
	if input.UserDoNotDisturb {
		risk += 30
	}
	if risk < 0 {
		risk = 0
	}
	if risk > 100 {
		risk = 100
	}
	return risk
}

func classifyTimePeriod(now time.Time) TimePeriod {
	hour := now.Hour()
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return TimePeriodWeekend
	}
	if hour >= 0 && hour < 6 {
		return TimePeriodLateNight
	}
	if hour >= 9 && hour < 12 {
		return TimePeriodWorkHour
	}
	if hour >= 14 && hour < 18 {
		return TimePeriodWorkHour
	}
	if hour >= 19 && hour < 22 {
		return TimePeriodEvening
	}
	return TimePeriodNormal
}

func computeTimePeriodRisk(now time.Time) float64 {
	period := classifyTimePeriod(now)
	switch period {
	case TimePeriodLateNight:
		return 40
	case TimePeriodWorkHour:
		return 20
	case TimePeriodEvening:
		return 0
	case TimePeriodWeekend:
		return -10
	case TimePeriodNormal:
		return 10
	default:
		return 10
	}
}

func computeActivityRisk(activity UserActivity) float64 {
	switch activity {
	case UserActivitySleeping:
		return 50
	case UserActivityBusy:
		return 30
	case UserActivityIdle:
		return 5
	case UserActivityActive:
		return 10
	default:
		return 20
	}
}

func computeIntimacyRisk(intimacy float64) float64 {
	if intimacy < 0 {
		intimacy = 0
	}
	if intimacy > 1 {
		intimacy = 1
	}
	return (1.0 - intimacy) * 20
}

func computeRepeatInterruptRisk(recentCount int) float64 {
	base := float64(recentCount) * 5.0
	if base > 50 {
		base = 50
	}
	return base
}
