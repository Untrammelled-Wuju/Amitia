package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type FakeEmergencyRuntimeStopper struct {
	mu        sync.Mutex
	active    map[domain.RuntimeInstanceID]bool
	stopped   map[domain.RuntimeInstanceID]bool
	stopDelay time.Duration
}

func NewFakeEmergencyRuntimeStopper() *FakeEmergencyRuntimeStopper {
	return &FakeEmergencyRuntimeStopper{
		active:  make(map[domain.RuntimeInstanceID]bool),
		stopped: make(map[domain.RuntimeInstanceID]bool),
	}
}

func (f *FakeEmergencyRuntimeStopper) SetActive(runtimeID domain.RuntimeInstanceID, active bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active[runtimeID] = active
}

func (f *FakeEmergencyRuntimeStopper) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopDelay > 0 {
		time.Sleep(f.stopDelay)
	}
	f.active[runtimeID] = false
	f.stopped[runtimeID] = true
	return nil
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

type FakePendingCanceller struct {
	mu        sync.Mutex
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

type FakeHostAPIWorkCanceller struct{}

func (FakeHostAPIWorkCanceller) CancelRuntimeHostAPIWork(context.Context, domain.RuntimeInstanceID) (int, error) {
	return 0, nil
}

type FakeChannelCleaner struct{}

func (FakeChannelCleaner) CleanupRuntimeChannels(context.Context, domain.RuntimeInstanceID) error {
	return nil
}

type FakeBinaryReleaser struct{}

func (FakeBinaryReleaser) ReleaseRuntimeTransientBinary(context.Context, domain.RuntimeInstanceID) error {
	return nil
}

type FakeSafetyVerifier struct{}

func (FakeSafetyVerifier) CountRuntimePending(domain.RuntimeInstanceID) int     { return 0 }
func (FakeSafetyVerifier) CountRuntimeConnections(domain.RuntimeInstanceID) int { return 0 }
func (FakeSafetyVerifier) CountRuntimeLeases(domain.RuntimeInstanceID) int      { return 0 }
func (FakeSafetyVerifier) CountRuntimeChannels(domain.RuntimeInstanceID) int    { return 0 }
func (FakeSafetyVerifier) CountRuntimeStreams(domain.RuntimeInstanceID) int     { return 0 }
func (FakeSafetyVerifier) CountRuntimeBinary(domain.RuntimeInstanceID) int      { return 0 }
func (FakeSafetyVerifier) CountRuntimeReady(domain.RuntimeInstanceID) int       { return 0 }
func (FakeSafetyVerifier) CountRuntimeHostAPIWork(domain.RuntimeInstanceID) int { return 0 }

type FakeLifecycleIntentWriter struct{}

func (FakeLifecycleIntentWriter) SetLifecycleIntent(domain.RuntimeInstanceID, string) error {
	return nil
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
	mu     sync.Mutex
	closed map[domain.RuntimeInstanceID]int
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
	mu      sync.Mutex
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

type FakeEmergencyMetrics struct {
	mu     sync.Mutex
	stops  int
	errors int
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
	mu      sync.Mutex
	stopped map[domain.RuntimeInstanceID]int
	err     error
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

type FakeIntentStore struct {
	mu      sync.RWMutex
	latched map[domain.RuntimeInstanceID]bool
}

func NewFakeIntentStore() *FakeIntentStore {
	return &FakeIntentStore{latched: make(map[domain.RuntimeInstanceID]bool)}
}

func (s *FakeIntentStore) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latched[runtimeID] = true
	return nil
}

func (s *FakeIntentStore) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latched[runtimeID]
}

func (s *FakeIntentStore) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	latched := s.latched[runtimeID]
	return "", latched
}

func (s *FakeIntentStore) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.latched, runtimeID)
	return nil
}

var _ EmergencyIntentStore = (*FakeIntentStore)(nil)

func newTestEmergencyService(t *testing.T, runtimeID domain.RuntimeInstanceID, mode domain.ControlMode, epoch uint64) (*EmergencyStopService, *ControlAuthorityManager, *PluginOutputGate, *FakeTopology, *FakeEmergencyRuntimeStopper) {
	t.Helper()
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeID, "plugin-1")

	targetMode := mode
	if targetMode == "" {
		targetMode = domain.ControlModeObserveOnly
	}

	modes := []domain.ControlMode{
		domain.ControlModeObserveOnly,
		domain.ControlModePluginControl,
		domain.ControlModeSharedControl,
		domain.ControlModeAssist,
		domain.ControlModeObserveOnly,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
	}

	currentIdx := 0
	for i := range modes {
		if modes[i] == targetMode {
			currentIdx = i
			break
		}
	}

	desiredEpoch := epoch
	if desiredEpoch < 2 {
		desiredEpoch = 2
	}

	for i := 0; i <= currentIdx && i < len(modes); i++ {
		if modes[i] == domain.ControlModeObserveOnly && i == 0 {
			continue
		}
		_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
			Target: modes[i],
			Actor:  ActorSystem,
			Reason: ReasonRuntimeLifecycle,
		})
	}

	for {
		snap, _ := mgr.Get(context.Background(), runtimeID)
		if snap.Mode == targetMode && snap.Epoch >= desiredEpoch {
			break
		}
		_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
			Target: domain.ControlModeAssist,
			Actor:  ActorSystem,
			Reason: ReasonRuntimeLifecycle,
		})
		snap2, _ := mgr.Get(context.Background(), runtimeID)
		if snap2.Mode != targetMode {
			_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
				Target: targetMode,
				Actor:  ActorSystem,
				Reason: ReasonRuntimeLifecycle,
			})
		}
		if snap2.Epoch >= desiredEpoch && snap2.Mode == targetMode {
			break
		}
	}

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeID, "plugin-1")
	runtimeReader := NewFakeRuntimeReader()
	runtimeReader.SetActive(runtimeID, true)
	runtimeReader.SetReady(runtimeID, true)
	generationReader := NewFakeGenerationReader()
	generationReader.SetGeneration(runtimeID, 1)

	authReader := mgr

	gate := newStrictEmergencyGate(t, topo, runtimeReader, generationReader, authReader)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive(runtimeID, true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Authority:        mgr,
		Gate:             gate,
		Intent:           NewFakeIntentStore(),
		RuntimeStopper:   rtStopper,
		PendingCanceller: NewFakePendingCanceller(),
		LeaseRevoker:     NewFakeLeaseRevoker(),
		ConnectionCloser: NewFakeConnectionCloser(),
		HandshakeReset:   NewFakeHandshakeResetter(),
		StreamStopper:    NewFakeStreamStopper(),
		Metrics:          &FakeEmergencyMetrics{},
	})

	return svc, mgr, gate, topo, rtStopper
}

func newStrictEmergencyGate(t testing.TB, topo *FakeTopology, runtimeReader *FakeRuntimeReader, generationReader RuntimeGenerationReader, authority ControlAuthoritySnapshotReader) *PluginOutputGate {
	t.Helper()
	if runtimeReader == nil {
		runtimeReader = NewFakeRuntimeReader()
	}
	if generationReader == nil {
		generationReader = NewFakeGenerationReader()
	}
	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    runtimeReader,
		GenerationReader: generationReader,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        authority,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NewControlCommitBarrier(),
	})
	if err != nil {
		t.Fatalf("new plugin output gate: %v", err)
	}
	return gate
}

func newStrictEmergencyStopService(t testing.TB, opts EmergencyStopServiceOptions) *EmergencyStopService {
	t.Helper()
	if opts.RuntimeStopper == nil {
		opts.RuntimeStopper = NewFakeEmergencyRuntimeStopper()
	}
	if opts.PendingCanceller == nil {
		opts.PendingCanceller = NewFakePendingCanceller()
	}
	if opts.ConnectionCloser == nil {
		opts.ConnectionCloser = NewFakeConnectionCloser()
	}
	if opts.HostAPICanceller == nil {
		opts.HostAPICanceller = FakeHostAPIWorkCanceller{}
	}
	if opts.LeaseRevoker == nil {
		opts.LeaseRevoker = NewFakeLeaseRevoker()
	}
	if opts.HandshakeReset == nil {
		opts.HandshakeReset = NewFakeHandshakeResetter()
	}
	if opts.StreamStopper == nil {
		opts.StreamStopper = NewFakeStreamStopper()
	}
	if opts.ChannelCleaner == nil {
		opts.ChannelCleaner = FakeChannelCleaner{}
	}
	if opts.BinaryReleaser == nil {
		opts.BinaryReleaser = FakeBinaryReleaser{}
	}
	if opts.LifecycleIntent == nil {
		opts.LifecycleIntent = FakeLifecycleIntentWriter{}
	}
	verifier := FakeSafetyVerifier{}
	if opts.PendingVerifier == nil {
		opts.PendingVerifier = verifier
	}
	if opts.ConnectionVerifier == nil {
		opts.ConnectionVerifier = verifier
	}
	if opts.LeaseVerifier == nil {
		opts.LeaseVerifier = verifier
	}
	if opts.ChannelVerifier == nil {
		opts.ChannelVerifier = verifier
	}
	if opts.StreamVerifier == nil {
		opts.StreamVerifier = verifier
	}
	if opts.BinaryVerifier == nil {
		opts.BinaryVerifier = verifier
	}
	if opts.ReadyVerifier == nil {
		opts.ReadyVerifier = verifier
	}
	if opts.HostAPIVerifier == nil {
		opts.HostAPIVerifier = verifier
	}
	svc, err := NewEmergencyStopService(opts)
	if err != nil {
		t.Fatalf("new emergency stop service: %v", err)
	}
	return svc
}

func TestEmergencyStop_BasicPluginRunning(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-emg-1")
	svc, mgr, gate, _, rtStopper := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

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
	svc, mgr, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModeSharedControl, 5)

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
	svc, _, _, _, rtStopper := newTestEmergencyService(t, runtimeID, domain.ControlModeUserControl, 20)

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
	svc, _, _, _, rtStopper := newTestEmergencyService(t, runtimeID, domain.ControlModeObserveOnly, 1)

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

	gate := newStrictEmergencyGate(t, NewFakeTopology(), nil, nil, mgr)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Authority:        mgr,
		Gate:             gate,
		Intent:           NewFakeIntentStore(),
		RuntimeStopper:   NewFakeEmergencyRuntimeStopper(),
		PendingCanceller: NewFakePendingCanceller(),
		ConnectionCloser: NewFakeConnectionCloser(),
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

	gate := newStrictEmergencyGate(t, topo, nil, nil, mgr)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.stopDelay = 20 * time.Millisecond
	rtStopper.SetActive(runtimeID, true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		Intent:         NewFakeIntentStore(),
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
	svc, mgr, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 50)

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
	svc, _, gate, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

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

func TestEmergencyStop_IdempotentByRuntimeID(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-idem")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	res1, err1 := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})
	if err1 != nil {
		t.Fatalf("first emergency stop failed: %v", err1)
	}
	if !res1.Success() {
		t.Fatal("first emergency stop should succeed")
	}

	res2, err2 := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})
	if err2 != nil {
		t.Fatalf("second emergency stop should not error: %v", err2)
	}
	if !res2.Success() {
		t.Fatal("second idempotent call should also report success")
	}
	if res1.OperationID != res2.OperationID {
		t.Fatalf("idempotent call should return same operation: %s vs %s", res1.OperationID, res2.OperationID)
	}
}

func TestEmergencyStop_IdempotentByKey(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-idem-key")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	res1, _ := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID:      runtimeID,
		Actor:          EmergencyActorUser,
		IdempotencyKey: "idem-key-xyz",
	})

	res2, _ := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID:      runtimeID,
		Actor:          EmergencyActorHost,
		IdempotencyKey: "idem-key-xyz",
	})

	if res1.OperationID != res2.OperationID {
		t.Fatalf("idempotency key must map to same operation: %s vs %s", res1.OperationID, res2.OperationID)
	}
}

func TestEmergencyStop_TOCTOU_GateAllowThenStopBlocksOldOutput(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-toctou")
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeID, "plugin-1")
	_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeID, "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive(runtimeID, true)
	rt.SetReady(runtimeID, true)

	gate := newStrictEmergencyGate(t, topo, rt, nil, mgr)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive(runtimeID, true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		Intent:         NewFakeIntentStore(),
		RuntimeStopper: rtStopper,
	})

	snap, _ := mgr.Get(context.Background(), runtimeID)
	req := OutputCheckRequest{
		Intent: newTestOutputIntent(runtimeID, "", snap.Epoch),
		Peer:   newTestPeer(runtimeID, "", "plugin-1"),
	}
	decision, permit := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("pre-stop gate should allow: reason=%s", decision.Reason)
	}
	if permit == nil {
		t.Fatal("expected permit before stop")
	}

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if permit.IsCurrent(snap.Epoch) && !gate.IsRuntimeClosed(runtimeID) {
		t.Fatal("permit should be invalid after emergency stop")
	}

	newReq := OutputCheckRequest{
		Intent: newTestOutputIntent(runtimeID, "", snap.Epoch),
		Peer:   newTestPeer(runtimeID, "", "plugin-1"),
	}
	newDecision, _ := gate.Check(context.Background(), newReq)
	if !newDecision.Deny() {
		t.Fatal("gate must deny after emergency stop")
	}
}

func TestEmergencyStop_VerificationResult(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-verify")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := result.Verification
	if !v.OutputGateClosed {
		t.Fatal("verification: gate should be closed")
	}
	if !v.AuthoritySuspended {
		t.Fatal("verification: authority should be suspended")
	}
	if !v.RuntimeStopped {
		t.Fatal("verification: runtime should be stopped")
	}
}

func TestEmergencyStop_RaceConcurrentCalls(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-race")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	var wg sync.WaitGroup
	const N = 20
	results := make([]EmergencyStopResult, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, _ := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
				RuntimeID: runtimeID,
				Actor:     EmergencyActorUser,
			})
			results[idx] = res
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.RuntimeID != runtimeID {
			t.Fatalf("result %d wrong runtime: %s", i, r.RuntimeID)
		}
	}
}

func TestEmergencyStop_RaceStopVsOutput(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-race-mix", "plugin-1")

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), "rt-race-mix", "plugin-1")
	_, _ = mgr.Transition(context.Background(), "rt-race-mix", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-race-mix", true)
	rt.SetReady("rt-race-mix", true)

	gate := newStrictEmergencyGate(t, topo, rt, nil, mgr)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive("rt-race-mix", true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		Intent:         NewFakeIntentStore(),
		RuntimeStopper: rtStopper,
	})

	var wg sync.WaitGroup
	const N = 50
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			snap, _ := mgr.Get(context.Background(), "rt-race-mix")
			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-race-mix", "", snap.Epoch),
				Peer:   newTestPeer("rt-race-mix", "", "plugin-1"),
			}
			_, _ = gate.Check(context.Background(), req)
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
				RuntimeID: "rt-race-mix",
				Actor:     EmergencyActorUser,
			})
		}()
	}
	wg.Wait()
}

func TestEmergencyStop_PendingCancelled(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-pending")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	pending := NewFakePendingCanceller()

	svc.mu.Lock()
	svc.pendingCanceller = pending
	svc.mu.Unlock()

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if pending.CancelCount(runtimeID) == 0 {
		t.Fatal("pending canceller must have been invoked")
	}
}

func TestEmergencyStop_LeaseRevoked(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-lease")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	revoker := NewFakeLeaseRevoker()

	svc.mu.Lock()
	svc.leaseRevoker = revoker
	svc.mu.Unlock()

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if revoker.RevokeCount(runtimeID) == 0 {
		t.Fatal("lease revoker must have been invoked")
	}
}

func TestEmergencyStop_ConnectionsClosed(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-conn")
	svc, _, _, _, _ := newTestEmergencyService(t, runtimeID, domain.ControlModePluginControl, 10)

	closer := NewFakeConnectionCloser()

	svc.mu.Lock()
	svc.connectionCloser = closer
	svc.mu.Unlock()

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})

	if closer.CloseCount(runtimeID) == 0 {
		t.Fatal("connection closer must have been invoked")
	}
}

func TestEmergencyStop_OtherRuntimeUnaffected(t *testing.T) {
	runtimeA := domain.RuntimeInstanceID("rt-isolation-a")
	runtimeB := domain.RuntimeInstanceID("rt-isolation-b")

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeA, "plugin-a")
	_, _ = mgr.Create(context.Background(), runtimeB, "plugin-b")
	_, _ = mgr.Transition(context.Background(), runtimeA, TransitionRequest{Target: domain.ControlModePluginControl, Actor: ActorSystem, Reason: ReasonRuntimeLifecycle})
	_, _ = mgr.Transition(context.Background(), runtimeB, TransitionRequest{Target: domain.ControlModePluginControl, Actor: ActorSystem, Reason: ReasonRuntimeLifecycle})

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeA, "plugin-a")
	topo.RegisterRuntime(runtimeB, "plugin-b")

	gate := newStrictEmergencyGate(t, topo, nil, nil, mgr)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive(runtimeA, true)
	rtStopper.SetActive(runtimeB, true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		Intent:         NewFakeIntentStore(),
		RuntimeStopper: rtStopper,
	})

	_, _ = svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeA,
		Actor:     EmergencyActorUser,
	})

	if !gate.IsRuntimeClosed(runtimeA) {
		t.Fatal("runtime A gate should be closed")
	}
	if gate.IsRuntimeClosed(runtimeB) {
		t.Fatal("runtime B gate should NOT be closed")
	}
	if active, _ := rtStopper.IsRuntimeActive(context.Background(), runtimeB); !active {
		t.Fatal("runtime B should still be active")
	}
}

func TestEmergencyStop_AlreadySuspendedAuthorityStillSucceeds(t *testing.T) {
	runtimeID := domain.RuntimeInstanceID("rt-already-suspended")
	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), runtimeID, "plugin-1")
	_, _ = mgr.Transition(context.Background(), runtimeID, TransitionRequest{
		Target: domain.ControlModeSuspended,
		Actor:  ActorSystem,
		Reason: ReasonSystemRecovery,
	})

	topo := NewFakeTopology()
	topo.RegisterRuntime(runtimeID, "plugin-1")

	gate := newStrictEmergencyGate(t, topo, nil, nil, mgr)

	rtStopper := NewFakeEmergencyRuntimeStopper()
	rtStopper.SetActive(runtimeID, true)

	svc := newStrictEmergencyStopService(t, EmergencyStopServiceOptions{
		Clock:          func() time.Time { return time.Now().UTC() },
		Authority:      mgr,
		Gate:           gate,
		Intent:         NewFakeIntentStore(),
		RuntimeStopper: rtStopper,
	})

	result, err := svc.EmergencyStop(context.Background(), EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
	})
	if err != nil {
		t.Fatalf("emergency stop should still succeed when already suspended: %v", err)
	}
	if !result.Success() {
		t.Fatal("already-suspended should still succeed")
	}
	if !gate.IsRuntimeClosed(runtimeID) {
		t.Fatal("gate must be closed even when authority already suspended")
	}
}
