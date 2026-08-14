package control

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPluginOutputGate_RuntimeSpoofDenied(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-victim", "plugin-victim")

	gen := NewFakeGenerationReader()
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    NewFakeRuntimeReader(),
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        mgr,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "spoof-1",
			RuntimeID:      "rt-other",
			AuthorityEpoch: 1,
			Kind:           KindCustomRPC,
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-evil",
			RuntimeID:  "rt-victim",
			Generation: 1,
		},
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for runtime spoof")
	}
	if decision.Reason != OutputDeniedInvalidPeer {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedInvalidPeer, decision.Reason)
	}
}

func TestPluginOutputGate_ServiceSpoofDenied(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")
	topo.RegisterService("rt-1", "svc-real")

	gen := NewFakeGenerationReader()
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    NewFakeRuntimeReader(),
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        mgr,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "svc-spoof-1",
			RuntimeID:      "rt-1",
			ServiceID:      "svc-other",
			AuthorityEpoch: 1,
			Kind:           KindCustomRPC,
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			ServiceID:  "svc-real",
			Generation: 1,
		},
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for service spoof")
	}
}

func TestPluginOutputGate_EmptyPeerIdentityDenied(t *testing.T) {
	gen := NewFakeGenerationReader()
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         NewFakeTopology(),
		RuntimeReader:    NewFakeRuntimeReader(),
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        mgr,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "empty-peer-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 1,
			Kind:           KindCustomRPC,
		},
		Peer: TrustedPluginIdentity{},
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for empty peer")
	}
	if decision.Reason != OutputDeniedInvalidPeer {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedInvalidPeer, decision.Reason)
	}
}

func TestPluginOutputGate_WrongPluginID(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-real")

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    rt,
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        auth,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-fake"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for wrong plugin ID")
	}
}
