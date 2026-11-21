package coordination_test

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestSelectRoleDomainKeepsCenterAndLocalPointersSeparately(t *testing.T) {
	db, svc := setup(t)
	ctx := t.Context()
	initial, err := svc.Get(ctx, "space", "a")
	if err != nil {
		t.Fatal(err)
	}
	center, err := svc.SelectRoleDomain(ctx, "space", "a", initial.ModeRevision, coordination.RoleDomainCenter, "core-role")
	if err != nil {
		t.Fatal(err)
	}
	if !center.Coordinated || center.SelectedRole != "core-role" || center.CenterRole != "core-role" || center.LocalRole != "" {
		t.Fatalf("center domain: %+v", center)
	}
	if center.ModeRevision != initial.ModeRevision+1 || center.PermissionRevision != initial.PermissionRevision+1 {
		t.Fatalf("revisions did not advance: %+v", center)
	}
	center, err = svc.GrantAdministrator(ctx, "space", "a", center.PermissionRevision, true)
	if err != nil || !center.Administrator {
		t.Fatalf("grant: %+v %v", center, err)
	}
	local, err := svc.SelectRoleDomain(ctx, "space", "a", center.ModeRevision, coordination.RoleDomainLocal, "local-role")
	if err != nil {
		t.Fatal(err)
	}
	if local.Coordinated || local.Administrator || local.SelectedRole != "local-role" || local.LocalRole != "local-role" {
		t.Fatalf("local domain: %+v", local)
	}
	if local.CenterRole != "core-role" {
		t.Fatalf("center pointer lost: %+v", local)
	}
	restarted := coordination.NewService(db)
	stored, err := restarted.Get(ctx, "space", "a")
	if err != nil {
		t.Fatal(err)
	}
	if stored.CenterRole != "core-role" || stored.LocalRole != "local-role" || stored.SelectedRole != "local-role" || stored.Coordinated {
		t.Fatalf("restart lost role domains: %+v", stored)
	}
}

func TestSelectRoleDomainRejectsStaleRevisionAndUnknownDomain(t *testing.T) {
	_, svc := setup(t)
	ctx := t.Context()
	initial, err := svc.Get(ctx, "space", "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SelectRoleDomain(ctx, "space", "a", initial.ModeRevision, "remote", "role"); !errors.Is(err, coordination.ErrRoleDomain) {
		t.Fatalf("unknown domain accepted: %v", err)
	}
	if _, err := svc.SelectRoleDomain(ctx, "space", "a", initial.ModeRevision+7, coordination.RoleDomainCenter, "role"); !errors.Is(err, coordination.ErrRevision) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := svc.SelectRoleDomain(ctx, "space", "a", initial.ModeRevision, coordination.RoleDomainCenter, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SelectRoleDomain(ctx, "space", "a", initial.ModeRevision, coordination.RoleDomainLocal, "role"); !errors.Is(err, coordination.ErrRevision) {
		t.Fatalf("replayed revision accepted: %v", err)
	}
}

func TestSelectRoleDomainDoesNotCrossDevices(t *testing.T) {
	_, svc := setup(t)
	ctx := t.Context()
	if _, err := svc.SelectRoleDomain(ctx, "space", "a", 1, coordination.RoleDomainCenter, "core-role"); err != nil {
		t.Fatal(err)
	}
	other, err := svc.Get(ctx, "space", "b")
	if err != nil {
		t.Fatal(err)
	}
	if other.Coordinated || other.SelectedRole != "" || other.CenterRole != "" {
		t.Fatalf("role domain crossed devices: %+v", other)
	}
}
