package runtime

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type ProcessSupervisorAdapter interface {
	StartProcess(ctx context.Context, def *trusted_service.ServiceRuntimeDefinition, execCtx ServiceExecutionContext) (*trusted_service.StartResult, *ServiceExecutionHandle, error)
	StopProcess(ctx context.Context, handle ServiceExecutionHandle, force bool) error
	IsRunning(supervisorKey string) bool
}

type processSupervisorAdapter struct {
	supervisor *trusted_service.ProcessSupervisor
}

func NewProcessSupervisorAdapter(supervisor *trusted_service.ProcessSupervisor) (ProcessSupervisorAdapter, error) {
	if supervisor == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "supervisor must not be nil"}
	}
	return &processSupervisorAdapter{
		supervisor: supervisor,
	}, nil
}

func (a *processSupervisorAdapter) StartProcess(ctx context.Context, def *trusted_service.ServiceRuntimeDefinition, execCtx ServiceExecutionContext) (*trusted_service.StartResult, *ServiceExecutionHandle, error) {
	if def == nil {
		return nil, nil, &ExecutionError{
			Code:      ErrServiceUnavailable,
			RuntimeID: string(execCtx.RuntimeID),
			PluginID:  string(execCtx.PluginID),
			ServiceID: string(execCtx.ServiceID),
			Message:   "service definition is nil",
		}
	}

	instanceID := BuildProcessInstanceID(execCtx.RuntimeID, execCtx.ServiceID)
	supervisorKey := instanceID

	env := make(map[string]string)
	for k, v := range execCtx.Env {
		env[k] = v
	}

	startReq := trusted_service.StartRequest{
		ServiceID:      supervisorKey,
		InstanceID:     instanceID,
		Generation:     execCtx.Generation,
		PublisherTrust: a.resolveTrustLevel(def),
		BasePath:       execCtx.BasePath,
		WorkingDir:     execCtx.ServicePaths.Data,
		SessionToken:   execCtx.SessionToken,
		SecretLease:    execCtx.SecretLease,
		LogLevel:       "info",
		Args:           env,
	}

	result, err := a.supervisor.Start(ctx, startReq)
	if err != nil {
		return nil, nil, &ExecutionError{
			Code:         ErrServiceLaunchFailed,
			RuntimeID:    string(execCtx.RuntimeID),
			PluginID:     string(execCtx.PluginID),
			ServiceID:    string(execCtx.ServiceID),
			DefinitionID: execCtx.DefinitionID,
			Message:      fmt.Sprintf("supervisor start failed: %v", err),
			Cause:        err,
		}
	}

	handle := &ServiceExecutionHandle{
		RuntimeID:  string(execCtx.RuntimeID),
		ServiceID:  string(execCtx.ServiceID),
		InstanceID: result.InstanceID,
		PID:        result.PID,
	}

	return result, handle, nil
}

func (a *processSupervisorAdapter) StopProcess(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	stopReq := trusted_service.StopRequest{
		ServiceID: handle.InstanceID,
		Reason:    "gamehost_stop",
		Force:     force,
	}
	_, err := a.supervisor.Stop(ctx, stopReq)
	if err != nil {
		return &ExecutionError{
			Code:      ErrServiceUnavailable,
			RuntimeID: handle.RuntimeID,
			ServiceID: handle.ServiceID,
			Message:   fmt.Sprintf("supervisor stop failed: %v", err),
			Cause:     err,
		}
	}
	return nil
}

func (a *processSupervisorAdapter) IsRunning(supervisorKey string) bool {
	_, err := a.supervisor.Get(supervisorKey)
	return err == nil
}

func (a *processSupervisorAdapter) resolveTrustLevel(def *trusted_service.ServiceRuntimeDefinition) trusted_service.TrustLevel {
	if def.TrustLevel != "" {
		return trusted_service.TrustLevel(def.TrustLevel)
	}
	return trusted_service.TrustLevelTrusted
}
