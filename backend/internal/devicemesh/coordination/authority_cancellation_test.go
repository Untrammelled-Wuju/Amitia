package coordination_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPermissionChangeStopsRunningSourceBeforeWaitingForItsFence(t *testing.T) {
	db, service := setup(t)
	execution, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "running")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(execution, scope, "session", 1); err != nil {
		t.Fatal(err)
	}
	service.SetAuthorityBarrier(func(ctx context.Context, space, device string, revision int64, _ []string) error {
		select {
		case <-execution.Done():
			if !errors.Is(context.Cause(execution), coordination.ErrScopeExpired) {
				t.Fatal("source received the wrong cancellation reason")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		return coordination.FenceSourceAuthority(ctx, db, space, device, revision)
	})
	change, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := service.ChangeMode(change, "space", "b", 1, true, "role"); err != nil {
		t.Fatalf("mode change waited for a running action that it had not cancelled: %v", err)
	}
}

func TestFailedOwnerFenceCancelsRunningWorkButDoesNotCommitNewPermission(t *testing.T) {
	_, service := setup(t)
	execution, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "running")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(execution, scope, "session", 1); err != nil {
		t.Fatal(err)
	}
	service.SetAuthorityBarrier(func(context.Context, string, string, int64, []string) error { return errors.New("source offline") })
	if _, err := service.ChangeMode(t.Context(), "space", "b", 1, true, "role"); !errors.Is(err, coordination.ErrAuthorityUnconfirmed) {
		t.Fatalf("unconfirmed mode was accepted: %v", err)
	}
	if !errors.Is(context.Cause(execution), coordination.ErrScopeExpired) {
		t.Fatal("old execution continued after attempted authority closure")
	}
	policy, err := service.Get(t.Context(), "space", "b")
	if err != nil || policy.Coordinated || policy.ModeRevision != 1 || policy.PermissionRevision != 1 {
		t.Fatalf("failed source fence committed policy: %+v %v", policy, err)
	}
}
