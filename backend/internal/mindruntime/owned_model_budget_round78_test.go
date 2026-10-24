package mindruntime

import (
	"context"
	"testing"
)

func TestRound78OnlyVerifiedConsistentUsageSettlesReservation(t *testing.T) {
	cases := []struct {
		usage ModelUsageRecord
		valid bool
	}{
		{ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 12, OutputTokens: 5, TotalTokens: 17}, true},
		{ModelUsageRecord{Status: "succeeded", UsageReported: false, InputTokens: 12, OutputTokens: 5, TotalTokens: 17}, false},
		{ModelUsageRecord{Status: "failed", UsageReported: true, InputTokens: 12, OutputTokens: 5, TotalTokens: 17}, false},
		{ModelUsageRecord{Status: "cancelled", UsageReported: true, InputTokens: 12, OutputTokens: 5, TotalTokens: 17}, false},
		{ModelUsageRecord{Status: "succeeded", UsageReported: true, TotalTokens: 0}, false},
		{ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 20, OutputTokens: 10, TotalTokens: 12}, false},
		{ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: -1, TotalTokens: 12}, false},
		{ModelUsageRecord{Status: "succeeded", UsageReported: true, CachedTokens: -1, TotalTokens: 12}, false},
	}
	for _, tc := range cases {
		if got := validOwnedModelSettlement(tc.usage); got != tc.valid {
			t.Fatalf("usage=%+v valid=%v want %v", tc.usage, got, tc.valid)
		}
	}
	store, db := usageTestStore(t)
	ctx := WithOwnedModelBudget(context.Background(), modelBudgetFixture())
	id, err := reserveModelCall(ctx, store, 256)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, id, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 80, OutputTokens: 20, TotalTokens: 90}); err != nil {
		t.Fatal(err)
	}
	usage, err := store.OwnedModelTokenUsage(ctx, "core", "coord-a")
	if err != nil || usage != 256 {
		t.Fatalf("unreliable totals incorrectly released reservation %d %v", usage, err)
	}
	var saved ModelUsageRecord
	if err := db.Where("id=?", id).First(&saved).Error; err != nil || saved.UsageReported {
		t.Fatalf("inconsistent usage incorrectly marked trustworthy: %+v %v", saved, err)
	}
}
