package coordination_test

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestCandidateManagementPagesDoNotExposeExecutionReceipts(t *testing.T) {
	db, _ := setup(t)
	for _, id := range []string{"memory-candidate/first", "memory-candidate/second", "memory-candidate-operation/request", "turn/request", "memory/request"} {
		if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','checkpoint',?,'role',1,?,'2026-10-08T00:00:00Z')`, id, []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	store := coordination.NewOwnershipStore(db, "a")
	query := coordination.DataQuery{Management: true, ResourceKind: "checkpoint", Limit: 1}
	first, cursor, err := store.ListPage(t.Context(), "checkpoint", "role", query)
	if err != nil || len(first) != 1 || cursor == "" {
		t.Fatalf("first=%v cursor=%q error=%v", first, cursor, err)
	}
	query.Cursor = cursor
	second, next, err := store.ListPage(t.Context(), "checkpoint", "role", query)
	if err != nil || len(second) != 1 || next != "" || second[0].ID == first[0].ID {
		t.Fatalf("second=%v next=%q error=%v", second, next, err)
	}
	for _, row := range append(first, second...) {
		if row.ID != "memory-candidate/first" && row.ID != "memory-candidate/second" {
			t.Fatal("internal execution receipt exposed in memory candidates")
		}
	}
	rows, _, err := store.ListPage(t.Context(), "checkpoint", "role", coordination.DataQuery{ResourceKind: "checkpoint"})
	if err != nil || len(rows) != 0 {
		t.Fatal("ordinary prompt reads exposed candidate management")
	}
	rows, _, err = store.ListPage(t.Context(), "checkpoint", "role", coordination.DataQuery{ResourceKind: "checkpoint", RequestID: "request"})
	if err != nil || len(rows) != 2 {
		t.Fatal("turn recovery lost its original receipts")
	}
}
