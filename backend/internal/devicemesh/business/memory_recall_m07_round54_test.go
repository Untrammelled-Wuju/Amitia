package business

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestM07OwnedContextRecallExcludesArchivedExpiredAndDisallowedBeforePagination(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "m07-recall"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	scope := view.Scope
	scope.RequestID = "m07-recall-seed"
	items := []struct{ id, body string }{
		{"a-archived", `{"key":"a","archived":true,"content":{"value":"source evidence"}}`},
		{"b-disallowed", `{"key":"b","allowContextUse":false,"content":{"value":"source evidence"}}`},
		{"c-expired", `{"key":"c","expiresAt":"2020-01-01T00:00:00Z","content":{"value":"source evidence"}}`},
		{"d-active", `{"key":"d","content":{"value":"source evidence"}}`},
	}
	mutations := make([]coordination.Mutation, 0, len(items))
	for _, item := range items {
		mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: item.id, RoleID: "role", Body: json.RawMessage(item.body)})
	}
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	result, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{ContextUsable: true, Query: "source evidence", Mode: "keyword", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Resources) != 1 || result.Resources[0].ID != "d-active" || result.NextCursor != "" {
		t.Fatalf("context recall leaked non-usable memories: %+v", result)
	}
	management, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{ContextUsable: false, Query: "source evidence", Mode: "keyword", Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	if management.Total != 4 {
		t.Fatalf("management visibility unexpectedly changed: %d", management.Total)
	}
}
