package control

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Clock func() time.Time

type ControlAuthorityManager struct {
	mu         sync.RWMutex
	states     map[domain.RuntimeInstanceID]*ControlAuthorityState
	perRuntime map[domain.RuntimeInstanceID]*sync.Mutex

	clock Clock
	audit AuthorityAuditSink
}

type ControlAuthorityManagerOptions struct {
	Clock Clock
	Audit AuthorityAuditSink
}

func NewControlAuthorityManager(opts ControlAuthorityManagerOptions) *ControlAuthorityManager {
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuthorityAuditSink{}
	}
	return &ControlAuthorityManager{
		states:     make(map[domain.RuntimeInstanceID]*ControlAuthorityState),
		perRuntime: make(map[domain.RuntimeInstanceID]*sync.Mutex),
		clock:      clock,
		audit:      audit,
	}
}

func (m *ControlAuthorityManager) getRuntimeLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.perRuntime[runtimeID]
	if !ok {
		lock = &sync.Mutex{}
		m.perRuntime[runtimeID] = lock
	}
	return lock
}

func (m *ControlAuthorityManager) Create(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) (ControlAuthoritySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return ControlAuthoritySnapshot{}, err
	}
	if runtimeID == "" {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}
	if pluginID == "" {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "plugin id must not be empty",
		}
	}

	lock := m.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	_, exists := m.states[runtimeID]
	m.mu.RUnlock()

	if exists {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrAlreadyExists,
			Message: "authority already exists for runtime: " + string(runtimeID),
		}
	}

	now := m.clock()
	state := NewControlAuthorityState(runtimeID, pluginID, now)

	m.mu.Lock()
	m.states[runtimeID] = state
	m.mu.Unlock()

	return state.Snapshot(), nil
}

func (m *ControlAuthorityManager) Get(ctx context.Context, runtimeID domain.RuntimeInstanceID) (ControlAuthoritySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return ControlAuthoritySnapshot{}, err
	}
	if runtimeID == "" {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}

	lock := m.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	state, ok := m.states[runtimeID]
	m.mu.RUnlock()

	if !ok {
		return ControlAuthoritySnapshot{}, errAuthorityNotFound(runtimeID)
	}

	return state.Snapshot(), nil
}

func (m *ControlAuthorityManager) Transition(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	request TransitionRequest,
) (ControlAuthoritySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return ControlAuthoritySnapshot{}, err
	}
	if runtimeID == "" {
		return ControlAuthoritySnapshot{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}
	if !IsValidControlMode(request.Target) {
		var zeroPlugin domain.PluginID
		var zeroMode domain.ControlMode
		m.recordDenied(runtimeID, zeroPlugin, zeroMode, zeroMode, 0, 0, request.Actor, request.Reason, "invalid target control mode")
		return ControlAuthoritySnapshot{}, errInvalidControlMode(request.Target)
	}

	lock := m.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	state, ok := m.states[runtimeID]
	m.mu.RUnlock()

	if !ok {
		var zeroPlugin domain.PluginID
		var zeroMode domain.ControlMode
		m.recordDenied(runtimeID, zeroPlugin, zeroMode, zeroMode, 0, 0, request.Actor, request.Reason, "runtime not found")
		return ControlAuthoritySnapshot{}, errAuthorityNotFound(runtimeID)
	}

	previousMode := state.Mode
	previousEpoch := state.Epoch

	if request.UseExpected && request.ExpectedEpoch != previousEpoch {
		m.recordDenied(runtimeID, state.PluginID, previousMode, previousMode, previousEpoch, previousEpoch, request.Actor, request.Reason, "stale epoch")
		return ControlAuthoritySnapshot{}, errStaleEpoch(runtimeID, request.ExpectedEpoch, previousEpoch)
	}

	if previousMode == request.Target {
		m.recordNoop(runtimeID, state.PluginID, previousMode, previousEpoch, request.Actor, request.Reason)
		return state.Snapshot(), nil
	}

	if !CanTransition(previousMode, request.Target) {
		m.recordDenied(runtimeID, state.PluginID, previousMode, previousMode, previousEpoch, previousEpoch, request.Actor, request.Reason, "invalid transition")
		return ControlAuthoritySnapshot{}, errInvalidTransition(previousMode, request.Target)
	}

	now := m.clock()
	newEpoch := previousEpoch + 1

	state.Mode = request.Target
	state.Epoch = newEpoch
	state.UpdatedAt = now
	state.LastTransitionReason = request.Reason
	state.LastTransitionActor = request.Actor

	m.recordSuccess(runtimeID, state.PluginID, previousMode, request.Target, previousEpoch, newEpoch, request.Actor, request.Reason)

	return state.Snapshot(), nil
}

func (m *ControlAuthorityManager) Remove(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtimeID == "" {
		return &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}

	lock := m.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	m.mu.Lock()
	delete(m.states, runtimeID)
	m.mu.Unlock()

	return nil
}

func (m *ControlAuthorityManager) List(ctx context.Context) ([]ControlAuthoritySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]ControlAuthoritySnapshot, 0, len(m.states))
	for _, state := range m.states {
		results = append(results, state.Snapshot())
	}

	sortSnapshotsByRuntimeID(results)
	return results, nil
}

func (m *ControlAuthorityManager) CleanupRuntimeLock(runtimeID domain.RuntimeInstanceID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.perRuntime, runtimeID)
}

func (m *ControlAuthorityManager) recordSuccess(
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	previousMode domain.ControlMode,
	newMode domain.ControlMode,
	previousEpoch uint64,
	newEpoch uint64,
	actor TransitionActor,
	reason TransitionReason,
) {
	m.audit.RecordTransition(AuthorityAuditEvent{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		PreviousMode:  previousMode,
		NewMode:       newMode,
		PreviousEpoch: previousEpoch,
		NewEpoch:      newEpoch,
		Actor:         actor,
		Reason:        reason,
		Result:        AuditResultSuccess,
		Timestamp:     m.clock(),
	})
}

func (m *ControlAuthorityManager) recordDenied(
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	previousMode domain.ControlMode,
	newMode domain.ControlMode,
	previousEpoch uint64,
	newEpoch uint64,
	actor TransitionActor,
	reason TransitionReason,
	errMsg string,
) {
	m.audit.RecordTransition(AuthorityAuditEvent{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		PreviousMode:  previousMode,
		NewMode:       newMode,
		PreviousEpoch: previousEpoch,
		NewEpoch:      newEpoch,
		Actor:         actor,
		Reason:        reason,
		Result:        AuditResultDenied,
		Error:         errMsg,
		Timestamp:     m.clock(),
	})
}

func (m *ControlAuthorityManager) recordNoop(
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	mode domain.ControlMode,
	epoch uint64,
	actor TransitionActor,
	reason TransitionReason,
) {
	m.audit.RecordTransition(AuthorityAuditEvent{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		PreviousMode:  mode,
		NewMode:       mode,
		PreviousEpoch: epoch,
		NewEpoch:      epoch,
		Actor:         actor,
		Reason:        reason,
		Result:        AuditResultNoop,
		Timestamp:     m.clock(),
	})
}

func sortSnapshotsByRuntimeID(snapshots []ControlAuthoritySnapshot) {
	quickSortSnapshots(snapshots, 0, len(snapshots)-1)
}

func quickSortSnapshots(snapshots []ControlAuthoritySnapshot, lo, hi int) {
	if lo >= hi || lo < 0 {
		return
	}
	p := partitionSnapshots(snapshots, lo, hi)
	quickSortSnapshots(snapshots, lo, p-1)
	quickSortSnapshots(snapshots, p+1, hi)
}

func partitionSnapshots(snapshots []ControlAuthoritySnapshot, lo, hi int) int {
	pivot := snapshots[hi].RuntimeID
	i := lo
	for j := lo; j < hi; j++ {
		if snapshots[j].RuntimeID < pivot {
			snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			i++
		}
	}
	snapshots[i], snapshots[hi] = snapshots[hi], snapshots[i]
	return i
}
