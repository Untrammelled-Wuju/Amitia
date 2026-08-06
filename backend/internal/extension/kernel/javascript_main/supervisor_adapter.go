// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package javascript_main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type SupervisorFactory struct {
	factory                 *RuntimeFactory
	nodeEnvironmentResolver script_host.NodeEnvironmentResolver
	hostArtifactResolver    script_host.ArtifactResolver
}

func NewSupervisorFactory(
	factory *RuntimeFactory,
	nodeResolver script_host.NodeEnvironmentResolver,
	artifactResolver script_host.ArtifactResolver,
) *SupervisorFactory {
	if nodeResolver == nil {
		nodeResolver = script_host.UnavailableNodeResolver()
	}
	if artifactResolver == nil {
		artifactResolver = script_host.UnavailableArtifactResolver()
	}
	return &SupervisorFactory{
		factory:                 factory,
		nodeEnvironmentResolver: nodeResolver,
		hostArtifactResolver:    artifactResolver,
	}
}

func (f *SupervisorFactory) Type() domain.RuntimeType {
	return domain.RuntimeTypeJavaScript
}

func (f *SupervisorFactory) Validate(spec runtime_supervisor.InstanceSpec) error {
	if spec.RuntimeType != domain.RuntimeTypeJavaScript {
		return errors.New("javascript_main: runtime type must be javascript")
	}
	if spec.ExtensionID == "" {
		return errors.New("javascript_main: extension id required")
	}
	if spec.ModuleID == "" {
		return errors.New("javascript_main: module id required")
	}
	if spec.EntryPoint == "" {
		return errors.New("javascript_main: entry point required")
	}
	return nil
}

func (f *SupervisorFactory) Create(ctx context.Context, spec runtime_supervisor.InstanceSpec) (runtime_supervisor.ManagedRuntime, error) {
	if err := f.Validate(spec); err != nil {
		return nil, err
	}

	nodeEnv, err := f.nodeEnvironmentResolver.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("javascript_main: resolve node: %w", err)
	}

	artifact, err := f.hostArtifactResolver.Resolve(ctx, script_host.KindPluginHost)
	if err != nil {
		return nil, fmt.Errorf("javascript_main: resolve plugin host: %w", err)
	}

	req := CreateHostRequest{
		ExtensionID:    string(spec.ExtensionID),
		ModuleID:       string(spec.ModuleID),
		Entry:          spec.EntryPoint,
		DefinitionHash: spec.DefinitionHash,
		Generation:     int(spec.Generation),
		NodePath:       nodeEnv.NodeBinary,
		PluginHostPath:  artifact.EntryPath,
		WorkingDirectory: artifact.DistributionRoot,
		ResourceLimits: runtime.ResourceLimits{
			MaxMemoryMB:        int(spec.Limits.MaxMemoryBytes / (1024 * 1024)),
			MaxConcurrentCalls: spec.Limits.MaxConcurrentCalls,
			MaxQueueDepth:      spec.Limits.MaxQueueDepth,
			SingleCallTimeout:  spec.Limits.MaxExecutionTime.String(),
			MaxOpenHandles:     spec.Limits.MaxOpenFiles,
		},
	}

	host, err := f.factory.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("javascript_main: create host: %w", err)
	}

	return &managedPluginHost{host: host}, nil
}

type managedPluginHost struct {
	host *PluginHost
}

func (m *managedPluginHost) Start(ctx context.Context) error {
	result := m.host.Start(ctx)
	if !result.Success {
		return fmt.Errorf("javascript_main: start failed: %s", result.Reason)
	}
	return nil
}

func (m *managedPluginHost) Invoke(ctx context.Context, request runtime_supervisor.InvocationRequest) runtime_supervisor.InvocationResult {
	input := request.Input
	if len(input) == 0 {
		input = []byte(`{}`)
	}

	invCtx := ctx
	if !request.Deadline.IsZero() {
		var cancel context.CancelFunc
		invCtx, cancel = context.WithDeadline(ctx, request.Deadline)
		defer cancel()
	}

	output, err := m.host.Invoke(invCtx, request.Operation, input)
	if err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        err,
		}
	}

	var outputBytes []byte
	if output != nil {
		outputBytes = []byte(fmt.Sprintf("%v", output))
	}

	return runtime_supervisor.InvocationResult{
		InvocationID: request.InvocationID,
		Status:       "success",
		Output:       outputBytes,
	}
}

func (m *managedPluginHost) Health(_ context.Context) runtime_supervisor.HealthReport {
	h := m.host.Health()
	status := runtime_supervisor.HealthUnknown
	switch h.State {
	case HostStateReady:
		status = runtime_supervisor.HealthHealthy
	case HostStateStarting, HostStateCreated:
		status = runtime_supervisor.HealthUnknown
	case HostStateUnhealthy:
		status = runtime_supervisor.HealthDegraded
	case HostStateCrashed, HostStateFailed:
		status = runtime_supervisor.HealthUnhealthy
	case HostStateStopped, HostStateStopping:
		status = runtime_supervisor.HealthUnknown
	}

	metrics := map[string]any{
		"crashCount":        h.CrashCount,
		"activeInvocations": h.ActiveInvocations,
		"queuedInvocations": h.QueuedInvocations,
	}

	return runtime_supervisor.HealthReport{
		Status:    status,
		Reason:    string(h.State),
		CheckedAt: time.Now().UTC(),
		Metrics:   metrics,
	}
}

func (m *managedPluginHost) Stop(ctx context.Context, reason runtime_supervisor.StopReason) error {
	return m.host.Stop(ctx, string(reason))
}

var _ runtime_supervisor.RuntimeFactory = (*SupervisorFactory)(nil)
var _ runtime_supervisor.ManagedRuntime = (*managedPluginHost)(nil)
