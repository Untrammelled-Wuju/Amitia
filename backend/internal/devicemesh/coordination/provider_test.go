package coordination_test

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestProviderBindingRevokesOldAdministratorsWithoutMigratingData(t *testing.T) {
	db, svc := setup(t)
	p, err := svc.ChangeMode(t.Context(), "space", "a", 1, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "a", p.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	commit := ownedCommit("old-data")
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), commit); err != nil {
		t.Fatal(err)
	}
	ctx, scope, finish, err := svc.Begin(t.Context(), "space", "a", "", "core-b", "role", "active")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	epoch, changed, err := svc.BindProvider(t.Context(), "core-c")
	if err != nil || !changed || epoch != 2 {
		t.Fatalf("switch: %d %v %v", epoch, changed, err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("old execution survived provider switch")
	}
	if err := svc.Validate(context.Background(), scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("late output accepted: %v", err)
	}
	if _, _, finish, err := svc.Begin(t.Context(), "space", "a", "", "core-b", "role", "new-old-provider"); err == nil {
		finish()
		t.Fatal("更换提供者后仍接受旧 Core 的新请求")
	}
	p, err = svc.Get(t.Context(), "space", "a")
	if err != nil || p.Administrator || !p.Coordinated {
		t.Fatalf("old administrator or independent mode changed: %+v %v", p, err)
	}
	items, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(items) != 1 {
		t.Fatalf("provider switch moved/deleted source data: %+v %v", items, err)
	}
	if next, changed, err := svc.BindProvider(t.Context(), "core-c"); err != nil || changed || next != epoch {
		t.Fatalf("reconnect changed epoch: %d %v %v", next, changed, err)
	}
	if _, changed, err := coordination.NewService(db).BindProvider(t.Context(), "core-c"); err != nil || changed {
		t.Fatalf("restart repeated provider switch: %v %v", changed, err)
	}
}

func TestProviderPathRejectsCyclesAndExcessiveDepth(t *testing.T) {
	for _, path := range [][]string{nil, {"a"}, {"b", "b"}, {"b", "a"}, {""}} {
		if err := coordination.ValidateProviderPath("a", path); !errors.Is(err, coordination.ErrProviderCycle) {
			t.Fatalf("accepted cycle: %v %v", path, err)
		}
	}
	if err := coordination.ValidateProviderPath("a", []string{"b", "c"}); err != nil {
		t.Fatal(err)
	}
}
