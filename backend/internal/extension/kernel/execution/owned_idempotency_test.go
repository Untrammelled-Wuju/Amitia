package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestOwnedIdempotencyRejectsFingerprintCollisionAndUnknownRestart(t *testing.T) {
	db := newTestIdempotencyDB(t)
	storage := NewExecutionIdempotencyStorage(db)
	guard := NewIdempotencyGuard(storage)
	t.Cleanup(guard.stopCleanup)
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{CoreID: "core", TargetDeviceID: "device", Coordinated: true})
	identity := IdempotencyIdentity{ToolID: "tool", CallerKey: "action", SpaceID: "core"}
	reservation, hit, err := guard.Begin(ctx, identity, "original")
	if err != nil || hit {
		t.Fatalf("first reservation: hit=%v err=%v", hit, err)
	}
	restarted := NewIdempotencyGuard(storage)
	t.Cleanup(restarted.stopCleanup)
	if _, _, err := restarted.Begin(ctx, identity, "original"); !errors.Is(err, ErrIdempotencyIndeterminate) {
		t.Fatalf("unknown action allowed after restart: %v", err)
	}
	result := capability.NewToolSuccessResult("invoke", "tool")
	if err := guard.Complete(ctx, reservation, &result); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := restarted.Begin(ctx, identity, "original"); err != nil || !hit {
		t.Fatalf("confirmed result lost: hit=%v err=%v", hit, err)
	}
	if _, _, err := restarted.Begin(ctx, identity, "changed"); !errors.Is(err, ErrIdempotencyFingerprint) {
		t.Fatalf("parameter collision accepted: %v", err)
	}
}

func TestIdempotencyWaitingCallerStopsOnProviderCancellation(t *testing.T) {
	guard := NewIdempotencyGuard(NewExecutionIdempotencyStorage(newTestIdempotencyDB(t)))
	t.Cleanup(guard.stopCleanup)
	identity := IdempotencyIdentity{ToolID: "tool", CallerKey: "action"}
	reservation, _, err := guard.Begin(t.Context(), identity, "original")
	if err != nil {
		t.Fatal(err)
	}
	defer guard.signalFlight(reservation.IdempotencyKey)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(coordination.ErrScopeExpired)
	if _, _, err := guard.Begin(ctx, identity, "original"); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("cancelled caller kept waiting: %v", err)
	}
}
