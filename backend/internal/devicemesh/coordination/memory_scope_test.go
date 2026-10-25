package coordination_test

import (
	"encoding/json"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
)

func TestMemoryScopeDefaultsToCharacterAndKeepsConversationIsolation(t *testing.T) {
	scope := coordination.ExecutionScope{RoleID: "role"}
	if err := coordination.ResolveMemoryScope(&scope, ""); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"memory", "fact", "episodic", "vector", "graph", "continuity", "profile"} {
		if coordination.StorageRole(scope, kind) != "role" {
			t.Fatalf("kind=%s", kind)
		}
	}
	for _, kind := range []string{"message", "conversation", "working", "summary", "checkpoint", "project"} {
		if coordination.StorageRole(scope, kind) != "role" {
			t.Fatalf("conversation isolation: %s", kind)
		}
	}
	if err := coordination.ResolveMemoryScope(&scope, "global"); err == nil {
		t.Fatal("global memory view must be rejected")
	}
	if err := coordination.ResolveMemoryScope(&scope, "role:other"); err != nil || coordination.StorageRole(scope, "profile") != "other" {
		t.Fatal("explicit role view must scope every memory kind")
	}
}

func TestGroupConversationRoleOnlyOverridesSharedChatResources(t *testing.T) {
	scope := coordination.ExecutionScope{RoleID: "speaker-b", ConversationRoleID: "storage-a", MemoryRoleID: "speaker-b"}
	for _, kind := range []string{"conversation", "message", "summary", "group-round", "group-delivery", "group-invitation"} {
		if coordination.StorageRole(scope, kind) != "storage-a" {
			t.Fatalf("群聊共享资源应使用会话角色: %s", kind)
		}
	}
	for _, kind := range []string{"memory", "fact", "checkpoint", "tool-result", "working", "project"} {
		if coordination.StorageRole(scope, kind) != "speaker-b" {
			t.Fatalf("角色私有资源不得跨角色存储: %s", kind)
		}
	}
}

func TestMemoryScopeAllViewKeepsOriginalPartitionsAndPaging(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	for index, role := range []string{"one", "two", "three"} {
		scope := coordination.ExecutionScope{RoleID: role, ResourceOwnerID: "a", RequestID: role}
		mutation := coordination.Mutation{Kind: "memory", ID: role, RoleID: role, Body: json.RawMessage(`{"content":{"value":"tea"}}`)}
		if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{mutation}}); err != nil {
			t.Fatalf("seed %d: %v", index, err)
		}
	}
	query := coordination.DataQuery{Management: true, ResourceKind: "memory", Limit: 2}
	rows, next, err := store.ListPage(t.Context(), "memory", coordination.AllMemoryRole, query)
	if err != nil || len(rows) != 2 || next == "" {
		t.Fatalf("page %v %s %v", rows, next, err)
	}
	query.Cursor = next
	last, _, err := store.ListPage(t.Context(), "memory", coordination.AllMemoryRole, query)
	if err != nil || len(last) != 1 {
		t.Fatalf("last=%v %v", last, err)
	}
	seen := map[string]bool{}
	for _, row := range append(rows, last...) {
		seen[row.RoleID] = true
	}
	if len(seen) != 3 {
		t.Fatal("all view overwrote partitions")
	}
	if _, _, err := store.ListPage(t.Context(), "message", coordination.AllMemoryRole, query); err == nil {
		t.Fatal("all memory view exposed chat messages")
	}
}

func TestMemoryScopeTransferMovesCompleteProjectionChainAndCanDemote(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	scope := coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", MemoryRoleID: "role", RequestID: "seed"}
	seed := []coordination.Mutation{{Kind: "memory", ID: "m", RoleID: "role", Body: json.RawMessage(`{"key":"饮品","content":{"value":"茶"}}`)}}
	for _, kind := range []string{"fact", "vector", "graph"} {
		seed = append(seed, coordination.Mutation{Kind: kind, ID: kind, RoleID: "role", SourceID: "m", Body: json.RawMessage(`{"content":{"value":"茶"}}`)})
	}
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: seed}); err != nil {
		t.Fatal(err)
	}
	moved := append([]coordination.Mutation(nil), seed...)
	for i := range moved {
		moved[i].ExpectedRevision = 1
		moved[i].RoleID = "target-role"
	}
	scope.RequestID = "promote"
	commit := coordination.Commit{Scope: scope, MemoryTransfer: &coordination.MemoryTransfer{Kind: "memory", ID: "m", ToRoleID: "target-role", TargetRoleRevision: 1}, Mutations: moved}
	ack, err := store.Apply(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Versions["memory/m"] != 2 || ack.Versions["graph/graph"] != 2 {
		t.Fatal("all revisions must change")
	}
	if _, err := store.Apply(t.Context(), commit); err != nil {
		t.Fatalf("replay=%v", err)
	}
	old, err := store.List(t.Context(), "memory", "role", false)
	if err != nil || len(old) != 0 {
		t.Fatal("source partition must not retain copies")
	}
	scope.MemoryRoleID = "target-role"
	scope.RequestID = "demote"
	for i := range moved {
		moved[i].ExpectedRevision = 2
		moved[i].RoleID = "third-role"
	}
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, MemoryTransfer: &coordination.MemoryTransfer{Kind: "memory", ID: "m", ToRoleID: "third-role", TargetRoleRevision: 1}, Mutations: moved}); err != nil {
		t.Fatal(err)
	}
	target, err := store.List(t.Context(), "memory", "third-role", false)
	if err != nil || len(target) != 1 || target[0].ID != "m" {
		t.Fatal("demotion must preserve record identity")
	}
}

func TestMemoryScopeTransferRejectsPartialChainsAndContentChanges(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	scope := coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", MemoryRoleID: "role", RequestID: "seed"}
	seed := []coordination.Mutation{{Kind: "memory", ID: "m", RoleID: "role", Body: json.RawMessage(`{"content":{"value":"茶"}}`)}, {Kind: "graph", ID: "g", RoleID: "role", SourceID: "m", Body: json.RawMessage(`{"content":{"relation":"喜欢","target":"茶"}}`)}}
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: seed}); err != nil {
		t.Fatal(err)
	}
	moved := append([]coordination.Mutation(nil), seed...)
	for i := range moved {
		moved[i].ExpectedRevision = 1
		moved[i].RoleID = "third-role"
	}
	scope.RequestID = "partial"
	transfer := &coordination.MemoryTransfer{Kind: "memory", ID: "m", ToRoleID: "third-role", TargetRoleRevision: 1}
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, MemoryTransfer: transfer, Mutations: moved[:1]}); err == nil {
		t.Fatal("partial projection migration accepted")
	}
	scope.RequestID = "tampered"
	moved[0].Body = json.RawMessage(`{"content":{"value":"changed"}}`)
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, MemoryTransfer: transfer, Mutations: moved}); err == nil {
		t.Fatal("migration must not modify content")
	}
	record, _ := store.Get(t.Context(), "memory", "m")
	if record.RoleID != "role" || record.Revision != 1 {
		t.Fatal("failed transaction changed source")
	}
}

func TestMemoryScopeContinuityRejectsRunningAndUnknownLeases(t *testing.T) {
	for _, state := range []string{"running", "unknown"} {
		t.Run(state, func(t *testing.T) {
			db, _ := setup(t)
			store := coordination.NewOwnershipStore(db, "a")
			scope := coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", MemoryRoleID: "role", RequestID: "seed"}
			original, _ := json.Marshal(map[string]any{"thread": map[string]any{"revision": 1, "characterId": "role"}, "lease": map[string]any{"state": state}})
			if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "continuity", ID: "task", RoleID: "role", Body: original}}}); err != nil {
				t.Fatal(err)
			}
			changed, _ := json.Marshal(map[string]any{"thread": map[string]any{"revision": 2, "characterId": "role"}, "lease": map[string]any{"state": state}})
			scope.RequestID = "transfer"
			_, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, MemoryTransfer: &coordination.MemoryTransfer{Kind: "continuity", ID: "task", ToRoleID: "other", TargetRoleRevision: 1}, Mutations: []coordination.Mutation{{Kind: "continuity", ID: "task", RoleID: "other", ExpectedRevision: 1, Body: changed}}})
			if err == nil {
				t.Fatal("live or uncertain execution partition changed")
			}
			row, _ := store.Get(t.Context(), "continuity", "task")
			if row.RoleID != "role" || row.Revision != 1 {
				t.Fatal("rejected transfer changed task")
			}
		})
	}
}
