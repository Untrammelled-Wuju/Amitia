package decision

import (
	"testing"
	"time"
)

func TestComputeInterruptionRiskLateNight(t *testing.T) {
	now := time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)
	input := InterruptionRiskInput{
		Now:           now,
		UserActivity:  UserActivityIdle,
		IntimacyLevel: 0.8,
	}
	risk := ComputeInterruptionRisk(input)
	if risk < 30 {
		t.Fatalf("深夜应返回较高风险值, 实际 %f", risk)
	}
}

func TestComputeInterruptionRiskUserDoNotDisturb(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	input := InterruptionRiskInput{
		Now:              now,
		UserActivity:     UserActivityActive,
		IntimacyLevel:    0.9,
		UserDoNotDisturb: true,
	}
	risk := ComputeInterruptionRisk(input)
	if risk < 30 {
		t.Fatalf("勿扰模式下风险应显著提升, 实际 %f", risk)
	}
}

func TestComputeInterruptionRiskSleepingUser(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	input := InterruptionRiskInput{
		Now:           now,
		UserActivity:  UserActivitySleeping,
		IntimacyLevel: 0.3,
	}
	risk := ComputeInterruptionRisk(input)
	if risk < 50 {
		t.Fatalf("用户睡眠时应返回高打扰风险, 实际 %f", risk)
	}
}

func TestClassifyTimePeriodLateNight(t *testing.T) {
	now := time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)
	period := classifyTimePeriod(now)
	if period != TimePeriodLateNight {
		t.Fatalf("凌晨3点应为深夜时段, 实际 %s", period)
	}
}

func TestClassifyTimePeriodWeekend(t *testing.T) {
	now := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	period := classifyTimePeriod(now)
	if period != TimePeriodWeekend {
		t.Fatalf("周六应为周末时段, 实际 %s", period)
	}
}

func TestComputeRepeatInterruptRisk(t *testing.T) {
	risk := computeRepeatInterruptRisk(5)
	if risk != 25 {
		t.Fatalf("5次应产生25分风险, 实际 %f", risk)
	}
	riskMax := computeRepeatInterruptRisk(20)
	if riskMax != 50 {
		t.Fatalf("应封顶为50, 实际 %f", riskMax)
	}
}
