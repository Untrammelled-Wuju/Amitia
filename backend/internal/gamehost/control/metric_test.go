package control

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type noopMetricsSink struct{}

func (noopMetricsSink) RecordOutputDecision(runtimeID domain.RuntimeInstanceID, kind ControlOutputKind, reason OutputDecisionReason, allowed bool) {}

func TestPluginOutputGate_MetricsRecordAllow(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	metrics := NewFakeMetrics()

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    rt,
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        auth,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          metrics,
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.Check(context.Background(), req)

	if metrics.allowed != 1 {
		t.Fatalf("expected 1 allowed metric, got %d", metrics.allowed)
	}
	if metrics.denied != 0 {
		t.Fatalf("expected 0 denied metric, got %d", metrics.denied)
	}
}

func TestPluginOutputGate_MetricsRecordDeny(t *testing.T) {
	topo := NewFakeTopology()
	gen := NewFakeGenerationReader()
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	metrics := NewFakeMetrics()

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    NewFakeRuntimeReader(),
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        mgr,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          metrics,
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-unknown", "", 1),
		Peer:   newTestPeer("rt-unknown", "", "plugin-x"),
	}

	_, _ = gate.Check(context.Background(), req)

	if metrics.denied != 1 {
		t.Fatalf("expected 1 denied metric, got %d", metrics.denied)
	}
	_, ok := metrics.denyReasons[OutputDeniedRuntimeNotFound]
	if !ok {
		t.Fatal("expected runtime_not_found deny reason in metrics")
	}
}

func TestPluginOutputGate_NoopMetricsDoesNotPanic(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

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
		Metrics:          noopMetricsSink{},
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.Check(context.Background(), req)
}
