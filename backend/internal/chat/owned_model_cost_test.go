package chat

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/mindruntime"
)

func TestRound79MissingConfiguredModelPriceBlocksProvider(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	ctx := mindruntime.WithOwnedModelBudget(context.Background(), mindruntime.OwnedModelBudget{
		SpaceID: "space", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "round79-cost", AssignmentID: "child", CostLimitMicroUSD: 1000000,
	})
	cfg := &ModelConfig{ModelName: "test", ContextWindow: 8192, MaxTokens: 1000}
	if _, err := f.model.meterModel(ctx, cfg); err == nil {
		t.Fatal("model without ceiling price admitted")
	}
	cfg.BudgetRateVersion = "operator-ceiling-v1"
	cfg.BudgetInputRate = 1000000
	cfg.BudgetOutputRate = 2000000
	if _, err := f.model.meterModel(ctx, cfg); err != nil {
		t.Fatalf("priced model blocked: %v", err)
	}
}
