package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type FakeEmergencyRuntimeStopper struct {
	mu          sync.Mutex
	active      map[domain.RuntimeInstanceID]bool
	stopped     map[domain.RuntimeInstanceID]bool
	suppressed  map[domain.RuntimeInstanceID]time.Duration
	stopDelay   time.Duration
}

func NewFakeEmergencyRuntimeStopper() *FakeEmergencyRuntimeStopper {
	return &FakeEmergencyRuntimeStopper{
		active:     make(map[domain.RuntimeInstanceID]bool),
		stopped:    make(map[domain.RuntimeInstanceID]bool),
		suppressed: make(map[domain.RuntimeInstanceID]time.Duration),
	}
}

func (f *FakeEmergencyRuntimeStopper) SetActive(runtimeID domain.RuntimeInstanceID, active bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active[runtimeID] = active
}

func (f *FakeEmergencyRuntimeStopper) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID, force bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopDelay > 0 {
		time.Sleep(f.stopDelay)
	}
	f.active[runtimeID] = false
	f.stopped[runtimeID] = true
	return nil
}

func (f *FakeEmergencyRuntimeStopper) SuppressAutoRestart(runtimeID domain.RuntimeInstanceID, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suppressed[runtimeID] = duration
}

func (f *FakeEmergencyRuntimeStopper) IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active[runtimeID], nil
}

func (f *FakeEmergencyRuntimeStopper) IsStopped(runtimeID domain.RuntimeInstanceID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped[runtimeID]
}

func (f *FakeEmergencyRuntimeStopper) IsSuppressed(runtimeID domain.RuntimeInstanceID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.suppressed[runtimeID]
	return ok
}

type FakePendingCanceller struct {
	mu      sync.Mutex
	cancelled map[domain.RuntimeInstanceID]int
}

func NewFakePendingCanceller() *FakePendingCanceller {
	return &FakePendingCanceller{cancelled: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakePendingCanceller) CancelRuntimeRequests(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled[runtimeID]++
	return 10, nil
}

func (f *FakePendingCanceller) CancelCount(runtimeID domain.RuntimeInstanceID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelled[runtimeID]
}

type FakeLeaseRevoker struct {
	mu      sync.Mutex
	revoked map[domain.RuntimeInstanceID]int
}

func NewFakeLeaseRevoker() *FakeLeaseRevoker {
	return &FakeLeaseRevoker{revoked: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakeLeaseRevoker) RevokeRuntimeLeases(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[runtimeID]++
	return 3, nil
}

func (f *FakeLeaseRevoker) RevokeCount(runtimeID domain.RuntimeInstanceID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revoked[runtimeID]
}

type FakeConnectionCloser struct {
	mu      sync.Mutex
	closed  map[domain.RuntimeInstanceID]int
}

func NewFakeConnectionCloser() *FakeConnectionCloser {
	return &FakeConnectionCloser{closed: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakeConnectionCloser) CloseRuntimeConnections(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed[runtimeID]++
	return 2, nil
}

func (f *FakeConnectionCloser) CloseCount(runtimeID domain.RuntimeInstanceID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed[runtimeID]
}

type FakeHandshakeResetter struct {
	mu     sync.Mutex
	cleared map[domain.RuntimeInstanceID]int
}

func NewFakeHandshakeResetter() *FakeHandshakeResetter {
	return &FakeHandshakeResetter{cleared: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakeHandshakeResetter) ClearRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared[runtimeID]++
	return nil
}

type FakeRestartSuppressor struct {
	mu         sync.Mutex
	suppressed map[domain.RuntimeInstanceID]time.Duration
}

func NewFakeRestartSuppressor() *FakeRestartSuppressor {
	return &FakeRestartSuppressor{suppressed: make(map[domain.RuntimeInstanceID]time.Duration)}
}

func (f *FakeRestartSuppressor) SuppressRestart(runtimeID domain.RuntimeInstanceID, duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suppressed[runtimeID] = duration
}

func (f *FakeRestartSuppressor) IsRestartSuppressed(runtimeID domain.RuntimeInstanceID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.suppressed[runtimeID]
	return ok
}

type FakeEmergencyMetrics struct {
	mu      sync.Mutex
	stops   int
	errors  int
}

func (m *FakeEmergencyMetrics) RecordEmergencyStop(runtimeID domain.RuntimeInstanceID, actor EmergencyStopActor, state EmergencyStopState, critical bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops++
}

func (m *FakeEmergencyMetrics) RecordEmergencyCleanupError(runtimeID domain.RuntimeInstanceID, stage EmergencyStopState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

type FakeStreamStopper struct {
	mu       sync.Mutex
	stopped  map[domain.RuntimeInstanceID]int
	err      error
}

func NewFakeStreamStopper() *FakeStreamStopper {
	return &FakeStreamStopper{stopped: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakeStreamStopper) StopRuntimeStreams(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped[runtimeID]++
	return f.err
}

type FakeProcessCleaner struct {
	mu       sync.Mutex
	cleaned  map[domain.RuntimeInstanceID]int
}

func NewFakeProcessCleaner() *FakeProcessCleaner {
	return &FakeProcessCleaner{cleaned: make(map[domain.RuntimeInstanceID]int)}
}

func (f *FakeProcessCleaner) CleanupProcessTree(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleaned[runtimeID]++
	return nil
}

func newTestEmergencyService(runtimeID domain.RuntimeInstanceID, mode domain.ControlMode, epoch uint64) (*EmergencyStopService, *ControlAuthorityManager, *PluginOutputGate, *FakeTopology, *FakeEmergencyRuntimeStopper) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeID, "plugin-1")
	if mode != domain.ControlModeObserveOnly || epoch > 1 {
		_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
			Target: mode,
			Actor:  ActorSystem,
			Reason: ReasonRuntimeLifecycle,
		})
	}
	for {
		snap, _ := mgr.Get(context.Background(), runtimeID)
		if snap.Epoch >= epoch {
			break
		}
		_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
			Target: mode,
			Actor:  ActorSystem,
			Reason: ReasonRuntimeLifecycle,
		})
	}

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeID, "plugin-1")

	authReader := mgr

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:    func() time.Time { return time.Now().UTC() },
		Topology: topo,
		Authority: authReader,
	})

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive(runtimeID, true)

	svc := NewEmergencyStopService(EmergencyStopServiceOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Authority:        mgr,
		Gate:             gate,
		RuntimeStopper:   rtStopper,
		PendingCanceller: NewFakePendingCanceller(),
		LeaseRevoker:     NewFakeLeaseRevoker(),
		ConnectionCloser: NewFakeConnectionCloser(),
		HandshakeReset:   NewFakeHandshakeResetter(),
		RestartSuppress:  NewFakeRestartSuppressor(),
		StreamStopper:    NewFakeStreamStopper(),
		ProcessCleaner:   NewFakeProcessCleaner(),
		Metrics:          &FakeEmergencyMetrics{},
	})

	return svc, mgr, gate, topo, rtStopper
}

func TestEmergencyStop_BasicPluginRunning(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-emg-1")
	svc, mgr, gate, _, rtStopper := newTestEmergencyService(runtimeID, domain.ControlModePluginControl, 10)

	beforeSnap, _ := mgr.Get(context.Background(), runtimeID)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
		Reason:    EmergencyReasonUserRequested,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success() {
		t.Fatalf("expected success, got state=%s critical=%v errors=%v", result.State, result.CriticalFailure, result.CleanupErrors)
	}

	afterSnap, _ := mgr.Get(context.Background(), runtimeID)

	if gate.IsRuntimeClosed(runtimeID) != true {
		t.Fatal("gate should be closed")
	}
	if afterSnap.Mode != domain.ControlModeSuspended {
		t.Fatalf("expected mode=suspended, got %s", afterSnap.Mode)
	}
	if afterSnap.Epoch <= beforeSnap.Epoch {
		t.Fatalf("expected epoch to increase after emergency stop: before=%d after=%d", beforeSnap.Epoch, afterSnap.Epoch)
	}
	if result.PreviousEpoch != beforeSnap.Epoch {
		t.Fatalf("expected result.PreviousEpoch=%d, got %d", beforeSnap.Epoch, result.PreviousEpoch)
	}
	if result.NewEpoch != afterSnap.Epoch {
		t.Fatalf("expected result.NewEpoch=%d, got %d", afterSnap.Epoch, result.NewEpoch)
	}
	if !rtStopper.IsStopped(runtimeID) {
		t.Fatal("runtime should be stopped")
	}
	if active, _ := rtStopper.IsRuntimeActive(context.Background(), runtimeID); active {
		t.Fatal("runtime should not be active after emergency stop")
	}
}

func TestEmergencyStop_SharedModeSuspended(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-emg-shared")
	svc, mgr, _, _, _ := newTestEmergencyService(runtimeID, domain.ControlModeSharedControl, 5)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorHost,
		Reason:    EmergencyReasonSafetyPolicy,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success() {
		t.Fatal("expected success for shared mode emergency stop")
	}

	snap, _ := mgr.Get(context.Background(), runtimeID)
	if snap.Mode != domain.ControlModeSuspended {
		t.Fatalf("expected suspended, got %s", snap.Mode)
	}
}

func TestEmergencyStop_UserModeStillStopsEverything(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-emg-user")
	svc, _, _, _, rtStopper := newTestEmergencyService(runtimeID, domain.ControlModeUserControl, 20)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorSystem,
		Reason:    EmergencyReasonRuntimeUnresponsive,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success() {
		t.Fatal("expected success even when mode=user")
	}
	if !rtStopper.IsStopped(runtimeID) {
		t.Fatal("runtime should be stopped even when user mode")
	}
}

func TestEmergencyStop_ObserveModeStillCleansUp(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-emg-observe")
	svc, _, _, _, rtStopper := newTestEmergencyService(runtimeID, domain.ControlModeObserveOnly, 1)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
		Reason:    EmergencyReasonResourceViolation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success() {
		t.Fatal("expected success for observe mode emergency stop")
	}
	if !rtStopper.IsStopped(runtimeID) {
		t.Fatal("runtime should be stopped even when observe mode")
	}
}

func TestEmergencyStop_RuntimeNotFound(t *testing.T) {
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock: func() time.Time { return time.Now().UTC() },
	})

	svc := NewEmergencyStopService(EmergencyStopServiceOptions{
		Clock:     func() time.Time { return time.Now().UTC() },
		Authority: mgr,
		Gate:      gate,
	})

	_, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: domain.RuntimeInstanceID("rt-ghost"),
		Actor:     EmergencyActorUser,
	})
	if err == nil {
		t.Fatal("expected error for unknown runtime")
	}
}

func TestEmergencyStop_GateFirstBeforeRuntimeStop(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-gate-first")
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeID, "plugin-1")
	_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeID, "plugin-1")

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:    func() time.Time { return time.Now().UTC() },
		Topology: topo,
	})

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.stopDelay = 20 * time.Millisecond
	rtStopper.SetActive(runtimeID, true)

	svc := NewEmergencyStopService(EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		RuntimeStopper: rtStopper,
	})

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if !gate.IsRuntimeClosed(runtimeID) {
		t.Fatal("gate must be closed in emergency stop")
	}
}

func TestEmergencyStop_AuthorityEpochIncremented(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-epoch")
	svc, mgr, _, _, _ := newTestEmergencyService(runtimeID, domain.ControlModePluginControl, 50)

	beforeSnap, _ := mgr.Get(context.Background(), runtimeID)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	afterSnap, _ := mgr.Get(context.Background(), runtimeID)

	if afterSnap.Epoch == beforeSnap.Epoch {
		t.Fatalf("epoch must change: before=%d after=%d", beforeSnap.Epoch, afterSnap.Epoch)
	}
	if result.NewEpoch <= result.PreviousEpoch {
		t.Fatalf("result epoch must advance: prev=%d new=%d", result.PreviousEpoch, result.NewEpoch)
	}
}

func TestEmergencyStop_OutputDeniedAfterStop(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-block-after")
	svc, _, gate, _, _ := newTestEmergencyService(runtimeID, domain.ControlModePluginControl, 10)

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	rt := NewFakeRuntimeReader()
	rt.SetActive(runtimeID, true)
	rt.SetReady(runtimeID, true)

	req := OutputCheckRequest{
		Intent: newTestOutputIntent(runtimeID, "", 999),
		Peer:   newTestPeer(runtimeID, "", "plugin-1"),
	}
	decision, _ := gate.Check(context.Background(), req)
	if !decision.Deny() {
		t.Fatal("gate must deny output after emergency stop")
	}
	if decision.Reason != OutputDeniedGateClosed {
		t.Fatalf("expected reason=%s, got %s", OutputDeniedGateClosed, decision.Reason)
	}
}

func TestEmergencyStop_RestartSuppressed(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-suppress")
	svc, _, _, _, rtStopper := newTestEmergencyService(runtimeID, domain.ControlModePluginControl, 10)

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if !svc.IsRestartSuppressed(runtimeID) {
		t.Fatal("restart must be suppressed after emergency stop")
	}
	if !rtStopper.IsSuppressed(runtimeID) {
		t.Fatal("auto restart must be suppressed via RuntimeStopper")
	}
}