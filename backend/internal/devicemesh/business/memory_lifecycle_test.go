package business

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

func TestOwnedMemoryReinforcementIsCommittedOncePerTurn(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "seed-memory"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "tea", Value: "喜欢茶", RetentionLevel: 3, Importance: 7})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ConversationID, request.Message, request.ExpectedScope = "recall-memory", "chat", "喜欢什么饮品", nil
	result, err := engine.Run(t.Context(), request)
	if err != nil || result.MemoryStatus != "saved" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	read := func() memorypolicy.Lifecycle {
		resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", created.Resources[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		_ = json.Unmarshal(resource.Body, &document)
		return memorypolicy.ReadLifecycle(document, time.Now())
	}
	state := read()
	if state.ReinforceCount != 1 || state.MemoryStrength <= .68 {
		t.Fatalf("not reinforced: %+v", state)
	}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if replay := read(); replay.ReinforceCount != 1 || replay.MemoryStrength != state.MemoryStrength {
		t.Fatalf("replay reinforced twice: %+v", replay)
	}
}

func TestOwnedMemoryLifecycleCreateEditCASAndFilters(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "lifecycle-create"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: "茶", Value: "喜欢茶", Importance: 7, RetentionLevel: 2, Pinned: true})
	if err != nil {
		t.Fatal(err)
	}
	row := created.Resources[0]
	var document map[string]json.RawMessage
	if err := json.Unmarshal(row.Body, &document); err != nil {
		t.Fatal(err)
	}
	state := memorypolicy.ReadLifecycle(document, time.Now())
	if !state.Pinned || state.RetentionLevel != 1 || state.MemoryStrength != 1 {
		t.Fatalf("state=%+v", state)
	}
	edit := EditRequest{RequestID: "lifecycle-edit", Kind: "memory", ID: row.ID, ExpectedRevision: row.Revision, ExpectedScope: &view.Scope, RoleID: "role", Changes: map[string]json.RawMessage{"pinned": body(false), "retentionLevel": body(4), "memoryStrength": body(.5)}}
	if _, err := engine.Edit(t.Context(), request, edit); err != nil {
		t.Fatal(err)
	}
	stored, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", row.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(stored.Body, &document)
	state = memorypolicy.ReadLifecycle(document, time.Now())
	if state.Pinned || state.RetentionLevel != 4 || state.MemoryStrength != .5 {
		t.Fatalf("edited=%+v", state)
	}
	edit.RequestID = "stale-edit"
	if _, err := engine.Edit(t.Context(), request, edit); err == nil {
		t.Fatal("stale edit accepted")
	}
	filtered, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{RetentionLevel: 4})
	if err != nil || len(filtered.Resources) != 1 {
		t.Fatalf("filter=%+v err=%v", filtered, err)
	}
	filtered, err = engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{RetentionLevel: 2})
	if err != nil || len(filtered.Resources) != 0 {
		t.Fatalf("wrong filter=%+v err=%v", filtered, err)
	}
}

func TestAutomaticExtractionCannotPinMemoryOrOverrideLifecycle(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "drink", Body: json.RawMessage(`{"value":"茶","importance":5,"pinned":true,"retentionLevel":1,"memoryStrength":1}`)}}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "model-pin", ConversationID: "chat", Message: "喜欢茶"}
	result, err := engine.Run(t.Context(), request)
	if err != nil || result.MemoryStatus != "saved" {
		t.Fatalf("run=%+v err=%v", result, err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	var document map[string]json.RawMessage
	_ = json.Unmarshal(rows[0].Body, &document)
	state := memorypolicy.ReadLifecycle(document, time.Now())
	if state.Pinned || state.RetentionLevel != 4 {
		t.Fatalf("model overrode policy: %+v", state)
	}
}
