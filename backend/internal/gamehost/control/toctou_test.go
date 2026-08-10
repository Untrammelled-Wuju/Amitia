package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPluginOutputGate_TOCTOU_TakeoverBlocksOldEffect(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), "rt-1", "plugin-1")
	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     mgr,
	})

	snap, _ := mgr.Get(context.Background(), "rt-1")

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("initial gate should allow: reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected permit")
	}
	originalEpoch := permit.OutputEpoch

	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})

	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonSystemRecovery,
	})

	currentSnap, _ := mgr.Get(context.Background(), "rt-1")
	if permit.IsCurrent(currentSnap.Epoch) {
		t.Fatal("permit should be stale after ABA (plugin->user->plugin)")
	}
	if currentSnap.Epoch == originalEpoch {
		t.Fatal("epoch should have advanced after ABA transitions")
	}

	newReq := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", originalEpoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}
	newDecision, _ := gate.Check(context.Background(), newReq)
	if !newDecision.Deny() {
		t.Fatal("expected DENY with stale epoch after ABA")
	}
	if newDecision.Reason != OutputDeniedStaleEpoch {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedStaleEpoch, newDecision.Reason)
	}
}

func TestPluginOutputGate_TOCTOU_BlockingSinkNotCalledAfterTakeover(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), "rt-1", "plugin-1")
	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     mgr,
	})

	var sinkCalls int
	var sinkMu sync.Mutex
	sink := ControlEffectSinkFunc(func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
		sinkMu.Lock()
		defer sinkMu.Unlock()

		currentSnap, _ := mgr.Get(ctx, runtimeID)
		if !permit.IsCurrent(currentSnap.Epoch) {
			return &AuthorityError{Code: domain.ErrInvalidState, Message: "stale permit at effect time"}
		}
		sinkCalls++
		return nil
	})

	snap, _ := mgr.Get(context.Background(), "rt-1")
	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.AuthorizeAndDispatch(context.Background(), req, sink)

	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})

	snap2, _ := mgr.Get(context.Background(), "rt-1")
	staleReq := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap2.Epoch-1),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}
	_, _ = gate.AuthorizeAndDispatch(context.Background(), staleReq, sink)

	sinkMu.Lock()
	defer sinkMu.Unlock()
	if sinkCalls != 1 {
		t.Fatalf("expected exactly 1 sink call (old epoch rejected), got %d", sinkCalls)
	}
}

func TestPluginOutputGate_AuthorizeAndDispatch_PermitTimeReCheck(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), "rt-1", "plugin-1")
	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	var gate *PluginOutputGate
	var sinkCalls int
	sink := ControlEffectSinkFunc(func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
		currentSnap, _ := mgr.Get(ctx, runtimeID)
		if !permit.IsCurrent(currentSnap.Epoch) {
			return &AuthorityError{Code: domain.ErrInvalidState, Message: "stale permit real-time check"}
		}
		sinkCalls++
		return nil
	})

	gate = NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     mgr,
	})

	snap, _ := mgr.Get(context.Background(), "rt-1")
	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sinkCalls != 1 {
		t.Fatalf("expected 1 sink call, got %d", sinkCalls)
	}

	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})

	staleReq := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}
	_, err = gate.AuthorizeAndDispatch(context.Background(), staleReq, sink)
	if err == nil {
		t.Fatal("expected error for stale epoch req after takeover")
	}
	if sinkCalls != 1 {
		t.Fatalf("expected still 1 sink call, got %d", sinkCalls)
	}
}
