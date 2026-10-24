package mindruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound86TokenOverrunRemainsUnsettledAndBlocksFurtherReservations(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	budget.TokenLimit = 256
	ctx := WithOwnedModelBudget(context.Background(), budget)
	id, err := reserveModelCall(ctx, store, 128)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 120, OutputTokens: 40, TotalTokens: 160}); err != nil {
		t.Fatal(err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.BudgetOverrun || row.UsageReported || row.ReservedTokens != 160 || row.TotalTokens != 160 || row.Status != "succeeded" {
		t.Fatalf("provider overrun must remain visible and preserve actual occupancy: %+v", row)
	}
	if _, err := reserveModelCall(ctx, store, 128); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("overrun incorrectly refunded shared budget: %v", err)
	}
	query := OwnedBudgetReconciliationScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, OwnerID: budget.OwnerID, ParentInteractionID: budget.ParentInteractionID, CoordinationID: budget.CoordinationID}
	read := coordination.WithScope(context.Background(), coordination.ExecutionScope{SpaceID: budget.SpaceID, RoleID: budget.RoleID, ResourceOwnerID: budget.OwnerID})
	pending, err := store.PendingOwnedModelReconciliation(read, query, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != id || !pending[0].BudgetOverrun {
		t.Fatalf("overrun disappeared from reconciliation: %+v %v", pending, err)
	}
	if _, err := NewModelUsageStore(db, budget.SpaceID); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id=?", id).First(&row).Error; err != nil || !row.BudgetOverrun || row.ReservedTokens != 160 {
		t.Fatalf("restart lost overrun evidence: %+v %v", row, err)
	}
}

func TestRound86CostOverrunConsumesRemainingParentAllowance(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	id, err := beginCostCall(store, budget, 200, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 150, OutputTokens: 100, TotalTokens: 250}); err != nil {
		t.Fatal(err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.BudgetOverrun || row.UsageReported || row.ReservedTokens != 250 || row.ReservedCostMicroUSD != budget.CostLimitMicroUSD {
		t.Fatalf("cost overrun must exhaust parent allowance without trusting refund: %+v", row)
	}
	if _, err := beginCostCall(store, budget, 1, 1000000); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("cost overrun incorrectly admitted next model call: %v", err)
	}
}

func TestRound86CostOverrunInsideTokenCeilingRetainsTokenReservation(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	id, err := beginCostCall(store, budget, 50, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 40, OutputTokens: 30, TotalTokens: 70}); err != nil {
		t.Fatal(err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.BudgetOverrun || row.UsageReported || row.ReservedTokens != 128 || row.ReservedCostMicroUSD != budget.CostLimitMicroUSD {
		t.Fatalf("rate snapshot under-reservation silently refunded: %+v", row)
	}
}

func TestRound86NormalProviderUsageStillReleasesUnusedQuota(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	id, err := beginCostCall(store, budget, 300, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 40, OutputTokens: 20, TotalTokens: 60}); err != nil {
		t.Fatal(err)
	}
	var row ModelUsageRecord
	if err := db.Where("id=?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.BudgetOverrun || !row.UsageReported || row.ReservedTokens != 60 || row.ReservedCostMicroUSD != 60 {
		t.Fatalf("normal usage stopped settling correctly: %+v", row)
	}
}
