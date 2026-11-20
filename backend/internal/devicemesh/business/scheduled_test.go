package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestScheduledMessageSharesKernelCommitFenceOnSingleConnection(t *testing.T) {
	for _, mode := range []string{"commit", "rollback", "stale"} {
		t.Run(mode, func(t *testing.T) {
			engine, db, _, model := engineHarness(t)
			scheduler, err := schedule.NewScheduleService(schedule.ScheduleDeps{Store: kernelsqlite.NewScheduleRepository(db)})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Add(time.Hour)
			definition := &schedule.ScheduleContributionDefinition{ScheduleID: "reminder-fence", ContributionID: "fence", ExtensionID: "host", ModuleID: "reminder", EnabledByDefault: true, Timezone: "UTC", Trigger: schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: now}}, Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeHostWorkload, TargetID: "test"}}
			if err := scheduler.InstallHostDefinition(t.Context(), definition); err != nil {
				t.Fatal(err)
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare-fence"}
			binding, err := engine.AuthorizeScheduledMessage(t.Context(), request, nil)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(binding)
			request.RequestID, request.Message = "fenced-task", "提醒喝水"
			model.generate = func(context.Context, Inference) (Generation, error) {
				if mode == "stale" {
					if err := scheduler.Disable(t.Context(), definition.ScheduleID, 1); err != nil {
						t.Fatal(err)
					}
				}
				return Generation{Text: "喝水"}, nil
			}
			result, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder", func(ctx context.Context, commit func(context.Context) error) error {
				return scheduler.GuardHostExecution(ctx, definition.ScheduleID, 1, func(shared context.Context) error {
					if err := commit(shared); err != nil {
						return err
					}
					if mode == "rollback" {
						return errors.New("commit rejected")
					}
					return nil
				})
			})
			store := coordination.NewOwnershipStore(db, "a")
			conversations, readErr := store.List(t.Context(), "conversation", "role", false)
			if readErr != nil {
				t.Fatal(readErr)
			}
			rows, readErr := store.List(t.Context(), "message", "role", false)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "commit" {
				if err != nil || !result.Saved || len(rows) != 1 || len(conversations) != 1 {
					t.Fatalf("shared transaction failed: %+v %v rows=%d", result, err, len(rows))
				}
			} else if err == nil || len(rows) != 0 || len(conversations) != 0 {
				t.Fatalf("rejected generation committed: %+v %v rows=%d", result, err, len(rows))
			}
		})
	}
}

func TestScheduledMessageUsesChosenOwnerAndStableSavedReply(t *testing.T) {
	for _, cloud := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "cloud"}[cloud], func(t *testing.T) {
			engine, db, service, model := engineHarness(t)
			owner := "a"
			if cloud {
				if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
					t.Fatal(err)
				}
				owner = "core"
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
			binding, err := engine.AuthorizeScheduledMessage(t.Context(), request, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.RequestID = "reminder:trigger"
			request.Message = "提醒喝水"
			encoded, _ := json.Marshal(binding)
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if !inference.ScheduledMessage {
					t.Error("scheduled message tools were not disabled")
				}
				return Generation{Text: "该喝水了"}, nil
			}
			first, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder")
			if err != nil {
				t.Fatal(err)
			}
			second, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder")
			if err != nil {
				t.Fatal(err)
			}
			if !first.Saved || first.MessageID != second.MessageID || first.ConversationID != second.ConversationID || model.calls.Load() != 1 || first.Scope.ResourceOwnerID != owner {
				t.Fatalf("duplicate generation or wrong owner: %+v %+v %d", first, second, model.calls.Load())
			}
			rows, err := coordination.NewOwnershipStore(db, owner).List(t.Context(), "message", "role", false)
			if err != nil || len(rows) != 1 {
				t.Fatalf("messages: %v %v", rows, err)
			}
			var document map[string]any
			if json.Unmarshal(rows[0].Body, &document) != nil || document["role"] != "assistant" {
				t.Fatalf("synthetic user message created: %s", rows[0].Body)
			}
			request.RequestID = "reminder:next-trigger"
			next, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder")
			if err != nil || next.ConversationID == first.ConversationID || model.calls.Load() != 2 {
				t.Fatalf("new trigger reused the conversation: %+v %v", next, err)
			}
			conversations, err := coordination.NewOwnershipStore(db, owner).List(t.Context(), "conversation", "role", false)
			if err != nil || len(conversations) != 2 {
				t.Fatalf("conversations: %v %v", conversations, err)
			}
		})
	}
}

func TestScheduledMessageUsesExplicitLocalOwnerInCoordinatedMode(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ResourceOwnerID: "a", RoleID: "role", RequestID: "local-prepare"}
	binding, err := engine.AuthorizeScheduledMessage(t.Context(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "local-trigger"
	request.Message = "提醒喝水"
	model.generate = func(context.Context, Inference) (Generation, error) {
		return Generation{Text: "该喝水了"}, nil
	}
	result, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder")
	if err != nil || result.Scope.ResourceOwnerID != "a" {
		t.Fatalf("scheduled local message: %+v %v", result, err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil || len(rows) != 1 || rows[0].ID != result.MessageID {
		t.Fatalf("local messages: %+v %v", rows, err)
	}
}

func TestScheduledMessageDoesNotFollowOwnerSwitch(t *testing.T) {
	engine, _, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
	binding, err := engine.AuthorizeScheduledMessage(t.Context(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "reminder:switch"
	request.Message = "提醒"
	encoded, _ := json.Marshal(binding)
	if _, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("task silently followed owner switch: %v", err)
	}
	if model.calls.Load() != 0 {
		t.Fatal("model ran after ownership changed")
	}
}

func TestScheduledMessagePreservesGenerationErrorAndRechecksAuthority(t *testing.T) {
	engine, _, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
	binding, err := engine.AuthorizeScheduledMessage(t.Context(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "reminder:fallback"
	request.Message = "提醒喝水"
	encoded, _ := json.Marshal(binding)
	modelError := errors.New("model offline")
	model.generate = func(context.Context, Inference) (Generation, error) { return Generation{}, modelError }
	result, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder")
	if !errors.Is(err, modelError) || result.Saved {
		t.Fatalf("generation error was lost: %+v %v", result, err)
	}
	request.RequestID = "reminder:revoked"
	model.generate = func(context.Context, Inference) (Generation, error) {
		_, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role")
		if err != nil {
			t.Fatal(err)
		}
		return Generation{Text: "late"}, nil
	}
	if _, err := engine.SendScheduledMessage(t.Context(), request, encoded, "提醒", "reminder"); err == nil {
		t.Fatal("late message committed after permission/owner change")
	}
}
