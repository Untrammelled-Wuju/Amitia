package control

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPluginOutputGate_CloseBlocksNewOutput(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	gate.CloseRuntimeOutputs("rt-1")

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}
	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for closed gate")
	}
	if decision.Reason != OutputDeniedGateClosed {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedGateClosed, decision.Reason)
	}
}

func TestPluginOutputGate_ReopenRestoresOutput(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	gate.CloseRuntimeOutputs("rt-1")
	gate.ReopenRuntimeOutputs("rt-1")

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}
	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("expected ALLOW after reopen, got DENY reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected permit after reopen")
	}
}

func TestPluginOutputGate_CloseOneRuntimeDoesNotAffectOther(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")
	topo.RegisterRuntime("rt-2", "plugin-2")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)
	auth.SetSnapshot("rt-2", domain.ControlModePluginControl, 3)

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)
	rt.SetActive("rt-2", true)
	rt.SetReady("rt-2", true)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	gate.CloseRuntimeOutputs("rt-1")

	req1 := OutputCheckRequest{Intent: newTestOutputIntent("rt-1", "", 5), Peer: newTestPeer("rt-1", "", "plugin-1")}
	req2 := OutputCheckRequest{Intent: newTestOutputIntent("rt-2", "", 3), Peer: newTestPeer("rt-2", "", "plugin-2")}

	d1, _ := gate.Check(context.Background(), req1)
	d2, _ := gate.Check(context.Background(), req2)

	if !d1.Deny() {
		t.Fatal("expected DENY for closed runtime")
	}
	if d2.Deny() {
		t.Fatalf("expected ALLOW for non-closed runtime, got DENY reason=%s", d2.Reason)
	}
}

func TestPluginOutputGate_IsRuntimeClosed(t *testing.T) {
	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock: func() time.Time { return time.Now().UTC() },
	})

	if gate.IsRuntimeClosed("rt-1") {
		t.Fatal("expected false for fresh runtime")
	}
	gate.CloseRuntimeOutputs("rt-1")
	if !gate.IsRuntimeClosed("rt-1") {
		t.Fatal("expected true after CloseRuntimeOutputs")
	}
	gate.ReopenRuntimeOutputs("rt-1")
	if gate.IsRuntimeClosed("rt-1") {
		t.Fatal("expected false after ReopenRuntimeOutputs")
	}
}
