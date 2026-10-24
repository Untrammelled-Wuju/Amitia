package mindruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func modelBudgetFixture() OwnedModelBudget {
	return OwnedModelBudget{SpaceID: "core", RoleID: "role-a", OwnerID: "owner-a", ParentInteractionID: "parent-a", CoordinationID: "coord-a", AssignmentID: "child-a", TokenLimit: 512}
}

func reserveModelCall(ctx context.Context, store *ModelUsageStore, units int64) (string, error) {
	return store.Begin(ctx, ModelUsageRecord{SpaceID: "core", Model: "model", Provider: "test", ReservedTokens: units})
}

func TestRound77AtomicSharedTokenBudgetAcrossChildren(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 48; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			child := budget
			child.AssignmentID = fmt.Sprintf("child-%d", index%6)
			if _, err := reserveModelCall(WithOwnedModelBudget(context.Background(), child), store, 128); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	sum, err := store.OwnedModelTokenUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil {
		t.Fatal(err)
	}
	if sum > budget.TokenLimit || ok.Load() != int32(sum/128) || sum == 0 {
		t.Fatalf("overspent shared budget admitted=%d reserved=%d", ok.Load(), sum)
	}
	var count int64
	if err := db.Model(&ModelUsageRecord{}).Where("coordination_id=?", budget.CoordinationID).Count(&count).Error; err != nil || count != int64(ok.Load()) {
		t.Fatalf("persisted=%d admitted=%d err=%v", count, ok.Load(), err)
	}
}

func TestRound77KnownUsageSettlementAndUnknownPreservedOnRestart(t *testing.T) {
	store, db := usageTestStore(t)
	budget := modelBudgetFixture()
	ctx := WithOwnedModelBudget(context.Background(), budget)
	first, err := reserveModelCall(ctx, store, 256)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, first, ModelUsageRecord{Status: "succeeded", UsageReported: true, InputTokens: 30, OutputTokens: 10, TotalTokens: 40}); err != nil {
		t.Fatal(err)
	}
	unknown, err := reserveModelCall(ctx, store, 256)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, unknown, ModelUsageRecord{Status: "failed"}); err != nil {
		t.Fatal(err)
	}
	sum, err := store.OwnedModelTokenUsage(ctx, "core", budget.CoordinationID)
	if err != nil || sum != 296 {
		t.Fatalf("known 40 unknown 256 expected 296 got %d %v", sum, err)
	}
	interrupted, err := reserveModelCall(ctx, store, 128)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	var pending ModelUsageRecord
	if err := db.Where("id=?", interrupted).First(&pending).Error; err != nil || pending.Status != "running" {
		t.Fatalf("another Core initialization must preserve uncertain owned reservations: %+v %v", pending, err)
	}
	sum, err = reloaded.OwnedModelTokenUsage(ctx, "core", budget.CoordinationID)
	if err != nil || sum != 424 {
		t.Fatalf("uncertain restart occupancy %d %v", sum, err)
	}
	if _, err := reserveModelCall(ctx, reloaded, 128); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("uncertain tokens refunded: %v", err)
	}
	other := budget
	other.CoordinationID = "coord-b"
	if _, err := reserveModelCall(WithOwnedModelBudget(context.Background(), other), reloaded, 128); err != nil {
		t.Fatalf("other parent blocked %v", err)
	}
}

func TestRound77ModelBudgetRejectsUntrustedScopeAndOversize(t *testing.T) {
	store, _ := usageTestStore(t)
	budget := modelBudgetFixture()
	for _, candidate := range []OwnedModelBudget{
		{SpaceID: "core", TokenLimit: 100},
		{SpaceID: "core", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "coord", AssignmentID: "child", TokenLimit: 0},
		{SpaceID: "other", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "coord", AssignmentID: "child", TokenLimit: 100},
	} {
		ctx := WithOwnedModelBudget(context.Background(), candidate)
		if _, err := reserveModelCall(ctx, store, 64); err == nil {
			t.Fatalf("invalid scope admitted %+v", candidate)
		}
	}
	ctx := WithOwnedModelBudget(context.Background(), budget)
	if _, err := reserveModelCall(ctx, store, 600); !errors.Is(err, ErrOwnedModelBudgetExceeded) {
		t.Fatalf("oversize admission: %v", err)
	}
}
