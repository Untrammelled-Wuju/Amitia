package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TopologyAccessor interface {
	GetService(serviceID domain.ServiceID) (*ServiceInstance, error)
	UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error
	Snapshot() RuntimeTopologySnapshot
	ListServices() []ServiceInstanceSnapshot
}

type ServiceExecutor interface {
	Start(ctx context.Context, entry ServicePlanEntry, resolveDefinition DefinitionResolverFunc) (*ServiceExecutionHandle, error)
	Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error
}

type DefinitionResolverFunc func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error)

type ServiceLeaseLifecycle interface {
	PrepareServiceStart(ctx context.Context, execCtx ServiceExecutionContext) (*contracts.RuntimeSecretLeaseSession, error)
	RevokeServiceLeases(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, generation int64, reason string)
}

type SecretLeaseAwareServiceExecutor interface {
	SetServiceLeaseLifecycle(lifecycle ServiceLeaseLifecycle)
}

// ServiceStopObserver receives a teardown callback only after host ownership of
// the service process has ended (or when topology already records it stopped).
// GameHost uses this for host-owned resources such as mediated network handles.
type ServiceStopObserver interface {
	OnServiceStopped(runtimeID, serviceID string)
}

// ServiceResourceAdmission is the runtime-facing boundary for resource startup
// admission. Implementations may validate identity, register per-service limits
// and return a cleanup function that releases transient startup state. The
// service executor calls it before any process is spawned.
type ServiceResourceAdmission interface {
	PrepareServiceStart(ctx context.Context, execCtx ServiceExecutionContext, definition *trusted_service.ServiceRuntimeDefinition) (finish func(started bool), err error)
	ReleaseService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID)
}

type ServiceStartPermissionGuard interface {
	AuthorizeServiceStart(ctx context.Context, execCtx ServiceExecutionContext, definition *trusted_service.ServiceRuntimeDefinition) error
}

type serviceExecutor struct {
	processAdapter     ProcessSupervisorAdapter
	externalAdapter    ExternalServiceAdapter
	topologyStore      RuntimeTopologyStore
	definitionResolver ServiceDefinitionBindingResolver
	startPermission    ServiceStartPermissionGuard
	leaseLifecycle     ServiceLeaseLifecycle
	resourceAdmission  ServiceResourceAdmission
	stopObserver       ServiceStopObserver
}

func (e *serviceExecutor) SetServiceLeaseLifecycle(lifecycle ServiceLeaseLifecycle) {
	e.leaseLifecycle = lifecycle
}

// SetServiceStopObserver wires host-owned resource cleanup without widening the
// public ServiceExecutor interface used by test/recovery fakes.
func SetServiceStopObserver(executor ServiceExecutor, observer ServiceStopObserver) error {
	concrete, ok := executor.(*serviceExecutor)
	if !ok || concrete == nil {
		return &TopologyError{Code: ErrInvalidArgument, Message: "service executor does not support stop observer"}
	}
	concrete.stopObserver = observer
	return nil
}

func (e *serviceExecutor) notifyServiceStopped(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	if e.stopObserver != nil {
		e.stopObserver.OnServiceStopped(string(runtimeID), string(serviceID))
	}
}

func NewServiceExecutor(
	processAdapter ProcessSupervisorAdapter,
	externalAdapter ExternalServiceAdapter,
	topologyStore RuntimeTopologyStore,
	definitionResolver ServiceDefinitionBindingResolver,
	startPermission ServiceStartPermissionGuard,
	resourceAdmission ServiceResourceAdmission,
) (ServiceExecutor, error) {
	if processAdapter == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "process adapter must not be nil"}
	}
	if externalAdapter == nil {
		externalAdapter = NewUnavailableExternalServiceAdapter()
	}
	if topologyStore == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "topology store must not be nil"}
	}
	if startPermission == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "service start permission guard must not be nil"}
	}
	if resourceAdmission == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "service resource admission must not be nil"}
	}
	return &serviceExecutor{
		processAdapter:     processAdapter,
		externalAdapter:    externalAdapter,
		topologyStore:      topologyStore,
		definitionResolver: definitionResolver,
		startPermission:    startPermission,
		resourceAdmission:  resourceAdmission,
	}, nil
}

func (e *serviceExecutor) Start(ctx context.Context, entry ServicePlanEntry, resolveDefinition DefinitionResolverFunc) (*ServiceExecutionHandle, error) {
	if resolveDefinition == nil {
		return nil, &ExecutionError{
			Code:      ErrDefinitionNotResolved,
			ServiceID: string(entry.ServiceID),
			Message:   "definition resolver is nil",
		}
	}

	runtimeID, serviceID, err := ParseServiceInstanceID(entry.ServiceInstanceID)
	if err != nil {
		return nil, &ExecutionError{
			Code:      ErrInvalidArgument,
			ServiceID: string(entry.ServiceID),
			Message:   fmt.Sprintf("invalid service instance id: %v", err),
		}
	}

	topology, err := e.topologyStore.GetTopology(runtimeID)
	if err != nil {
		return nil, &ExecutionError{
			Code:      ErrNotFound,
			RuntimeID: string(runtimeID),
			ServiceID: string(serviceID),
			Message:   "runtime topology not found",
			Cause:     err,
		}
	}

	definitionID, err := e.resolveDefinitionIDForService(runtimeID, serviceID)
	if err != nil {
		return nil, err
	}

	svc, err := topology.GetService(serviceID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if svc.State == ServiceStateRunning {
		return &ServiceExecutionHandle{
			RuntimeID: string(svc.RuntimeID),
			ServiceID: string(svc.ServiceID),
		}, nil
	}

	if !canStartService(svc.State) {
		return nil, &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: string(svc.RuntimeID),
			ServiceID: string(serviceID),
			Message:   "service not in startable state: " + string(svc.State),
		}
	}

	execCtx := ServiceExecutionContext{
		RuntimeID:    svc.RuntimeID,
		PluginID:     svc.PluginID,
		ServiceID:    svc.ServiceID,
		DefinitionID: definitionID,
		ServiceKind:  svc.ServiceKind,
		Required:     svc.Required,
		Generation:   executionGeneration(ctx),
	}
	if execCtx.Generation <= 0 {
		return nil, &ExecutionError{Code: ErrInvalidArgument, RuntimeID: string(svc.RuntimeID), ServiceID: string(svc.ServiceID), Message: "execution generation is required"}
	}

	var def *trusted_service.ServiceRuntimeDefinition
	if svc.ServiceKind != domain.ServiceKindExternal {
		def, err = resolveDefinition(definitionID)
		if err != nil {
			return nil, &ExecutionError{
				Code:         ErrDefinitionNotResolved,
				RuntimeID:    string(execCtx.RuntimeID),
				PluginID:     string(execCtx.PluginID),
				ServiceID:    string(execCtx.ServiceID),
				DefinitionID: definitionID,
				Message:      "failed to resolve service definition",
				Cause:        err,
			}
		}
	}
	if err := e.startPermission.AuthorizeServiceStart(ctx, execCtx, def); err != nil {
		return nil, &ExecutionError{
			Code:         ErrServiceLaunchFailed,
			RuntimeID:    string(execCtx.RuntimeID),
			PluginID:     string(execCtx.PluginID),
			ServiceID:    string(execCtx.ServiceID),
			DefinitionID: definitionID,
			Message:      "service start permission denied",
			Cause:        err,
		}
	}

	// Resource admission must be part of the actual launch path. Keeping it as
	// an observability-only helper would allow a process to start even when its
	// identity/profile is invalid. External services have no host-spawned
	// process definition, so process resource admission does not apply to them.
	var finishStartup func(started bool)
	startupCommitted := false
	if def != nil {
		if e.resourceAdmission == nil {
			return nil, &ExecutionError{
				Code:         ErrServiceLaunchFailed,
				RuntimeID:    string(execCtx.RuntimeID),
				PluginID:     string(execCtx.PluginID),
				ServiceID:    string(execCtx.ServiceID),
				DefinitionID: definitionID,
				Message:      "service resource admission unavailable",
			}
		}
		finishStartup, err = e.resourceAdmission.PrepareServiceStart(ctx, execCtx, def)
		if err != nil {
			return nil, &ExecutionError{
				Code:         ErrServiceLaunchFailed,
				RuntimeID:    string(execCtx.RuntimeID),
				PluginID:     string(execCtx.PluginID),
				ServiceID:    string(execCtx.ServiceID),
				DefinitionID: definitionID,
				Message:      "service resource admission denied",
				Cause:        err,
			}
		}
		if finishStartup != nil {
			defer func() { finishStartup(startupCommitted) }()
		}
	}

	if err := topology.UpdateServiceState(serviceID, ServiceStateStarting, now); err != nil {
		return nil, err
	}
	startCompleted := false
	defer func() {
		if !startCompleted {
			_ = topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
		}
	}()

	execCtx.SessionToken, err = newExecutionSessionToken()
	if err != nil {
		return nil, &ExecutionError{Code: ErrServiceLaunchFailed, RuntimeID: string(svc.RuntimeID), ServiceID: string(svc.ServiceID), Message: "create execution session token", Cause: err}
	}
	leasePrepared := false
	if e.leaseLifecycle != nil && svc.ServiceKind != domain.ServiceKindExternal {
		execCtx.SecretLeaseSession, err = e.leaseLifecycle.PrepareServiceStart(ctx, execCtx)
		if err != nil {
			return nil, &ExecutionError{Code: ErrServiceLaunchFailed, RuntimeID: string(svc.RuntimeID), ServiceID: string(svc.ServiceID), Message: "acquire service secret lease", Cause: err}
		}
		leasePrepared = true
		defer func() {
			if leasePrepared {
				e.leaseLifecycle.RevokeServiceLeases(svc.RuntimeID, svc.ServiceID, execCtx.Generation, "service startup failed")
			}
		}()
	}

	if svc.ServiceKind == domain.ServiceKindExternal {
		if err := e.externalAdapter.Start(ctx, execCtx); err != nil {
			return nil, &ExecutionError{
				Code:         ErrServiceLaunchFailed,
				RuntimeID:    string(execCtx.RuntimeID),
				PluginID:     string(execCtx.PluginID),
				ServiceID:    string(execCtx.ServiceID),
				DefinitionID: definitionID,
				Message:      "external adapter start failed",
				Cause:        err,
			}
		}
		if err := topology.UpdateServiceState(serviceID, ServiceStateRunning, time.Now()); err != nil {
			// The external service has already accepted Start. If topology persistence
			// fails, roll it back immediately so callers never observe a failed start
			// while the service continues running outside host ownership. Surface a
			// rollback failure because the external service may otherwise remain live.
			if cleanupErr := e.externalAdapter.Stop(context.WithoutCancel(ctx), execCtx); cleanupErr != nil {
				return nil, fmt.Errorf("persist running external service state: %w; rollback stop failed: %v", err, cleanupErr)
			}
			e.notifyServiceStopped(execCtx.RuntimeID, execCtx.ServiceID)
			return nil, err
		}
		startCompleted = true
		return &ServiceExecutionHandle{
			RuntimeID: string(svc.RuntimeID),
			ServiceID: string(svc.ServiceID),
		}, nil
	}
	result, handle, err := e.processAdapter.StartProcess(ctx, def, execCtx)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		// A successful StartProcess without an ownership handle is an adapter
		// contract violation. Treat the process as potentially live and make a
		// best-effort force-stop using the deterministic instance key (plus any
		// PID/instance data returned by the adapter) before reporting failure.
		cleanupHandle := ServiceExecutionHandle{
			RuntimeID:  string(execCtx.RuntimeID),
			ServiceID:  string(execCtx.ServiceID),
			InstanceID: BuildProcessInstanceID(execCtx.RuntimeID, execCtx.ServiceID),
		}
		if result != nil {
			if result.InstanceID != "" {
				cleanupHandle.InstanceID = result.InstanceID
			}
			cleanupHandle.PID = result.PID
		}
		cleanupErr := e.processAdapter.StopProcess(context.WithoutCancel(ctx), cleanupHandle, true)
		if cleanupErr == nil {
			e.notifyServiceStopped(execCtx.RuntimeID, execCtx.ServiceID)
		}
		return nil, &ExecutionError{
			Code:         ErrServiceLaunchFailed,
			RuntimeID:    string(execCtx.RuntimeID),
			PluginID:     string(execCtx.PluginID),
			ServiceID:    string(execCtx.ServiceID),
			DefinitionID: definitionID,
			Message:      "process adapter returned nil execution handle",
			Cause:        cleanupErr,
		}
	}

	if err := topology.UpdateServiceState(serviceID, ServiceStateRunning, time.Now()); err != nil {
		// Process startup succeeded but durable topology did not. Force-stop the
		// process before returning the failure; deferred cleanup revokes leases and
		// releases transient resource admission state. Surface cleanup failure so an
		// operator is not told that rollback was clean while a process may remain.
		if cleanupErr := e.processAdapter.StopProcess(context.WithoutCancel(ctx), *handle, true); cleanupErr != nil {
			return nil, fmt.Errorf("persist running service state: %w; force-stop failed: %v", err, cleanupErr)
		}
		e.notifyServiceStopped(execCtx.RuntimeID, execCtx.ServiceID)
		return nil, err
	}
	startupCommitted = true
	leasePrepared = false
	startCompleted = true

	return handle, nil
}

func (e *serviceExecutor) Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	runtimeID := domain.RuntimeInstanceID(handle.RuntimeID)
	serviceID := domain.ServiceID(handle.ServiceID)

	topology, err := e.topologyStore.GetTopology(runtimeID)
	if err != nil {
		return &ExecutionError{
			Code:      ErrNotFound,
			RuntimeID: handle.RuntimeID,
			ServiceID: handle.ServiceID,
			Message:   "runtime topology not found",
			Cause:     err,
		}
	}

	svc, err := topology.GetService(serviceID)
	if err != nil {
		return &ExecutionError{
			Code:      ErrNotFound,
			RuntimeID: handle.RuntimeID,
			ServiceID: handle.ServiceID,
			Message:   "service not found in topology",
			Cause:     err,
		}
	}

	if svc.State == ServiceStateStopped {
		if e.resourceAdmission != nil {
			e.resourceAdmission.ReleaseService(runtimeID, serviceID)
		}
		e.notifyServiceStopped(runtimeID, serviceID)
		return nil
	}

	if !canStopService(svc.State) {
		return &ExecutionError{
			Code:      ErrInvalidState,
			RuntimeID: handle.RuntimeID,
			ServiceID: handle.ServiceID,
			Message:   "service not in stoppable state: " + string(svc.State),
		}
	}

	now := time.Now()

	if err := topology.UpdateServiceState(serviceID, ServiceStateStopping, now); err != nil {
		return err
	}

	execCtx := ServiceExecutionContext{
		RuntimeID:   svc.RuntimeID,
		PluginID:    svc.PluginID,
		ServiceID:   svc.ServiceID,
		ServiceKind: svc.ServiceKind,
		Generation:  executionGeneration(ctx),
	}
	if e.leaseLifecycle != nil {
		e.leaseLifecycle.RevokeServiceLeases(svc.RuntimeID, svc.ServiceID, execCtx.Generation, "service stopping")
	}

	if svc.ServiceKind == domain.ServiceKindExternal {
		if err := e.externalAdapter.Stop(ctx, execCtx); err != nil {
			topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
			return &ExecutionError{
				Code:      ErrServiceUnavailable,
				RuntimeID: handle.RuntimeID,
				ServiceID: handle.ServiceID,
				Message:   "external adapter stop failed",
				Cause:     err,
			}
		}
		if e.resourceAdmission != nil {
			e.resourceAdmission.ReleaseService(runtimeID, serviceID)
		}
		e.notifyServiceStopped(runtimeID, serviceID)
		return topology.UpdateServiceState(serviceID, ServiceStateStopped, time.Now())
	}

	if err := e.processAdapter.StopProcess(ctx, handle, force); err != nil {
		topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
		return err
	}
	if e.resourceAdmission != nil {
		e.resourceAdmission.ReleaseService(runtimeID, serviceID)
	}
	e.notifyServiceStopped(runtimeID, serviceID)

	return topology.UpdateServiceState(serviceID, ServiceStateStopped, time.Now())
}

func (e *serviceExecutor) resolveDefinitionIDForService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (string, error) {
	if e.definitionResolver != nil {
		return e.definitionResolver.ResolveDefinitionID(runtimeID, serviceID)
	}
	return "", &ExecutionError{
		Code:      ErrDefinitionNotResolved,
		RuntimeID: string(runtimeID),
		ServiceID: string(serviceID),
		Message:   "definition resolver not configured",
	}
}

func canStartService(state ServiceRuntimeState) bool {
	return state == ServiceStateCreated || state == ServiceStateStopped
}

func canStopService(state ServiceRuntimeState) bool {
	return state == ServiceStateRunning
}
