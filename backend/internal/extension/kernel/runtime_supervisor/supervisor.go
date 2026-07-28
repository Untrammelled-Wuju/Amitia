package runtime_supervisor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var (
	ErrFactoryExists        = errors.New("runtime_supervisor: factory already registered")
	ErrFactoryNotFound      = errors.New("runtime_supervisor: factory not found for type")
	ErrInstanceNotFound     = errors.New("runtime_supervisor: instance not found")
	ErrInstanceExists       = errors.New("runtime_supervisor: instance already exists")
	ErrInvalidSpec          = errors.New("runtime_supervisor: invalid spec")
	ErrCircuitOpen          = errors.New("runtime_supervisor: circuit open")
	ErrQueueFull            = errors.New("runtime_supervisor: queue full")
	ErrGenerationMismatch   = errors.New("runtime_supervisor: generation mismatch")
	ErrDependencyMissing    = errors.New("runtime_supervisor: dependency snapshot missing")
	ErrMaxRestartsExceeded  = errors.New("runtime_supervisor: max restarts exceeded")
	ErrInstanceDraining     = errors.New("runtime_supervisor: instance draining")
)

type DefaultSupervisor struct {
	mu            sync.RWMutex
	factories     map[domain.RuntimeType]RuntimeFactory
	instances     map[string]*instanceEntry
	byDefinition  map[DefinitionID][]string
	circuitConfig CircuitConfig
}

type CircuitConfig struct {
	FailureThreshold int
	RecoveryAfter    time.Duration
	HalfOpenAttempts int
}

func NewDefaultSupervisor() *DefaultSupervisor {
	return &DefaultSupervisor{
		factories:     make(map[domain.RuntimeType]RuntimeFactory),
		instances:     make(map[string]*instanceEntry),
		byDefinition:  make(map[DefinitionID][]string),
		circuitConfig: CircuitConfig{
			FailureThreshold: 5,
			RecoveryAfter:    30 * time.Second,
			HalfOpenAttempts: 1,
		},
	}
}

func (s *DefaultSupervisor) SetCircuitConfig(config CircuitConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.circuitConfig = config
}

func (s *DefaultSupervisor) RegisterFactory(factory RuntimeFactory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := factory.Type()
	if _, exists := s.factories[t]; exists {
		return fmt.Errorf("%w: %s", ErrFactoryExists, t)
	}
	s.factories[t] = factory
	return nil
}

func (s *DefaultSupervisor) Reconcile(ctx context.Context, request ReconcileRequest) ReconcileResult {
	s.mu.Lock()
	factory, ok := s.factories[request.Spec.RuntimeType]
	s.mu.Unlock()
	if !ok {
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        fmt.Errorf("%w: %s", ErrFactoryNotFound, request.Spec.RuntimeType),
		}
	}
	if err := factory.Validate(request.Spec); err != nil {
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        fmt.Errorf("%w: %v", ErrInvalidSpec, err),
		}
	}
	instanceID := s.findInstance(request.DefinitionID, request.Spec.Strategy, request.Spec.Generation)
	switch request.Desired {
	case DesiredRunning, DesiredConnected:
		if instanceID == "" {
			return s.startInstance(ctx, factory, request)
		}
		return s.verifyRunning(ctx, instanceID, request)
	case DesiredStopped, DesiredDisconnected:
		if instanceID == "" {
			return ReconcileResult{
				DefinitionID: request.DefinitionID,
				Desired:      request.Desired,
				Actual:       ActualStopped,
			}
		}
		return s.stopReconcile(ctx, instanceID, request)
	case DesiredPaused:
		if instanceID == "" {
			return ReconcileResult{
				DefinitionID: request.DefinitionID,
				Desired:      request.Desired,
				Actual:       ActualStopped,
				Error:        ErrInstanceNotFound,
			}
		}
		return s.pauseInstance(ctx, instanceID, request)
	}
	return ReconcileResult{
		DefinitionID: request.DefinitionID,
		Desired:      request.Desired,
		Actual:       ActualFailed,
		Error:        fmt.Errorf("runtime_supervisor: unknown desired state %s", request.Desired),
	}
}

func (s *DefaultSupervisor) startInstance(ctx context.Context, factory RuntimeFactory, request ReconcileRequest) ReconcileResult {
	managed, err := factory.Create(ctx, request.Spec)
	if err != nil {
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        err,
		}
	}
	if err := managed.Start(ctx); err != nil {
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        err,
		}
	}
	instanceID := newInstanceID(request.Spec)
	now := time.Now().UTC()
	identity := RuntimeIdentity{
		InstanceID:         instanceID,
		RuntimeDefinitionID: request.DefinitionID,
		ExtensionID:        request.Spec.ExtensionID,
		ModuleID:           request.Spec.ModuleID,
		RuntimeType:        request.Spec.RuntimeType,
		Generation:         request.Spec.Generation,
		SessionNonce:       uuid.NewString(),
	}
	entry := &instanceEntry{
		identity:   identity,
		runtime:    managed,
		desired:    request.Desired,
		actual:     ActualReady,
		health:     HealthHealthy,
		circuit:    CircuitClosed,
		startedAt:  &now,
		limits:     request.Spec.Limits,
		definition: request.DefinitionID,
		generation: request.Spec.Generation,
	}
	s.mu.Lock()
	s.instances[instanceID] = entry
	s.byDefinition[request.DefinitionID] = append(s.byDefinition[request.DefinitionID], instanceID)
	s.mu.Unlock()
	return ReconcileResult{
		InstanceID:  instanceID,
		DefinitionID: request.DefinitionID,
		Desired:     request.Desired,
		Actual:      ActualReady,
		Health:      HealthHealthy,
		Circuit:     CircuitClosed,
		Action:      "started",
	}
}

func (s *DefaultSupervisor) verifyRunning(ctx context.Context, instanceID string, request ReconcileRequest) ReconcileResult {
	s.mu.RLock()
	entry, ok := s.instances[instanceID]
	s.mu.RUnlock()
	if !ok {
		return s.startInstance(ctx, nil, request)
	}
	if entry.generation != request.Spec.Generation {
		if err := s.stopLocked(ctx, instanceID, StopReasonUpdate); err != nil {
			return ReconcileResult{
				InstanceID:  instanceID,
				DefinitionID: request.DefinitionID,
				Desired:     request.Desired,
				Actual:      entry.actual,
				Error:       err,
			}
		}
		return s.startInstance(ctx, mustFactory(s, request.Spec.RuntimeType), request)
	}
	if entry.actual == ActualCrashed || entry.actual == ActualFailed {
		return s.restartInstance(ctx, instanceID, request)
	}
	return ReconcileResult{
		InstanceID:  instanceID,
		DefinitionID: request.DefinitionID,
		Desired:     request.Desired,
		Actual:      entry.actual,
		Health:      entry.health,
		Circuit:     entry.circuit,
		Action:      "noop",
	}
}

func (s *DefaultSupervisor) stopReconcile(ctx context.Context, instanceID string, request ReconcileRequest) ReconcileResult {
	if err := s.stopLocked(ctx, instanceID, StopReasonManual); err != nil {
		return ReconcileResult{
			InstanceID:  instanceID,
			DefinitionID: request.DefinitionID,
			Desired:     request.Desired,
			Actual:      ActualFailed,
			Error:       err,
		}
	}
	return ReconcileResult{
		InstanceID:  instanceID,
		DefinitionID: request.DefinitionID,
		Desired:     request.Desired,
		Actual:      ActualStopped,
		Action:      "stopped",
	}
}

func (s *DefaultSupervisor) pauseInstance(ctx context.Context, instanceID string, request ReconcileRequest) ReconcileResult {
	s.mu.Lock()
	entry, ok := s.instances[instanceID]
	if !ok {
		s.mu.Unlock()
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        ErrInstanceNotFound,
		}
	}
	entry.desired = DesiredPaused
	entry.actual = ActualDegraded
	s.mu.Unlock()
	return ReconcileResult{
		InstanceID:  instanceID,
		DefinitionID: request.DefinitionID,
		Desired:     DesiredPaused,
		Actual:      ActualDegraded,
		Health:      HealthDegraded,
		Action:      "paused",
	}
}

func (s *DefaultSupervisor) restartInstance(ctx context.Context, instanceID string, request ReconcileRequest) ReconcileResult {
	s.mu.Lock()
	entry, ok := s.instances[instanceID]
	if !ok {
		s.mu.Unlock()
		return ReconcileResult{
			DefinitionID: request.DefinitionID,
			Desired:      request.Desired,
			Actual:       ActualFailed,
			Error:        ErrInstanceNotFound,
		}
	}
	if entry.restarts >= request.Spec.MaxRestarts && request.Spec.MaxRestarts > 0 {
		entry.actual = ActualQuarantined
		entry.health = HealthUnhealthy
		s.mu.Unlock()
		return ReconcileResult{
			InstanceID:  instanceID,
			DefinitionID: request.DefinitionID,
			Desired:     request.Desired,
			Actual:      ActualQuarantined,
			Health:      HealthUnhealthy,
			Error:       ErrMaxRestartsExceeded,
		}
	}
	entry.restarts++
	runtime := entry.runtime
	s.mu.Unlock()
	if runtime != nil {
		_ = runtime.Stop(ctx, StopReasonCrash)
	}
	if err := runtime.Start(ctx); err != nil {
		s.mu.Lock()
		entry.actual = ActualCrashed
		entry.health = HealthUnhealthy
		entry.consecFails++
		s.evaluateCircuitLocked(entry)
		s.mu.Unlock()
		return ReconcileResult{
			InstanceID:  instanceID,
			DefinitionID: request.DefinitionID,
			Desired:     request.Desired,
			Actual:      ActualCrashed,
			Health:      HealthUnhealthy,
			Error:       err,
		}
	}
	s.mu.Lock()
	entry.actual = ActualReady
	entry.health = HealthHealthy
	entry.consecFails = 0
	entry.circuit = CircuitClosed
	now := time.Now().UTC()
	entry.startedAt = &now
	s.mu.Unlock()
	return ReconcileResult{
		InstanceID:  instanceID,
		DefinitionID: request.DefinitionID,
		Desired:     request.Desired,
		Actual:      ActualReady,
		Health:      HealthHealthy,
		Circuit:     CircuitClosed,
		Action:      "restarted",
	}
}

func (s *DefaultSupervisor) Invoke(ctx context.Context, request InvocationRequest) InvocationResult {
	s.mu.RLock()
	entry, ok := s.instances[request.InstanceID]
	s.mu.RUnlock()
	if !ok {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        ErrInstanceNotFound,
		}
	}
	if entry.circuit == CircuitOpen {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "rejected",
			Error:        ErrCircuitOpen,
		}
	}
	if entry.actual == ActualDraining {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "rejected",
			Error:        ErrInstanceDraining,
		}
	}
	if entry.generation != request.Generation && request.Generation != 0 {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("%w: instance=%d request=%d", ErrGenerationMismatch, entry.generation, request.Generation),
		}
	}
	if entry.actual != ActualReady && entry.actual != ActualDegraded {
		return InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("runtime_supervisor: instance not ready (state=%s)", entry.actual),
		}
	}
	s.mu.Lock()
	entry.activeCalls++
	s.mu.Unlock()
	result := entry.runtime.Invoke(ctx, request)
	s.mu.Lock()
	entry.activeCalls--
	if entry.activeCalls < 0 {
		entry.activeCalls = 0
	}
	if result.Error != nil {
		entry.consecFails++
		s.evaluateCircuitLocked(entry)
	} else {
		entry.consecFails = 0
		entry.circuit = CircuitClosed
	}
	s.mu.Unlock()
	return result
}

func (s *DefaultSupervisor) Stop(ctx context.Context, instanceID string, reason StopReason) error {
	return s.stopLocked(ctx, instanceID, reason)
}

func (s *DefaultSupervisor) Drain(ctx context.Context, instanceID string, timeout time.Duration) error {
	s.mu.Lock()
	entry, ok := s.instances[instanceID]
	if !ok {
		s.mu.Unlock()
		return ErrInstanceNotFound
	}
	entry.actual = ActualDraining
	s.mu.Unlock()

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.RLock()
		active := entry.activeCalls
		s.mu.RUnlock()
		if active <= 0 {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return s.stopLocked(ctx, instanceID, StopReasonDrain)
}

func (s *DefaultSupervisor) stopLocked(ctx context.Context, instanceID string, reason StopReason) error {
	s.mu.Lock()
	entry, ok := s.instances[instanceID]
	if !ok {
		s.mu.Unlock()
		return ErrInstanceNotFound
	}
	runtime := entry.runtime
	s.mu.Unlock()
	if runtime != nil {
		_ = runtime.Stop(ctx, reason)
	}
	s.mu.Lock()
	now := time.Now().UTC()
	entry.stoppedAt = &now
	entry.actual = ActualStopped
	entry.desired = DesiredStopped
	entry.health = HealthUnknown
	s.mu.Unlock()
	return nil
}

func (s *DefaultSupervisor) Restart(ctx context.Context, instanceID string) error {
	s.mu.RLock()
	entry, ok := s.instances[instanceID]
	if !ok {
		s.mu.RUnlock()
		return ErrInstanceNotFound
	}
	spec := InstanceSpec{
		RuntimeType: entry.identity.RuntimeType,
		ExtensionID: entry.identity.ExtensionID,
		ModuleID:    entry.identity.ModuleID,
		Generation:  entry.generation,
		Strategy:    StrategySingletonPerModule,
		Limits:      entry.limits,
	}
	defID := entry.definition
	s.mu.RUnlock()
	result := s.restartInstance(ctx, instanceID, ReconcileRequest{
		DefinitionID: defID,
		Desired:      DesiredRunning,
		Spec:         spec,
	})
	return result.Error
}

func (s *DefaultSupervisor) Snapshot(_ context.Context, defID DefinitionID) StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var instances []InstanceSnapshot
	for _, id := range s.byDefinition[defID] {
		entry, ok := s.instances[id]
		if !ok {
			continue
		}
		instances = append(instances, InstanceSnapshot{
			InstanceID: entry.identity.InstanceID,
			Identity:   entry.identity,
			Desired:    entry.desired,
			Actual:     entry.actual,
			Health:     entry.health,
			Circuit:    entry.circuit,
			StartedAt:  entry.startedAt,
			StoppedAt:  entry.stoppedAt,
			Restarts:   entry.restarts,
			Limits:     entry.limits,
		})
	}
	return StateSnapshot{
		DefinitionID: defID,
		Instances:    instances,
		Generation:   s.maxGeneration(defID),
		CapturedAt:   time.Now().UTC(),
	}
}

func (s *DefaultSupervisor) GetInstance(_ context.Context, instanceID string) (InstanceSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.instances[instanceID]
	if !ok {
		return InstanceSnapshot{}, ErrInstanceNotFound
	}
	return InstanceSnapshot{
		InstanceID: entry.identity.InstanceID,
		Identity:   entry.identity,
		Desired:    entry.desired,
		Actual:     entry.actual,
		Health:     entry.health,
		Circuit:    entry.circuit,
		StartedAt:  entry.startedAt,
		StoppedAt:  entry.stoppedAt,
		Restarts:   entry.restarts,
		Limits:     entry.limits,
	}, nil
}

func (s *DefaultSupervisor) findInstance(defID DefinitionID, strategy InstanceStrategy, generation int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byDefinition[defID]
	if len(ids) == 0 {
		return ""
	}
	switch strategy {
	case StrategySingletonPerModule, StrategySingletonPerExtension, StrategySingletonGlobal:
		for _, id := range ids {
			if entry, ok := s.instances[id]; ok && entry.generation == generation && entry.actual != ActualStopped {
				return id
			}
		}
		for _, id := range ids {
			if entry, ok := s.instances[id]; ok && entry.generation == generation && entry.actual == ActualCrashed {
				return id
			}
		}
	default:
		for _, id := range ids {
			if entry, ok := s.instances[id]; ok && entry.actual != ActualStopped {
				return id
			}
		}
	}
	return ""
}

func (s *DefaultSupervisor) evaluateCircuitLocked(entry *instanceEntry) {
	cfg := s.circuitConfig
	if entry.consecFails >= cfg.FailureThreshold {
		entry.circuit = CircuitOpen
	}
}

func (s *DefaultSupervisor) maxGeneration(defID DefinitionID) int64 {
	var max int64
	for _, id := range s.byDefinition[defID] {
		entry, ok := s.instances[id]
		if !ok {
			continue
		}
		if entry.generation > max {
			max = entry.generation
		}
	}
	return max
}

func newInstanceID(spec InstanceSpec) string {
	return fmt.Sprintf("rt-%s-%s", spec.DefinitionID, uuid.NewString())
}

func mustFactory(s *DefaultSupervisor, t domain.RuntimeType) RuntimeFactory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.factories[t]
}

var _ Supervisor = (*DefaultSupervisor)(nil)
