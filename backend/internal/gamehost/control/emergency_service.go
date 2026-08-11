package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	DefaultEmergencyMaxDeadline = 30 * time.Second
	EmergencyRestartWindow       = 30 * time.Second
)

type PendingWorkCanceller interface {
	CancelRuntimeRequests(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type HostAPIWorkCanceller interface {
	CancelRuntimeHostAPIWork(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type SecretLeaseRevoker interface {
	RevokeRuntimeLeases(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type ConnectionCloser interface {
	CloseRuntimeConnections(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type HandshakeResetter interface {
	ClearRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type StreamStopper interface {
	StopRuntimeStreams(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type ChannelCleaner interface {
	CleanupRuntimeChannels(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type BinaryReleaser interface {
	ReleaseRuntimeTransientBinary(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type RuntimeStopper interface {
	StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID, force bool) error
	SuppressAutoRestart(runtimeID domain.RuntimeInstanceID, duration time.Duration)
	IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
}

type ProcessTreeCleaner interface {
	CleanupProcessTree(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type RestartSuppressor interface {
	SuppressRestart(runtimeID domain.RuntimeInstanceID, duration time.Duration)
	IsRestartSuppressed(runtimeID domain.RuntimeInstanceID) bool
}

type EmergencyOperationStore struct {
	mu         sync.RWMutex
	operations map[string]*EmergencyStopOperation
}

type EmergencyStopOperation struct {
	OperationID   string
	RuntimeID     domain.RuntimeInstanceID
	State         EmergencyStopState
	Actor         EmergencyStopActor
	Reason        EmergencyStopReason
	PreviousMode  domain.ControlMode
	PreviousEpoch uint64
	NewEpoch      uint64
	StartedAt     time.Time
	FinishedAt    time.Time
	Errors        []error
}

func NewEmergencyOperationStore() *EmergencyOperationStore {
	return &EmergencyOperationStore{
		operations: make(map[string]*EmergencyStopOperation),
	}
}

func (s *EmergencyOperationStore) Get(operationID string) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.operations[operationID]
	return op, ok
}

func (s *EmergencyOperationStore) GetByRuntime(runtimeID domain.RuntimeInstanceID) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, op := range s.operations {
		if op.RuntimeID == runtimeID && !op.State.IsTerminal() {
			return copyOperation(op), true
		}
	}
	return nil, false
}

func (s *EmergencyOperationStore) Put(op *EmergencyStopOperation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[op.OperationID] = op
}

func (s *EmergencyOperationStore) UpdateState(operationID string, state EmergencyStopState, errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if op, ok := s.operations[operationID]; ok {
		op.State = state
		if len(errs) > 0 {
			op.Errors = append(op.Errors, errs...)
		}
		if state.IsTerminal() {
			op.FinishedAt = time.Now().UTC()
		}
	}
}

func (s *EmergencyOperationStore) UpdateEpoch(operationID string, epoch uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if op, ok := s.operations[operationID]; ok {
		op.NewEpoch = epoch
	}
}

func (s *EmergencyOperationStore) Clear(operationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.operations, operationID)
}

func (s *EmergencyOperationStore) GetLastByRuntime(runtimeID domain.RuntimeInstanceID) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *EmergencyStopOperation
	for _, op := range s.operations {
		if op.RuntimeID == runtimeID {
			if latest == nil || op.StartedAt.After(latest.StartedAt) {
				latest = op
			}
		}
	}
	if latest == nil {
		return nil, false
	}
	return copyOperation(latest), true
}

func copyOperation(op *EmergencyStopOperation) *EmergencyStopOperation {
	if op == nil {
		return nil
	}
	c := *op
	if op.Errors != nil {
		c.Errors = make([]error, len(op.Errors))
		copy(c.Errors, op.Errors)
	}
	return &c
}

func (s *EmergencyOperationStore) ListActive() []*EmergencyStopOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []*EmergencyStopOperation
	for _, op := range s.operations {
		if !op.State.IsTerminal() {
			active = append(active, copyOperation(op))
		}
	}
	return active
}

type EmergencyStopService struct {
	clock     Clock
	authority *ControlAuthorityManager
	gate      *PluginOutputGate
	store     *EmergencyOperationStore

	pendingCanceller PendingWorkCanceller
	hostAPICanceller HostAPIWorkCanceller
	leaseRevoker     SecretLeaseRevoker
	connectionCloser ConnectionCloser
	handshakeReset   HandshakeResetter
	streamStopper    StreamStopper
	channelCleaner   ChannelCleaner
	binaryReleaser   BinaryReleaser
	runtimeStopper   RuntimeStopper
	processCleaner   ProcessTreeCleaner
	restartSuppress  RestartSuppressor
	audit            AuthorityAuditSink
	metrics          EmergencyMetrics
	topology         TopologyReader

	mu       sync.Mutex
	routines map[domain.RuntimeInstanceID]*sync.Mutex
	opMu     sync.Mutex

	maxDeadline time.Duration
}

type EmergencyStopServiceOptions struct {
	Clock            Clock
	Authority        *ControlAuthorityManager
	Gate             *PluginOutputGate
	PendingCanceller PendingWorkCanceller
	HostAPICanceller HostAPIWorkCanceller
	LeaseRevoker     SecretLeaseRevoker
	ConnectionCloser ConnectionCloser
	HandshakeReset   HandshakeResetter
	StreamStopper    StreamStopper
	ChannelCleaner   ChannelCleaner
	BinaryReleaser   BinaryReleaser
	RuntimeStopper   RuntimeStopper
	ProcessCleaner   ProcessTreeCleaner
	RestartSuppress  RestartSuppressor
	Audit            AuthorityAuditSink
	Metrics          EmergencyMetrics
	Topology         TopologyReader
	MaxDeadline      time.Duration
}

type EmergencyMetrics interface {
	RecordEmergencyStop(runtimeID domain.RuntimeInstanceID, actor EmergencyStopActor, state EmergencyStopState, critical bool)
	RecordEmergencyCleanupError(runtimeID domain.RuntimeInstanceID, stage EmergencyStopState)
}

func NewEmergencyStopService(opts EmergencyStopServiceOptions) *EmergencyStopService {
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	maxDL := opts.MaxDeadline
	if maxDL <= 0 {
		maxDL = DefaultEmergencyMaxDeadline
	}
	return &EmergencyStopService{
		clock:            clock,
		authority:        opts.Authority,
		gate:             opts.Gate,
		store:            NewEmergencyOperationStore(),
		pendingCanceller: opts.PendingCanceller,
		hostAPICanceller: opts.HostAPICanceller,
		leaseRevoker:     opts.LeaseRevoker,
		connectionCloser: opts.ConnectionCloser,
		handshakeReset:   opts.HandshakeReset,
		streamStopper:    opts.StreamStopper,
		channelCleaner:   opts.ChannelCleaner,
		binaryReleaser:   opts.BinaryReleaser,
		runtimeStopper:   opts.RuntimeStopper,
		processCleaner:   opts.ProcessCleaner,
		restartSuppress:  opts.RestartSuppress,
		audit:            opts.Audit,
		metrics:          opts.Metrics,
		topology:         opts.Topology,
		routines:         make(map[domain.RuntimeInstanceID]*sync.Mutex),
		maxDeadline:      maxDL,
	}
}

func (s *EmergencyStopService) getRuntimeLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	lock, ok := s.routines[runtimeID]
	if !ok {
		lock = &sync.Mutex{}
		s.routines[runtimeID] = lock
	}
	return lock
}

func (s *EmergencyStopService) EmergencyStop(ctx context.Context, req EmergencyStopRequest) (EmergencyStopResult, error) {
	now := s.clock()

	runtimeID := req.RuntimeID
	if runtimeID == "" {
		return EmergencyStopResult{}, errEmergencyRuntimeNotFound(runtimeID)
	}

	if err := ctx.Err(); err != nil {
		return EmergencyStopResult{}, err
	}

	operationID := req.OperationID
	if operationID == "" {
		operationID = generateEmergencyOperationID()
	}

	activeOp, exists := s.store.GetByRuntime(runtimeID)
	if exists {
		return s.buildResult(activeOp), nil
	}

	if lastOp, ok := s.store.GetLastByRuntime(runtimeID); ok {
		return s.buildResult(lastOp), nil
	}

	if s.authority != nil {
		if _, err := s.authority.Get(ctx, runtimeID); err != nil {
			return EmergencyStopResult{}, errEmergencyRuntimeNotFound(runtimeID)
		}
	}

	var previousMode domain.ControlMode = domain.ControlModeObserveOnly
	var previousEpoch uint64
	if s.authority != nil {
		snap, err := s.authority.Get(ctx, runtimeID)
		if err != nil {
			return EmergencyStopResult{}, errEmergencyRuntimeNotFound(runtimeID)
		}
		previousMode = snap.Mode
		previousEpoch = snap.Epoch
	}

	op := &EmergencyStopOperation{
		OperationID:   operationID,
		RuntimeID:     runtimeID,
		State:         EmergencyStateRequested,
		Actor:         req.Actor,
		Reason:        req.Reason,
		PreviousMode:  previousMode,
		PreviousEpoch: previousEpoch,
		StartedAt:     now,
	}
	s.store.Put(op)

	lock := s.getRuntimeLock(runtimeID)
	if !lock.TryLock() {
		existing, _ := s.store.GetByRuntime(runtimeID)
		if existing != nil {
			return s.buildResult(existing), nil
		}
		return EmergencyStopResult{}, errEmergencyStateConflict(runtimeID, op.State)
	}
	defer lock.Unlock()

	deadline := s.maxDeadline
	if !req.Deadline.IsZero() {
		deadline = time.Until(req.Deadline)
		if deadline > s.maxDeadline {
			deadline = s.maxDeadline
		}
	}
	stopCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	result := s.executeSafetyChain(stopCtx, op, previousMode, previousEpoch)
	return result, nil
}

func (s *EmergencyStopService) executeSafetyChain(ctx context.Context, op *EmergencyStopOperation, previousMode domain.ControlMode, previousEpoch uint64) EmergencyStopResult {
	cleanupErrors := make([]error, 0)
	criticalFailure := false

	transitionTo := func(state EmergencyStopState, errs ...error) {
		s.store.UpdateState(op.OperationID, state, errs...)
		if len(errs) > 0 {
			cleanupErrors = append(cleanupErrors, errs...)
		}
	}

	fail := func(state EmergencyStopState, cause error) EmergencyStopResult {
		transitionTo(EmergencyStateFailed, cause)
		return s.buildFailedResult(op.OperationID, op.RuntimeID, op.Actor, op.Reason, previousMode, previousEpoch, op.StartedAt, append(cleanupErrors, cause))
	}

	// SAFETY DOOR 1: Close G9 Plugin Output Gate
	transitionTo(EmergencyStateClosingGate)
	if s.gate != nil {
		s.gate.CloseRuntimeOutputs(op.RuntimeID)
	}

	if err := ctx.Err(); err != nil {
		return fail(EmergencyStateClosingGate, errEmergencyDeadlineExceeded(op.OperationID))
	}

	// Restart Suppression BEFORE Runtime Stop
	if s.restartSuppress != nil {
		s.restartSuppress.SuppressRestart(op.RuntimeID, EmergencyRestartWindow)
	}
	if s.runtimeStopper != nil {
		s.runtimeStopper.SuppressAutoRestart(op.RuntimeID, EmergencyRestartWindow)
	}

	// SAFETY DOOR 2: Authority -> suspended + Epoch++
	transitionTo(EmergencyStateRevokingAuthority)
	var newEpoch uint64
	if s.authority != nil {
		currentSnap, err := s.authority.Get(ctx, op.RuntimeID)
		if err == nil && currentSnap.Mode != domain.ControlModeSuspended {
			_, err = s.authority.Transition(ctx, op.RuntimeID, TransitionRequest{
				Target: domain.ControlModeSuspended,
				Actor:  ActorSystem,
				Reason: ReasonSystemRecovery,
			})
			if err != nil {
				transitionTo(EmergencyStateRevokingAuthority, &AuthorityError{
					Code:    domain.ErrInvalidState,
					Message: "emergency stop: authority transition failed: " + err.Error(),
				})
			}
		}
		afterSnap, _ := s.authority.Get(ctx, op.RuntimeID)
		newEpoch = afterSnap.Epoch
		s.store.UpdateEpoch(op.OperationID, newEpoch)
	}

	if err := ctx.Err(); err != nil {
		criticalFailure = true
		cleanupErrors = append(cleanupErrors, errEmergencyDeadlineExceeded(op.OperationID))
	}

	// SAFETY DOOR 3: Mark intent / Cancel pending work (parallel where safe)
	transitionTo(EmergencyStateCancelling)
	s.cancelPendingWork(ctx, op.RuntimeID, &cleanupErrors)

	// Stop Runtime
	transitionTo(EmergencyStateStoppingRuntime)
	if s.runtimeStopper != nil {
		force := false
		if s.gate != nil && s.gate.IsRuntimeClosed(op.RuntimeID) {
			force = false
		}
		if err := s.runtimeStopper.StopRuntime(ctx, op.RuntimeID, force); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	if s.processCleaner != nil {
		if err := s.processCleaner.CleanupProcessTree(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	// Close Connections
	transitionTo(EmergencyStateClosingConnections)
	if s.connectionCloser != nil {
		if _, err := s.connectionCloser.CloseRuntimeConnections(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	if s.handshakeReset != nil {
		if err := s.handshakeReset.ClearRuntimeReady(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	// Revoke SecretLease
	transitionTo(EmergencyStateRevokingLeases)
	if s.leaseRevoker != nil {
		if _, err := s.leaseRevoker.RevokeRuntimeLeases(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	// Clean transient resources
	transitionTo(EmergencyStateCleaningResources)
	if s.streamStopper != nil {
		if err := s.streamStopper.StopRuntimeStreams(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	if s.channelCleaner != nil {
		if err := s.channelCleaner.CleanupRuntimeChannels(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	if s.binaryReleaser != nil {
		if err := s.binaryReleaser.ReleaseRuntimeTransientBinary(ctx, op.RuntimeID); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	// Final Verification
	transitionTo(EmergencyStateVerifying)
	verification := s.verifyCleanup(ctx, op, newEpoch)
	if verification.ResidueErrors != nil {
		cleanupErrors = append(cleanupErrors, verificationErrors(verification.ResidueErrors)...)
	}

	s.store.UpdateState(op.OperationID, EmergencyStateCompleted)

	finalOp, _ := s.store.Get(op.OperationID)
	finishedAt := s.clock()
	startedAt := finishedAt
	if finalOp != nil {
		finishedAt = finalOp.FinishedAt
		startedAt = finalOp.StartedAt
	}

	result := EmergencyStopResult{
		OperationID:     op.OperationID,
		RuntimeID:       op.RuntimeID,
		State:           EmergencyStateCompleted,
		Actor:           op.Actor,
		Reason:          op.Reason,
		PreviousMode:    previousMode,
		PreviousEpoch:   previousEpoch,
		NewEpoch:        newEpoch,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Duration:        finishedAt.Sub(startedAt),
		Verification:    verification,
		CleanupErrors:   cleanupErrors,
		CriticalFailure: criticalFailure,
	}

	if s.metrics != nil {
		s.metrics.RecordEmergencyStop(op.RuntimeID, op.Actor, EmergencyStateCompleted, criticalFailure)
	}
	return result
}

func (s *EmergencyStopService) cancelPendingWork(ctx context.Context, runtimeID domain.RuntimeInstanceID, errs *[]error) {
	if s.pendingCanceller != nil {
		if _, err := s.pendingCanceller.CancelRuntimeRequests(ctx, runtimeID); err != nil {
			*errs = append(*errs, err)
		}
	}
	if s.hostAPICanceller != nil {
		if _, err := s.hostAPICanceller.CancelRuntimeHostAPIWork(ctx, runtimeID); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (s *EmergencyStopService) verifyCleanup(ctx context.Context, op *EmergencyStopOperation, newEpoch uint64) VerificationResult {
	result := VerificationResult{}

	if s.gate != nil {
		result.OutputGateClosed = s.gate.IsRuntimeClosed(op.RuntimeID)
	}

	if s.authority != nil {
		snap, err := s.authority.Get(ctx, op.RuntimeID)
		if err == nil {
			result.AuthoritySuspended = snap.Mode == domain.ControlModeSuspended && snap.Epoch == newEpoch
		}
	}

	if s.runtimeStopper != nil {
		active, err := s.runtimeStopper.IsRuntimeActive(ctx, op.RuntimeID)
		result.RuntimeStopped = (err != nil || !active)
	}

	if s.connectionCloser != nil {
		result.ConnectionsClosed = true
	}
	if s.leaseRevoker != nil {
		result.LeasesRevoked = true
	}
	if s.pendingCanceller != nil {
		result.PendingCleared = true
	}
	if s.processCleaner != nil {
		result.ProcessCleanedUp = true
	}

	if !result.OutputGateClosed {
		result.ResidueErrors = append(result.ResidueErrors, "output_gate_not_closed")
	}
	if !result.AuthoritySuspended {
		result.ResidueErrors = append(result.ResidueErrors, "authority_not_suspended")
	}
	if !result.RuntimeStopped {
		result.ResidueErrors = append(result.ResidueErrors, "runtime_not_stopped")
	}

	return result
}

func (s *EmergencyStopService) buildResult(op *EmergencyStopOperation) EmergencyStopResult {
	finished := op.FinishedAt
	if finished.IsZero() {
		finished = s.clock()
	}
	return EmergencyStopResult{
		OperationID:   op.OperationID,
		RuntimeID:     op.RuntimeID,
		State:         op.State,
		Actor:         op.Actor,
		Reason:        op.Reason,
		PreviousMode:  op.PreviousMode,
		PreviousEpoch: op.PreviousEpoch,
		NewEpoch:      op.NewEpoch,
		StartedAt:     op.StartedAt,
		FinishedAt:    finished,
		Duration:      finished.Sub(op.StartedAt),
		CleanupErrors: op.Errors,
	}
}

func (s *EmergencyStopService) buildFailedResult(operationID string, runtimeID domain.RuntimeInstanceID, actor EmergencyStopActor, reason EmergencyStopReason, previousMode domain.ControlMode, previousEpoch uint64, startedAt time.Time, errs []error) EmergencyStopResult {
	finished := s.clock()
	return EmergencyStopResult{
		OperationID:     operationID,
		RuntimeID:       runtimeID,
		State:           EmergencyStateFailed,
		Actor:           actor,
		Reason:          reason,
		PreviousMode:    previousMode,
		PreviousEpoch:   previousEpoch,
		StartedAt:       startedAt,
		FinishedAt:      finished,
		Duration:        finished.Sub(startedAt),
		CleanupErrors:   errs,
		CriticalFailure: true,
	}
}

func (s *EmergencyStopService) IsRestartSuppressed(runtimeID domain.RuntimeInstanceID) bool {
	if s.restartSuppress != nil {
		return s.restartSuppress.IsRestartSuppressed(runtimeID)
	}
	return false
}

func (s *EmergencyStopService) GetOperation(operationID string) (*EmergencyStopOperation, bool) {
	return s.store.Get(operationID)
}

func (s *EmergencyStopService) ListActiveOperations() []*EmergencyStopOperation {
	return s.store.ListActive()
}

func verificationErrors(reasons []string) []error {
	errs := make([]error, 0, len(reasons))
	for _, r := range reasons {
		errs = append(errs, fmt.Errorf("verification residue: %s", r))
	}
	return errs
}

func generateEmergencyOperationID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "emg-" + hex.EncodeToString(b[:])
}
