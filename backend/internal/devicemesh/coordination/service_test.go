package coordination_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func setup(t *testing.T) (*sql.DB, *coordination.Service) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"a", "b"} {
		if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES(?,'space','trusted','now','now')`, device); err != nil {
			t.Fatal(err)
		}
	}
	return db, coordination.NewService(db)
}

func TestAdministratorRequiresModeAndNeverRestoresImplicitly(t *testing.T) {
	db, svc := setup(t)
	ctx := t.Context()
	if _, err := svc.GrantAdministrator(ctx, "space", "a", 1, true); !errors.Is(err, coordination.ErrAdministratorMode) {
		t.Fatalf("grant while off: %v", err)
	}
	p, err := svc.ChangeMode(ctx, "space", "a", 1, true, "core-role")
	if err != nil || p.Administrator {
		t.Fatalf("mode alone granted administration: %+v %v", p, err)
	}
	p, err = svc.GrantAdministrator(ctx, "space", "a", p.PermissionRevision, true)
	if err != nil || !p.Administrator {
		t.Fatalf("explicit grant failed: %+v %v", p, err)
	}
	p, err = svc.ChangeMode(ctx, "space", "a", p.ModeRevision, false, "device-role")
	if err != nil || p.Administrator {
		t.Fatalf("off retained administrator: %+v %v", p, err)
	}
	p, err = svc.ChangeMode(ctx, "space", "a", p.ModeRevision, true, "core-role")
	if err != nil || p.Administrator {
		t.Fatalf("on restored administrator: %+v %v", p, err)
	}
	restarted := coordination.NewService(db)
	p, err = restarted.Get(ctx, "space", "a")
	if err != nil || !p.Coordinated || p.Administrator {
		t.Fatalf("restart lost policy: %+v %v", p, err)
	}
	other, err := restarted.Get(ctx, "space", "b")
	if err != nil || other.Coordinated || other.Administrator {
		t.Fatalf("policy crossed devices: %+v %v", other, err)
	}
}

func TestModeCancelsActiveGenerationAndRejectsLateCommit(t *testing.T) {
	_, svc := setup(t)
	ctx, scope, finish, err := svc.Begin(t.Context(), "space", "a", "", "core-b", "device-role", "request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if scope.RoleOwnerID != "a" || scope.ResourceOwnerID != "a" {
		t.Fatalf("off owner: %+v", scope)
	}
	if _, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "core-role"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("generation was not interrupted")
	}
	if err := svc.Validate(context.Background(), scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("late commit accepted: %v", err)
	}
	nextCtx, next, nextFinish, err := svc.Begin(t.Context(), "space", "a", "", "core-b", "core-role", "request-2")
	if err != nil {
		t.Fatal(err)
	}
	defer nextFinish()
	if next.ResourceOwnerID != "core-b" || next.RoleOwnerID != "core-b" {
		t.Fatalf("on owner: %+v", next)
	}
	if err := svc.Validate(nextCtx, next); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentModeUpdatesHaveOneWinner(t *testing.T) {
	_, svc := setup(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "role")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, coordination.ErrRevision) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
}

func TestRevocationInvalidatesScopeAndAdministrator(t *testing.T) {
	db, svc := setup(t)
	p, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "a", p.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	ctx, scope, finish, err := svc.Begin(t.Context(), "space", "a", "", "core", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetTx(t.Context(), tx, "space", "a"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	svc.Invalidate("space", "a")
	if err := svc.Validate(ctx, scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revoked execution survived: %v", err)
	}
	p, err = svc.Get(t.Context(), "space", "a")
	if err != nil || p.Administrator || p.ProviderEpoch != scope.ProviderEpoch+1 {
		t.Fatalf("revocation state: %+v %v", p, err)
	}
}

func TestRoleSelection(t *testing.T) {
	for _, tc := range []struct {
		requested string
		roles     []string
		want      string
		err       error
	}{
		{"", nil, "", coordination.ErrRoleRequired},
		{"", []string{"one"}, "one", nil},
		{"", []string{"one", "two"}, "", coordination.ErrRoleSelection},
		{"two", []string{"one", "two"}, "two", nil},
		{"deleted", []string{"same-name-new-id"}, "", coordination.ErrRoleRequired},
	} {
		got, err := coordination.SelectRole(tc.requested, tc.roles)
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Fatalf("%+v got=%s err=%v", tc, got, err)
		}
	}
}

func TestDelegatedScopeUsesTargetModeAndBothDevicesFenceExecution(t *testing.T) {
	_, svc := setup(t)
	if _, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "core-role"); err != nil {
		t.Fatal(err)
	}
	ctx, scope, finish, err := svc.Begin(t.Context(), "space", "a", "b", "core", "device-role", "delegated")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if scope.Coordinated || scope.RoleOwnerID != "b" || scope.ResourceOwnerID != "b" {
		t.Fatalf("caller policy replaced target ownership: %+v", scope)
	}
	if err := svc.Validate(ctx, scope); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangeMode(t.Context(), "space", "b", 1, true, "core-role"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("target change did not interrupt caller execution")
	}
	next, nextScope, nextFinish, err := svc.Begin(t.Context(), "space", "a", "b", "core", "core-role", "delegated-next")
	if err != nil {
		t.Fatal(err)
	}
	defer nextFinish()
	if !nextScope.Coordinated || nextScope.ResourceOwnerID != "core" {
		t.Fatalf("target on ownership: %+v", nextScope)
	}
	if _, err := svc.ChangeMode(t.Context(), "space", "a", 2, false, "device-role"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(next), coordination.ErrScopeExpired) {
		t.Fatal("initiator authority change did not interrupt delegated execution")
	}
}
