package control

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPluginOutputGate_RunningAndReady(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

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
		Intent: newTestOutputIntent("rt-1", "", 10),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("expected ALLOW for running+ready, got DENY reason=%s", decision.Reason)
	}
}

func TestPluginOutputGate_NotReadyDenied(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), false)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

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
		Intent: newTestOutputIntent("rt-1", "", 10),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY when not ready")
	}
	if decision.Reason != OutputDeniedNotReady {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedNotReady, decision.Reason)
	}
}

func TestPluginOutputGate_InactiveRuntimeDenied(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), false)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), false)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

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
		Intent: newTestOutputIntent("rt-1", "", 10),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for inactive runtime")
	}
	if decision.Reason != OutputDeniedNotEligible {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedNotEligible, decision.Reason)
	}
}

func TestPluginOutputGate_StoppingRuntimeDenied(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetStopping(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

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
		Intent: newTestOutputIntent("rt-1", "", 10),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY when stopping")
	}
	if decision.Reason != OutputDeniedNotEligible {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedNotEligible, decision.Reason)
	}
}
