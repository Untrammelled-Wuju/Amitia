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
			conversation, binding, err := engine.PrepareScheduledConversation(t.Context(), request, "提醒", nil)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(binding)
			request.ConversationID, request.RequestID, request.Message = conversation, "fenced-task", "提醒喝水"
			model.generate = func(context.Context, Inference) (Generation, error) {
				if mode == "stale" {
					if err := scheduler.Disable(t.Context(), definition.ScheduleID, 1); err != nil {
						t.Fatal(err)
					}
				}
				return Generation{Text: "喝水"}, nil
			}
			result, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "", func(ctx context.Context, commit func(context.Context) error) error {
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
			rows, readErr := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "commit" {
				if err != nil || !result.Saved || len(rows) != 1 {
					t.Fatalf("shared transaction failed: %+v %v rows=%d", result, err, len(rows))
				}
			} else if err == nil || len(rows) != 0 {
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
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				owner = "core"
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
			conversation, binding, err := engine.PrepareScheduledConversation(t.Context(), request, "提醒", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.ConversationID = conversation
			request.RequestID = "reminder:trigger"
			request.Message = "提醒喝水"
			encoded, _ := json.Marshal(binding)
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if !inference.ScheduledMessage {
					t.Error("scheduled message tools were not disabled")
				}
				return Generation{Text: "该喝水了"}, nil
			}
			first, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "喝水")
			if err != nil {
				t.Fatal(err)
			}
			second, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "喝水")
			if err != nil {
				t.Fatal(err)
			}
			if !first.Saved || first.MessageID != second.MessageID || model.calls.Load() != 1 || first.Scope.ResourceOwnerID != owner {
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
		})
	}
}

func TestScheduledMessageDoesNotFollowOwnerSwitch(t *testing.T) {
	engine, _, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
	conversation, binding, err := engine.PrepareScheduledConversation(t.Context(), request, "提醒", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request.ConversationID = conversation
	request.RequestID = "reminder:switch"
	request.Message = "提醒"
	encoded, _ := json.Marshal(binding)
	if _, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "提醒"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("task silently followed owner switch: %v", err)
	}
	if model.calls.Load() != 0 {
		t.Fatal("model ran after ownership changed")
	}
}

func TestScheduledMessagePersistsFallbackButRechecksAuthority(t *testing.T) {
	engine, _, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
	conversation, binding, err := engine.PrepareScheduledConversation(t.Context(), request, "提醒", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.ConversationID = conversation
	request.RequestID = "reminder:fallback"
	request.Message = "提醒喝水"
	encoded, _ := json.Marshal(binding)
	model.generate = func(context.Context, Inference) (Generation, error) { return Generation{}, errors.New("model offline") }
	result, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "喝水")
	if err != nil || !result.Saved || !result.Degraded || result.Content != "喝水" {
		t.Fatalf("fallback: %+v %v", result, err)
	}
	request.RequestID = "reminder:revoked"
	model.generate = func(context.Context, Inference) (Generation, error) {
		_, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role")
		if err != nil {
			t.Fatal(err)
		}
		return Generation{Text: "late"}, nil
	}
	if _, err := engine.SendScheduledMessage(t.Context(), request, encoded, "reminder", "喝水"); err == nil {
		t.Fatal("late message committed after permission/owner change")
	}
}
