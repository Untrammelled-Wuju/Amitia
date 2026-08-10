package control

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestRelease_UserToObserve(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-1")
	pluginID := domain.PluginID("plugin-release-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeObserveOnly,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if result.NewMode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q, got %q", domain.ControlModeObserveOnly, result.NewMode)
	}
	if result.NewEpoch != 3 {
		t.Errorf("expected epoch 3, got %d", result.NewEpoch)
	}
}

func TestRelease_UserToPlugin(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NoopPermissionChecker{}
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-2")
	pluginID := domain.PluginID("plugin-release-2")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if result.NewMode != domain.ControlModePluginControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModePluginControl, result.NewMode)
	}
}

func TestRelease_UserToShared(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NoopPermissionChecker{}
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-3")
	pluginID := domain.PluginID("plugin-release-3")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeSharedControl,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if result.NewMode != domain.ControlModeSharedControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeSharedControl, result.NewMode)
	}
}

func TestRelease_UserToAssist(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NoopPermissionChecker{}
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-4")
	pluginID := domain.PluginID("plugin-release-4")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeAssist,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	if result.NewMode != domain.ControlModeAssist {
		t.Errorf("expected mode %q, got %q", domain.ControlModeAssist, result.NewMode)
	}
}

func TestRelease_PermissionRevoked(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NewRevokedPermissionChecker()
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-5")
	pluginID := domain.PluginID("plugin-release-5")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	permChecker.Revoke(runtimeID)

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected permission denied error, got nil")
	}

	snap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Mode != domain.ControlModeUserControl {
		t.Errorf("expected mode to remain %q, got %q", domain.ControlModeUserControl, snap.Mode)
	}
}

func TestRelease_PermissionRevokedObserveAllowed(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NewRevokedPermissionChecker()
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-6")
	pluginID := domain.PluginID("plugin-release-6")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	permChecker.Revoke(runtimeID)

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeObserveOnly,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release to observe should succeed after permission revoke: %v", err)
	}
	if result.NewMode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q, got %q", domain.ControlModeObserveOnly, result.NewMode)
	}
}

func TestRelease_HostPolicyDenied(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NoopPermissionChecker{}
	policyChecker := AlwaysDenyHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-7")
	pluginID := domain.PluginID("plugin-release-7")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected host policy denied error, got nil")
	}
}

func TestRelease_RuntimeStopping(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-8")
	pluginID := domain.PluginID("plugin-release-8")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	rtReader.SetStopping(runtimeID, true)

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected error for stopping runtime, got nil")
	}
}

func TestRelease_RuntimeNotReady(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	rtReader := NewFakeRuntimeReader()
	permChecker := NoopPermissionChecker{}
	policyChecker := NoopHostPolicyChecker{}
	audit := NewInMemoryAuthorityAuditSink()

	svc := NewTakeoverService(TakeoverServiceOptions{
		Manager:       mgr,
		RuntimeReader: rtReader,
		PermChecker:   permChecker,
		PolicyChecker: policyChecker,
		Audit:         audit,
	})

	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-9")
	pluginID := domain.PluginID("plugin-release-9")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, false)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected error for not-ready runtime, got nil")
	}
}

func TestRelease_NotUserMode(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-10")
	pluginID := domain.PluginID("plugin-release-10")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeObserveOnly,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected error when not in user mode, got nil")
	}
}

func TestRelease_TargetUserRejected(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-11")
	pluginID := domain.PluginID("plugin-release-11")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeUserControl,
		Actor:      ActorUser,
	})
	if err == nil {
		t.Fatal("expected error for target=user, got nil")
	}
}

func TestRelease_DefaultTargetIsObserve(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-12")
	pluginID := domain.PluginID("plugin-release-12")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		TargetMode: "",
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release with empty target failed: %v", err)
	}
	if result.NewMode != domain.ControlModeObserveOnly {
		t.Errorf("expected default mode %q, got %q", domain.ControlModeObserveOnly, result.NewMode)
	}
}

func TestRelease_StaleEpochRejected(t *testing.T) {
	svc, mgr, rtReader, audit := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-13")
	pluginID := domain.PluginID("plugin-release-13")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	staleEpoch := uint64(1)
	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		TargetMode:    domain.ControlModeObserveOnly,
		Actor:         ActorUser,
		ExpectedEpoch: staleEpoch,
		UseExpected:   true,
	})
	if err == nil {
		t.Fatal("expected stale epoch error, got nil")
	}

	events := audit.Events()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", len(events))
	}

	var deniedFound bool
	for _, ev := range events {
		if ev.Result == AuditResultDenied {
			deniedFound = true
			break
		}
	}
	if !deniedFound {
		t.Error("expected denied audit event for stale epoch")
	}
}

func TestRelease_AuditSuccess(t *testing.T) {
	svc, mgr, rtReader, audit := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-release-14")
	pluginID := domain.PluginID("plugin-release-14")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeObserveOnly,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	events := audit.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}

	if events[0].Result != AuditResultSuccess {
		t.Errorf("takeover audit: expected result %q, got %q", AuditResultSuccess, events[0].Result)
	}
	if events[1].Result != AuditResultSuccess {
		t.Errorf("release audit: expected result %q, got %q", AuditResultSuccess, events[1].Result)
	}
}
