package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
)

func TestPlanSparseOrderAppendsAfterLast(t *testing.T) {
	entries := []orderPlanEntry{{ID: "a", Order: 1024}, {ID: "b", Order: 2048}, {ID: "c", Order: 0}}
	plan, err := planSparseOrder(entries, "c", "", 1024)
	if err != nil {
		t.Fatalf("plan append: %v", err)
	}
	if len(plan) != 1 || plan[0].ID != "c" || plan[0].Order != 3072 {
		t.Fatalf("unexpected append plan: %+v", plan)
	}
}

func TestPlanSparseOrderPrependsBeforeFirst(t *testing.T) {
	entries := []orderPlanEntry{{ID: "a", Order: 2048}, {ID: "b", Order: 3072}, {ID: "c", Order: 4096}}
	plan, err := planSparseOrder(entries, "c", "a", 1024)
	if err != nil {
		t.Fatalf("plan prepend: %v", err)
	}
	if len(plan) != 1 || plan[0].ID != "c" || plan[0].Order != 1024 {
		t.Fatalf("unexpected prepend plan: %+v", plan)
	}
}

func TestPlanSparseOrderUsesMidpointBetweenNeighbors(t *testing.T) {
	entries := []orderPlanEntry{{ID: "a", Order: 1024}, {ID: "b", Order: 4096}, {ID: "c", Order: 1024}}
	plan, err := planSparseOrder(entries, "c", "b", 1024)
	if err != nil {
		t.Fatalf("plan midpoint: %v", err)
	}
	if len(plan) != 1 || plan[0].ID != "c" || plan[0].Order != 2560 {
		t.Fatalf("unexpected midpoint plan: %+v", plan)
	}
}

func TestPlanSparseOrderRenumbersWhenGapIsExhausted(t *testing.T) {
	entries := []orderPlanEntry{{ID: "a", Order: 1024}, {ID: "b", Order: 1025}, {ID: "c", Order: 4096}}
	plan, err := planSparseOrder(entries, "c", "b", 1024)
	if err != nil {
		t.Fatalf("plan renumber: %v", err)
	}
	expected := map[string]int64{"a": 1024, "c": 2048, "b": 3072}
	if len(plan) != 2 {
		t.Fatalf("unexpected renumber plan: %+v", plan)
	}
	for _, change := range plan {
		if expected[change.ID] != change.Order {
			t.Fatalf("unexpected renumber order for %s: %+v", change.ID, plan)
		}
	}
}

func TestPlanSparseOrderRejectsUnknownAnchorAndSelf(t *testing.T) {
	entries := []orderPlanEntry{{ID: "a", Order: 1024}, {ID: "b", Order: 2048}}
	if _, err := planSparseOrder(entries, "a", "missing", 1024); err == nil {
		t.Fatal("unknown anchor accepted")
	}
	if _, err := planSparseOrder(entries, "a", "a", 1024); err == nil {
		t.Fatal("self anchor accepted")
	}
	if _, err := planSparseOrder(entries, "missing", "", 1024); err == nil {
		t.Fatal("unknown moving entry accepted")
	}
}

func TestOwnedChatReorderMovesConversationBeforeAnchor(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	ctx := t.Context()
	scope := prepareTestScope(p, "reorder-scope")
	for index, id := range []string{"conv-a", "conv-b", "conv-c"} {
		request := coordination.ChatManagementRequest{Operation: "conversation/create", ConversationID: id, ConversationOwnerID: p.ownerID, RequestID: "reorder-create-" + id, Title: "会话"}
		if _, err := p.ManageChat(ctx, scope, request); err != nil {
			t.Fatalf("create conversation %d: %v", index, err)
		}
	}
	request := coordination.ChatManagementRequest{Operation: "conversation/reorder", ConversationID: "conv-c", BeforeConversationID: "conv-a", ConversationOwnerID: p.ownerID, RequestID: "reorder-move"}
	result, err := p.ManageChat(ctx, scope, request)
	if err != nil {
		t.Fatalf("reorder conversation: %v", err)
	}
	ids, _ := result["conversationIds"].([]string)
	expected := []string{"conv-c", "conv-a", "conv-b"}
	if len(ids) != len(expected) {
		t.Fatalf("unexpected reordered ids: %+v", result)
	}
	for index := range expected {
		if ids[index] != expected[index] {
			t.Fatalf("unexpected reordered ids: %+v", ids)
		}
	}
	orders := make([]int64, 0, len(expected))
	for _, id := range expected {
		resource, err := p.store.Get(ctx, "conversation", id)
		if err != nil || resource == nil {
			t.Fatalf("read conversation %s: %+v %v", id, resource, err)
		}
		var document map[string]any
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			t.Fatalf("decode conversation %s: %v", id, err)
		}
		order, _ := document["order"].(float64)
		orders = append(orders, int64(order))
	}
	for index := 1; index < len(orders); index++ {
		if orders[index] <= orders[index-1] {
			t.Fatalf("conversation order not ascending: %+v", orders)
		}
	}
}

func TestProjectUpdateChangesSkipUnsetPointers(t *testing.T) {
	order := int64(4096)
	changes, err := projectUpdateChanges(kernel.ProjectUpdateRequest{ProjectID: "project-1", Order: &order, RequestID: "reorder"})
	if err != nil {
		t.Fatalf("build changes: %v", err)
	}
	if len(changes) != 1 || string(changes["order"]) != "4096" {
		t.Fatalf("unexpected order changes: %v", changes)
	}
	pinned := true
	changes, err = projectUpdateChanges(kernel.ProjectUpdateRequest{ProjectID: "project-1", Pinned: &pinned})
	if err != nil {
		t.Fatalf("build changes: %v", err)
	}
	if len(changes) != 1 || string(changes["pinned"]) != "true" {
		t.Fatalf("unexpected pinned changes: %v", changes)
	}
	cleared := ""
	changes, err = projectUpdateChanges(kernel.ProjectUpdateRequest{ProjectID: "project-1", PrimaryWorkspaceID: &cleared})
	if err != nil {
		t.Fatalf("build changes: %v", err)
	}
	if len(changes) != 1 || string(changes["primaryWorkspaceId"]) != `""` {
		t.Fatalf("unexpected primary workspace changes: %v", changes)
	}
}
