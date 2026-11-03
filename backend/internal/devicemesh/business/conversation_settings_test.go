package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedConversationSettingsPersistAndReachModel(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	enabled := false
	model.generate = func(_ context.Context, input Inference) (Generation, error) {
		if input.ModelConfigID != 7 || input.ReasoningEffort != "high" || input.ReasoningEnabled == nil || *input.ReasoningEnabled || input.PermissionMode != "request_approval" {
			t.Fatalf("settings missing from inference: %+v", input)
		}
		return Generation{Text: "saved"}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "settings", RequestID: "first", Message: "hello", ModelConfigID: 7, ReasoningEffort: "high", ReasoningEnabled: &enabled}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "conversation", "settings")
	if err != nil || row == nil {
		t.Fatalf("conversation missing: %v", err)
	}
	var saved conversationSettings
	if err := json.Unmarshal(row.Body, &saved); err != nil || saved.ModelConfigID != 7 || saved.ReasoningEnabled == nil || *saved.ReasoningEnabled {
		t.Fatalf("settings not persisted: %+v %v", saved, err)
	}
	request.RequestID, request.ModelConfigID, request.ReasoningEffort, request.ReasoningEnabled = "second", 0, "", nil
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	request.ModelConfigID = 9
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("changed request settings reused prior execution: %v", err)
	}
}

func TestOwnedConversationSettingsRejectInvalidValues(t *testing.T) {
	for _, request := range []Request{{ModelConfigID: -1}, {ReasoningEffort: "unbounded"}, {PermissionMode: "admin"}} {
		if err := validateConversationSettings(request); err == nil {
			t.Fatalf("accepted invalid settings: %+v", request)
		}
	}
	for _, changes := range []map[string]json.RawMessage{{"modelConfigId": json.RawMessage(`"wrong"`)}, {"reasoningEnabled": json.RawMessage(`1`)}, {"permissionMode": json.RawMessage(`null`)}} {
		if err := validateSettingsChanges(changes); err == nil {
			t.Fatalf("accepted invalid changes: %v", changes)
		}
	}
}

func TestRoleModelSettingsOverrideConversationDefaults(t *testing.T) {
	disabled := false
	request := Request{
		ConversationID: "group-conversation",
		ModelConfigID:  0, ReasoningEffort: "", ReasoningEnabled: nil,
	}
	role := coordination.Role{
		ModelConfigID: 11, ReasoningEffort: "xhigh", ReasoningEnabled: 0,
	}
	snapshot := coordination.DataSnapshot{Resources: []coordination.Resource{{
		Kind: "conversation", ID: "group-conversation",
		Body: json.RawMessage(`{"modelConfigId":7,"reasoningEffort":"low","reasoningEnabled":true}`),
	}}}
	if err := resolveConversationSettings(&request, snapshot, role); err != nil {
		t.Fatal(err)
	}
	if request.ModelConfigID != 11 || request.ReasoningEffort != "xhigh" || request.ReasoningEnabled == nil || *request.ReasoningEnabled {
		t.Fatalf("role settings did not override conversation defaults: %+v", request)
	}
	request = Request{
		ConversationID: "group-conversation", ModelConfigID: 9,
		ReasoningEffort: "medium", ReasoningEnabled: &disabled,
	}
	if err := resolveConversationSettings(&request, snapshot, role); err != nil {
		t.Fatal(err)
	}
	if request.ModelConfigID != 9 || request.ReasoningEffort != "medium" || request.ReasoningEnabled == nil || *request.ReasoningEnabled {
		t.Fatalf("explicit request settings did not retain priority: %+v", request)
	}
	request = Request{ConversationID: "group-conversation"}
	role = coordination.Role{ModelConfigID: 12, ReasoningEffort: "", ReasoningEnabled: -1}
	if err := resolveConversationSettings(&request, snapshot, role); err != nil {
		t.Fatal(err)
	}
	if request.ModelConfigID != 12 || request.ReasoningEffort != "" || request.ReasoningEnabled != nil {
		t.Fatalf("role model without parameter overrides should use model defaults: %+v", request)
	}
}

type committedTestModel struct {
	*testModel
	onCommit func(context.Context, Inference, Generation, string) error
}

func (m committedTestModel) OwnedReplyCommitted(ctx context.Context, input Inference, generation Generation, id string) error {
	return m.onCommit(ctx, input, generation, id)
}

func TestOwnedReplyHookOnlyRunsAfterOwnerAcknowledgement(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "rejected"}[fail], func(t *testing.T) {
			engine, db, _, model := engineHarness(t)
			called := 0
			engine.model = committedTestModel{testModel: model, onCommit: func(ctx context.Context, input Inference, generation Generation, id string) error {
				called++
				row, err := coordination.NewOwnershipStore(db, "a").Get(ctx, "message", id)
				if err != nil || row == nil {
					t.Fatalf("hook preceded owner acknowledgement: %v", err)
				}
				return nil
			}}
			if fail {
				engine.data = completionFailurePort{engine.data}
			}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "hook", RequestID: "hook", Message: "hello"})
			if fail && (err == nil || called != 0) || !fail && (err != nil || called != 1) {
				t.Fatalf("hook count=%d err=%v", called, err)
			}
		})
	}
}
