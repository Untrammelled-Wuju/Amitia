package mindruntime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound81DurableAdmissionRevalidatesOwnerAuthority(t *testing.T) {
	store, db := usageTestStore(t)
	var allowed atomic.Bool
	allowed.Store(true)
	revoked := errors.New("durable parent authorization revoked")
	ctx := coordination.WithAdditionalGuard(context.Background(), func(context.Context) error {
		if !allowed.Load() {
			return revoked
		}
		return nil
	})
	model := modelBudgetFixture()
	tools := testOwnedToolBudget()
	if _, err := store.Begin(WithOwnedModelBudget(ctx, model), ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tools, "before-revocation", "read_file"); err != nil {
		t.Fatal(err)
	}
	allowed.Store(false)
	if _, err := store.Begin(WithOwnedModelBudget(ctx, model), ModelUsageRecord{SpaceID: model.SpaceID, ReservedTokens: 128}); !errors.Is(err, revoked) {
		t.Fatalf("revoked model admission was not rejected: %v", err)
	}
	if err := store.ReserveOwnedToolCall(ctx, tools, "after-revocation", "read_file"); !errors.Is(err, revoked) {
		t.Fatalf("revoked tool admission was not rejected: %v", err)
	}
	var modelCount, toolCount int64
	if err := db.Model(&ModelUsageRecord{}).Count(&modelCount).Error; err != nil || modelCount != 1 {
		t.Fatalf("unexpected model ledger entries: %d %v", modelCount, err)
	}
	if err := db.Model(&OwnedToolCallRecord{}).Count(&toolCount).Error; err != nil || toolCount != 1 {
		t.Fatalf("unexpected tool ledger entries: %d %v", toolCount, err)
	}
}
