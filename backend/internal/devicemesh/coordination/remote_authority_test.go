package coordination_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRemoteAuthorityWaitsForOwnerFenceAndRejectsDelayedOldRequest(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(ctx, scope, "session", 1); err != nil {
		t.Fatal(err)
	}
	var sourceLock sync.Mutex
	sourceLock.Lock()
	entered := make(chan struct{})
	service.SetAuthorityBarrier(func(ctx context.Context, space, device string, revision int64, sources []string) error {
		if space != "space" || device != "b" || revision != 1 || len(sources) != 1 || sources[0] != "b" {
			t.Errorf("incorrect fence: %s %s %d %v", space, device, revision, sources)
		}
		close(entered)
		sourceLock.Lock()
		defer sourceLock.Unlock()
		return coordination.FenceSourceAuthority(ctx, db, space, device, revision)
	})
	changed := make(chan error, 1)
	go func() { _, err := service.InitializeCoordinated(t.Context(), "space", "b", 1, "role"); changed <- err }()
	<-entered
	select {
	case err := <-changed:
		sourceLock.Unlock()
		t.Fatalf("policy committed before owner fence: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	sourceLock.Unlock()
	if err := <-changed; err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("caller did not stop after target change")
	}
	if err := coordination.ValidateSourceAuthority(t.Context(), db, scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("delayed request accepted: %v", err)
	}
	if _, err := service.TrackRemoteAuthority(t.Context(), scope, "session", 1); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old request re-dispatched: %v", err)
	}
	nextCtx, next, nextFinish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "new")
	if err != nil {
		t.Fatal(err)
	}
	defer nextFinish()
	if err := coordination.ValidateSourceAuthority(nextCtx, db, next); err != nil {
		t.Fatalf("new scope rejected: %v", err)
	}
}

func TestUnknownRemoteCallSurvivesRestartAndCannotPretendPermissionChanged(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	id, err := service.TrackRemoteAuthority(ctx, scope, "session", 1)
	if err != nil {
		t.Fatal(err)
	}
	restarted := coordination.NewService(db)
	if _, err := restarted.InitializeCoordinated(t.Context(), "space", "a", 1, "role"); !errors.Is(err, coordination.ErrAuthorityUnconfirmed) {
		t.Fatalf("unknown remote call ignored after restart: %v", err)
	}
	policy, err := restarted.Get(t.Context(), "space", "a")
	if err != nil || policy.ModeRevision != 1 || policy.Coordinated {
		t.Fatalf("unconfirmed policy reported changed: %+v %v", policy, err)
	}
	if _, _, err := restarted.BindProvider(t.Context(), "new-core"); !errors.Is(err, coordination.ErrAuthorityUnconfirmed) {
		t.Fatalf("provider changed while remote result unknown: %v", err)
	}
	if err := restarted.ConfirmRemoteAuthority(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.InitializeCoordinated(t.Context(), "space", "a", 1, "role"); err != nil {
		t.Fatalf("confirmed call kept policy blocked: %v", err)
	}
}

func TestProviderSwitchClosesOfflineRemoteAuthorityWithoutBlocking(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(ctx, scope, "session", 1); err != nil {
		t.Fatal(err)
	}
	service.SetAuthorityBarrier(func(context.Context, string, string, int64, []string) error {
		return errors.Join(coordination.ErrAuthoritySourceOffline, errors.New("devicemesh: target device is offline"))
	})
	epoch, changed, err := service.BindProvider(t.Context(), "new-core")
	if err != nil || !changed || epoch < 2 {
		t.Fatalf("provider switch blocked by offline old authority: epoch=%d changed=%t err=%v", epoch, changed, err)
	}
	var pending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_remote_authority`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("stale authority retained: pending=%d err=%v", pending, err)
	}
}

func TestAuthorityFenceIsMonotonicAndSeparatesCallerTargetAndRealm(t *testing.T) {
	db, _ := setup(t)
	scope := coordination.ExecutionScope{SpaceID: "space", InitiatorDeviceID: "a", TargetDeviceID: "b", PermissionRevision: 5, TargetPermissionRevision: 7}
	if err := coordination.FenceSourceAuthority(t.Context(), db, "space", "a", 5); err != nil {
		t.Fatal(err)
	}
	if err := coordination.FenceSourceAuthority(t.Context(), db, "space", "a", 2); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateSourceAuthority(t.Context(), db, scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatal("caller fence moved backwards")
	}
	scope.PermissionRevision = 6
	if err := coordination.ValidateSourceAuthority(t.Context(), db, scope); err != nil {
		t.Fatal(err)
	}
	if err := coordination.FenceSourceAuthority(t.Context(), db, "space", "b", 7); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateSourceAuthority(t.Context(), db, scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatal("target fence ignored")
	}
	scope.SpaceID = "other-core"
	if err := coordination.ValidateSourceAuthority(t.Context(), db, scope); err != nil {
		t.Fatal("fence leaked into successor realm")
	}
}

func TestStalePolicyRequestDoesNotFenceValidCalls(t *testing.T) {
	_, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(ctx, scope, "session", 1); err != nil {
		t.Fatal(err)
	}
	service.SetAuthorityBarrier(func(context.Context, string, string, int64, []string) error {
		t.Fatal("stale request fenced active calls")
		return nil
	})
	if _, err := service.InitializeCoordinated(t.Context(), "space", "a", 99, "role"); !errors.Is(err, coordination.ErrRevision) {
		t.Fatal(err)
	}
	if _, err := service.SetCapabilityGrant(t.Context(), "space", "a", "b", "ai.chat", 99, true); !errors.Is(err, coordination.ErrRevision) {
		t.Fatal(err)
	}
}

func TestRemoteReconciliationDoesNotBlockCapabilitySynchronization(t *testing.T) {
	db, service := setup(t)
	ctx, scope, finish, err := service.Begin(t.Context(), "space", "a", "b", "space", "role", "remote")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, err := service.TrackRemoteAuthority(ctx, scope, "old-session", 1); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	reconciled := make(chan error, 1)
	go func() {
		reconciled <- service.ReconcileRemoteAuthority(t.Context(), "space", "b", "new-session", 2, func(context.Context, []string) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	exposure := make(chan error, 1)
	go func() {
		_, err := service.SetCapabilityExposure(t.Context(), "space", "a", 1, nil)
		exposure <- err
	}()
	select {
	case err := <-exposure:
		if err != nil {
			close(release)
			<-reconciled
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		close(release)
		<-reconciled
		<-exposure
		t.Fatal("旧设备对账网络阻塞了其他设备能力同步")
	}
	close(release)
	if err := <-reconciled; err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_remote_authority WHERE space_id='space' AND target_id='b'`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("旧远端请求未清理: remaining=%d err=%v", remaining, err)
	}
}
