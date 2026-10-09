package coordination_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestManagementIntentTransactionRejectsPolicyAndTrustChanges(t *testing.T) {
	for _, change := range []string{"permission", "mode", "provider", "trust"} {
		t.Run(change, func(t *testing.T) {
			db, service := setup(t)
			ctx, _, finish, err := service.Begin(t.Context(), "space", "a", "", "space", "", "original")
			if err != nil {
				t.Fatal(err)
			}
			defer finish()
			guarded, err := coordination.WithRequestAuthority(ctx, ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO kernel_device_coordination(space_id,device_id) VALUES('space','a')`); err != nil {
				t.Fatal(err)
			}
			statement := map[string]string{
				"permission": `UPDATE kernel_device_coordination SET permission_revision=permission_revision+2 WHERE device_id='a'`,
				"mode":       `UPDATE kernel_device_coordination SET mode_revision=mode_revision+2 WHERE device_id='a'`,
				"provider":   `UPDATE kernel_device_coordination SET provider_epoch=provider_epoch+2 WHERE device_id='a'`,
				"trust":      `UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='a'`,
			}[change]
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err := coordination.ValidateRequestAuthorityTx(guarded, tx); !errors.Is(err, coordination.ErrScopeExpired) {
				t.Fatalf("transaction accepted %s drift: %v", change, err)
			}
		})
	}
}

func TestManagementIntentBlocksEveryMutationAfterActorModeABA(t *testing.T) {
	_, service := setup(t)
	ctx, _, finish, err := service.Begin(t.Context(), "space", "a", "", "space", "", "original")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	guarded, err := coordination.WithRequestAuthority(ctx, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "space", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "space", "a", 2, false, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetCapabilityGrant(guarded, "space", "a", "b", "ai.chat", 0, true); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old grant: %v", err)
	}
	if _, err := service.ChangeMode(guarded, "space", "b", 1, true, "role"); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old mode: %v", err)
	}
	if _, err := service.GrantAdministrator(guarded, "space", "b", 1, false); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old admin: %v", err)
	}
	called := false
	if err := service.RevokeDevice(guarded, "space", "b", func(*sql.Tx) error { called = true; return nil }); !errors.Is(err, coordination.ErrScopeExpired) || called {
		t.Fatalf("old revoke: called=%v %v", called, err)
	}
}

func TestManagementIntentSelfModeAndAdministratorReturnCommittedPolicy(t *testing.T) {
	_, service := setup(t)
	ctx, _, finish, err := service.Begin(t.Context(), "space", "a", "", "space", "", "mode")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	guarded, err := coordination.WithRequestAuthority(ctx, ctx)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := service.ChangeMode(guarded, "space", "a", 1, true, "role")
	if err != nil || policy.ModeRevision != 2 || policy.PermissionRevision != 2 || !policy.Coordinated {
		t.Fatalf("committed self mode: %+v %v", policy, err)
	}
	ctx2, _, finish2, err := service.Begin(t.Context(), "space", "a", "", "space", "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer finish2()
	guarded2, err := coordination.WithRequestAuthority(ctx2, ctx2)
	if err != nil {
		t.Fatal(err)
	}
	policy, err = service.GrantAdministrator(guarded2, "space", "a", 2, true)
	if err != nil || policy.PermissionRevision != 3 || !policy.Administrator {
		t.Fatalf("committed self admin: %+v %v", policy, err)
	}
}
