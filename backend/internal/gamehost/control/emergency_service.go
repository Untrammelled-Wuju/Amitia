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
)

type PendingWorkCanceller interface {
	CancelRuntimeRequests(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type HostAPIWorkCanceller interface {
	CancelRuntimeHostAPIWork(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
}

type HostAPIWorkRearm interface {
	RearmRuntimeHostAPIWork(runtimeID domain.RuntimeInstanceID)
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
	StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
}

type EmergencyOperationStore struct {
	mu               sync.RWMutex
	operations       map[string]*EmergencyStopOperation
	activeByRuntime  map[domain.RuntimeInstanceID]string
	byIdempotencyKey map[string]string
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
		operations:       make(map[string]*EmergencyStopOperation),
		activeByRuntime:  make(map[domain.RuntimeInstanceID]string),
		byIdempotencyKey: make(map[string]string),
	}
}

func (s *EmergencyOperationStore) Get(operationID string) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.operations[operationID]
	if !ok {
		return nil, false
	}
	return copyOperation(op), true
}

func (s *EmergencyOperationStore) GetByRuntime(runtimeID domain.RuntimeInstanceID) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	opID, ok := s.activeByRuntime[runtimeID]
	if !ok {
		return nil, false
	}
	op, exists := s.operations[opID]
	if !exists || op.State.IsTerminal() {
		return nil, false
	}
	return copyOperation(op), true
}

func (s *EmergencyOperationStore) GetLatestByRuntime(runtimeID domain.RuntimeInstanceID) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *EmergencyStopOperation
	for _, op := range s.operations {
		if op.RuntimeID != runtimeID {
			continue
		}
		if latest == nil || op.StartedAt.After(latest.StartedAt) {
			latest = op
		}
	}
	if latest == nil {
		return nil, false
	}
	return copyOperation(latest), true
}

func (s *EmergencyOperationStore) GetByIdempotencyKey(key string) (*EmergencyStopOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	opID, ok := s.byIdempotencyKey[key]
	if !ok {
		return nil, false
	}
	op, exists := s.operations[opID]
	if !exists {
		return nil, false
	}
	return copyOperation(op), true
}

func (s *EmergencyOperationStore) Put(op *EmergencyStopOperation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[op.OperationID] = op
	if !op.State.IsTerminal() {
		s.activeByRuntime[op.RuntimeID] = op.OperationID
	}
}

func (s *EmergencyOperationStore) UpdateState(operationID string, state EmergencyStopState, finishedAt time.Time, errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if op, ok := s.operations[operationID]; ok {
		op.State = state
		if len(errs) > 0 {
			op.Errors = append(op.Errors, errs...)
		}
		if state.IsTerminal() {
			op.FinishedAt = finishedAt
			if op.RuntimeID != "" {
				if activeID, exists := s.activeByRuntime[op.RuntimeID]; exists && activeID == operationID {
					delete(s.activeByRuntime, op.RuntimeID)
				}
			}
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

func (s *EmergencyOperationStore) MarkActive(op *EmergencyStopOperation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeByRuntime[op.RuntimeID] = op.OperationID
}

func (s *EmergencyOperationStore) RegisterIdempotencyKey(key string, operationID string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byIdempotencyKey[key] = operationID
}

func (s *EmergencyOperationStore) Clear(operationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if op, ok := s.operations[operationID]; ok {
		if op.RuntimeID != "" {
			if activeID, exists := s.activeByRuntime[op.RuntimeID]; exists && activeID == operationID {
				delete(s.activeByRuntime, op.RuntimeID)
			}
		}
	}
	delete(s.operations, operationID)
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

type EmergencyStopService struct {
	clock           Clock
	authority       *ControlAuthorityManager
	gate            *PluginOutputGate
	store           *EmergencyOperationStore
	intent          EmergencyIntentStore
	lifecycleIntent LifecycleIntentWriter

	pendingCanceller PendingWorkCanceller
	hostAPICanceller HostAPIWorkCanceller
	leaseRevoker     SecretLeaseRevoker
	connectionCloser ConnectionCloser
	handshakeReset   HandshakeResetter
	streamStopper    StreamStopper
	channelCleaner   ChannelCleaner
	binaryReleaser   BinaryReleaser
	runtimeStopper   RuntimeStopper
	audit            AuthorityAuditSink
	metrics          EmergencyMetrics
	topology         TopologyReader

	pendingVerifier    PendingVerifier
	connectionVerifier ConnectionVerifier
	leaseVerifier      SecretLeaseVerifier
	channelVerifier    ChannelVerifier
	streamVerifier     StreamVerifier
	binaryVerifier     BinaryVerifier
	readyVerifier      ReadyVerifier
	hostAPIVerifier    HostAPIWorkVerifier

	mu       sync.Mutex
	routines map[domain.RuntimeInstanceID]*sync.Mutex
	opMu     sync.Mutex

	maxDeadline time.Duration
}

type EmergencyStopServiceOptions struct {
	Clock              Clock
	Authority          *ControlAuthorityManager
	Gate               *PluginOutputGate
	Intent             EmergencyIntentStore
	LifecycleIntent    LifecycleIntentWriter
	PendingCanceller   PendingWorkCanceller
	HostAPICanceller   HostAPIWorkCanceller
	LeaseRevoker       SecretLeaseRevoker
	ConnectionCloser   ConnectionCloser
	HandshakeReset     HandshakeResetter
	StreamStopper      StreamStopper
	ChannelCleaner     ChannelCleaner
	BinaryReleaser     BinaryReleaser
	RuntimeStopper     RuntimeStopper
	Audit              AuthorityAuditSink
	Metrics            EmergencyMetrics
	Topology           TopologyReader
	PendingVerifier    PendingVerifier
	ConnectionVerifier ConnectionVerifier
	LeaseVerifier      SecretLeaseVerifier
	ChannelVerifier    ChannelVerifier
	StreamVerifier     StreamVerifier
	BinaryVerifier     BinaryVerifier
	ReadyVerifier      ReadyVerifier
	HostAPIVerifier    HostAPIWorkVerifier
	MaxDeadline        time.Duration
}

type EmergencyMetrics interface {
	RecordEmergencyStop(runtimeID domain.RuntimeInstanceID, actor EmergencyStopActor, state EmergencyStopState, critical bool)
	RecordEmergencyCleanupError(runtimeID domain.RuntimeInstanceID, stage EmergencyStopState)
}

func NewEmergencyStopService(opts EmergencyStopServiceOptions) (*EmergencyStopService, error) {
	if opts.Authority == nil {
		return nil, fmt.Errorf("emergency stop service: authority is required")
	}
	if opts.Gate == nil {
		return nil, fmt.Errorf("emergency stop service: gate is required")
	}
	if opts.RuntimeStopper == nil {
		return nil, fmt.Errorf("emergency stop service: runtime stopper is required")
	}
	if opts.Intent == nil {
		return nil, fmt.Errorf("emergency stop service: emergency intent store is required")
	}
	if opts.ConnectionCloser == nil {
		return nil, fmt.Errorf("emergency stop service: connection closer is required")
	}
	if opts.PendingCanceller == nil {
		return nil, fmt.Errorf("emergency stop service: pending canceller is required")
	}
	if opts.HostAPICanceller == nil || opts.LeaseRevoker == nil || opts.HandshakeReset == nil || opts.StreamStopper == nil || opts.ChannelCleaner == nil || opts.BinaryReleaser == nil {
		return nil, fmt.Errorf("emergency stop service: complete safety cleanup dependencies are required")
	}
	if opts.PendingVerifier == nil || opts.ConnectionVerifier == nil || opts.LeaseVerifier == nil || opts.ChannelVerifier == nil || opts.StreamVerifier == nil || opts.BinaryVerifier == nil || opts.ReadyVerifier == nil || opts.HostAPIVerifier == nil {
		return nil, fmt.Errorf("emergency stop service: complete safety verification dependencies are required")
	}
	if opts.LifecycleIntent == nil {
		return nil, fmt.Errorf("emergency stop service: lifecycle intent writer is required")
	}

	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	maxDL := opts.MaxDeadline
	if maxDL <= 0 {
		maxDL = DefaultEmergencyMaxDeadline
	}
	return &EmergencyStopService{
		clock:              clock,
		authority:          opts.Authority,
		gate:               opts.Gate,
		store:              NewEmergencyOperationStore(),
		intent:             opts.Intent,
		lifecycleIntent:    opts.LifecycleIntent,
		pendingCanceller:   opts.PendingCanceller,
		hostAPICanceller:   opts.HostAPICanceller,
		leaseRevoker:       opts.LeaseRevoker,
		connectionCloser:   opts.ConnectionCloser,
		handshakeReset:     opts.HandshakeReset,
		streamStopper:      opts.StreamStopper,
		channelCleaner:     opts.ChannelCleaner,
		binaryReleaser:     opts.BinaryReleaser,
		runtimeStopper:     opts.RuntimeStopper,
		audit:              opts.Audit,
		metrics:            opts.Metrics,
		topology:           opts.Topology,
		pendingVerifier:    opts.PendingVerifier,
		connectionVerifier: opts.ConnectionVerifier,
		leaseVerifier:      opts.LeaseVerifier,
		channelVerifier:    opts.ChannelVerifier,
		streamVerifier:     opts.StreamVerifier,
		binaryVerifier:     opts.BinaryVerifier,
		readyVerifier:      opts.ReadyVerifier,
		hostAPIVerifier:    opts.HostAPIVerifier,
		routines:           make(map[domain.RuntimeInstanceID]*sync.Mutex),
		maxDeadline:        maxDL,
	}, nil
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

	if activeOp, exists := s.store.GetByRuntime(runtimeID); exists {
		return s.buildResult(activeOp), nil
	}

	if req.IdempotencyKey != "" {
		if existingOp, ok := s.store.GetByIdempotencyKey(req.IdempotencyKey); ok {
			return s.buildResult(existingOp), nil
		}
	}

	if s.intent != nil && s.intent.IsEmergencyLatched(ctx, runtimeID) {
		if latchedOpID, ok := s.intent.GetEmergencyOperationID(ctx, runtimeID); ok {
			if latchedOp, exists := s.store.Get(latchedOpID); exists {
				return s.buildResult(latchedOp), nil
			}
		}
		if previousOp, exists := s.store.GetLatestByRuntime(runtimeID); exists {
			return s.buildResult(previousOp), nil
		}
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
	if req.IdempotencyKey != "" {
		s.store.RegisterIdempotencyKey(req.IdempotencyKey, operationID)
	}

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
		s.store.UpdateState(op.OperationID, state, s.clock(), errs...)
		if len(errs) > 0 {
			cleanupErrors = append(cleanupErrors, errs...)
		}
	}

	fail := func(state EmergencyStopState, cause error) EmergencyStopResult {
		finishedAt := s.clock()
		s.store.UpdateState(op.OperationID, EmergencyStateFailed, finishedAt, cause)
		return s.buildFailedResult(op.OperationID, op.RuntimeID, op.Actor, op.Reason, previousMode, previousEpoch, op.StartedAt, finishedAt, append(cleanupErrors, cause))
	}

	transitionTo(EmergencyStateCommittingIntent)
	if err := s.lifecycleIntent.SetLifecycleIntent(op.RuntimeID, "emergency"); err != nil {
		return fail(EmergencyStateCommittingIntent, err)
	}
	if s.intent != nil {
		if err := s.intent.CommitEmergencyIntent(ctx, op.RuntimeID, op.OperationID); err != nil {
			return fail(EmergencyStateCommittingIntent, err)
		}
	}

	transitionTo(EmergencyStateClosingGate)
	s.gate.CloseRuntimeOutputs(op.RuntimeID)

	if err := ctx.Err(); err != nil {
		return fail(EmergencyStateClosingGate, errEmergencyDeadlineExceeded(op.OperationID))
	}

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
				cleanupErrors = append(cleanupErrors, &AuthorityError{
					Code:    domain.ErrInvalidState,
					Message: "emergency stop: authority transition failed: " + err.Error(),
				})
				criticalFailure = true
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

	transitionTo(EmergencyStateCancelling)
	s.cancelPendingWork(ctx, op.RuntimeID, &cleanupErrors)

	transitionTo(EmergencyStateRevokingLeases)
	if _, err := s.leaseRevoker.RevokeRuntimeLeases(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}

	transitionTo(EmergencyStateClosingConnections)
	if _, err := s.connectionCloser.CloseRuntimeConnections(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := s.handshakeReset.ClearRuntimeReady(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}

	if err := ctx.Err(); err != nil {
		criticalFailure = true
		cleanupErrors = append(cleanupErrors, errEmergencyDeadlineExceeded(op.OperationID))
	}

	transitionTo(EmergencyStateStoppingRuntime)
	if err := s.runtimeStopper.StopRuntime(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
		criticalFailure = true
	}

	transitionTo(EmergencyStateCleaningResources)
	if err := s.streamStopper.StopRuntimeStreams(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := s.channelCleaner.CleanupRuntimeChannels(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := s.binaryReleaser.ReleaseRuntimeTransientBinary(ctx, op.RuntimeID); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}

	transitionTo(EmergencyStateVerifying)
	verification := s.verifyCleanup(ctx, op, newEpoch)
	if len(verification.ResidueErrors) > 0 {
		cleanupErrors = append(cleanupErrors, verificationErrors(verification.ResidueErrors)...)
	}

	finishedAt := s.clock()
	safetyGatesSatisfied := verification.EmergencyLatched &&
		verification.OutputGateClosed &&
		verification.AuthoritySuspended &&
		verification.RuntimeStopped &&
		verification.ConnectionsClosed &&
		verification.PendingCleared &&
		verification.HostAPIInflightCleared &&
		verification.LeasesRevoked &&
		verification.ReadyCleared &&
		verification.ChannelsCleared &&
		verification.StreamsCleared &&
		verification.TransientBinaryReleased

	if safetyGatesSatisfied && !criticalFailure {
		s.store.UpdateState(op.OperationID, EmergencyStateCompleted, finishedAt)
	} else {
		s.store.UpdateState(op.OperationID, EmergencyStateFailed, finishedAt)
		criticalFailure = true
	}

	result := s.buildResultFromVerification(op, previousMode, previousEpoch, newEpoch, finishedAt, verification, cleanupErrors, criticalFailure)

	if s.metrics != nil {
		state := EmergencyStateCompleted
		if criticalFailure {
			state = EmergencyStateFailed
		}
		s.metrics.RecordEmergencyStop(op.RuntimeID, op.Actor, state, criticalFailure)
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

	result.OutputGateClosed = s.gate.IsRuntimeClosed(op.RuntimeID)

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

	result.ConnectionsClosed = s.connectionVerifier.CountRuntimeConnections(op.RuntimeID) == 0

	result.LeasesRevoked = s.leaseVerifier.CountRuntimeLeases(op.RuntimeID) == 0

	result.PendingCleared = s.pendingVerifier.CountRuntimePending(op.RuntimeID) == 0
	result.HostAPIInflightCleared = s.hostAPIVerifier.CountRuntimeHostAPIWork(op.RuntimeID) == 0
	result.ReadyCleared = s.readyVerifier.CountRuntimeReady(op.RuntimeID) == 0

	result.ChannelsCleared = s.channelVerifier.CountRuntimeChannels(op.RuntimeID) == 0

	result.StreamsCleared = s.streamVerifier.CountRuntimeStreams(op.RuntimeID) == 0

	result.TransientBinaryReleased = s.binaryVerifier.CountRuntimeBinary(op.RuntimeID) == 0

	if s.intent != nil {
		result.EmergencyLatched = s.intent.IsEmergencyLatched(ctx, op.RuntimeID)
		result.RecoverySuppressed = result.EmergencyLatched
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
	if !result.ConnectionsClosed {
		result.ResidueErrors = append(result.ResidueErrors, "connections_not_closed")
	}
	if !result.EmergencyLatched {
		result.ResidueErrors = append(result.ResidueErrors, "emergency_not_latched")
	}
	if !result.PendingCleared {
		result.ResidueErrors = append(result.ResidueErrors, "pending_rpc_not_cleared")
	}
	if !result.HostAPIInflightCleared {
		result.ResidueErrors = append(result.ResidueErrors, "hostapi_inflight_not_cleared")
	}
	if !result.LeasesRevoked {
		result.ResidueErrors = append(result.ResidueErrors, "secret_leases_not_revoked")
	}
	if !result.ReadyCleared {
		result.ResidueErrors = append(result.ResidueErrors, "ready_not_cleared")
	}
	if !result.ChannelsCleared {
		result.ResidueErrors = append(result.ResidueErrors, "channels_not_cleared")
	}
	if !result.StreamsCleared {
		result.ResidueErrors = append(result.ResidueErrors, "streams_not_cleared")
	}
	if !result.TransientBinaryReleased {
		result.ResidueErrors = append(result.ResidueErrors, "transient_binary_not_released")
	}

	return result
}

func (s *EmergencyStopService) buildResult(op *EmergencyStopOperation) EmergencyStopResult {
	finished := op.FinishedAt
	if finished.IsZero() {
		finished = s.clock()
	}
	return EmergencyStopResult{
		OperationID:     op.OperationID,
		RuntimeID:       op.RuntimeID,
		State:           op.State,
		Actor:           op.Actor,
		Reason:          op.Reason,
		PreviousMode:    op.PreviousMode,
		PreviousEpoch:   op.PreviousEpoch,
		NewEpoch:        op.NewEpoch,
		StartedAt:       op.StartedAt,
		FinishedAt:      finished,
		Duration:        finished.Sub(op.StartedAt),
		CleanupErrors:   op.Errors,
		CriticalFailure: op.State == EmergencyStateFailed,
	}
}

func (s *EmergencyStopService) buildResultFromVerification(op *EmergencyStopOperation, previousMode domain.ControlMode, previousEpoch uint64, newEpoch uint64, finishedAt time.Time, verification VerificationResult, errs []error, criticalFailure bool) EmergencyStopResult {
	state := EmergencyStateCompleted
	if criticalFailure {
		state = EmergencyStateFailed
	}
	return EmergencyStopResult{
		OperationID:     op.OperationID,
		RuntimeID:       op.RuntimeID,
		State:           state,
		Actor:           op.Actor,
		Reason:          op.Reason,
		PreviousMode:    previousMode,
		PreviousEpoch:   previousEpoch,
		NewEpoch:        newEpoch,
		StartedAt:       op.StartedAt,
		FinishedAt:      finishedAt,
		Duration:        finishedAt.Sub(op.StartedAt),
		Verification:    verification,
		CleanupErrors:   errs,
		CriticalFailure: criticalFailure,
		Residue:         verification.ResidueErrors,
	}
}

func (s *EmergencyStopService) buildFailedResult(operationID string, runtimeID domain.RuntimeInstanceID, actor EmergencyStopActor, reason EmergencyStopReason, previousMode domain.ControlMode, previousEpoch uint64, startedAt time.Time, finishedAt time.Time, errs []error) EmergencyStopResult {
	return EmergencyStopResult{
		OperationID:     operationID,
		RuntimeID:       runtimeID,
		State:           EmergencyStateFailed,
		Actor:           actor,
		Reason:          reason,
		PreviousMode:    previousMode,
		PreviousEpoch:   previousEpoch,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Duration:        finishedAt.Sub(startedAt),
		CleanupErrors:   errs,
		CriticalFailure: true,
	}
}

func (s *EmergencyStopService) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	if s.intent == nil {
		return fmt.Errorf("emergency stop service: intent store not available")
	}
	if err := s.intent.ClearEmergencyLatch(ctx, runtimeID, actor); err != nil {
		return err
	}
	if err := s.lifecycleIntent.SetLifecycleIntent(runtimeID, ""); err != nil {
		return err
	}
	s.gate.ReopenRuntimeOutputs(runtimeID)
	if rearm, ok := s.hostAPICanceller.(HostAPIWorkRearm); ok {
		rearm.RearmRuntimeHostAPIWork(runtimeID)
	}
	return nil
}

func (s *EmergencyStopService) IsEmergencyLatched(runtimeID domain.RuntimeInstanceID) bool {
	if s.intent == nil {
		return false
	}
	return s.intent.IsEmergencyLatched(context.Background(), runtimeID)
}

func (s *EmergencyStopService) Execute(ctx context.Context, runtimeID domain.RuntimeInstanceID) (EmergencyStopResult, error) {
	return s.EmergencyStop(ctx, EmergencyStopRequest{
		RuntimeID: runtimeID,
		Actor:     EmergencyActorUser,
		Reason:    EmergencyReasonUserRequested,
	})
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
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return "emg-" + hex.EncodeToString(b[:])
}
