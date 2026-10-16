package schedule

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestScannerMaintenanceUsesExistingScanClockAndRetriesFailure(t *testing.T) {
	ctx := t.Context()
	clock := NewFakeClock(time.Now().UTC())
	store := newMemStore()
	service, err := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	service.SetMaintenance(func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("maintenance failure")
		}
		return nil
	})
	if _, err := service.scanner.ScanOnce(ctx); err == nil {
		t.Fatal("maintenance error hidden")
	}
	if _, err := service.scanner.ScanOnce(ctx); err != nil || calls != 2 {
		t.Fatalf("failed maintenance not retried: %d %v", calls, err)
	}
	clock.Advance(59 * time.Minute)
	if _, err := service.scanner.ScanOnce(ctx); err != nil || calls != 2 {
		t.Fatalf("maintenance ran too early: %d %v", calls, err)
	}
	clock.Advance(time.Minute)
	if _, err := service.scanner.ScanOnce(ctx); err != nil || calls != 3 {
		t.Fatalf("hourly maintenance did not run: %d %v", calls, err)
	}
}

func TestArchivedTaskRestoresPausedAndPreservesOccurrenceBudgetAcrossUpdates(t *testing.T) {
	ctx := t.Context()
	clock := NewFakeClock(time.Now().UTC())
	store := newMemStore()
	service, err := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	def := &ScheduleContributionDefinition{ScheduleID: "archived", EnabledByDefault: true, Timezone: "UTC", Target: ScheduleTargetDefinition{Type: TargetTypeHostWorkload, TargetID: "reminder"}, Trigger: ScheduleTriggerDefinition{Type: TriggerTypeInterval, Interval: &IntervalTriggerDefinition{AnchorAt: clock.Now().Add(time.Minute), Interval: time.Hour}, MaxOccurrences: 5, RestoredOccurrences: 4}}
	if err := service.InstallArchivedHostDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	state, err := store.GetState(ctx, def.ScheduleID)
	if err != nil || state.Status != DefinitionStatusPaused || state.Enabled || !state.Paused || state.NextScheduledAt != nil {
		t.Fatalf("restored task auto-activated: %+v %v", state, err)
	}
	if count, err := service.CountScheduledTriggers(ctx, def.ScheduleID); err != nil || count != 4 {
		t.Fatalf("restored quota reset: %d %v", count, err)
	}
	if err := service.Resume(ctx, def.ScheduleID, 1); err != nil {
		t.Fatal(err)
	}
	changed := *def
	changed.Name = "edited after restore"
	changed.Trigger.RestoredOccurrences = 0
	if err := service.UpdateHostDefinition(ctx, def.ScheduleID, 2, &changed); err != nil {
		t.Fatal(err)
	}
	if count, err := service.CountScheduledTriggers(ctx, def.ScheduleID); err != nil || count != 4 {
		t.Fatalf("ordinary edit reset restored quota: %d %v", count, err)
	}
	clock.Advance(2 * time.Minute)
	if _, err := service.scanner.ScanOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := service.CountScheduledTriggers(ctx, def.ScheduleID); err != nil || count != 5 {
		t.Fatalf("last allowed occurrence missing: %d %v", count, err)
	}
	state, err = store.GetState(ctx, def.ScheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Pause(ctx, def.ScheduleID, state.Generation); err != nil {
		t.Fatal(err)
	}
	state, err = store.GetState(ctx, def.ScheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Resume(ctx, def.ScheduleID, state.Generation); !errors.Is(err, ErrScheduleExpired) {
		t.Fatalf("exhausted restored task resumed: %v", err)
	}
	if err := service.Uninstall(ctx, def.ScheduleID); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("technical uninstall deleted user task: %v", err)
	}
}

func TestScannerFiresPersistedDueOccurrence(t *testing.T) {
	for _, kind := range []TriggerType{TriggerTypeOneShot, TriggerTypeInterval, TriggerTypeCron} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			clock := NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
			store := newMemStore()
			service, err := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
			if err != nil {
				t.Fatal(err)
			}
			def := &ScheduleContributionDefinition{ScheduleID: "reminder", Timezone: "UTC", EnabledByDefault: true, Trigger: ScheduleTriggerDefinition{Type: kind}, JitterPolicy: ScheduleJitterPolicy{SeedMode: "disabled"}}
			switch kind {
			case TriggerTypeOneShot:
				def.Trigger.OneShot = &OneShotTriggerDefinition{RunAt: clock.Now().Add(time.Minute)}
			case TriggerTypeInterval:
				def.Trigger.Interval = &IntervalTriggerDefinition{AnchorAt: clock.Now(), Interval: time.Minute}
			case TriggerTypeCron:
				def.Trigger.Cron = &CronTriggerDefinition{Expression: "* * * * *"}
			}
			if err := service.InstallDefinition(ctx, def); err != nil {
				t.Fatal(err)
			}
			state, _ := store.GetState(ctx, def.ScheduleID)
			due := *state.NextScheduledAt
			clock.Set(due.Add(5 * time.Second))
			if _, err := service.scanner.ScanOnce(ctx); err != nil {
				t.Fatal(err)
			}
			triggers, _ := service.GetTriggers(ctx, def.ScheduleID, 10)
			if len(triggers) != 1 || !triggers[0].ScheduledAt.Equal(due) {
				t.Fatalf("due occurrence skipped: %+v", triggers)
			}
			_, _ = service.scanner.ScanOnce(ctx)
			triggers, _ = service.GetTriggers(ctx, def.ScheduleID, 10)
			if len(triggers) != 1 {
				t.Fatalf("duplicate trigger: %d", len(triggers))
			}
		})
	}
}

func TestManualRunPreservesRecurrenceCursor(t *testing.T) {
	ctx := context.Background()
	clock := NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	store := newMemStore()
	service, _ := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
	def := &ScheduleContributionDefinition{ScheduleID: "manual", Timezone: "UTC", EnabledByDefault: true, Trigger: ScheduleTriggerDefinition{Type: TriggerTypeInterval, Interval: &IntervalTriggerDefinition{AnchorAt: clock.Now(), Interval: time.Hour}}}
	if err := service.InstallDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	state, _ := store.GetState(ctx, def.ScheduleID)
	expected := *state.NextScheduledAt
	trigger, err := service.RunNow(ctx, def.ScheduleID)
	if err != nil || trigger == nil || !trigger.Manual {
		t.Fatalf("manual trigger: %v %v", trigger, err)
	}
	state, _ = store.GetState(ctx, def.ScheduleID)
	if state.LastScheduledAt != nil || state.NextScheduledAt == nil || !state.NextScheduledAt.Equal(expected) {
		t.Fatalf("manual run consumed scheduled occurrence: %+v", state)
	}
}

func TestCalendarMonthlyPreservesAnchor(t *testing.T) {
	anchor := time.Date(2026, 1, 31, 9, 30, 0, 0, time.UTC)
	clock := NewFakeClock(anchor)
	def := &ScheduleContributionDefinition{Timezone: "UTC", Trigger: ScheduleTriggerDefinition{Type: TriggerTypeCalendar, Calendar: &CalendarTriggerDefinition{Frequency: "monthly", AnchorAt: anchor}}}
	calc := NewScheduleCalculator(clock)
	result, err := calc.CalculateNext(def, &ScheduleState{LastScheduledAt: &anchor})
	if err != nil || result.NextScheduledAt == nil || !result.NextScheduledAt.Equal(time.Date(2026, 2, 28, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("February: %v %v", result, err)
	}
	clock.Set(*result.NextScheduledAt)
	result, err = calc.CalculateNext(def, &ScheduleState{LastScheduledAt: result.NextScheduledAt})
	if err != nil || result.NextScheduledAt == nil || !result.NextScheduledAt.Equal(time.Date(2026, 3, 31, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("March anchor drift: %v %v", result, err)
	}
}

func TestCalendarDSTGapAndFold(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                  string
		anchor, now, expected time.Time
	}{
		{"gap", time.Date(2026, 3, 7, 2, 30, 0, 0, loc), time.Date(2026, 3, 8, 0, 0, 0, 0, loc), time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)},
		{"fold", time.Date(2026, 10, 31, 1, 30, 0, 0, loc), time.Date(2026, 11, 1, 0, 0, 0, 0, loc), time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)},
	} {
		t.Run(test.name, func(t *testing.T) {
			clock := NewFakeClock(test.now)
			def := &ScheduleContributionDefinition{Timezone: loc.String(), Trigger: ScheduleTriggerDefinition{Type: TriggerTypeCalendar, Calendar: &CalendarTriggerDefinition{Frequency: "daily", AnchorAt: test.anchor}}}
			result, err := NewScheduleCalculator(clock).CalculateNext(def, nil)
			if err != nil || result.NextScheduledAt == nil || !result.NextScheduledAt.Equal(test.expected) {
				t.Fatalf("DST: %v %v", result, err)
			}
			clock.Set(test.expected.Add(15 * time.Minute))
			result, err = NewScheduleCalculator(clock).CalculateNext(def, &ScheduleState{LastScheduledAt: &test.expected})
			if err != nil || result.NextScheduledAt == nil || result.NextScheduledAt.In(loc).Day() == test.expected.In(loc).Day() {
				t.Fatalf("duplicate wall clock occurrence: %v %v", result, err)
			}
		})
	}
}

func TestInvalidDefinitionDoesNotPersist(t *testing.T) {
	store := newMemStore()
	service, _ := NewScheduleService(ScheduleDeps{Store: store})
	def := &ScheduleContributionDefinition{ScheduleID: "bad", Timezone: "UTC", EnabledByDefault: true, Trigger: ScheduleTriggerDefinition{Type: TriggerTypeCron, Cron: &CronTriggerDefinition{Expression: "invalid"}}}
	if err := service.InstallDefinition(context.Background(), def); err == nil {
		t.Fatal("invalid trigger accepted")
	}
	if _, err := store.GetDefinition(context.Background(), def.ScheduleID); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("invalid definition persisted: %v", err)
	}
}

func TestDefinitionHashIncludesPayloadAndPolicies(t *testing.T) {
	def := &ScheduleContributionDefinition{ScheduleID: "hash"}
	original := GenerateDefinitionHash(def)
	def.Target.InputTemplate = []byte(`{"characterId":"role"}`)
	if original == GenerateDefinitionHash(def) {
		t.Fatal("payload excluded from hash")
	}
	original = GenerateDefinitionHash(def)
	end := time.Now()
	def.EndAt = &end
	if original == GenerateDefinitionHash(def) {
		t.Fatal("end condition excluded from hash")
	}
}

func TestDefinitionHashSurvivesStoredTimeNormalization(t *testing.T) {
	local := time.FixedZone("local", 8*60*60)
	start := time.Date(2026, 10, 14, 8, 0, 0, 0, local)
	def := &ScheduleContributionDefinition{ScheduleID: "stable-hash", StartAt: &start, EnabledByDefault: true}
	expected := GenerateDefinitionHash(def)
	utc := start.UTC()
	def.StartAt = &utc
	def.EnabledByDefault = false
	if expected != GenerateDefinitionHash(def) {
		t.Fatal("persisted definition hash changed after SQLite normalization")
	}
}

func TestFirstOneShotRecoversAfterRestart(t *testing.T) {
	ctx := context.Background()
	clock := NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	store := newMemStore()
	service, _ := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
	due := clock.Now().Add(time.Minute)
	def := &ScheduleContributionDefinition{ScheduleID: "restart", Timezone: "UTC", EnabledByDefault: true, Trigger: ScheduleTriggerDefinition{Type: TriggerTypeOneShot, OneShot: &OneShotTriggerDefinition{RunAt: due}}, MisfirePolicy: ScheduleMisfirePolicy{Policy: MisfirePolicyFireOnce, GracePeriod: 24 * time.Hour}}
	if err := service.InstallDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	clock.Set(due.Add(time.Hour))
	if err := service.recovery.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	triggers, _ := service.GetTriggers(ctx, def.ScheduleID, 10)
	if len(triggers) != 1 || !triggers[0].ScheduledAt.Equal(due) {
		t.Fatalf("first occurrence lost at restart: %+v", triggers)
	}
	if err := service.recovery.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	triggers, _ = service.GetTriggers(ctx, def.ScheduleID, 10)
	if len(triggers) != 1 {
		t.Fatalf("duplicate recovered trigger: %d", len(triggers))
	}
}

func TestMisfireBeyondGracePeriodDoesNotSend(t *testing.T) {
	ctx := context.Background()
	clock := NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	store := newMemStore()
	service, _ := NewScheduleService(ScheduleDeps{Store: store, Clock: clock})
	due := clock.Now().Add(time.Minute)
	def := &ScheduleContributionDefinition{ScheduleID: "expired-window", Timezone: "UTC", EnabledByDefault: true, Trigger: ScheduleTriggerDefinition{Type: TriggerTypeOneShot, OneShot: &OneShotTriggerDefinition{RunAt: due}}, MisfirePolicy: ScheduleMisfirePolicy{Policy: MisfirePolicyFireOnce, GracePeriod: 24 * time.Hour}}
	if err := service.InstallDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	clock.Set(due.Add(25 * time.Hour))
	if err := service.recovery.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	triggers, _ := service.GetTriggers(ctx, def.ScheduleID, 10)
	if len(triggers) != 0 {
		t.Fatalf("expired occurrence sent: %+v", triggers)
	}
	misfires, _ := service.GetMisfires(ctx, def.ScheduleID, 10)
	if len(misfires) != 1 || misfires[0].Action != "expired_window" {
		t.Fatalf("missing expired result: %+v", misfires)
	}
}
