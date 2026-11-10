package coordination_test

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestConversationDeletionCascadesCoreOwnedDerivedResources(t *testing.T) {
	db, _ := setup(t)
	device := coordination.NewOwnershipStore(db, "a")
	core := coordination.NewOwnershipStore(db, "core")
	deviceScope := coordination.ExecutionScope{SpaceID: "space", InitiatorDeviceID: "a", TargetDeviceID: "a", CoreID: "core", ProviderEpoch: 1, ModeRevision: 1, PermissionRevision: 1, ResourceOwnerID: "a", RoleOwnerID: "a", RoleID: "role", RequestID: "seed-chat", Coordinated: true}
	if _, err := device.Apply(t.Context(), coordination.Commit{Scope: deviceScope, Mutations: []coordination.Mutation{
		{Kind: "conversation", ID: "chat", RoleID: "role", Body: json.RawMessage(`{"id":"chat"}`)},
		{Kind: "message", ID: "chat/user", RoleID: "role", Body: json.RawMessage(`{"id":"chat/user","conversationId":"chat","content":"hello"}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	coreScope := deviceScope
	coreScope.ResourceOwnerID, coreScope.RoleOwnerID, coreScope.RequestID = "core", "core", "seed-core"
	if _, err := core.Apply(t.Context(), coordination.Commit{Scope: coreScope, Mutations: []coordination.Mutation{
		{Kind: "working", ID: "chat", RoleID: "role", Body: json.RawMessage(`{"conversationId":"chat","notes":["hello"]}`)},
		{Kind: "summary", ID: "chat", RoleID: "role", Body: json.RawMessage(`{"conversationId":"chat","summary":"hello"}`)},
		{Kind: "memory", ID: "chat-memory", RoleID: "role", Body: json.RawMessage(`{"conversationId":"chat","content":"remember"}`)},
	}}); err != nil {
		t.Fatal(err)
	}
	deleteScope := deviceScope
	deleteScope.RequestID = "delete-chat"
	if _, err := device.Apply(t.Context(), coordination.Commit{Scope: deleteScope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: "chat", RoleID: "role", ExpectedRevision: 1, Deleted: true, Body: json.RawMessage(`null`)}}}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []struct{ kind, id string }{{"working", "chat"}, {"summary", "chat"}, {"memory", "chat-memory"}} {
		row, err := core.Get(t.Context(), key.kind, key.id)
		if err != nil || row == nil || !row.Deleted || string(row.Body) != "null" {
			t.Fatalf("core owned %s/%s survived: %+v %v", key.kind, key.id, row, err)
		}
	}
	if row, err := device.Get(t.Context(), "message", "chat/user"); err != nil || row == nil || !row.Deleted {
		t.Fatalf("message survived: %+v %v", row, err)
	}
}
