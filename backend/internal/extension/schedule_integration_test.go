package extension

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func makeTestScheduleDef() *schedule.ScheduleContributionDefinition {
	anchor := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return &schedule.ScheduleContributionDefinition{
		ContributionID: "contrib-test",
		ExtensionID:    "com.test/schedule",
		ModuleID:       "main",
		ScheduleID:     "sched-test-1",
		Name:           "Test Schedule",
		Description:    "Test schedule for integration",
		Trigger: schedule.ScheduleTriggerDefinition{
			Type: schedule.TriggerTypeInterval,
			Interval: &schedule.IntervalTriggerDefinition{
				Interval: 5 * time.Minute,
				AnchorAt: anchor,
			},
		},
		Target: schedule.ScheduleTargetDefinition{
			Type:            schedule.TargetTypeTool,
			TargetID:        "tool.echo",
			IdempotencyMode: schedule.IdempotencyModeIdempotent,
		},
		Timezone:         "UTC",
		EnabledByDefault: true,
	}
}

func makeTestScheduleService(t *testing.T) (*schedule.ScheduleService, *schedule.FakeClock, *memScheduleStore) {
	t.Helper()
	store := newMemScheduleStore()
	clock := schedule.NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc, err := schedule.NewScheduleService(schedule.ScheduleDeps{
		Store: store,
		Clock: clock,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc, clock, store
}

func TestScheduleServiceInstallAndGet(t *testing.T) {
	store := newMemScheduleStore()
	clock := schedule.NewFakeClock(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	svc, err := schedule.NewScheduleService(schedule.ScheduleDeps{
		Store: store,
		Clock: clock,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	gotDef, gotState, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if gotDef.Name != "Test Schedule" {
		t.Fatalf("expected name 'Test Schedule', got %s", gotDef.Name)
	}
	if gotState.Status != schedule.DefinitionStatusEnabled {
		t.Fatalf("expected enabled, got %s", gotState.Status)
	}
	if gotState.NextScheduledAt == nil {
		t.Fatal("expected next scheduled at")
	}

	expected := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	if !gotState.NextScheduledAt.Equal(expected) {
		t.Fatalf("expected next scheduled at %v, got %v", expected, *gotState.NextScheduledAt)
	}
}

func TestScheduleServiceEnableDisable(t *testing.T) {
	svc, _, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := svc.Disable(context.Background(), "sched-test-1", 1); err != nil {
		t.Fatalf("disable: %v", err)
	}
	_, state, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after disable: %v", err)
	}
	if state.Status != schedule.DefinitionStatusDisabled {
		t.Fatalf("expected disabled, got %s", state.Status)
	}
	if state.NextScheduledAt != nil {
		t.Fatalf("expected nil next scheduled at after disable, got %v", *state.NextScheduledAt)
	}

	if err := svc.Enable(context.Background(), "sched-test-1", 1); err != nil {
		t.Fatalf("enable: %v", err)
	}
	_, state, err = svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after enable: %v", err)
	}
	if state.Status != schedule.DefinitionStatusEnabled {
		t.Fatalf("expected enabled, got %s", state.Status)
	}
	if state.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at after enable")
	}

	if err := svc.Pause(context.Background(), "sched-test-1", 1); err != nil {
		t.Fatalf("pause: %v", err)
	}
	_, state, err = svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after pause: %v", err)
	}
	if state.Status != schedule.DefinitionStatusPaused {
		t.Fatalf("expected paused, got %s", state.Status)
	}
	if state.NextScheduledAt != nil {
		t.Fatalf("expected nil next scheduled at after pause, got %v", *state.NextScheduledAt)
	}

	if err := svc.Resume(context.Background(), "sched-test-1", 1); err != nil {
		t.Fatalf("resume: %v", err)
	}
	_, state, err = svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after resume: %v", err)
	}
	if state.Status != schedule.DefinitionStatusEnabled {
		t.Fatalf("expected enabled after resume, got %s", state.Status)
	}
	if state.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at after resume")
	}
}

func TestScheduleServiceRunNow(t *testing.T) {
	svc, _, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	trigger, err := svc.RunNow(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("run now: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected non-nil trigger record")
	}
	if trigger.ScheduleID != "sched-test-1" {
		t.Fatalf("expected schedule id 'sched-test-1', got %s", trigger.ScheduleID)
	}
	if trigger.TriggerID == "" {
		t.Fatal("expected non-empty trigger id")
	}
	if trigger.Status != schedule.RunStatusWaiting {
		t.Fatalf("expected waiting status, got %s", trigger.Status)
	}
	if !trigger.Manual {
		t.Fatal("expected manual trigger")
	}

	triggers, err := svc.GetTriggers(context.Background(), "sched-test-1", 10)
	if err != nil {
		t.Fatalf("get triggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}

	_, state, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after run now: %v", err)
	}
	if state.LastScheduledAt == nil {
		t.Fatal("expected non-nil last scheduled at after run now")
	}
}

func TestScheduleServiceSkipNext(t *testing.T) {
	svc, _, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	_, stateBefore, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if stateBefore.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at before skip")
	}
	originalNext := *stateBefore.NextScheduledAt

	if err := svc.SkipNext(context.Background(), "sched-test-1"); err != nil {
		t.Fatalf("skip next: %v", err)
	}

	_, stateAfter, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after skip: %v", err)
	}
	if stateAfter.LastScheduledAt == nil {
		t.Fatal("expected non-nil last scheduled at after skip")
	}
	if !stateAfter.LastScheduledAt.Equal(originalNext) {
		t.Fatalf("expected last scheduled at %v, got %v", originalNext, *stateAfter.LastScheduledAt)
	}
	if stateAfter.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at after skip")
	}
	if stateAfter.NextScheduledAt.Equal(originalNext) {
		t.Fatalf("expected next scheduled at to differ from skipped time %v, got %v", originalNext, *stateAfter.NextScheduledAt)
	}

	triggers, err := svc.GetTriggers(context.Background(), "sched-test-1", 10)
	if err != nil {
		t.Fatalf("get triggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Status != schedule.RunStatusSkipped {
		t.Fatalf("expected skipped status, got %s", triggers[0].Status)
	}
}

func TestScheduleServiceRecalculate(t *testing.T) {
	svc, clock, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	_, stateBefore, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if stateBefore.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at before recalculate")
	}
	originalNext := *stateBefore.NextScheduledAt

	clock.Advance(7 * time.Minute)

	if err := svc.Recalculate(context.Background(), "sched-test-1"); err != nil {
		t.Fatalf("recalculate: %v", err)
	}

	_, stateAfter, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule after recalculate: %v", err)
	}
	if stateAfter.NextScheduledAt == nil {
		t.Fatal("expected non-nil next scheduled at after recalculate")
	}
	if stateAfter.NextScheduledAt.Equal(originalNext) {
		t.Fatalf("expected next scheduled at to change after recalculate, was %v, still %v", originalNext, *stateAfter.NextScheduledAt)
	}
	if !stateAfter.NextScheduledAt.After(clock.Now()) {
		t.Fatalf("expected next scheduled at after now %v, got %v", clock.Now(), *stateAfter.NextScheduledAt)
	}
}

func TestScheduleServiceUninstall(t *testing.T) {
	svc, _, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := svc.Uninstall(context.Background(), "sched-test-1"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	_, _, err := svc.GetSchedule(context.Background(), "sched-test-1")
	if !errors.Is(err, schedule.ErrScheduleNotFound) {
		t.Fatalf("expected ErrScheduleNotFound, got %v", err)
	}

	state, err := svc.GetScheduleState(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get schedule state after uninstall: %v", err)
	}
	if state.Status != schedule.DefinitionStatusUninstalled {
		t.Fatalf("expected uninstalled status, got %s", state.Status)
	}
	if state.NextScheduledAt != nil {
		t.Fatalf("expected nil next scheduled at after uninstall, got %v", *state.NextScheduledAt)
	}
}

func TestScheduleServiceCircuitReset(t *testing.T) {
	svc, _, _ := makeTestScheduleService(t)
	def := makeTestScheduleDef()

	if err := svc.InstallDefinition(context.Background(), def); err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := svc.ResetCircuit(context.Background(), "sched-test-1"); err != nil {
		t.Fatalf("reset circuit: %v", err)
	}

	circuit, err := svc.GetCircuit(context.Background(), "sched-test-1")
	if err != nil {
		t.Fatalf("get circuit: %v", err)
	}
	if circuit == nil {
		t.Fatal("expected non-nil circuit record after reset")
	}
	if circuit.State != schedule.CircuitStateClosed {
		t.Fatalf("expected closed circuit state, got %s", circuit.State)
	}
	if circuit.ConsecutiveFails != 0 {
		t.Fatalf("expected 0 consecutive fails, got %d", circuit.ConsecutiveFails)
	}
}
