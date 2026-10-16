package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestScheduleInstallRollsBackDefinitionOnStateFailure(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TRIGGER reject_schedule_state BEFORE INSERT ON extension_schedule_states BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	repo := NewScheduleRepository(db)
	clock := schedule.NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	service, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	def := &schedule.ScheduleContributionDefinition{ScheduleID: "atomic", EnabledByDefault: true, Timezone: "UTC", Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: clock.Now().Add(time.Hour)}}}
	if err := service.InstallDefinition(ctx, def); err == nil {
		t.Fatal("state write failure reported as success")
	}
	if _, err := repo.GetDefinition(ctx, def.ScheduleID); !errors.Is(err, schedule.ErrScheduleNotFound) {
		t.Fatalf("definition survived rollback: %v", err)
	}
}

func TestScheduleConcurrentVersionMutationHasOneWinner(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	repo := NewScheduleRepository(db)
	clock := schedule.NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	service, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	def := &schedule.ScheduleContributionDefinition{ScheduleID: "concurrent", EnabledByDefault: true, Timezone: "UTC", Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: clock.Now().Add(time.Hour)}}}
	if err := service.InstallDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- service.Pause(ctx, def.ScheduleID, 1)
		}()
	}
	wg.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, schedule.ErrGenerationMismatch) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
}

func TestScheduleTerminalTriggerCannotBeLeasedAgain(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	repo := NewScheduleRepository(db)
	now := time.Now().UTC()
	trigger := &schedule.ScheduleTriggerRecord{TriggerID: "completed", ScheduleID: "schedule", ScheduledAt: now, EffectiveAt: now, IdempotencyKey: "one-occurrence", Status: schedule.RunStatusWaiting, Generation: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.PutTrigger(ctx, trigger); err != nil {
		t.Fatal(err)
	}
	if acquired, err := repo.AcquireTriggerLease(ctx, trigger.TriggerID, "first", now.Add(time.Hour)); err != nil || !acquired {
		t.Fatalf("initial lease: %v %v", acquired, err)
	}
	if err := repo.UpdateTriggerStatus(ctx, trigger.TriggerID, schedule.RunStatusCompleted, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReleaseTriggerLease(ctx, trigger.TriggerID); err != nil {
		t.Fatal(err)
	}
	if acquired, err := repo.AcquireTriggerLease(ctx, trigger.TriggerID, "stale-worker", now.Add(time.Hour)); err != nil || acquired {
		t.Fatalf("terminal trigger reacquired: %v %v", acquired, err)
	}
}

func TestScheduleUpdateRollsBackPayloadAndGeneration(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	repo := NewScheduleRepository(db)
	clock := schedule.NewFakeClock(time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC))
	service, _ := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Clock: clock})
	def := &schedule.ScheduleContributionDefinition{ScheduleID: "update-rollback", Name: "original", EnabledByDefault: true, Timezone: "UTC", Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeCalendar, Calendar: &schedule.CalendarTriggerDefinition{Frequency: "monthly", AnchorAt: clock.Now().Add(time.Hour)}}}
	if err := service.InstallDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_schedule_update BEFORE UPDATE ON extension_schedule_states BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	updated := *def
	updated.Name = "changed"
	updated.Target.InputTemplate = []byte(`{"title":"changed"}`)
	if err := service.Update(ctx, def.ScheduleID, 1, &updated); err == nil {
		t.Fatal("failed update reported success")
	}
	actual, state, err := service.GetSchedule(ctx, def.ScheduleID)
	if err != nil || actual.Name != "original" || state.Generation != 1 || len(actual.Target.InputTemplate) != 0 {
		t.Fatalf("failed update was not rolled back: %v %v %v", actual, state, err)
	}
	if actual.Trigger.Calendar == nil || actual.Trigger.Calendar.Frequency != "monthly" || !actual.Trigger.Calendar.AnchorAt.Equal(def.Trigger.Calendar.AnchorAt) {
		t.Fatalf("calendar definition did not survive persistence: %+v", actual.Trigger)
	}
}
