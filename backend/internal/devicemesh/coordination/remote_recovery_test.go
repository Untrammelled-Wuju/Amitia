package coordination_test

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRemoteRecoveryCancelsOldInvocationIDsAndPreservesPolicyAndCurrentGeneration(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	old, err := service.TrackRemoteAuthority(ctx, scope, "session", 1)
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.TrackRemoteAuthority(ctx, scope, "session", 2)
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.Get(t.Context(), "space", "b")
	if err != nil {
		t.Fatal(err)
	}
	err = service.ReconcileRemoteAuthority(t.Context(), "space", "b", "session", 2, func(ctx context.Context, ids []string) error {
		if len(ids) != 1 || ids[0] != old {
			t.Fatalf("current generation cancelled: %v", ids)
		}
		return coordination.CancelSourceAuthority(ctx, db, "space", ids)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateSourceCall(t.Context(), db, "space", old); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("delayed old invocation accepted: %v", err)
	}
	if err := coordination.ValidateSourceCall(t.Context(), db, "space", current); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateSourceCall(t.Context(), db, "other-core", old); err != nil {
		t.Fatal("cancellation crossed realm")
	}
	after, err := service.Get(t.Context(), "space", "b")
	if err != nil || after != before {
		t.Fatal("reconnection changed ownership or administration")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_remote_authority`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("current flight lost: %d %v", count, err)
	}
	if err := service.ConfirmRemoteAuthority(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "space", "b", 1, true, "role"); err != nil {
		t.Fatalf("confirmed recovery still blocked: %v", err)
	}
}

func TestRemoteRecoveryWithoutOwnerAckRetainsUnknownCall(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(ctx, scope, "old-session", 8); err != nil {
		t.Fatal(err)
	}
	missing := errors.New("source unavailable")
	if err := service.ReconcileRemoteAuthority(t.Context(), "space", "b", "new-session", 1, func(context.Context, []string) error { return missing }); !errors.Is(err, coordination.ErrAuthorityUnconfirmed) {
		t.Fatalf("missing owner ACK ignored: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_remote_authority`).Scan(&count); err != nil || count != 1 {
		t.Fatal("unknown call erased without owner ACK")
	}
}
