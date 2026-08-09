package runtime

import (
	"context"
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
	processAdapter  ProcessSupervisorAdapter
	externalAdapter ExternalServiceAdapter
	topology        TopologyAccessor
	topologySnapshot RuntimeTopologySnapshot
	definitionIDs   map[string]string
}

func NewServiceExecutor(
	processAdapter ProcessSupervisorAdapter,
	externalAdapter ExternalServiceAdapter,
	topologyAccessor TopologyAccessor,
	topologySnapshot RuntimeTopologySnapshot,
	definitionIDs map[string]string,
) (ServiceExecutor, error) {
	if processAdapter == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "process adapter must not be nil"}
	}
	if externalAdapter == nil {
		externalAdapter = NewExternalServiceAdapter()
	}
	if topologyAccessor == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "topology accessor must not be nil"}
	}
	return &serviceExecutor{
		processAdapter:   processAdapter,
		externalAdapter:  externalAdapter,
		topology:         topologyAccessor,
		topologySnapshot: topologySnapshot,
		definitionIDs:    definitionIDs,
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

	definitionID, ok := e.definitionIDs[string(entry.ServiceID)]
	if !ok || definitionID == "" {
		return nil, &ExecutionError{
			Code:      ErrDefinitionNotResolved,
			ServiceID: string(entry.ServiceID),
			Message:   "definition id not mapped for service",
		}
	}

	svc, err := e.topology.GetService(entry.ServiceID)
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
			ServiceID: string(entry.ServiceID),
			Message:   "service not in startable state: " + string(svc.State),
		}
	}

	if err := e.topology.UpdateServiceState(entry.ServiceID, ServiceStateStarting, now); err != nil {
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
			e.topology.UpdateServiceState(entry.ServiceID, ServiceStateFailed, time.Now())
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
		if err := e.topology.UpdateServiceState(entry.ServiceID, ServiceStateRunning, time.Now()); err != nil {
			return nil, err
		}
		return &ServiceExecutionHandle{
			RuntimeID: string(svc.RuntimeID),
			ServiceID: string(svc.ServiceID),
		}, nil
	}

	def, err := resolveDefinition(definitionID)
	if err != nil {
		e.topology.UpdateServiceState(entry.ServiceID, ServiceStateFailed, time.Now())
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
		e.topology.UpdateServiceState(entry.ServiceID, ServiceStateFailed, time.Now())
		return nil, err
	}

	if err := e.topology.UpdateServiceState(entry.ServiceID, ServiceStateRunning, time.Now()); err != nil {
		return nil, err
	}

	return handle, nil
}

func (e *serviceExecutor) Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	svc, err := e.topology.GetService(domain.ServiceID(handle.ServiceID))
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

	if err := e.topology.UpdateServiceState(domain.ServiceID(handle.ServiceID), ServiceStateStopping, now); err != nil {
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
			e.topology.UpdateServiceState(domain.ServiceID(handle.ServiceID), ServiceStateFailed, time.Now())
			return &ExecutionError{
				Code:      ErrServiceUnavailable,
				RuntimeID: handle.RuntimeID,
				ServiceID: handle.ServiceID,
				Message:   "external adapter stop failed",
				Cause:     err,
			}
		}
		return e.topology.UpdateServiceState(domain.ServiceID(handle.ServiceID), ServiceStateStopped, time.Now())
	}

	if err := e.processAdapter.StopProcess(ctx, handle, force); err != nil {
		e.topology.UpdateServiceState(domain.ServiceID(handle.ServiceID), ServiceStateFailed, time.Now())
		return err
	}

	return e.topology.UpdateServiceState(domain.ServiceID(handle.ServiceID), ServiceStateStopped, time.Now())
}

func canStartService(state ServiceRuntimeState) bool {
	return state == ServiceStateCreated || state == ServiceStateStopped
}

func canStopService(state ServiceRuntimeState) bool {
	return state == ServiceStateRunning
}
