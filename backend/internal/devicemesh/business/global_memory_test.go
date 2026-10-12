package business

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
	"time"
)

func TestGlobalMemoryPolicyPreservesOldPartitionAndSharesNewWrites(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "private", ConversationID: "chat", Message: "hello"}
	saved, err := engine.Run(t.Context(), request)
	if err != nil || saved.MemoryStatus != "saved" {
		t.Fatalf("private=%+v %v", saved, err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	old, _ := store.List(t.Context(), "memory", "role", false)
	if len(old) != 1 {
		t.Fatalf("private memories=%d", len(old))
	}
	engine.data = testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 2, Profile: json.RawMessage(`{"globalMemoryEnabled":true}`)}}
	request.RequestID = "global"
	request.ConversationID = "shared-chat"
	saved, err = engine.Run(t.Context(), request)
	if err != nil || saved.MemoryStatus != "saved" {
		t.Fatalf("global=%+v %v", saved, err)
	}
	unchanged, _ := store.List(t.Context(), "memory", "role", false)
	global, _ := store.List(t.Context(), "memory", coordination.GlobalMemoryRole, false)
	if len(unchanged) != 1 || unchanged[0].ID != old[0].ID || len(global) != 1 {
		t.Fatal("toggle migrated or lost existing data")
	}
	other := NewEngine(service, testDataPort{db: db, role: coordination.Role{ID: "other", Revision: 1, Profile: json.RawMessage(`{}`)}}, model)
	view, err := other.Query(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "other"}, coordination.DataQuery{ResourceKind: "memory", Management: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range view.Snapshot.Resources {
		if row.Kind == "memory" && row.ID == global[0].ID {
			found = true
		}
		if row.ID == old[0].ID {
			t.Fatal("private data crossed roles")
		}
	}
	if !found {
		t.Fatal("global memory was not shared")
	}
}

func TestGlobalMemoryContinuityTransferPreservesExecutorAndWake(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	engine.data = testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"globalMemoryEnabled":true}`)}}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create-global-task"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "跟进目标"})
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Minute)
	request.RequestID = "add-global-wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: 1, Action: "add_wait", Wait: &continuity.Wait{WaitType: "time", DueAt: &due, AutoResume: true}})
	if err != nil {
		t.Fatal(err)
	}
	source := document.Scope
	request.ExpectedScope = &source
	request.RequestID = "demote-global-task"
	ack, err := engine.TransferMemory(t.Context(), request, MemoryTransferInput{Kind: "continuity", ID: document.Thread.ID, ExpectedRevision: 2, TargetScope: "character", TargetRoleID: "role"})
	if err != nil || ack.Versions["continuity/"+document.Thread.ID] != 3 {
		t.Fatalf("transfer=%+v %v", ack, err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "continuity", document.Thread.ID)
	if err != nil || row == nil {
		t.Fatal(err)
	}
	var finished OwnedContinuity
	if err := json.Unmarshal(row.Body, &finished); err != nil {
		t.Fatal(err)
	}
	if row.RoleID != "role" || finished.Thread.CharacterID != "role" || finished.Lease == nil || finished.Lease.State != "completed" || model.calls.Load() != 1 {
		t.Fatalf("migrated task did not wake: %+v calls=%d", finished, model.calls.Load())
	}
	if finished.Scope.MemoryRoleID != coordination.GlobalMemoryRole || finished.Scope.ContinuityRoleID != "role" {
		t.Fatal("task partition changed executor memory policy")
	}
}

func TestGlobalMemorySourceEditInvalidatesSharedProjections(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	engine.data = testDataPort{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"globalMemoryEnabled":true}`)}}
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "饮品", Body: json.RawMessage(`{"value":"茶"}`)}, {Kind: "profile", Key: "偏好", Body: json.RawMessage(`{"value":"茶"}`)}}, nil
	}
	authority := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "global-chat", RequestID: "global-source", Message: "茶"}
	saved, err := engine.Run(t.Context(), authority)
	if err != nil || saved.MemoryStatus != "saved" {
		t.Fatalf("save=%+v %v", saved, err)
	}
	if _, err := engine.Edit(t.Context(), authority, EditRequest{RoleID: "role", RequestID: "edit-source", Kind: "message", ID: "global-source/user", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"content": json.RawMessage(`"咖啡"`)}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"memory", "fact", "profile"} {
		rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), kind, coordination.GlobalMemoryRole, false)
		if err != nil || len(rows) != 0 {
			t.Fatalf("stale shared %s=%+v %v", kind, rows, err)
		}
	}
}
