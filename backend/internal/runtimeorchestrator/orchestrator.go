package runtimeorchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/platform"
)

type RuntimeOrchestrator struct {
	mu         sync.RWMutex
	descriptor platform.RuntimeDescriptor
	profile    runtimeprofile.Profile
	components map[ComponentID]ManagedComponent
	statuses   map[ComponentID]*componentRuntimeState
	startOrder []ComponentID
	state      OrchestratorState
	stopOnce   sync.Once
	stopErr    error
	nowFn      func() time.Time
}

type componentRuntimeState struct {
	status   ComponentStatus
	started  bool
	stopped  bool
	startErr error
}

type RestartableComponent interface {
	Restart(ctx context.Context) error
}

type ShutdownAwareComponent interface {
	PrepareShutdown(ctx context.Context) error
}

func New(descriptor platform.RuntimeDescriptor) *RuntimeOrchestrator {
	return NewWithProfile(descriptor, runtimeprofile.ProfileLocal)
}

func NewWithProfile(descriptor platform.RuntimeDescriptor, profile runtimeprofile.Profile) *RuntimeOrchestrator {
	return &RuntimeOrchestrator{
		descriptor: descriptor,
		profile:    profile,
		components: make(map[ComponentID]ManagedComponent),
		statuses:   make(map[ComponentID]*componentRuntimeState),
		state:      OrchestratorCreated,
		nowFn:      func() time.Time { return time.Now().UTC() },
	}
}

func (o *RuntimeOrchestrator) RuntimeProfile() runtimeprofile.Profile {
	if o == nil {
		return ""
	}
	return o.profile
}

func descriptorEnabledForProfile(desc ComponentDescriptor, profile runtimeprofile.Profile) bool {
	return desc.Enabled && desc.SupportsProfile(profile)
}

func (o *RuntimeOrchestrator) Register(component ManagedComponent) error {
	if component == nil {
		return invalidDescriptorErr("nil component")
	}
	desc := component.Descriptor()

	o.mu.Lock()
	defer o.mu.Unlock()

	if err := validateDescriptor(desc, o.profile); err != nil {
		return err
	}

	if o.state == OrchestratorStopping || o.state == OrchestratorStopped {
		return newErrOrchestratorStopped()
	}
	if _, exists := o.components[desc.ID]; exists {
		return duplicateComponentErr(desc.ID)
	}

	enabled := descriptorEnabledForProfile(desc, o.profile)

	o.components[desc.ID] = component
	o.statuses[desc.ID] = &componentRuntimeState{
		status: ComponentStatus{
			ID:           desc.ID,
			Phase:        desc.Phase,
			Enabled:      enabled,
			Required:     desc.Required,
			Capabilities: cloneCapabilities(desc.Capabilities),
			Dependencies: cloneDependencies(desc.Dependencies),
			State:        StateRegistered,
		},
	}
	if !enabled {
		o.statuses[desc.ID].status.State = StateDisabled
	}
	return nil
}

func (o *RuntimeOrchestrator) StartPhase(ctx context.Context, phase ComponentPhase) error {
	if phase != PhaseInfrastructure && phase != PhaseApplication {
		return phaseOrderErr(phase)
	}

	o.mu.Lock()
	if o.state == OrchestratorStopping || o.state == OrchestratorStopped {
		o.mu.Unlock()
		return newErrOrchestratorStopped()
	}
	if o.state == OrchestratorBlocked {
		o.mu.Unlock()
		return wrapComponentErr(ErrPhaseOrder, "", "", "orchestrator is blocked")
	}
	if phase == PhaseApplication && o.state == OrchestratorCreated {
		o.mu.Unlock()
		return phaseOrderErr(phase)
	}

	o.state = OrchestratorStarting

	var batch []ComponentID
	for id, comp := range o.components {
		desc := comp.Descriptor()
		if desc.Phase != phase {
			continue
		}
		st := o.statuses[id]
		if !st.status.Enabled || st.status.State == StateReady {
			continue
		}
		batch = append(batch, id)
	}

	sorted, err := o.topologicalSort(batch)
	if err != nil {
		o.mu.Unlock()
		return err
	}

	if err := o.verifyDependencies(batch); err != nil {
		o.mu.Unlock()
		return err
	}

	runtime := newPhaseRuntime(o, phase, sorted)
	o.mu.Unlock()

	return runtime.execute(ctx)
}

func (o *RuntimeOrchestrator) doStopAll(ctx context.Context) error {
	o.mu.Lock()
	o.state = OrchestratorStopping
	order := make([]ComponentID, len(o.startOrder))
	copy(order, o.startOrder)
	o.mu.Unlock()

	var errs []error
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		o.mu.RLock()
		comp, ok := o.components[id]
		st := o.statuses[id]
		o.mu.RUnlock()
		if !ok || st == nil {
			continue
		}
		if !st.started || st.stopped {
			continue
		}
		state := st.status.State
		if state != StateReady && state != StateDegraded && state != StateStarting {
			continue
		}

		o.mu.Lock()
		st.status.State = StateStopping
		o.mu.Unlock()

		stopErr := comp.Stop(ctx)
		o.mu.Lock()
		st.stopped = true
		st.status.State = StateStopped
		st.status.StoppedAt = o.nowFn()
		if stopErr != nil {
			st.status.LastError = stopErr.Error()
			errs = append(errs, stopErr)
		}
		o.mu.Unlock()
	}

	o.mu.Lock()
	o.state = OrchestratorStopped
	o.mu.Unlock()

	return joinErrors(errs)
}

func (o *RuntimeOrchestrator) ReportComponentState(id ComponentID, state ComponentState, err error) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	st, ok := o.statuses[id]
	if !ok {
		return unknownComponentErr(id)
	}

	st.status.State = state
	if err != nil {
		st.status.LastError = err.Error()
	}

	o.updateOverallLocked()
	return nil
}

func (o *RuntimeOrchestrator) RestartComponent(ctx context.Context, id ComponentID) error {
	o.mu.Lock()
	if o.state == OrchestratorStopping || o.state == OrchestratorStopped || o.state == OrchestratorBlocked {
		o.mu.Unlock()
		return newErrOrchestratorStopped()
	}

	comp, ok := o.components[id]
	st, stOk := o.statuses[id]
	if !ok || !stOk {
		o.mu.Unlock()
		return unknownComponentErr(id)
	}

	desc := comp.Descriptor()
	if !desc.Enabled {
		o.mu.Unlock()
		return wrapComponentErr(ErrComponentDisabled, id, "", "component is disabled")
	}

	if !o.checkDepsReady(id) {
		o.mu.Unlock()
		return wrapComponentErr(ErrDependencyNotReady, id, "", "dependencies not ready")
	}

	restartable, isRestartable := comp.(RestartableComponent)
	o.mu.Unlock()

	if isRestartable {
		if err := restartable.Restart(ctx); err != nil {
			o.mu.Lock()
			st.status.State = StateDegraded
			st.status.LastError = err.Error()
			o.updateOverallLocked()
			o.mu.Unlock()
			return wrapComponentErr(ErrRestartFailed, id, "", err.Error())
		}
	} else {
		stopErr := comp.Stop(ctx)
		if stopErr != nil {
			o.mu.Lock()
			st.status.State = StateDegraded
			o.mu.Unlock()
			return stopErr
		}

		startErr := comp.Start(ctx)
		if startErr != nil {
			o.mu.Lock()
			if desc.Required {
				st.status.State = StateFailed
				o.state = OrchestratorBlocked
			} else {
				st.status.State = StateDegraded
			}
			st.status.LastError = startErr.Error()
			o.updateOverallLocked()
			o.mu.Unlock()
			return startErr
		}

		readyErr := comp.Ready(ctx)
		if readyErr != nil {
			o.mu.Lock()
			st.status.State = StateDegraded
			st.status.LastError = readyErr.Error()
			o.updateOverallLocked()
			o.mu.Unlock()
			return readyErr
		}
	}

	o.mu.Lock()
	st.status.State = StateReady
	st.started = true
	st.status.ReadyAt = o.nowFn()
	o.updateOverallLocked()
	o.mu.Unlock()

	return nil
}

func (o *RuntimeOrchestrator) StopAll(ctx context.Context) (err error) {
	o.mu.Lock()
	var shutdownAware []ShutdownAwareComponent
	for id, comp := range o.components {
		if sc, ok := comp.(ShutdownAwareComponent); ok {
			st := o.statuses[id]
			if st != nil && st.started && !st.stopped {
				shutdownAware = append(shutdownAware, sc)
			}
		}
	}
	o.mu.Unlock()

	for _, sc := range shutdownAware {
		_ = sc.PrepareShutdown(ctx)
	}

	o.stopOnce.Do(func() {
		o.stopErr = o.doStopAll(ctx)
	})
	return o.stopErr
}

func (o *RuntimeOrchestrator) Snapshot() RuntimeSnapshot {
	o.mu.RLock()
	defer o.mu.RUnlock()

	components := make(map[ComponentID]ComponentStatus)
	var readyCount, disabledCount, degradedCount, failedCount, blockingCount int
	for id, st := range o.statuses {
		cpy := st.status.clone()
		switch cpy.State {
		case StateReady:
			readyCount++
		case StateDisabled:
			disabledCount++
		case StateDegraded:
			degradedCount++
			if cpy.Required {
				blockingCount++
			}
		case StateFailed:
			failedCount++
			if cpy.Required {
				blockingCount++
			}
		}
		components[id] = cpy
	}

	return RuntimeSnapshot{
		State:         o.state,
		Runtime:       o.descriptor,
		Profile:       o.profile,
		Components:    components,
		ReadyCount:    readyCount,
		DisabledCount: disabledCount,
		DegradedCount: degradedCount,
		FailedCount:   failedCount,
		BlockingCount: blockingCount,
		Timestamp:     o.nowFn(),
	}
}

func (o *RuntimeOrchestrator) verifyDependencies(batch []ComponentID) error {
	registered := make(map[ComponentID]bool)
	for id := range o.components {
		registered[id] = true
	}
	for _, id := range batch {
		desc := o.components[id].Descriptor()
		for _, dep := range desc.Dependencies {
			if !registered[dep] {
				return unknownDependencyErr(dep)
			}
			depDesc := o.components[dep].Descriptor()
			if depDesc.Phase != PhaseInfrastructure && depDesc.Phase == desc.Phase {
				continue
			}
			if depDesc.Phase == PhaseApplication && desc.Phase == PhaseInfrastructure {
				return wrapComponentErr(ErrDependencyCycle, id, "", string(dep))
			}
		}
	}
	return nil
}

func (o *RuntimeOrchestrator) topologicalSort(batch []ComponentID) ([]ComponentID, error) {
	inBatch := make(map[ComponentID]bool)
	for _, id := range batch {
		inBatch[id] = true
	}

	visited := make(map[ComponentID]int)
	result := make([]ComponentID, 0, len(batch))

	var visit func(id ComponentID) error
	visit = func(id ComponentID) error {
		switch visited[id] {
		case 2:
			return nil
		case 1:
			return dependencyCycleErr(id, id)
		}
		visited[id] = 1
		desc := o.components[id].Descriptor()
		for _, dep := range desc.Dependencies {
			if !inBatch[dep] {
				continue
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[id] = 2
		result = append(result, id)
		return nil
	}

	for _, id := range batch {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return result, nil
}

type phaseRuntime struct {
	owner *RuntimeOrchestrator
	phase ComponentPhase
	batch []ComponentID
}

func newPhaseRuntime(o *RuntimeOrchestrator, phase ComponentPhase, batch []ComponentID) *phaseRuntime {
	return &phaseRuntime{owner: o, phase: phase, batch: batch}
}

func (r *phaseRuntime) execute(ctx context.Context) error {
	sorted := r.batch
	o := r.owner

	for _, id := range sorted {
		if err := ctx.Err(); err != nil {
			return err
		}

		o.mu.RLock()
		comp := o.components[id]
		st := o.statuses[id]
		desc := comp.Descriptor()
		o.mu.RUnlock()

		if !o.checkDepsReady(id) {
			o.mu.Lock()
			if desc.Required {
				st.status.State = StateFailed
				st.status.LastError = "dependency not ready"
				o.state = OrchestratorBlocked
			} else {
				st.status.State = StateDegraded
				st.status.LastError = "dependency not ready"
			}
			o.mu.Unlock()

			if desc.Required {
				o.rollback(id)
				return requiredComponentFailedErr(id, r.phase, "dependency not ready")
			}
			continue
		}

		o.mu.Lock()
		st.status.State = StateStarting
		st.status.StartedAt = o.nowFn()
		o.mu.Unlock()

		if err := comp.Start(ctx); err != nil {
			o.mu.Lock()
			if desc.Required {
				st.status.State = StateFailed
				st.status.LastError = err.Error()
				o.state = OrchestratorBlocked
			} else {
				st.status.State = StateDegraded
				st.status.LastError = err.Error()
				o.updateOverallLocked()
			}
			o.mu.Unlock()

			if desc.Required {
				o.rollback(id)
				return requiredComponentFailedErr(id, r.phase, err.Error())
			}
			continue
		}

		if err := comp.Ready(ctx); err != nil {
			o.mu.Lock()
			if desc.Required {
				st.status.State = StateFailed
				st.status.LastError = err.Error()
				o.state = OrchestratorBlocked
			} else {
				st.status.State = StateDegraded
				st.status.LastError = err.Error()
				o.updateOverallLocked()
			}
			o.mu.Unlock()

			if desc.Required {
				o.rollback(id)
				return requiredComponentFailedErr(id, r.phase, err.Error())
			}
			continue
		}

		o.mu.Lock()
		st.started = true
		st.status.State = StateReady
		st.status.ReadyAt = o.nowFn()
		st.status.ProviderID = string(desc.ID)
		o.startOrder = append(o.startOrder, id)
		o.updateOverallLocked()
		o.mu.Unlock()
	}

	o.mu.Lock()
	o.updateOverallLocked()
	isFinalPhase := true
	for _, comp := range o.components {
		d := comp.Descriptor()
		if d.Phase == PhaseApplication && d.Enabled {
			isFinalPhase = false
			break
		}
	}
	_ = isFinalPhase
	o.mu.Unlock()

	return nil
}

func (o *RuntimeOrchestrator) checkDepsReady(id ComponentID) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	desc := o.components[id].Descriptor()
	for _, dep := range desc.Dependencies {
		st, ok := o.statuses[dep]
		if !ok {
			return false
		}
		if st.status.State != StateReady {
			return false
		}
	}
	return true
}

func (o *RuntimeOrchestrator) rollback(failedID ComponentID) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := len(o.startOrder) - 1; i >= 0; i-- {
		startedID := o.startOrder[i]
		if startedID == failedID {
			continue
		}
		comp := o.components[startedID]
		st := o.statuses[startedID]
		if !st.started || st.stopped {
			continue
		}
		st.status.State = StateStopping
		o.mu.Unlock()
		_ = comp.Stop(context.Background())
		o.mu.Lock()
		st.stopped = true
		st.status.State = StateStopped
		st.status.StoppedAt = o.nowFn()
	}
}

func (o *RuntimeOrchestrator) updateOverallLocked() {
	if o.state == OrchestratorBlocked || o.state == OrchestratorStopping || o.state == OrchestratorStopped {
		return
	}
	hasReady, hasDegraded, hasStarting := false, false, false
	for _, st := range o.statuses {
		switch st.status.State {
		case StateReady:
			hasReady = true
		case StateDegraded:
			hasDegraded = true
		case StateStarting:
			hasStarting = true
		}
	}
	if hasDegraded {
		o.state = OrchestratorDegraded
	} else if hasReady && !hasStarting {
		o.state = OrchestratorReady
	} else {
		o.state = OrchestratorStarting
	}
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return &multiError{errs: errs, msgs: msgs}
}

type multiError struct {
	errs []error
	msgs []string
}

func (m *multiError) Error() string {
	out := ""
	for i, msg := range m.msgs {
		if i > 0 {
			out += "; "
		}
		out += msg
	}
	return out
}

func (m *multiError) Unwrap() error {
	if len(m.errs) > 0 {
		return m.errs[0]
	}
	return nil
}
