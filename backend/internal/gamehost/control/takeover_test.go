package control

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func setupTakeoverService(t *testing.T) (*TakeoverService, *ControlAuthorityManager, *FakeRuntimeReader, *InMemoryAuthorityAuditSink) {
	t.Helper()
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
	return svc, mgr, rtReader, audit
}

func TestTakeover_PluginToUser(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-1")
	pluginID := domain.PluginID("plugin-takeover-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	result, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	if result.NewMode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, result.NewMode)
	}
	if result.NewEpoch != 3 {
		t.Errorf("expected epoch 3, got %d", result.NewEpoch)
	}
	if result.PreviousMode != domain.ControlModePluginControl {
		t.Errorf("expected previous mode %q, got %q", domain.ControlModePluginControl, result.PreviousMode)
	}
}

func TestTakeover_SharedToUser(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-2")
	pluginID := domain.PluginID("plugin-takeover-2")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeSharedControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	result, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}
	if result.NewMode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, result.NewMode)
	}
}

func TestTakeover_AssistToUser(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-3")
	pluginID := domain.PluginID("plugin-takeover-3")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeAssist,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	result, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}
	if result.NewMode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, result.NewMode)
	}
}

func TestTakeover_ObserveToUser(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-4")
	pluginID := domain.PluginID("plugin-takeover-4")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	result, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}
	if result.NewMode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, result.NewMode)
	}
}

func TestTakeover_UserToUserNoop(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-5")
	pluginID := domain.PluginID("plugin-takeover-5")

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
		t.Fatalf("First Takeover failed: %v", err)
	}

	firstSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	result, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Second Takeover failed: %v", err)
	}

	if result.NewEpoch != firstSnap.Epoch {
		t.Errorf("expected epoch unchanged at %d, got %d", firstSnap.Epoch, result.NewEpoch)
	}
}

func TestTakeover_NotInactiveRuntime(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-6")
	pluginID := domain.PluginID("plugin-takeover-6")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, false)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err == nil {
		t.Fatal("expected error for inactive runtime, got nil")
	}
}

func TestTakeover_EmptyRuntimeID(t *testing.T) {
	svc, _, _, _ := setupTakeoverService(t)
	ctx := context.Background()

	_, err := svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: "",
		Actor:     ActorUser,
	})
	if err == nil {
		t.Fatal("expected error for empty runtime id, got nil")
	}
}

func TestTakeover_AuditSuccess(t *testing.T) {
	svc, mgr, rtReader, audit := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-7")
	pluginID := domain.PluginID("plugin-takeover-7")

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

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.Result != AuditResultSuccess {
		t.Errorf("expected result %q, got %q", AuditResultSuccess, ev.Result)
	}
	if ev.NewMode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, ev.NewMode)
	}
}

func TestTakeover_StaleEpochRejected(t *testing.T) {
	svc, mgr, rtReader, audit := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-takeover-8")
	pluginID := domain.PluginID("plugin-takeover-8")

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

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeSharedControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	staleEpoch := uint64(1)
	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		Actor:         ActorUser,
		ExpectedEpoch: &staleEpoch,
	})
	if err == nil {
		t.Fatal("expected stale epoch error, got nil")
	}

	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Result != AuditResultDenied {
		t.Errorf("expected result %q, got %q", AuditResultDenied, events[0].Result)
	}
}
