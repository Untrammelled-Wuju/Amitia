package reminder

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestReminderConcurrentIdenticalCreateReturnsOneConfirmedTask(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db")+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if err := sqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	clock := schedule.NewFakeClock(time.Now().UTC())
	scheduler, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: sqlite.NewScheduleRepository(db), Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{
		Scheduler: scheduler,
		OwnerID:   "core",
		Mode:      "cloud_core",
		Authorize: func(_ context.Context, caller Caller, item *Reminder) (json.RawMessage, error) {
			if item.Caller.DeviceID != caller.DeviceID || item.CharacterID != "role" || item.ConversationID != "conversation" {
				return nil, ErrForbidden
			}
			return json.RawMessage(`{"ownerId":"core"}`), nil
		},
		Deliver: func(_ context.Context, item *Reminder, trigger string) (DeliveryResult, error) {
			return DeliveryResult{MessageID: trigger + "/message", ConversationID: item.ConversationID, MessageSaved: true, Targets: []TargetResult{{Kind: "conversation", ID: item.ConversationID, State: "saved"}}}, nil
		},
	})
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	request := newReminderRequest()
	const workers = 16
	start := make(chan struct{})
	results := make([]*Reminder, workers)
	errors := make([]error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for index := range workers {
		go func() {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.Create(t.Context(), caller, request, "manual")
		}()
	}
	close(start)
	wait.Wait()
	for index, err := range errors {
		if err != nil {
			t.Fatalf("worker %d failed: %v", index, err)
		}
	}
	first := results[0]
	for index, item := range results {
		if item == nil || !item.Saved || item.ID != first.ID || item.Generation != 1 {
			t.Fatalf("worker %d received %+v, first %+v", index, item, first)
		}
	}
	defs, err := scheduler.ListAllSchedules(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].ScheduleID != first.ID {
		t.Fatalf("schedules=%+v", defs)
	}
}
