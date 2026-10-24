package mindruntime

import (
	"context"
	"testing"
)

func TestRound80ConcurrentStoreStartupPreservesActiveOwnedReservation(t *testing.T) {
	first, db := usageTestStore(t)
	budget := costBudgetFixture()
	id, err := beginCostCall(first, budget, 300, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewModelUsageStore(db, budget.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	var before ModelUsageRecord
	if err := db.Where("id=?", id).First(&before).Error; err != nil || before.Status != "running" {
		t.Fatalf("another Core must not interrupt live owned call: %+v %v", before, err)
	}
	if err := first.Finish(context.Background(), id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 25, OutputTokens: 10, TotalTokens: 35}); err != nil {
		t.Fatalf("first Core could not settle after second startup: %v", err)
	}
	usage, err := second.OwnedModelCostUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil || usage != 35 {
		t.Fatalf("shared cost settlement should be 35, got %d %v", usage, err)
	}
}

func TestRound80UnclassifiedTokensCannotReleaseCostAtZeroRate(t *testing.T) {
	store, db := usageTestStore(t)
	budget := costBudgetFixture()
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{
		SpaceID: budget.SpaceID, ReservedTokens: 128, ReservedCostMicroUSD: 300,
		BudgetInputRate: 1000000, BudgetOutputRate: 3000000, BudgetRateVersion: "ceiling-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{
		Status: "succeeded", UsageReported: true, InputTokens: 10, OutputTokens: 5, TotalTokens: 30,
	}); err != nil {
		t.Fatal(err)
	}
	var settled ModelUsageRecord
	if err := db.Where("id=?", id).First(&settled).Error; err != nil {
		t.Fatal(err)
	}
	if !settled.UsageReported || settled.ReservedTokens != 30 || settled.ReservedCostMicroUSD != 70 {
		t.Fatalf("unclassified tokens must be charged at highest rate: %+v", settled)
	}
}

func TestRound80UnclassifiedCostIsIncludedInSharedAtomicAdmission(t *testing.T) {
	store, _ := usageTestStore(t)
	budget := costBudgetFixture()
	budget.CostLimitMicroUSD = 100
	id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{
		SpaceID: budget.SpaceID, ReservedTokens: 128, ReservedCostMicroUSD: 80,
		BudgetInputRate: 1000000, BudgetOutputRate: 3000000, BudgetRateVersion: "ceiling-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(context.Background(), id, ModelUsageRecord{
		Status: "succeeded", UsageReported: true, InputTokens: 5, OutputTokens: 5, TotalTokens: 40,
	}); err != nil {
		t.Fatal(err)
	}
	used, err := store.OwnedModelCostUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil || used != 100 {
		t.Fatalf("cost ceiling must remain exhausted after over-reservation actual use: %d %v", used, err)
	}
}

func TestRound80FailedOrCancelledStreamingUsageCannotRefund(t *testing.T) {
	for _, status := range []string{"failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			store, db := usageTestStore(t)
			budget := costBudgetFixture()
			budget.TokenLimit = 128
			id, err := store.Begin(WithOwnedModelBudget(context.Background(), budget), ModelUsageRecord{
				SpaceID: budget.SpaceID, ReservedTokens: 128, ReservedCostMicroUSD: 300,
				BudgetInputRate: 1000000, BudgetOutputRate: 1000000, BudgetRateVersion: "ceiling-v1",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Finish(context.Background(), id, ModelUsageRecord{Status: status, UsageReported: true, InputTokens: 10, OutputTokens: 5, TotalTokens: 15}); err != nil {
				t.Fatal(err)
			}
			var saved ModelUsageRecord
			if err := db.Where("id=?", id).First(&saved).Error; err != nil {
				t.Fatal(err)
			}
			if saved.UsageReported || saved.ReservedTokens != 128 || saved.ReservedCostMicroUSD != 300 {
				t.Fatalf("unconfirmed provider effects cannot release shared allowance: %+v", saved)
			}
		})
	}
}
