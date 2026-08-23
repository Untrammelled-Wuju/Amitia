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
	return &processSupervisorAdapter{supervisor: supervisor}, nil
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
	if err := a.ensureInstanceDefinition(supervisorKey, def); err != nil {
		return nil, nil, &ExecutionError{
			Code:         ErrServiceLaunchFailed,
			RuntimeID:    string(execCtx.RuntimeID),
			PluginID:     string(execCtx.PluginID),
			ServiceID:    string(execCtx.ServiceID),
			DefinitionID: execCtx.DefinitionID,
			Message:      fmt.Sprintf("prepare supervisor definition: %v", err),
			Cause:        err,
		}
	}

	env := make(map[string]string)
	for k, v := range execCtx.Env {
		env[k] = v
	}
	if execCtx.SecretLeaseSession != nil && execCtx.SecretLeaseSession.SessionID != "" {
		env["AMITIA_SECRET_LEASE_SESSION"] = execCtx.SecretLeaseSession.SessionID
	}

	canonicalPluginID := string(execCtx.PluginID)
	if canonicalPluginID == "" {
		canonicalPluginID = fmt.Sprintf("%s/%s", execCtx.ExtensionID, execCtx.ContributionID)
	}
	logicalServiceID := string(execCtx.ServiceID)
	if logicalServiceID == "" {
		logicalServiceID = execCtx.DefinitionID
	}

	startReq := trusted_service.StartRequest{
		ServiceID:        supervisorKey,
		InstanceID:       instanceID,
		RuntimeID:        string(execCtx.RuntimeID),
		PluginID:         canonicalPluginID,
		LogicalServiceID: logicalServiceID,
		ExtensionID:      execCtx.ExtensionID,
		ContributionID:   execCtx.ContributionID,
		Generation:       execCtx.Generation,
		PublisherTrust:   a.resolveTrustLevel(def),
		BasePath:         execCtx.BasePath,
		WorkingDir:       execCtx.ServicePaths.Data,
		SessionToken:     execCtx.SessionToken,
		SecretLease:      secretSessionID(execCtx),
		LogLevel:         "info",
		Args:             env,
	}

	result, err := a.supervisor.Start(ctx, startReq)
	if err != nil {
		_ = a.supervisor.Unregister(supervisorKey)
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

func (a *processSupervisorAdapter) ensureInstanceDefinition(supervisorKey string, source *trusted_service.ServiceRuntimeDefinition) error {
	if existing, err := a.supervisor.GetDefinition(supervisorKey); err == nil {
		if existing.ManifestHash == source.ManifestHash {
			return nil
		}
		if a.IsRunning(supervisorKey) {
			return fmt.Errorf("runtime definition changed while instance %s is running", supervisorKey)
		}
		if err := a.supervisor.Unregister(supervisorKey); err != nil {
			return err
		}
	}
	clone := cloneServiceRuntimeDefinition(source)
	clone.ServiceID = supervisorKey
	return a.supervisor.Register(clone)
}

func (a *processSupervisorAdapter) StopProcess(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	stopReq := trusted_service.StopRequest{ServiceID: handle.InstanceID, Reason: "gamehost_stop", Force: force}
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
	_ = a.supervisor.Unregister(handle.InstanceID)
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
	return trusted_service.TrustLevelUnknown
}

func secretSessionID(execCtx ServiceExecutionContext) string {
	if execCtx.SecretLeaseSession == nil {
		return ""
	}
	return execCtx.SecretLeaseSession.SessionID
}

func cloneServiceRuntimeDefinition(in *trusted_service.ServiceRuntimeDefinition) *trusted_service.ServiceRuntimeDefinition {
	if in == nil {
		return nil
	}
	out := *in
	out.Executables = make([]trusted_service.PlatformExecutable, len(in.Executables))
	for i := range in.Executables {
		out.Executables[i] = in.Executables[i]
		out.Executables[i].ArgsTemplate = append([]string(nil), in.Executables[i].ArgsTemplate...)
		out.Executables[i].Dependencies = append([]trusted_service.LibraryDep(nil), in.Executables[i].Dependencies...)
		if in.Executables[i].EnvTemplate != nil {
			out.Executables[i].EnvTemplate = make(map[string]string, len(in.Executables[i].EnvTemplate))
			for k, v := range in.Executables[i].EnvTemplate {
				out.Executables[i].EnvTemplate[k] = v
			}
		}
	}
	out.AllowedNamespaces = append([]string(nil), in.AllowedNamespaces...)
	out.Network.AllowedDomains = append([]string(nil), in.Network.AllowedDomains...)
	return &out
}
