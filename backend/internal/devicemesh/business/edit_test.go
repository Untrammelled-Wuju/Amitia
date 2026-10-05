package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestEditingMessageInvalidatesSummaryAndMemoryIncludingDerivedData(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "tea", Body: json.RawMessage(`{"value":"tea"}`)}, {Kind: "vector", Key: "tea", Body: json.RawMessage(`{"values":[1]}`)}, {Kind: "summary", Key: "summary", Body: json.RawMessage(`{"summary":"tea"}`)}}, nil
	}
	authority := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "turn", Message: "tea"}
	if _, err := engine.Run(t.Context(), authority); err != nil {
		t.Fatal(err)
	}
	edit := EditRequest{RoleID: "role", RequestID: "edit", Kind: "message", ID: "turn/user", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"content": json.RawMessage(`"coffee"`)}}
	ack, err := engine.Edit(t.Context(), authority, edit)
	if err != nil || ack.OwnerID != "a" || ack.Versions["message/turn/user"] != 2 {
		t.Fatalf("edit: %+v %v", ack, err)
	}
	if repeated, err := engine.Edit(t.Context(), authority, edit); err != nil || repeated.Versions["message/turn/user"] != 2 {
		t.Fatalf("repeated edit did not return original acknowledgement: %+v %v", repeated, err)
	}
	changed := edit
	changed.Changes = map[string]json.RawMessage{"content": json.RawMessage(`"different"`)}
	if _, err := engine.Edit(t.Context(), authority, changed); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("changed request reused edit identity: %v", err)
	}
	for _, kind := range []string{"summary", "memory", "fact", "vector"} {
		rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), kind, "role", false)
		if err != nil || len(rows) != 0 {
			t.Fatalf("stale %s remained: %+v %v", kind, rows, err)
		}
	}
	stale := edit
	stale.RequestID = "another-edit"
	if _, err := engine.Edit(t.Context(), authority, stale); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	edit.ExpectedRevision = 2
	edit.Changes = map[string]json.RawMessage{"ownerId": json.RawMessage(`"core"`)}
	if _, err := engine.Edit(t.Context(), authority, edit); err == nil {
		t.Fatal("client changed authority")
	}
}

func TestEditingRejectsStaleDisplayedAuthority(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	authority := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "turn", Message: "tea"}
	response, err := engine.Run(t.Context(), authority)
	if err != nil {
		t.Fatal(err)
	}
	for _, alter := range []func(*coordination.ExecutionScope){
		func(s *coordination.ExecutionScope) { s.CoreID = "old-core" },
		func(s *coordination.ExecutionScope) { s.ResourceOwnerID = "other-device" },
		func(s *coordination.ExecutionScope) { s.ModeRevision++ },
		func(s *coordination.ExecutionScope) { s.PermissionRevision++ },
		func(s *coordination.ExecutionScope) { s.TargetPermissionRevision++ },
		func(s *coordination.ExecutionScope) { s.RoleRevision++ },
	} {
		expected := response.Scope
		alter(&expected)
		_, err := engine.Edit(t.Context(), authority, EditRequest{ExpectedScope: &expected, RoleID: "role", RequestID: "stale-edit", Kind: "message", ID: "turn/user", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"content": json.RawMessage(`"changed"`)}})
		if !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatalf("stale authority accepted: %v", err)
		}
	}
	resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "message", "turn/user")
	if err != nil || resource == nil || resource.Revision != 1 {
		t.Fatalf("stale page changed canonical data: %v", err)
	}
}

func TestMessageEditedDuringGenerationRejectsLateReply(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "turn", Message: "tea"}
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		_, err := engine.Edit(ctx, request, EditRequest{RoleID: "role", RequestID: "edit-running", Kind: "message", ID: "turn/user", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"content": json.RawMessage(`"coffee"`)}})
		if err != nil {
			return Generation{}, err
		}
		return Generation{Text: "reply based on tea"}, nil
	}
	response, err := engine.Run(t.Context(), request)
	if !errors.Is(err, coordination.ErrResourceVersion) || response.Saved {
		t.Fatalf("late reply accepted: %+v %v", response, err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "message", "role", false)
	if err != nil || len(rows) != 1 || rows[0].ID != "turn/user" {
		t.Fatalf("late reply written: %+v %v", rows, err)
	}
	if model.extractions.Load() != 0 {
		t.Fatal("memory extracted from superseded message")
	}
}

func TestDeletingConversationRemovesOwnedChildrenAndPreventsResurrection(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "conversation", RequestID: "turn", Message: "tea"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Edit(t.Context(), request, EditRequest{RoleID: "role", RequestID: "delete", Kind: "conversation", ID: "conversation", ExpectedRevision: 1, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	for _, kind := range []string{"conversation", "message", "checkpoint", "memory", "fact", "vector", "graph", "working"} {
		rows, err := store.List(t.Context(), kind, "role", false)
		if kind == "checkpoint" && len(rows) == 1 && rows[0].ID == "edit/delete" {
			rows = nil
		}
		if err != nil || len(rows) != 0 {
			t.Fatalf("conversation child %s survived: %+v %v", kind, rows, err)
		}
	}
	_, err := store.Apply(t.Context(), coordination.Commit{Scope: coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", RequestID: "resurrect"}, Mutations: []coordination.Mutation{{Kind: "conversation", ID: "conversation", RoleID: "role", ExpectedRevision: 2, Body: json.RawMessage(`{"id":"conversation"}`)}}})
	if !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("deleted conversation restored: %v", err)
	}
}
