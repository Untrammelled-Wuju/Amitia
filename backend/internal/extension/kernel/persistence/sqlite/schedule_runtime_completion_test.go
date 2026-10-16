package sqlite

import (
	"context"
	"errors"
	"github.com/u-ai/backend/internal/dbtransaction"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestScheduleExplicitFutureEditRearmsExpiredOneShotAtomically(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	repo := NewScheduleRepository(db)
	clock := schedule.NewFakeClock(time.Now().UTC())
	service, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	due := clock.Now().Add(time.Minute)
	def := &schedule.ScheduleContributionDefinition{ScheduleID: "rearm", EnabledByDefault: true, Timezone: "UTC", Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeHostWorkload, TargetID: "reminder"}, Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: due}}}
	if err := service.InstallHostDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	state, err := repo.GetState(ctx, def.ScheduleID)
	if err != nil {
		t.Fatal(err)
	}
	state.LastScheduledAt = &due
	state.Enabled = false
	state.Status = schedule.DefinitionStatusExpired
	state.NextScheduledAt = nil
	state.NextEffectiveAt = nil
	if err := repo.PutState(ctx, state); err != nil {
		t.Fatal(err)
	}
	enabled := true
	contentOnly := *def
	contentOnly.Name = "content edit"
	if err := service.UpdateHostDefinitionState(ctx, def.ScheduleID, 1, &contentOnly, &enabled); !errors.Is(err, schedule.ErrScheduleExpired) {
		t.Fatalf("content revived expired schedule: %v", err)
	}
	unchanged, _, err := service.GetSchedule(ctx, def.ScheduleID)
	if err != nil || unchanged.Name != "" {
		t.Fatalf("failed update leaked definition: %v %+v", err, unchanged)
	}
	future := clock.Now().Add(time.Hour)
	changed := *def
	changed.Trigger.OneShot = &schedule.OneShotTriggerDefinition{RunAt: future}
	if err := service.UpdateHostDefinitionState(ctx, def.ScheduleID, 1, &changed, &enabled); err != nil {
		t.Fatal(err)
	}
	_, state, err = service.GetSchedule(ctx, def.ScheduleID)
	if err != nil || !state.Enabled || state.Generation != 2 || state.NextScheduledAt == nil || !state.NextScheduledAt.Equal(future) {
		t.Fatalf("rearm failed: %+v %v", state, err)
	}
	if _, err := service.RunNowVersion(ctx, def.ScheduleID, 1); !errors.Is(err, schedule.ErrGenerationMismatch) {
		t.Fatalf("stale trigger request accepted: %v", err)
	}
	if err := service.CancelHostDefinition(ctx, def.ScheduleID, 2); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateHostDefinitionState(ctx, def.ScheduleID, 3, &changed, &enabled); !errors.Is(err, schedule.ErrScheduleNotFound) {
		t.Fatalf("cancelled schedule revived: %v", err)
	}
}

func TestScheduleCommitFenceSharesTransactionAndRejectsStaleGeneration(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	repo := NewScheduleRepository(db)
	service, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo})
	if err != nil {
		t.Fatal(err)
	}
	def := &schedule.ScheduleContributionDefinition{ScheduleID: "commit-fence", EnabledByDefault: true, Timezone: "UTC", Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeHostWorkload, TargetID: "test"}, Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: time.Now().Add(time.Hour)}}}
	if err := service.InstallHostDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE fence_messages(id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	wrote := false
	err = service.GuardHostExecution(ctx, def.ScheduleID, 1, func(ctx context.Context) error {
		tx, ok := dbtransaction.From(ctx, db)
		if !ok {
			t.Fatal("commit did not receive database transaction")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO fence_messages VALUES('rolled-back')`); err != nil {
			return err
		}
		wrote = true
		return errors.New("reject final commit")
	})
	if err == nil || !wrote {
		t.Fatal("failed final write reported as committed")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM fence_messages`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed final message survived: %d %v", count, err)
	}
	if err := service.Pause(ctx, def.ScheduleID, 1); err != nil {
		t.Fatal(err)
	}
	wrote = false
	err = service.GuardHostExecution(ctx, def.ScheduleID, 1, func(ctx context.Context) error { wrote = true; return nil })
	if !errors.Is(err, schedule.ErrGenerationMismatch) || wrote {
		t.Fatalf("stale generation saved message: %v", err)
	}
	if err := service.Resume(ctx, def.ScheduleID, 2); err != nil {
		t.Fatal(err)
	}
	err = service.GuardHostExecution(ctx, def.ScheduleID, 3, func(ctx context.Context) error {
		tx, ok := dbtransaction.From(ctx, db)
		if !ok {
			t.Fatal("missing shared transaction")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO fence_messages VALUES('saved')`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM fence_messages`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("final commit missing: %d %v", count, err)
	}
}

func TestScheduleHeartbeatIsOwnerFencedAndCleanupRetainsIdempotency(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	repo := NewScheduleRepository(db)
	now := time.Now().UTC()
	trigger := &schedule.ScheduleTriggerRecord{TriggerID: "heartbeat", ScheduleID: "task", ScheduledAt: now, EffectiveAt: now, IdempotencyKey: "stable", Status: schedule.RunStatusWaiting, Generation: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.PutTrigger(ctx, trigger); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.AcquireTriggerLease(ctx, trigger.TriggerID, "owner", now.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("acquire: %v %v", ok, err)
	}
	if ok, err := repo.RenewTriggerLease(ctx, trigger.TriggerID, "other", now.Add(time.Hour)); err != nil || ok {
		t.Fatalf("foreign renewal: %v %v", ok, err)
	}
	if err := repo.ReleaseOwnedTriggerLease(ctx, trigger.TriggerID, "other"); err != nil {
		t.Fatal(err)
	}
	fresh, err := repo.GetTrigger(ctx, trigger.TriggerID)
	if err != nil || fresh.LeaseOwner == nil || *fresh.LeaseOwner != "owner" {
		t.Fatalf("foreign release: %+v %v", fresh, err)
	}
	if ok, err := repo.RenewTriggerLease(ctx, trigger.TriggerID, "owner", now.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("renew: %v %v", ok, err)
	}
	if count, err := repo.ReclaimExpiredLeases(ctx, now.Add(2*time.Minute)); err != nil || count != 0 {
		t.Fatalf("renewed lease reclaimed: %d %v", count, err)
	}
	if err := repo.PutRun(ctx, &schedule.ScheduleRunRecord{RunID: "detail", TriggerID: trigger.TriggerID, ScheduleID: "task", Status: schedule.RunStatusCompleted, StartedAt: now, Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CleanTriggerRunHistory(ctx, trigger.TriggerID); err != nil {
		t.Fatal(err)
	}
	runs, err := repo.ListRunsByTrigger(ctx, trigger.TriggerID)
	if err != nil || len(runs) != 1 {
		t.Fatal("cleanup removed active leased task")
	}
	if err := repo.UpdateTriggerStatus(ctx, trigger.TriggerID, schedule.RunStatusCompleted, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReleaseOwnedTriggerLease(ctx, trigger.TriggerID, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := repo.CleanTriggerRunHistory(ctx, trigger.TriggerID); err != nil {
		t.Fatal(err)
	}
	runs, err = repo.ListRunsByTrigger(ctx, trigger.TriggerID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("old details retained: %d %v", len(runs), err)
	}
	fresh, err = repo.GetTriggerByIdempotencyKey(ctx, "stable")
	if err != nil || fresh == nil || fresh.Status != schedule.RunStatusCompleted {
		t.Fatalf("dedup evidence deleted: %+v %v", fresh, err)
	}
}

func TestScheduleLongExecutionRenewsLeaseAndLimitsConcurrency(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	ctx := t.Context()
	repo := NewScheduleRepository(db)
	config := schedule.DefaultScheduleConfig()
	config.GlobalMaxConcurrent = 1
	config.LeaseDuration = 150 * time.Millisecond
	service, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: repo, Config: config})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finish := make(chan struct{})
	service.GetExecutor().RegisterTargetAdapter(schedule.NewHostWorkloadAdapter(func(ctx context.Context, def *schedule.ScheduleContributionDefinition, trigger *schedule.ScheduleTriggerRecord) (*schedule.TargetExecutionResult, error) {
		close(started)
		select {
		case <-finish:
			return &schedule.TargetExecutionResult{Success: true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	for _, id := range []string{"first", "second"} {
		def := &schedule.ScheduleContributionDefinition{ScheduleID: id, EnabledByDefault: true, Timezone: "UTC", Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeHostWorkload, TargetID: "test"}, Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: time.Now().Add(time.Hour)}}}
		if err := service.InstallHostDefinition(ctx, def); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.RunNow(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.GetExecutor().Execute(ctx, first); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("target not started")
	}
	second, err := service.RunNow(ctx, "second")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.GetExecutor().Execute(ctx, second)
	if err != nil || result.ErrorCode != "CONCURRENCY_LIMIT" {
		t.Fatalf("global limit bypassed: %+v %v", result, err)
	}
	time.Sleep(350 * time.Millisecond)
	leased, err := repo.GetTrigger(ctx, first.TriggerID)
	if err != nil || leased.LeaseExpiresAt == nil || !leased.LeaseExpiresAt.After(time.Now()) {
		t.Fatalf("long task lease expired: %+v %v", leased, err)
	}
	clock := schedule.NewRealClock()
	recovery := schedule.NewRecoveryService(repo, clock, config, service.GetCalculator(), schedule.NewMisfireService(repo, clock), schedule.NewRetryService(repo, clock, config), schedule.NewCircuitService(repo, clock, config))
	if err := recovery.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	leased, err = repo.GetTrigger(ctx, first.TriggerID)
	if err != nil || leased.Status != schedule.RunStatusTriggering {
		t.Fatalf("recovery stole running lease: %+v %v", leased, err)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
