package schedule

import (
	"context"
	"testing"
	"time"
)

func makeCronDef6Field() *ScheduleContributionDefinition {
	return &ScheduleContributionDefinition{
		ContributionID: "contrib-1",
		ExtensionID:    "com.example",
		ModuleID:       "main",
		ScheduleID:     "sched-1",
		Name:           "test schedule 6-field",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{
				Expression: "0 */5 * * * *",
				Seconds:    true,
			},
		},
		Target: ScheduleTargetDefinition{
			Type:            TargetTypeTool,
			TargetID:        "tool-1",
			IdempotencyMode: IdempotencyModeIdempotent,
		},
		Timezone: "UTC",
		Version:  "1",
	}
}

func TestCalculateNextCron6Field(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	def := makeCronDef6Field()

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("6-field cron should be supported: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time for 6-field cron")
	}
	expected := base.Add(5 * time.Minute)
	if !result.NextScheduledAt.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, *result.NextScheduledAt)
	}
}

func TestParseCron6FieldInvalid(t *testing.T) {
	exprs := []string{
		"* * * *",
		"* * * * * * *",
		"invalid * * * * *",
		"60 * * * * *",
	}
	for _, expr := range exprs {
		def := &ScheduleContributionDefinition{
			Trigger: ScheduleTriggerDefinition{
				Type: TriggerTypeCron,
				Cron: &CronTriggerDefinition{Expression: expr, Seconds: true},
			},
			Timezone: "UTC",
		}
		clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
		calc := NewScheduleCalculator(clock)
		_, err := calc.CalculateNext(def, nil)
		if err == nil {
			t.Fatalf("expected error for invalid 6-field cron: %s", expr)
		}
	}
}

func TestCalculateNextCronInvalid(t *testing.T) {
	def := &ScheduleContributionDefinition{
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "invalid * * * *"},
		},
		Timezone: "UTC",
	}
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	calc := NewScheduleCalculator(clock)
	_, err := calc.CalculateNext(def, nil)
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestCalculateNextIntervalWithAnchor(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 3, 0, 0, time.UTC)
	anchor := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)

	def := &ScheduleContributionDefinition{
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeInterval,
			Interval: &IntervalTriggerDefinition{
				Interval: 5 * time.Minute,
				AnchorAt: anchor,
			},
		},
		Timezone: "UTC",
	}
	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
	expected := anchor.Add(5 * time.Minute)
	if !result.NextScheduledAt.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, *result.NextScheduledAt)
	}
}

func TestCalculateNextTimezone(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)

	def := &ScheduleContributionDefinition{
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "0 9 * * *"},
		},
		Timezone: "Asia/Shanghai",
	}
	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextScheduledAt == nil {
		t.Fatal("expected next scheduled time")
	}
}

func TestCalculateNextDSTSpring(t *testing.T) {
	base := time.Date(2026, 3, 8, 1, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)

	def := &ScheduleContributionDefinition{
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "30 2 * * *"},
		},
		Timezone:        "America/New_York",
		DSTSpringPolicy: DSTSpringSkip,
	}
	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestCalculateNextDSTFall(t *testing.T) {
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)

	def := &ScheduleContributionDefinition{
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "30 1 * * *"},
		},
		Timezone:      "America/New_York",
		DSTFallPolicy: DSTFallFireTwice,
	}
	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestApplyJitterDeterministic(t *testing.T) {
	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-jitter",
		JitterPolicy: ScheduleJitterPolicy{
			Enabled:  true,
			MaxDelay: 30 * time.Second,
			SeedMode: "schedule_id_and_scheduled_at",
		},
	}
	scheduledAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	calc := NewScheduleCalculator(NewFakeClock(scheduledAt))

	first := calc.applyJitter(def, scheduledAt)
	second := calc.applyJitter(def, scheduledAt)
	if !first.Equal(second) {
		t.Fatalf("jitter should be deterministic: got %v and %v", first, second)
	}
	if first.Before(scheduledAt) {
		t.Fatal("jitter should not advance before scheduled time")
	}
}

func TestCalculateNextEffectiveAt(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(base)
	calc := NewScheduleCalculator(clock)
	def := makeCronDef()

	result, err := calc.CalculateNext(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NextEffectiveAt == nil {
		t.Fatal("expected next effective at")
	}
}

func TestMisfireSkip(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc := NewMisfireService(store, clock)

	lastScheduled := time.Date(2026, 1, 1, 10, 55, 0, 0, time.UTC)

	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-misfire",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "*/5 * * * *"},
		},
		Timezone:      "UTC",
		MisfirePolicy: ScheduleMisfirePolicy{Policy: MisfirePolicySkip},
	}
	state := &ScheduleState{
		ScheduleID:      "sched-misfire",
		LastScheduledAt: &lastScheduled,
		Generation:      1,
	}

	detection, err := svc.DetectMisfire(context.Background(), def, state)
	if err != nil {
		t.Fatalf("unexpected detection error: %v", err)
	}

	result, err := svc.ApplyMisfirePolicy(context.Background(), def, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "skip_all" {
		t.Fatalf("expected skip_all, got %s", result.Action)
	}
}

func TestMisfireFireOnce(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc := NewMisfireService(store, clock)

	missed1 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	missed2 := time.Date(2026, 1, 1, 11, 5, 0, 0, time.UTC)
	missed3 := time.Date(2026, 1, 1, 11, 10, 0, 0, time.UTC)
	def := &ScheduleContributionDefinition{
		MisfirePolicy: ScheduleMisfirePolicy{Policy: MisfirePolicyFireOnce},
	}
	detection := &MisfireDetection{
		HasMisfire:   true,
		MissedCount:  3,
		MissedTimes:  []time.Time{missed1, missed2, missed3},
		LatestMissed: &missed3,
	}
	result, err := svc.ApplyMisfirePolicy(context.Background(), def, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "fire_once" {
		t.Fatalf("expected fire_once, got %s", result.Action)
	}
	if result.FireCount != 1 {
		t.Fatalf("expected fire count 1, got %d", result.FireCount)
	}
}

func TestMisfireCatchUpLimited(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc := NewMisfireService(store, clock)

	def := &ScheduleContributionDefinition{
		MisfirePolicy: ScheduleMisfirePolicy{
			Policy:     MisfirePolicyCatchUpLimited,
			MaxCatchUp: 2,
		},
	}
	detection := &MisfireDetection{
		HasMisfire:  true,
		MissedCount: 5,
	}
	result, err := svc.ApplyMisfirePolicy(context.Background(), def, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "catch_up_limited" {
		t.Fatalf("expected catch_up_limited, got %s", result.Action)
	}
	if result.FireCount != 2 {
		t.Fatalf("expected fire count 2, got %d", result.FireCount)
	}
}

func TestMisfireRescheduleFromNow(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc := NewMisfireService(store, clock)

	def := &ScheduleContributionDefinition{
		MisfirePolicy: ScheduleMisfirePolicy{Policy: MisfirePolicyRescheduleFromNow},
	}
	detection := &MisfireDetection{
		HasMisfire:  true,
		MissedCount: 3,
	}
	result, err := svc.ApplyMisfirePolicy(context.Background(), def, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "reschedule_from_now" {
		t.Fatalf("expected reschedule_from_now, got %s", result.Action)
	}
	if !result.Reschedule {
		t.Fatal("expected reschedule to be true")
	}
}

func TestIdempotencyKeyStable(t *testing.T) {
	scheduledAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	key1 := GenerateIdempotencyKey("sched-1", scheduledAt, 1)
	key2 := GenerateIdempotencyKey("sched-1", scheduledAt, 1)
	if key1 != key2 {
		t.Fatalf("idempotency key should be stable: %s vs %s", key1, key2)
	}
	key3 := GenerateIdempotencyKey("sched-1", scheduledAt, 2)
	if key1 == key3 {
		t.Fatal("different generation should produce different key")
	}
}

func TestParseCron6(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"0 */5 * * * *", false},
		{"*/10 * * * * *", false},
		{"0 0 12 * * *", false},
		{"* * * *", true},
		{"* * * * * * *", true},
		{"invalid * * * * *", true},
	}
	for _, tt := range tests {
		sched, err := parseCron6(tt.expr)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseCron6(%q) expected error", tt.expr)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCron6(%q) unexpected error: %v", tt.expr, err)
			continue
		}
		if sched == nil {
			t.Errorf("parseCron6(%q) returned nil sched", tt.expr)
		}
	}
}

func TestCircuitStates(t *testing.T) {
	store := newMemStore()
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := NewFakeClock(baseTime)
	config := DefaultScheduleConfig()
	config.CircuitFailThreshold = 3
	config.CircuitRecoveryAfter = 5 * time.Minute
	svc := NewCircuitService(store, clock, config)

	state, _ := svc.GetState(context.Background(), "sched-circuit")
	if state != CircuitStateClosed {
		t.Fatalf("expected closed, got %s", state)
	}

	_ = svc.RecordFailure(context.Background(), "sched-circuit", "error")
	_ = svc.RecordFailure(context.Background(), "sched-circuit", "error")
	_ = svc.RecordFailure(context.Background(), "sched-circuit", "error")

	open, _ := svc.IsOpen(context.Background(), "sched-circuit")
	if !open {
		t.Fatal("expected circuit to be open after consecutive failures")
	}

	clock.Advance(6 * time.Minute)
	open, _ = svc.IsOpen(context.Background(), "sched-circuit")
	if open {
		t.Fatal("expected circuit to be half-open after recovery period")
	}

	_ = svc.RecordSuccess(context.Background(), "sched-circuit")
	open, _ = svc.IsOpen(context.Background(), "sched-circuit")
	if open {
		t.Fatal("expected circuit to be closed after success in half-open state")
	}
}

func TestOverlapReplaceCancellable(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	calc := NewScheduleCalculator(clock)
	planner := NewTriggerPlanner(store, clock, DefaultScheduleConfig(), calc, NewMisfireService(store, clock))

	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-replace",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "*/5 * * * *"},
		},
		Target: ScheduleTargetDefinition{
			Type: TargetTypeTask,
		},
		Timezone:      "UTC",
		OverlapPolicy: ScheduleOverlapPolicy{Policy: OverlapPolicyReplace},
	}
	state := &ScheduleState{
		ScheduleID: "sched-replace",
		Generation: 1,
	}

	existingRun := &ScheduleRunRecord{
		RunID:      "run-old",
		ScheduleID: "sched-replace",
		Status:     RunStatusRunning,
		TargetType: TargetTypeTask,
	}
	_ = store.PutRun(context.Background(), existingRun)

	decision, err := planner.checkOverlap(def, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != "replace_cancellable" {
		t.Fatalf("expected replace_cancellable, got %s", decision)
	}
}

func TestOverlapReplaceUncancellable(t *testing.T) {
	store := newMemStore()
	clock := NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	calc := NewScheduleCalculator(clock)

	def := &ScheduleContributionDefinition{
		ScheduleID: "sched-replace-unc",
		Trigger: ScheduleTriggerDefinition{
			Type: TriggerTypeCron,
			Cron: &CronTriggerDefinition{Expression: "*/5 * * * *"},
		},
		Target: ScheduleTargetDefinition{
			Type: TargetTypeTool,
		},
		Timezone:      "UTC",
		OverlapPolicy: ScheduleOverlapPolicy{Policy: OverlapPolicyReplace},
	}
	state := &ScheduleState{
		ScheduleID: "sched-replace-unc",
		Generation: 1,
	}

	existingRun := &ScheduleRunRecord{
		RunID:      "run-tool",
		ScheduleID: "sched-replace-unc",
		Status:     RunStatusRunning,
		TargetType: TargetTypeTool,
	}
	_ = store.PutRun(context.Background(), existingRun)

	planner := NewTriggerPlanner(store, clock, DefaultScheduleConfig(), calc, NewMisfireService(store, clock))
	decision, err := planner.checkOverlap(def, state)
	if err == nil {
		t.Fatal("expected error for uncancellable replace")
	}
	if decision != "blocked_uncancellable" {
		t.Fatalf("expected blocked_uncancellable, got %s", decision)
	}
}

func TestInvalidDefinitionTransition(t *testing.T) {
	if IsValidDefinitionTransition(DefinitionStatusUninstalled, DefinitionStatusEnabled) {
		t.Fatal("uninstalled -> enabled should be invalid")
	}
	if IsValidDefinitionTransition(DefinitionStatusExpired, DefinitionStatusEnabled) {
		t.Fatal("expired -> enabled should be invalid")
	}
	if !IsValidDefinitionTransition(DefinitionStatusCreated, DefinitionStatusEnabled) {
		t.Fatal("created -> enabled should be valid")
	}
}
