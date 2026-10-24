package mindruntime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound82ModelAdmissionRevokedAfterAtomicReservation(t *testing.T) {
	store, db := usageTestStore(t)
	var checks atomic.Int32
	revoked := errors.New("parent authority revoked during model reservation")
	ctx := coordination.WithAdditionalGuard(context.Background(), func(context.Context) error {
		if checks.Add(1) > 2 {
			return revoked
		}
		return nil
	})
	budget := modelBudgetFixture()
	if _, err := store.Begin(WithOwnedModelBudget(ctx, budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128}); !errors.Is(err, revoked) {
		t.Fatalf("post-reservation revocation accepted: %v", err)
	}
	if checks.Load() != 3 {
		t.Fatalf("expected admission guard before, during and after write: %d", checks.Load())
	}
	var row ModelUsageRecord
	if err := db.Where("coordination_id=?", budget.CoordinationID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "interrupted" || row.CompletedAt == "" || row.ReservedTokens != 128 || row.UsageReported {
		t.Fatalf("revoked reservation must be retained as uncertain: %+v", row)
	}
	used, err := store.OwnedModelTokenUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil || used != 128 {
		t.Fatalf("revocation incorrectly refunded budget: %d %v", used, err)
	}
	restarted, err := NewModelUsageStore(db, budget.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	used, err = restarted.OwnedModelTokenUsage(context.Background(), budget.SpaceID, budget.CoordinationID)
	if err != nil || used != 128 {
		t.Fatalf("restart lost uncertain reservation: %d %v", used, err)
	}
}

func TestRound82ToolAdmissionRevokedAfterAtomicReservation(t *testing.T) {
	store, db := usageTestStore(t)
	var checks atomic.Int32
	revoked := errors.New("parent authority revoked during tool reservation")
	ctx := coordination.WithAdditionalGuard(context.Background(), func(context.Context) error {
		if checks.Add(1) > 2 {
			return revoked
		}
		return nil
	})
	budget := testOwnedToolBudget()
	if err := store.ReserveOwnedToolCall(ctx, budget, "call-1", "shell_run"); !errors.Is(err, revoked) {
		t.Fatalf("post-reservation revocation accepted: %v", err)
	}
	if checks.Load() != 3 {
		t.Fatalf("expected tool guard before, during and after write: %d", checks.Load())
	}
	var rows int64
	if err := db.Model(&OwnedToolCallRecord{}).Where("coordination_id=?", budget.CoordinationID).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("revoked tool allocation should remain reserved: %d %v", rows, err)
	}
	restarted, err := NewModelUsageStore(db, budget.SpaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.ReserveOwnedToolCall(context.Background(), budget, "call-1", "shell_run"); !errors.Is(err, ErrOwnedToolEffectUncertain) {
		t.Fatalf("revoked effect must not be retried after restart: %v", err)
	}
}

func TestRound82ValidPostReservationAuthority(t *testing.T) {
	store, _ := usageTestStore(t)
	var checks atomic.Int32
	ctx := coordination.WithAdditionalGuard(context.Background(), func(context.Context) error {
		checks.Add(1)
		return nil
	})
	budget := modelBudgetFixture()
	if _, err := store.Begin(WithOwnedModelBudget(ctx, budget), ModelUsageRecord{SpaceID: budget.SpaceID, ReservedTokens: 128}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveOwnedToolCall(ctx, testOwnedToolBudget(), "call-2", "read_file"); err != nil {
		t.Fatal(err)
	}
	if checks.Load() != 6 {
		t.Fatalf("expected three guard checks per reservation, got %d", checks.Load())
	}
}
