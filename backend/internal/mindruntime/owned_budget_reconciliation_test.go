package mindruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound82ReconciliationNeverErasesReservedAllowance(t *testing.T) {
	store, db := usageTestStore(t)
	model := modelBudgetFixture()
	ctx := WithOwnedModelBudget(context.Background(), model)
	confirmed, err := store.Begin(ctx, ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), confirmed, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 16, OutputTokens: 16, TotalTokens: 32}); err != nil {
		t.Fatal(err)
	}
	failed, err := store.Begin(ctx, ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), failed, ModelUsageRecord{Status: "failed", UsageReported: false}); err != nil {
		t.Fatal(err)
	}
	running, err := store.Begin(ctx, ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	tools := testOwnedToolBudget()
	if err := store.ReserveOwnedToolCall(context.Background(), tools, "effect-1", "read_file"); err != nil {
		t.Fatal(err)
	}
	inspection := OwnedBudgetReconciliationScope{SpaceID: model.SpaceID, RoleID: model.RoleID, OwnerID: model.OwnerID, ParentInteractionID: model.ParentInteractionID, CoordinationID: model.CoordinationID}
	read := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: model.SpaceID, RoleID: model.RoleID, ResourceOwnerID: model.OwnerID})
	modelRows, err := store.PendingOwnedModelReconciliation(read, inspection, 10)
	if err != nil || len(modelRows) != 2 || modelRows[0].ID != failed || modelRows[1].ID != running {
		t.Fatalf("unexpected model reconciliation queue: %+v %v", modelRows, err)
	}
	toolRows, err := store.ReservedOwnedToolReconciliation(read, inspection, 10)
	if err != nil || len(toolRows) != 1 || toolRows[0].ToolName != "read_file" {
		t.Fatalf("unexpected tool reconciliation queue: %+v %v", toolRows, err)
	}
	used, err := store.OwnedModelTokenUsage(context.Background(), model.SpaceID, model.CoordinationID)
	if err != nil || used != 288 {
		t.Fatalf("inspection incorrectly modified model budget: %d %v", used, err)
	}
	var reserved int64
	if err := db.Model(&OwnedToolCallRecord{}).Count(&reserved).Error; err != nil || reserved != 1 {
		t.Fatalf("inspection incorrectly modified tool ledger: %d %v", reserved, err)
	}
}

func TestRound82ReconciliationFailClosedOutsideParentScope(t *testing.T) {
	store, _ := usageTestStore(t)
	base := OwnedBudgetReconciliationScope{SpaceID: "core", RoleID: "role-a", OwnerID: "owner-a", ParentInteractionID: "parent-a", CoordinationID: "coord-a"}
	read := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: "core", RoleID: "role-a", ResourceOwnerID: "owner-a"})
	for _, query := range []struct {
		scope OwnedBudgetReconciliationScope
		limit int
		ctx   context.Context
	}{
		{OwnedBudgetReconciliationScope{SpaceID: "core", RoleID: "role-a", OwnerID: "owner-a", ParentInteractionID: "parent-a"}, 10, read},
		{OwnedBudgetReconciliationScope{SpaceID: "other", RoleID: "role-a", OwnerID: "owner-a", ParentInteractionID: "parent-a", CoordinationID: "coord-a"}, 10, read},
		{base, 0, read},
		{base, 201, read},
		{base, 10, context.Background()},
		{base, 10, coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: "core", RoleID: "role-other", ResourceOwnerID: "owner-a"})},
		{base, 10, coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: "core", RoleID: "role-a", ResourceOwnerID: "owner-other"})},
	} {
		if _, err := store.PendingOwnedModelReconciliation(query.ctx, query.scope, query.limit); !errors.Is(err, ErrOwnedBudgetInspectionScope) {
			t.Fatalf("model inspection accepted invalid query: %+v %v", query, err)
		}
		if _, err := store.ReservedOwnedToolReconciliation(query.ctx, query.scope, query.limit); !errors.Is(err, ErrOwnedBudgetInspectionScope) {
			t.Fatalf("tool inspection accepted invalid query: %+v %v", query, err)
		}
	}
}
