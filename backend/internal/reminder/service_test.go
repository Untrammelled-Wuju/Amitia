package reminder

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func newReminderTestService(t *testing.T) (*Service, *schedule.ScheduleService, *schedule.FakeClock, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	clock := schedule.NewFakeClock(time.Now().UTC())
	scheduler, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: sqlite.NewScheduleRepository(db), Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{Scheduler: scheduler, OwnerID: "core", Mode: "cloud_core", Authorize: func(_ context.Context, caller Caller, item *Reminder) (json.RawMessage, error) {
		if item.Caller.DeviceID != caller.DeviceID || item.CharacterID != "role" || item.ConversationID != "conversation" {
			return nil, ErrForbidden
		}
		return json.RawMessage(`{"ownerId":"core"}`), nil
	}, Deliver: func(_ context.Context, item *Reminder, trigger string) (DeliveryResult, error) {
		return DeliveryResult{MessageID: trigger + "/message", ConversationID: item.ConversationID, MessageSaved: true, Targets: []TargetResult{{Kind: "conversation", ID: item.ConversationID, State: "saved"}}}, nil
	}})
	return service, scheduler, clock, db
}

func newReminderRequest() CreateReminderRequest {
	return CreateReminderRequest{Title: "喝水", Content: "提醒喝水", CharacterID: "role", ConversationID: "conversation", RemindAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), Timezone: "Asia/Shanghai", Recurrence: Recurrence{Type: "monthly"}, RequestID: "create-request"}
}

func TestReminderCreatesOneKernelDefinitionAndNoLegacyRows(t *testing.T) {
	service, scheduler, _, db := newReminderTestService(t)
	request := newReminderRequest()
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	first, err := service.Create(t.Context(), caller, request, "manual")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(t.Context(), caller, request, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || !first.Saved || first.Generation != 1 {
		t.Fatalf("creation confirmation: %+v %+v", first, second)
	}
	defs, err := scheduler.ListAllSchedules(t.Context())
	if err != nil || len(defs) != 1 || defs[0].Target.Type != schedule.TargetTypeHostWorkload || defs[0].Trigger.Calendar == nil {
		t.Fatalf("ledger: %+v %v", defs, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('reminders','schedules','trigger_histories')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("legacy tables created: %d %v", count, err)
	}
	request.Title = "different"
	if _, err := service.Create(t.Context(), caller, request, "manual"); !errors.Is(err, schedule.ErrIdempotencyConflict) {
		t.Fatalf("changed request accepted: %v", err)
	}
}

func TestReminderManualTriggerUsesKernelAndKeepsHistoryAfterCancel(t *testing.T) {
	service, scheduler, clock, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := service.TriggerNow(t.Context(), caller, item.ID, item.Generation)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Second)
	result, err := scheduler.GetExecutor().Execute(t.Context(), trigger)
	if err != nil || result.Status != schedule.RunStatusCompleted {
		t.Fatalf("execute: %+v %v", result, err)
	}
	if _, err := service.Change(t.Context(), caller, item.ID, item.Generation, "cancel"); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(t.Context(), caller, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("cancelled task visible: %v %v", items, err)
	}
	history, total, err := service.History(t.Context(), caller, "", "", 1, 20, "")
	if err != nil || total != 1 || len(history) != 1 || history[0].State != "completed" {
		t.Fatalf("history lost: %+v %d %v", history, total, err)
	}
}

func TestReminderHTTPUsesAuthenticatedCallerWithoutAdministratorGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _, _ := newReminderTestService(t)
	item, err := service.Create(t.Context(), Caller{SpaceID: "space", DeviceID: "device-a"}, newReminderRequest(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Actor") == "a" {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space", DeviceID: "device-a"})
		}
		if c.GetHeader("X-Test-Actor") == "b" {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "space", DeviceID: "device-b"})
		}
	})
	RegisterRouter(router.Group("/api"), service, "")
	for _, test := range []struct {
		actor  string
		status int
	}{{"a", 200}, {"b", 403}, {"", 403}} {
		req := httptest.NewRequest("GET", "/api/reminders/"+item.ID, nil)
		req.Header.Set("X-Test-Actor", test.actor)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != test.status {
			t.Fatalf("actor %q: %d %s", test.actor, response.Code, response.Body.String())
		}
	}
	req := httptest.NewRequest("POST", "/api/reminders/"+item.ID+"/trigger", strings.NewReader(`{"generation":1}`))
	req.Header.Set("X-Test-Actor", "a")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"accepted":true`) || !strings.Contains(response.Body.String(), `"triggered":false`) {
		t.Fatalf("false delivery confirmation: %d %s", response.Code, response.Body.String())
	}
}

func TestReminderRejectsStaleVersionAndCrossPluginAccess(t *testing.T) {
	service, _, _, _ := newReminderTestService(t)
	caller := Caller{SpaceID: "space", DeviceID: "device", PluginID: "plugin-a", ModuleID: "module"}
	item, err := service.Create(t.Context(), caller, newReminderRequest(), "plugin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Find(t.Context(), Caller{SpaceID: "space", DeviceID: "device", PluginID: "plugin-b", ModuleID: "module"}, item.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross plugin access: %v", err)
	}
	if _, err := service.Change(t.Context(), caller, item.ID, 1, "pause"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Change(t.Context(), caller, item.ID, 1, "resume"); !errors.Is(err, schedule.ErrGenerationMismatch) {
		t.Fatalf("stale mutation accepted: %v", err)
	}
}
