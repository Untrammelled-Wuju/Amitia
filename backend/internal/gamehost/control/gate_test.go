package control

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type FakeTopology struct {
	mu sync.Mutex
	runtimes map[domain.RuntimeInstanceID]domain.PluginID
	services map[domain.RuntimeInstanceID]map[domain.ServiceID]bool
}

func NewFakeTopology() *FakeTopology {
	return &FakeTopology{
		runtimes: make(map[domain.RuntimeInstanceID]domain.PluginID),
		services: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]bool),
	}
}

func (f *FakeTopology) RegisterRuntime(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimes[runtimeID] = pluginID
}

func (f *FakeTopology) RegisterService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.services[runtimeID] == nil {
		f.services[runtimeID] = make(map[domain.ServiceID]bool)
	}
	f.services[runtimeID][serviceID] = true
}

func (f *FakeTopology) HasRuntime(runtimeID domain.RuntimeInstanceID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.runtimes[runtimeID]
	return ok
}

func (f *FakeTopology) GetPluginID(runtimeID domain.RuntimeInstanceID) (domain.PluginID, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pid, ok := f.runtimes[runtimeID]
	return pid, ok
}

func (f *FakeTopology) ServiceBelongsToRuntime(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	svcs, ok := f.services[runtimeID]
	if !ok {
		return false
	}
	return svcs[serviceID]
}

type FakeEffPermChecker struct {
	mu      sync.Mutex
	allowed map[string]bool
}

func NewFakeEffPermChecker() *FakeEffPermChecker {
	return &FakeEffPermChecker{
		allowed: make(map[string]bool),
	}
}

func (c *FakeEffPermChecker) SetAllowed(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.allowed[key] = allowed
}

func (c *FakeEffPermChecker) CheckControlOutput(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID) (PermissionCheckResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s/%s/%s", runtimeID, serviceID, pluginID)
	if allowed, ok := c.allowed[key]; ok {
		if allowed {
			return PermissionCheckResult{Allowed: true}, nil
		}
		return PermissionCheckResult{Allowed: false, Reason: "test: permission denied"}, nil
	}
	return PermissionCheckResult{Allowed: true}, nil
}

type FakeAuthorityReader struct {
	mu       sync.Mutex
	snapshots map[domain.RuntimeInstanceID]ControlAuthoritySnapshot
}

func NewFakeAuthorityReader() *FakeAuthorityReader {
	return &FakeAuthorityReader{
		snapshots: make(map[domain.RuntimeInstanceID]ControlAuthoritySnapshot),
	}
}

func (r *FakeAuthorityReader) SetSnapshot(runtimeID domain.RuntimeInstanceID, mode domain.ControlMode, epoch uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots[runtimeID] = ControlAuthoritySnapshot{
		RuntimeID: runtimeID,
		Mode:      mode,
		Epoch:     epoch,
	}
}

func (r *FakeAuthorityReader) GetSnapshot(ctx context.Context, runtimeID domain.RuntimeInstanceID) (ControlAuthoritySnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.snapshots[runtimeID]
	if !ok {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrNotFound,
			Message: "not found: " + string(runtimeID),
		}
	}
	return snap, nil
}

type FakeMetrics struct {
	mu       sync.Mutex
	allowed  int
	denied   int
	denyReasons map[OutputDecisionReason]int
}

func NewFakeMetrics() *FakeMetrics {
	return &FakeMetrics{denyReasons: make(map[OutputDecisionReason]int)}
}

func (m *FakeMetrics) RecordOutputDecision(runtimeID domain.RuntimeInstanceID, kind ControlOutputKind, reason OutputDecisionReason, allowed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if allowed {
		m.allowed++
		return
	}
	m.denied++
	if reason != "" {
		m.denyReasons[reason]++
	}
}

func newTestOutputIntent(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, epoch uint64) ControlOutputIntent {
	return ControlOutputIntent{
		OutputID:       "output-" + string(runtimeID) + "-" + fmt.Sprint(epoch),
		RuntimeID:      runtimeID,
		ServiceID:      serviceID,
		AuthorityEpoch: epoch,
		Kind:           KindCustomRPC,
		Payload:        []byte(`{}`),
	}
}

func newTestPeer(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID) TrustedPluginIdentity {
	return TrustedPluginIdentity{
		PluginID:  pluginID,
		RuntimeID: runtimeID,
		ServiceID: serviceID,
	}
}

func TestPluginOutputGate_AllowPluginMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")
	topo.RegisterService("rt-1", "svc-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

	perm := NewFakeEffPermChecker()
	policy := NoopHostPolicyChecker{}

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   perm,
		PolicyChecker: policy,
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "svc-1", 10),
		Peer:   newTestPeer("rt-1", "svc-1", "plugin-1"),
	}

	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("expected ALLOW, got DENY reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected non-nil permit")
	}
	if permit.OutputEpoch != 10 {
		t.Fatalf("expected permit epoch=10, got %d", permit.OutputEpoch)
	}
}

func TestPluginOutputGate_AllowSharedMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModeSharedControl, 8)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 8),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("expected ALLOW, got DENY reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected non-nil permit")
	}
}

func TestPluginOutputGate_AllowAssistMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModeAssist, 5)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("expected ALLOW, got DENY reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected non-nil permit")
	}
}

func TestPluginOutputGate_DenyObserveMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModeObserveOnly, 5)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for observe mode, got ALLOW")
	}
	if decision.Reason != OutputDeniedAuthorityMode {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedAuthorityMode, decision.Reason)
	}
}

func TestPluginOutputGate_DenyUserMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModeUserControl, 20)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 20),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for user mode, got ALLOW")
	}
	if decision.Reason != OutputDeniedAuthorityMode {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedAuthorityMode, decision.Reason)
	}
}

func TestPluginOutputGate_DenySuspendedMode(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModeSuspended, 3)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 3),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for suspended mode, got ALLOW")
	}
	if decision.Reason != OutputDeniedAuthorityMode {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedAuthorityMode, decision.Reason)
	}
}

func TestPluginOutputGate_EpochComparison(t *testing.T) {
	tests := []struct {
		name   string
		output uint64
		auth   uint64
		deny   bool
		reason OutputDecisionReason
	}{
		{name: "current epoch allow", output: 10, auth: 10, deny: false},
		{name: "stale epoch deny", output: 9, auth: 10, deny: true, reason: OutputDeniedStaleEpoch},
		{name: "future epoch deny", output: 11, auth: 10, deny: true, reason: OutputDeniedStaleEpoch},
		{name: "missing epoch deny", output: 0, auth: 10, deny: true, reason: OutputDeniedStaleEpoch},
		{name: "epoch 0 current 0", output: 0, auth: 0, deny: true, reason: OutputDeniedStaleEpoch},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rt *FakeRuntimeReader
			if tc.output > 0 || tc.auth > 0 {
				rt = NewFakeRuntimeReader()
				rt.SetActive(domain.RuntimeInstanceID("rt-e"), true)
				rt.SetReady(domain.RuntimeInstanceID("rt-e"), true)
			}

			topo := NewFakeTopology()
			topo.RegisterRuntime("rt-e", "plugin-e")

			authReader := NewFakeAuthorityReader()
			if tc.auth > 0 {
				authReader.SetSnapshot("rt-e", domain.ControlModePluginControl, tc.auth)
			}

			gate := NewPluginOutputGate(PluginOutputGateOptions{
				Clock:         func() time.Time { return time.Now().UTC() },
				Topology:      topo,
				RuntimeReader: rt,
				PermChecker:   NewFakeEffPermChecker(),
				Authority:     authReader,
			})

			if rt == nil && tc.output == 0 && tc.auth == 0 {
				gateNoAuth := NewPluginOutputGate(PluginOutputGateOptions{
					Clock:    func() time.Time { return time.Now().UTC() },
					Topology: topo,
				})
				req := OutputCheckRequest{
					Intent: newTestOutputIntent("rt-e", "", 0),
					Peer:   newTestPeer("rt-e", "", "plugin-e"),
				}
				decision, _ := gateNoAuth.Check(context.Background(), req)
				if decision.Reason != OutputDeniedStaleEpoch {
					t.Fatalf("expected reason=%s, got %s", OutputDeniedStaleEpoch, decision.Reason)
				}
				return
			}

			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-e", "", tc.output),
				Peer:   newTestPeer("rt-e", "", "plugin-e"),
			}

			decision, permit := gate.Check(context.Background(), req)
			if tc.deny {
				if !decision.Deny() {
					t.Fatalf("expected DENY, got ALLOW")
				}
				if tc.reason != "" && decision.Reason != tc.reason {
					t.Fatalf("expected reason=%s, got %s", tc.reason, decision.Reason)
				}
			} else {
				if decision.Deny() {
					t.Fatalf("expected ALLOW, got DENY reason=%s", decision.Reason)
				}
				if permit == nil {
					t.Fatal("expected non-nil permit")
				}
			}
		})
	}
}

func TestPluginOutputGate_PermissionDeny(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

	perm := NewFakeEffPermChecker()
	perm.SetAllowed("rt-1//plugin-1", false)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   perm,
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 10),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for permission denied, got ALLOW")
	}
	if decision.Reason != OutputDeniedPermission {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedPermission, decision.Reason)
	}
}

func TestPluginOutputGate_RuntimeNotFound(t *testing.T) {
	topo := NewFakeTopology()

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:    func() time.Time { return time.Now().UTC() },
		Topology: topo,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-ghost", "", 1),
		Peer:   newTestPeer("rt-ghost", "", "plugin-x"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for unknown runtime")
	}
	if decision.Reason != OutputDeniedRuntimeNotFound {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedRuntimeNotFound, decision.Reason)
	}
}

func TestPluginOutputGate_ServiceNotFound(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")
	topo.RegisterService("rt-1", "svc-real")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-svc-ghost",
			RuntimeID:      "rt-1",
			ServiceID:      "svc-ghost",
			AuthorityEpoch: 5,
			Kind:           KindCustomRPC,
		},
		Peer: TrustedPluginIdentity{
			PluginID:  "plugin-1",
			RuntimeID: "rt-1",
			ServiceID: "svc-real",
		},
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for service mismatch")
	}
}

func TestPluginOutputGate_PeerMismatch(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:    func() time.Time { return time.Now().UTC() },
		Topology: topo,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 1),
		Peer:   newTestPeer("rt-1", "", "plugin-evil"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("expected DENY for plugin mismatch")
	}
}

func TestPluginOutputGate_AuditEvent(t *testing.T) {
	rt := NewFakeRuntimeReader()
	rt.SetActive(domain.RuntimeInstanceID("rt-1"), true)
	rt.SetReady(domain.RuntimeInstanceID("rt-1"), true)

	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	audit := NewInMemoryAuthorityAuditSink()

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
		Audit:         audit,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 5),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.Check(context.Background(), req)
	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].RuntimeID != "rt-1" {
		t.Fatalf("expected audit runtime=rt-1, got %s", events[0].RuntimeID)
	}
	if events[0].Result != AuditResultSuccess {
		t.Fatalf("expected audit result=success, got %s", events[0].Result)
	}
}

func TestPluginOutputGate_AuditOnDeny(t *testing.T) {
	topo := NewFakeTopology()
	audit := NewInMemoryAuthorityAuditSink()

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:    func() time.Time { return time.Now().UTC() },
		Topology: topo,
		Audit:    audit,
	})

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", 1),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.Check(context.Background(), req)
	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Result != AuditResultDenied {
		t.Fatalf("expected audit result=denied, got %s", events[0].Result)
	}
}
