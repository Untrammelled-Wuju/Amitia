package coordination_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestMessageBytePaginationKeepsEveryLargeMessageExactlyOnce(t *testing.T) {
	db, _ := setup(t)
	store := coordination.NewOwnershipStore(db, "a")
	for i := 0; i < 5; i++ {
		payload, _ := json.Marshal(map[string]string{"id": fmt.Sprint(i), "conversationId": "conversation", "createdAt": fmt.Sprintf("2026-10-04T00:00:0%dZ", i), "content": strings.Repeat("x", 1<<20)})
		commit := coordination.Commit{Scope: coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", RequestID: fmt.Sprint(i)}, Mutations: []coordination.Mutation{{Kind: "message", ID: fmt.Sprint(i), RoleID: "role", Body: payload}}}
		if _, err := store.Apply(t.Context(), commit); err != nil {
			t.Fatal(err)
		}
	}
	query := coordination.DataQuery{ConversationID: "conversation", ResourceKind: "message", Limit: 128}
	seen := map[string]bool{}
	pages := 0
	for {
		resources, cursor, err := store.ListPage(t.Context(), "message", "role", query)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(resources)
		if len(encoded) > 3<<20 || len(resources) == 0 {
			t.Fatalf("oversized or empty page: %d", len(encoded))
		}
		for _, resource := range resources {
			if seen[resource.ID] {
				t.Fatal("message repeated across byte pages")
			}
			seen[resource.ID] = true
		}
		pages++
		if cursor == "" {
			break
		}
		if pages > 5 {
			t.Fatal("cursor did not advance")
		}
		query.Cursor = cursor
	}
	if len(seen) != 5 || pages != 3 {
		t.Fatalf("messages=%d pages=%d", len(seen), pages)
	}
}
