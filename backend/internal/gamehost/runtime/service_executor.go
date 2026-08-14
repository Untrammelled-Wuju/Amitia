package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TopologyAccessor interface {
	GetService(serviceID domain.ServiceID) (*ServiceInstance, error)
	UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error
	Snapshot() RuntimeTopologySnapshot
}

type ServiceExecutor interface {
	Start(ctx context.Context, entry ServicePlanEntry, resolveDefinition DefinitionResolverFunc) (*ServiceExecutionHandle, error)
	Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error
}

type DefinitionResolverFunc func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error)

type serviceExecutor struct {
	processAdapter     ProcessSupervisorAdapter
	externalAdapter    ExternalServiceAdapter
	topologyStore      RuntimeTopologyStore
	definitionResolver ServiceDefinitionBindingResolver
}

func NewServiceExecutor(
	processAdapter ProcessSupervisorAdapter,
	externalAdapter ExternalServiceAdapter,
	topologyStore RuntimeTopologyStore,
	definitionResolver ServiceDefinitionBindingResolver,
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
	return &serviceExecutor{
		processAdapter:     processAdapter,
		externalAdapter:    externalAdapter,
		topologyStore:      topologyStore,
		definitionResolver: definitionResolver,
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

	if err := topology.UpdateServiceState(serviceID, ServiceStateStarting, now); err != nil {
		return nil, err
	}

	execCtx := ServiceExecutionContext{
		RuntimeID:    svc.RuntimeID,
		PluginID:     svc.PluginID,
		ServiceID:    svc.ServiceID,
		DefinitionID: definitionID,
		ServiceKind:  svc.ServiceKind,
		Required:     svc.Required,
	}

	if svc.ServiceKind == domain.ServiceKindExternal {
		if err := e.externalAdapter.Start(ctx, execCtx); err != nil {
			topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
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
			return nil, err
		}
		return &ServiceExecutionHandle{
			RuntimeID: string(svc.RuntimeID),
			ServiceID: string(svc.ServiceID),
		}, nil
	}

	def, err := resolveDefinition(definitionID)
	if err != nil {
		topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
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

	_, handle, err := e.processAdapter.StartProcess(ctx, def, execCtx)
	if err != nil {
		topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
		return nil, err
	}

	if err := topology.UpdateServiceState(serviceID, ServiceStateRunning, time.Now()); err != nil {
		return nil, err
	}

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
		return topology.UpdateServiceState(serviceID, ServiceStateStopped, time.Now())
	}

	if err := e.processAdapter.StopProcess(ctx, handle, force); err != nil {
		topology.UpdateServiceState(serviceID, ServiceStateFailed, time.Now())
		return err
	}

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
