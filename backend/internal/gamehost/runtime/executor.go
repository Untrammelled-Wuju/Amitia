package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeExecutor interface {
	StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	RestartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID, reason string) error
	StartServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error
	StopServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error
	CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	SetResolveDefinition(fn DefinitionResolverFunc)
}

type RuntimeInstanceRef struct {
	ID       domain.RuntimeInstanceID
	PluginID domain.PluginID
	State    domain.RuntimeState
}

type RuntimeManager interface {
	GetRuntime(runtimeID domain.RuntimeInstanceID) (*RuntimeInstanceRef, error)
	UpdateRuntimeState(runtimeID domain.RuntimeInstanceID, next domain.RuntimeState, reason string, now time.Time) error
	ListRuntimes() []*RuntimeInstanceRef
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
	AllocateGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
	GetLifecycleIntent(runtimeID domain.RuntimeInstanceID) (string, error)
	SetLifecycleIntent(runtimeID domain.RuntimeInstanceID, intent string) error
}

type RuntimeTopologyStore interface {
	GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (RuntimeTopologySnapshot, error)
	GetDependencyGraphSnapshot(runtimeID domain.RuntimeInstanceID) (DependencyGraphSnapshot, error)
	GetTopology(runtimeID domain.RuntimeInstanceID) (TopologyAccessor, error)
}

type runtimeExecutor struct {
	mu                sync.Mutex
	runtimeLocks      map[domain.RuntimeInstanceID]*sync.Mutex
	topology          RuntimeTopologyStore
	runtimeManager    RuntimeManager
	planner           *LifecyclePlanner
	executor          ServiceExecutor
	handleStore       *RuntimeHandleStore
	rollbackExecutor  RollbackExecutor
	resolveDefinition DefinitionResolverFunc
}

func NewRuntimeExecutor(
	topologyStore RuntimeTopologyStore,
	runtimeManager RuntimeManager,
	executor ServiceExecutor,
	planner *LifecyclePlanner,
) (RuntimeExecutor, error) {
	if topologyStore == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "topology store must not be nil"}
	}
	if runtimeManager == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "runtime manager must not be nil"}
	}
	if executor == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "service executor must not be nil"}
	}
	if planner == nil {
		planner = NewLifecyclePlanner()
	}
	return &runtimeExecutor{
		topology:         topologyStore,
		runtimeManager:   runtimeManager,
		planner:          planner,
		executor:         executor,
		handleStore:      NewRuntimeHandleStore(),
		rollbackExecutor: NewRollbackExecutor(),
		runtimeLocks:     make(map[domain.RuntimeInstanceID]*sync.Mutex),
	}, nil
}

func (e *runtimeExecutor) SetResolveDefinition(fn DefinitionResolverFunc) {
	e.resolveDefinition = fn
}

func (e *runtimeExecutor) getRuntimeLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	if lock, ok := e.runtimeLocks[runtimeID]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	e.runtimeLocks[runtimeID] = lock
	return lock
}

func (e *runtimeExecutor) StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	lock := e.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()
	return e.startRuntimeLocked(ctx, runtimeID)
}

func (e *runtimeExecutor) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	lock := e.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()
	return e.stopRuntimeLocked(ctx, runtimeID)
}

func (e *runtimeExecutor) RestartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID, reason string) error {
	lock := e.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now()

	runtimeInfo, err := e.runtimeManager.GetRuntime(runtimeID)
	if err != nil {
		return err
	}

	if runtimeInfo.State != domain.RuntimeStateRunning && runtimeInfo.State != domain.RuntimeStateDegraded {
		return &RuntimeRestartError{
			Code:      ErrLifecycleConflict,
			RuntimeID: string(runtimeID),
			Cause:     &ExecutionError{Code: ErrInvalidState, RuntimeID: string(runtimeID), Message: "runtime not in restartable state: " + string(runtimeInfo.State)},
		}
	}

	if err := e.runtimeManager.SetLifecycleIntent(runtimeID, "restart"); err != nil {
		return err
	}
	defer func() {
		intent, intentErr := e.runtimeManager.GetLifecycleIntent(runtimeID)
		if intentErr == nil && intent == "restart" {
			_ = e.runtimeManager.SetLifecycleIntent(runtimeID, "")
		}
	}()

	oldGeneration, err := e.runtimeManager.GetCurrentGeneration(runtimeID)
	if err != nil {
		return err
	}

	if err := e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateRestarting, reason, now); err != nil {
		return err
	}

	stopErr := e.stopRuntimeLocked(ctx, runtimeID)
	if stopErr != nil {
		e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, "restart stop phase failed", time.Now())
		return &RuntimeRestartError{
			Code:          ErrRestartFailed,
			RuntimeID:     string(runtimeID),
			OldGeneration: oldGeneration,
			Cause:         stopErr,
		}
	}
	if err := e.CleanupRuntime(ctx, runtimeID); err != nil {
		e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, "restart cleanup phase failed", time.Now())
		return &RuntimeRestartError{
			Code:          ErrRestartFailed,
			RuntimeID:     string(runtimeID),
			OldGeneration: oldGeneration,
			Cause:         err,
		}
	}

	currentIntent, err := e.runtimeManager.GetLifecycleIntent(runtimeID)
	if err != nil {
		return err
	}
	if currentIntent != "restart" {
		return &RuntimeRestartError{
			Code:          ErrRestartFailed,
			RuntimeID:     string(runtimeID),
			OldGeneration: oldGeneration,
			Cause:         &ExecutionError{Code: ErrLifecycleConflict, RuntimeID: string(runtimeID), Message: "restart superseded by another operation"},
		}
	}

	startErr := e.startRuntimeLocked(ctx, runtimeID)
	if startErr != nil {
		newGeneration, _ := e.runtimeManager.GetCurrentGeneration(runtimeID)
		e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, "restart start phase failed", time.Now())
		return &RuntimeRestartError{
			Code:          ErrRestartFailed,
			RuntimeID:     string(runtimeID),
			OldGeneration: oldGeneration,
			NewGeneration: newGeneration,
			Cause:         startErr,
		}
	}

	return nil
}

func (e *runtimeExecutor) startRuntimeLocked(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	runtimeInfo, err := e.runtimeManager.GetRuntime(runtimeID)
	if err != nil {
		return err
	}

	if runtimeInfo.State != domain.RuntimeStateCreated && runtimeInfo.State != domain.RuntimeStateStopped {
		return &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: string(runtimeID),
			Message:   "runtime not in startable state: " + string(runtimeInfo.State),
		}
	}

	topologySnapshot, err := e.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return err
	}

	graphSnapshot, err := e.topology.GetDependencyGraphSnapshot(runtimeID)
	if err != nil {
		return err
	}

	_, err = e.topology.GetTopology(runtimeID)
	if err != nil {
		return err
	}

	plan, err := e.planner.BuildStartupPlan(topologySnapshot, graphSnapshot)
	if err != nil {
		return err
	}

	generation, err := e.runtimeManager.AllocateGeneration(runtimeID)
	if err != nil {
		return err
	}
	ctx = WithExecutionGeneration(ctx, generation)

	now := time.Now()
	if err := e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateStarting, "", now); err != nil {
		return err
	}

	progress := &StartupProgress{
		RuntimeID:            runtimeID,
		StartedThisOperation: make([]domain.ServiceID, 0),
	}

	startedServices := make(map[domain.ServiceID]struct{})
	failedServices := make(map[domain.ServiceID]struct{})

	resolveDefinition := e.getResolveDefinition()

	for _, stage := range plan.Stages {
		for _, entry := range stage.Services {
			if _, failed := failedServices[entry.ServiceID]; failed {
				continue
			}

			depsOK := true
			for _, depID := range entry.Dependencies {
				if _, failed := failedServices[depID]; failed {
					depsOK = false
					if entry.Required {
						failedServices[entry.ServiceID] = struct{}{}
					}
					break
				}
				if _, started := startedServices[depID]; !started {
					depsOK = false
					if entry.Required {
						failedServices[entry.ServiceID] = struct{}{}
					}
					break
				}
			}

			if _, failed := failedServices[entry.ServiceID]; failed {
				continue
			}
			if !depsOK {
				continue
			}

			handle, err := e.executor.Start(ctx, entry, resolveDefinition)
			if err != nil {
				failedServices[entry.ServiceID] = struct{}{}
				if entry.Required {
					rollbackPlan, rbErr := e.planner.BuildRollbackPlan(*progress, topologySnapshot, graphSnapshot)
					if rbErr != nil {
						return &RuntimeStartError{
							Cause:     err,
							RuntimeID: string(runtimeID),
						}
					}
					rbResult := e.rollbackExecutor.Execute(ctx, rollbackPlan, e.handleStore, e.executor, progress)
					e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, err.Error(), time.Now())
					return &RuntimeStartError{
						Cause:          err,
						RuntimeID:      string(runtimeID),
						RollbackErrors: rbResult.Errors,
					}
				}
				continue
			}

			e.handleStore.Put(runtimeID, entry.ServiceID, handle)
			startedServices[entry.ServiceID] = struct{}{}
			progress.RecordStarted(entry.ServiceID)
		}
	}

	allRequiredRunning := true
	for _, svc := range topologySnapshot.Services {
		if !svc.Required {
			continue
		}
		if _, ok := startedServices[svc.ServiceID]; !ok {
			allRequiredRunning = false
			break
		}
	}

	optionalFailed := false
	for _, svc := range topologySnapshot.Services {
		if svc.Required {
			continue
		}
		if _, failed := failedServices[svc.ServiceID]; failed {
			optionalFailed = true
			break
		}
	}

	if !allRequiredRunning {
		e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, "required services not all started", time.Now())
		return &ExecutionError{
			Code:      ErrRuntimeUnavailable,
			RuntimeID: string(runtimeID),
			Message:   "not all required services started",
		}
	}

	finalState := domain.RuntimeStateRunning
	if optionalFailed && len(startedServices) > 0 {
		finalState = domain.RuntimeStateDegraded
	}

	if err := e.runtimeManager.UpdateRuntimeState(runtimeID, finalState, "", time.Now()); err != nil {
		return err
	}

	return nil
}

func (e *runtimeExecutor) stopRuntimeLocked(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	runtimeInfo, err := e.runtimeManager.GetRuntime(runtimeID)
	if err != nil {
		return err
	}

	if runtimeInfo.State == domain.RuntimeStateStopped {
		return nil
	}

	if runtimeInfo.State == domain.RuntimeStateStopping {
		return &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: string(runtimeID),
			Message:   "runtime already stopping",
		}
	}

	topologySnapshot, err := e.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return err
	}

	graphSnapshot, err := e.topology.GetDependencyGraphSnapshot(runtimeID)
	if err != nil {
		return err
	}

	plan, err := e.planner.BuildShutdownPlan(topologySnapshot, graphSnapshot)
	if err != nil {
		return err
	}

	if !domain.IsTerminalRuntimeState(runtimeInfo.State) {
		e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateStopping, "runtime_stop", time.Now())
	}

	var stopErrors []error

	for _, stage := range plan.Stages {
		for _, entry := range stage.Services {
			handle, found := e.handleStore.Get(runtimeID, entry.ServiceID)
			if !found {
				continue
			}

			err := e.executor.Stop(ctx, *handle, false)
			if err != nil {
				stopErrors = append(stopErrors, err)
				continue
			}
			e.handleStore.Remove(runtimeID, entry.ServiceID)
		}
	}

	if !domain.IsTerminalRuntimeState(runtimeInfo.State) {
		if len(stopErrors) > 0 {
			e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateFailed, "stop failed", time.Now())
		} else {
			e.runtimeManager.UpdateRuntimeState(runtimeID, domain.RuntimeStateStopped, "runtime_stop", time.Now())
		}
	}

	if len(stopErrors) > 0 {
		return &RuntimeStopError{
			RuntimeID:  string(runtimeID),
			StopErrors: stopErrors,
		}
	}

	return nil
}

func (e *runtimeExecutor) StartServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	lock := e.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	runtimeInfo, err := e.runtimeManager.GetRuntime(runtimeID)
	if err != nil {
		return err
	}

	if runtimeInfo.State != domain.RuntimeStateRunning && runtimeInfo.State != domain.RuntimeStateDegraded {
		return &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: string(runtimeID),
			Message:   "runtime not in a state to start additional services",
		}
	}
	generation, err := e.runtimeManager.GetCurrentGeneration(runtimeID)
	if err != nil {
		return err
	}
	ctx = WithExecutionGeneration(ctx, generation)

	topologySnapshot, err := e.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return err
	}

	graphSnapshot, err := e.topology.GetDependencyGraphSnapshot(runtimeID)
	if err != nil {
		return err
	}

	plan, err := e.planner.BuildStartupPlanFor(topologySnapshot, graphSnapshot, serviceIDs)
	if err != nil {
		return err
	}

	progress := &StartupProgress{
		RuntimeID:            runtimeID,
		StartedThisOperation: make([]domain.ServiceID, 0),
	}

	startedServices := make(map[domain.ServiceID]struct{})
	resolveDefinition := e.getResolveDefinition()

	for _, stage := range plan.Stages {
		for _, entry := range stage.Services {
			topology, topoErr := e.topology.GetTopology(runtimeID)
			if topoErr != nil {
				return topoErr
			}

			svc, svcErr := topology.GetService(entry.ServiceID)
			if svcErr != nil {
				return svcErr
			}

			if svc.State == ServiceStateRunning {
				startedServices[entry.ServiceID] = struct{}{}
				continue
			}

			handle, err := e.executor.Start(ctx, entry, resolveDefinition)
			if err != nil {
				if entry.Required {
					rollbackPlan, _ := e.planner.BuildRollbackPlan(*progress, topologySnapshot, graphSnapshot)
					e.rollbackExecutor.Execute(ctx, rollbackPlan, e.handleStore, e.executor, progress)
					return &RuntimeStartError{
						Cause:     err,
						RuntimeID: string(runtimeID),
					}
				}
				continue
			}

			e.handleStore.Put(runtimeID, entry.ServiceID, handle)
			startedServices[entry.ServiceID] = struct{}{}
			progress.RecordStarted(entry.ServiceID)
		}
	}

	return nil
}

func (e *runtimeExecutor) StopServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	lock := e.getRuntimeLock(runtimeID)
	lock.Lock()
	defer lock.Unlock()

	runtimeInfo, err := e.runtimeManager.GetRuntime(runtimeID)
	if err != nil {
		return err
	}

	if runtimeInfo.State != domain.RuntimeStateRunning && runtimeInfo.State != domain.RuntimeStateDegraded {
		return &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: string(runtimeID),
			Message:   "runtime not in a state to stop services",
		}
	}

	topologySnapshot, err := e.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return err
	}

	graphSnapshot, err := e.topology.GetDependencyGraphSnapshot(runtimeID)
	if err != nil {
		return err
	}

	plan, err := e.planner.BuildShutdownPlanFor(topologySnapshot, graphSnapshot, serviceIDs)
	if err != nil {
		return err
	}

	var stopErrors []error

	for _, stage := range plan.Stages {
		for _, entry := range stage.Services {
			handle, found := e.handleStore.Get(runtimeID, entry.ServiceID)
			if !found {
				continue
			}

			err := e.executor.Stop(ctx, *handle, false)
			if err != nil {
				stopErrors = append(stopErrors, err)
				continue
			}
			e.handleStore.Remove(runtimeID, entry.ServiceID)
		}
	}

	if len(stopErrors) > 0 {
		return &RuntimeStopError{
			RuntimeID:  string(runtimeID),
			StopErrors: stopErrors,
		}
	}

	return nil
}

func (e *runtimeExecutor) CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	handles := e.handleStore.RemoveRuntime(runtimeID)

	var cleanupErrors []error
	for _, handle := range handles {
		err := e.executor.Stop(ctx, *handle, true)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	if len(cleanupErrors) > 0 {
		return &RuntimeStopError{
			RuntimeID:     string(runtimeID),
			CleanupErrors: cleanupErrors,
		}
	}

	return nil
}

func (e *runtimeExecutor) getResolveDefinition() DefinitionResolverFunc {
	if e.resolveDefinition != nil {
		return e.resolveDefinition
	}
	return func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return nil, &ExecutionError{
			Code:         ErrDefinitionNotResolved,
			DefinitionID: definitionID,
			Message:      "no definition resolver configured",
		}
	}
}
