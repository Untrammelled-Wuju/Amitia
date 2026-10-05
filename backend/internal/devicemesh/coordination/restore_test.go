package coordination_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type authorityCheckingRoles struct {
	coordination.DataPort
	expected coordination.ExecutionScope
	calls    atomic.Int32
}

func (p *authorityCheckingRoles) Roles(ctx context.Context, expected coordination.ExecutionScope) ([]coordination.Role, error) {
	if p.calls.Add(1) > 32 {
		return nil, errors.New("角色复核发生递归")
	}
	if actual, ok := coordination.FromContext(ctx); !ok || actual != p.expected || expected != p.expected {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	callID, confirm, err := coordination.TrackCurrentRemoteAuthority(ctx, expected.SpaceID, expected.TargetDeviceID, "role-session", 1)
	if err != nil {
		return nil, err
	}
	if callID == "" || confirm == nil {
		return nil, coordination.ErrWrongOwner
	}
	if err := confirm(ctx); err != nil {
		return nil, err
	}
	return []coordination.Role{{ID: expected.RoleID, Revision: expected.RoleRevision, Profile: []byte(`{}`)}}, nil
}

func TestRestoreCreatesRemoteAuthorityBeforeReadingRoleAndAvoidsRecursiveRoleChecks(t *testing.T) {
	_, service := setup(t)
	_, saved, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	finish()
	saved.RoleRevision, saved.TurnID, saved.ExecutionID = 7, "turn", "execution"
	roles := &authorityCheckingRoles{expected: saved}
	restored, release, err := service.Restore(t.Context(), saved, roles)
	if err != nil {
		t.Fatalf("恢复授权前无法读取受保护的设备角色: %v", err)
	}
	defer release()
	if err := coordination.ValidateCurrent(restored); err != nil {
		t.Fatalf("恢复后的角色复核递归或权限丢失: %v", err)
	}
	if roles.calls.Load() != 2 {
		t.Fatalf("角色复核次数异常: %d", roles.calls.Load())
	}
	if sources, err := service.PendingRemoteSources(t.Context()); err != nil || len(sources) != 0 {
		t.Fatalf("角色复核调用未完成确认: %+v %v", sources, err)
	}
}

func TestRestoredExecutionRetainsScopeAndIsCancelledByPolicyChanges(t *testing.T) {
	_, service := setup(t)
	_, saved, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	finish()
	saved.RoleRevision = 7
	saved.TurnID = "turn"
	saved.ExecutionID = "execution"
	roles := pendingRolePort{roles: []coordination.Role{{ID: "role", Revision: 7, Profile: []byte(`{}`)}}}
	restored, release, err := service.Restore(t.Context(), saved, roles)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	actual, ok := coordination.FromContext(restored)
	if !ok || actual != saved {
		t.Fatal("restore changed immutable authority")
	}
	if _, err := service.ChangeMode(t.Context(), "space", "b", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(restored), coordination.ErrScopeExpired) {
		t.Fatal("restored execution survived a mode change")
	}
	if _, _, err := service.Restore(t.Context(), saved, roles); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("stale scope was restored: %v", err)
	}
}

func TestRestoreRejectsChangedOwnerRealmAndIncompleteExecution(t *testing.T) {
	_, service := setup(t)
	_, saved, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "request")
	if err != nil {
		t.Fatal(err)
	}
	finish()
	saved.RoleRevision, saved.TurnID, saved.ExecutionID = 1, "turn", "execution"
	roles := pendingRolePort{roles: []coordination.Role{{ID: "role", Revision: 1, Profile: []byte(`{}`)}}}
	for _, change := range []func(*coordination.ExecutionScope){
		func(s *coordination.ExecutionScope) { s.AuthorizationRealm = "other" },
		func(s *coordination.ExecutionScope) { s.ResourceOwnerID = "other" },
		func(s *coordination.ExecutionScope) { s.RoleOwnerID = "other" },
		func(s *coordination.ExecutionScope) { s.TargetPermissionRevision++ },
		func(s *coordination.ExecutionScope) { s.RoleRevision = 0 },
		func(s *coordination.ExecutionScope) { s.ExecutionID = "" },
	} {
		changed := saved
		change(&changed)
		if _, release, err := service.Restore(t.Context(), changed, roles); err == nil {
			release()
			t.Fatalf("changed authority was accepted: %+v", changed)
		}
	}
	if _, _, err := service.Restore(t.Context(), saved, nil); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("missing role source was accepted: %v", err)
	}
	changedRoles := pendingRolePort{roles: []coordination.Role{{ID: "role", Revision: 2, Profile: []byte(`{}`)}}}
	if _, _, err := service.Restore(t.Context(), saved, changedRoles); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("changed role was accepted: %v", err)
	}
}
