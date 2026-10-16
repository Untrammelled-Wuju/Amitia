package reminder

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestReminderCleanupPrunesArchivedDetailsWithoutChangingExecution(t *testing.T) {
	service, scheduler, _, db := newReminderTestService(t)
	service.deps.Settings = reminderCleanupSettings{}
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	def, state, err := scheduler.GetSchedule(t.Context(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(def.Target.InputTemplate, &input); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-72 * time.Hour)
	input["archiveHistory"], _ = json.Marshal([]TriggerHistory{
		{ID: "archived-finished", TriggerID: "archived-finished", State: "completed", UpdatedAt: old, Archived: true},
		{ID: "archived-unknown", TriggerID: "archived-unknown", State: "unknown", UpdatedAt: old, Archived: true},
	})
	def.Target.InputTemplate, _ = json.Marshal(input)
	hash := schedule.GenerateDefinitionHash(def)
	def.DefinitionHash = hash
	if err := sqlite.NewScheduleRepository(db).PutDefinition(t.Context(), def); err != nil {
		t.Fatal(err)
	}
	if err := service.SetCleanupDays(t.Context(), caller, "1"); err != nil {
		t.Fatal(err)
	}
	updated, updatedState, err := scheduler.GetSchedule(t.Context(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decode(updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.ArchiveHistory) != 1 || decoded.ArchiveHistory[0].TriggerID != "archived-unknown" {
		t.Fatalf("archive details not really pruned: %+v", decoded.ArchiveHistory)
	}
	if updatedState.Generation != state.Generation || updated.DefinitionHash != hash || updated.Trigger.RestoredOccurrences != def.Trigger.RestoredOccurrences {
		t.Fatal("history maintenance changed execution configuration")
	}
}

func TestProactiveCooldownPreservesFractionAndSaturates(t *testing.T) {
	if ScaleCooldown(time.Hour, 1.5) != 90*time.Minute {
		t.Fatal("fractional slowdown lost")
	}
	policy := &ProactivePolicy{MinInterval: 525600, ActiveLevel: 1}
	gap := ScaleCooldown(ProactiveMinimumInterval(policy), 100)
	if gap != time.Duration(math.MaxInt64) {
		t.Fatalf("cooldown overflow: %v", gap)
	}
	if ProactiveMinimumInterval(&ProactivePolicy{MinInterval: 60, ActiveLevel: 20}) != 2*time.Hour {
		t.Fatal("activity level ignored")
	}
}

type reminderCleanupSettings map[string]string

func (s reminderCleanupSettings) Read(ctx context.Context, key string) (string, error) {
	return s[key], nil
}
func (s reminderCleanupSettings) Write(ctx context.Context, key, value string) error {
	s[key] = value
	return nil
}

func TestReminderCleanupDeletesFinishedDetailsButKeepsCountsAndUnknownTargets(t *testing.T) {
	service, scheduler, _, db := newReminderTestService(t)
	service.deps.Settings = reminderCleanupSettings{}
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	repo := sqlite.NewScheduleRepository(db)
	old := time.Now().UTC().Add(-72 * time.Hour)
	for index, state := range []string{"completed", "queued"} {
		id := "old-" + state
		occurrence := old.Add(time.Duration(index) * time.Hour)
		trigger := &schedule.ScheduleTriggerRecord{TriggerID: id, ScheduleID: item.ID, ScheduledAt: occurrence, EffectiveAt: occurrence, IdempotencyKey: id, Status: schedule.RunStatusCompleted, Generation: 1, CreatedAt: old, UpdatedAt: old}
		if err := repo.PutTrigger(t.Context(), trigger); err != nil {
			t.Fatal(err)
		}
		result, err := json.Marshal(DeliveryResult{MessageSaved: true, MessageID: id + "/message", ConversationID: item.ConversationID, Targets: []TargetResult{{Kind: "notification", ID: id, State: state}}})
		if err != nil {
			t.Fatal(err)
		}
		if state == "completed" {
			result, _ = json.Marshal(DeliveryResult{MessageSaved: true, MessageID: id + "/message", ConversationID: item.ConversationID, Targets: []TargetResult{{Kind: "conversation", ID: item.ConversationID, State: "saved"}}})
		}
		if err := repo.PutRun(t.Context(), &schedule.ScheduleRunRecord{RunID: id + "/run", TriggerID: id, ScheduleID: item.ID, Status: schedule.RunStatusCompleted, StartedAt: old, ResultJSON: result, Generation: 1, CreatedAt: old, UpdatedAt: old}); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.SetCleanupDays(t.Context(), caller, "1"); err != nil {
		t.Fatal(err)
	}
	runs, err := repo.ListRunsByTrigger(t.Context(), "old-completed")
	if err != nil || len(runs) != 0 {
		t.Fatalf("finished details were only hidden: %d %v", len(runs), err)
	}
	runs, err = repo.ListRunsByTrigger(t.Context(), "old-queued")
	if err != nil || len(runs) != 1 {
		t.Fatalf("pending delivery evidence deleted: %d %v", len(runs), err)
	}
	if count, err := scheduler.CountScheduledTriggers(t.Context(), item.ID); err != nil || count != 2 {
		t.Fatalf("cleanup reset occurrence budget: %d %v", count, err)
	}
	trigger, err := repo.GetTriggerByIdempotencyKey(t.Context(), "old-completed")
	if err != nil || trigger == nil {
		t.Fatalf("cleanup lost idempotency receipt: %+v %v", trigger, err)
	}
	entries, total, err := service.History(t.Context(), caller, UserReminder, item.ID, 1, 20, "")
	if err != nil || total != 1 || len(entries) != 1 || entries[0].State != "partial" {
		t.Fatalf("unknown delivery hidden from history: %+v %d %v", entries, total, err)
	}
}

func TestReminderHistoryRefreshesDisplayedReceiptAndReportsSavedMessages(t *testing.T) {
	service, scheduler, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	service.deps.Deliver = func(ctx context.Context, item *Reminder, trigger string) (DeliveryResult, error) {
		if item.Generation != 1 {
			t.Fatalf("generation missing in delivery: %d", item.Generation)
		}
		return DeliveryResult{MessageSaved: true, MessageID: "saved", ConversationID: item.ConversationID, Targets: []TargetResult{{Kind: "notification", ID: "receipt", State: "queued"}}}, nil
	}
	trigger, err := service.TriggerNow(t.Context(), caller, item.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.GetExecutor().Execute(t.Context(), trigger); err != nil {
		t.Fatal(err)
	}
	service.deps.ResolveTarget = func(ctx context.Context, target TargetResult) (TargetResult, error) {
		target.State = "displayed"
		return target, nil
	}
	entries, total, err := service.History(t.Context(), caller, UserReminder, item.ID, 1, 20, "completed")
	if err != nil || total != 1 || len(entries) != 1 {
		t.Fatalf("displayed target remained partial: %+v %d %v", entries, total, err)
	}
	var result DeliveryResult
	if err := json.Unmarshal(entries[0].Result, &result); err != nil || result.Targets[0].State != "displayed" {
		t.Fatalf("true receipt status overwritten: %+v %v", result, err)
	}
	status, err := service.Status(t.Context(), caller, UserReminder)
	if err != nil || status["sentToday"] != 1 {
		t.Fatalf("saved message count faked zero: %+v %v", status, err)
	}
}
