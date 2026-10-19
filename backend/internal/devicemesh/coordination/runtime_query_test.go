package coordination_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRuntimeToolResultQueryFiltersRequestBeforeLimit(t *testing.T) {
	db, _ := setup(t)
	for index := 0; index < 140; index++ {
		request := "other"
		if index < 2 {
			request = "selected"
		}
		raw, _ := json.Marshal(map[string]any{"conversationId": "conversation", "executionScope": map[string]any{"requestId": request}})
		if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','tool-result',?,'role',1,?,'2026-10-18T00:00:00Z')`, fmt.Sprintf("result-%03d", index), raw); err != nil {
			t.Fatal(err)
		}
	}
	store := coordination.NewOwnershipStore(db, "a")
	query := coordination.DataQuery{Management: true, RuntimeTurns: true, ResourceKind: "tool-result", ConversationID: "conversation", RequestID: "selected", Limit: 1}
	rows, next, err := store.ListPage(t.Context(), "tool-result", "role", query)
	if err != nil || len(rows) != 1 || rows[0].ID != "result-001" || next == "" {
		t.Fatalf("selected result hidden by other requests: %#v %s %v", rows, next, err)
	}
	query.Cursor, query.RequestID = next, "other"
	if _, _, err := store.ListPage(t.Context(), "tool-result", "role", query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign request cursor accepted: %v", err)
	}
}
