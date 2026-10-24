package mindruntime

import (
	"context"
	"errors"
	"testing"
)

func costBudgetFixture() OwnedModelBudget {
	budget := modelBudgetFixture()
	budget.TokenLimit = 0
	budget.CostLimitMicroUSD = 500
	return budget
}

func beginCostCall(store *ModelUsageStore, budget OwnedModelBudget, cost int64, rate int64) (string, error) {
	return store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{
		SpaceID: budget.SpaceID, Model: "priced", Provider: "test",
		ReservedTokens: 128, ReservedCostMicroUSD: cost,
		BudgetInputRate: rate, BudgetOutputRate: rate, BudgetRateVersion: "operator-ceiling-v1",
	})
}

func TestRound79CostAtomicAdmissionAndSettlement(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	first, err := beginCostCall(store, budget, 300, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := beginCostCall(store, budget, 300, 1000000); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("cost overrun allowed: %v", err)
	}
	if err := store.Finish(context.Background(), first, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 50, OutputTokens: 10, TotalTokens: 60}); err != nil {
		t.Fatal(err)
	}
	var saved ModelUsageRecord
	if err := db.Where("id=?", first).First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ReservedCostMicroUSD != 60 || saved.BudgetRateVersion != "operator-ceiling-v1" {
		t.Fatalf("incorrect settled rate snapshot %+v", saved)
	}
	if _, err := beginCostCall(store, budget, 300, 1000000); err != nil {
		t.Fatal(err)
	}
	if _, err := beginCostCall(store, budget, 300, 1000000); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("concurrent cost cap exceeded %v", err)
	}
}

func TestRound79UnknownCostCannotRefundAfterRestart(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	first, err := beginCostCall(store, budget, 300, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), first, ModelUsageRecord{Status: "failed"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := beginCostCall(reloaded, budget, 300, 1000000); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("unknown cost refunded: %v", err)
	}
	other := budget
	other.CoordinationID = "other-budget"
	if _, err := beginCostCall(reloaded, other, 300, 1000000); err != nil {
		t.Fatal(err)
	}
}

func TestRound79CostRejectsMissingRate(t *testing.T) {
	store, _ := usageTestStore(t)
	budget := costBudgetFixture()
	_, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{SpaceID: "core", ReservedTokens: 128})
	if !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("unpriced model admitted: %v", err)
	}
}

func TestRound79CombinedCostAndTokenLimits(t *testing.T) {
	store, _ := usageTestStore(t)
	budget := costBudgetFixture()
	budget.TokenLimit = 256
	budget.CostLimitMicroUSD = 500
	if _, err := beginCostCall(store, budget, 300, 1000000); err != nil {
		t.Fatal(err)
	}
	budget.CostLimitMicroUSD = 500
	if _, err := beginCostCall(store, budget, 200, 1000000); err != nil {
		t.Fatal(err)
	}
	if _, err := beginCostCall(store, budget, 1, 1000000); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("token and cost cap bypassed: %v", err)
	}
	used, err := store.OwnedModelCostUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil || used != 500 {
		t.Fatalf("cost occupancy %d %v", used, err)
	}
}
