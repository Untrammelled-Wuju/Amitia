// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package javascript_main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	extensionRoot           string
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

func (f *SupervisorFactory) SetExtensionRoot(root string) {
	f.extensionRoot = root
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
	entryPoint := resolveExtensionEntryPoint(f.extensionRoot, string(spec.ExtensionID), string(spec.ModuleID), spec.EntryPoint)

	req := CreateHostRequest{
		ExtensionID:      string(spec.ExtensionID),
		ModuleID:         string(spec.ModuleID),
		Entry:            entryPoint,
		DefinitionHash:   spec.DefinitionHash,
		Generation:       int(spec.Generation),
		NodePath:         nodeEnv.NodeBinary,
		PluginHostPath:   artifact.EntryPath,
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

	return &managedPluginHost{factory: f.factory, host: host}, nil
}

func resolveExtensionEntryPoint(extensionRoot, extensionID, moduleID, entryPoint string) string {
	if extensionRoot == "" || extensionID == "" || moduleID == "" || entryPoint == "" || filepath.IsAbs(entryPoint) {
		return entryPoint
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installationsRoot := filepath.Join(extensionRoot, "installations", safeID)
	if currentData, err := os.ReadFile(filepath.Join(installationsRoot, "current.json")); err == nil {
		var current struct {
			GenerationID string `json:"generationID"`
		}
		if json.Unmarshal(currentData, &current) == nil && current.GenerationID != "" {
			candidate := filepath.Join(installationsRoot, "generations", current.GenerationID, "modules", moduleID, entryPoint)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return entryPoint
}

type managedPluginHost struct {
	factory *RuntimeFactory
	host    *PluginHost
}

func (m *managedPluginHost) Start(ctx context.Context) error {
	result := m.host.Start(ctx)
	if !result.Success {
		if m.factory != nil {
			_ = m.factory.Remove(m.host.InstanceID())
		}
		return fmt.Errorf("javascript_main: start failed: %s", result.Reason)
	}
	return nil
}

func (m *managedPluginHost) Invoke(ctx context.Context, request runtime_supervisor.InvocationRequest) runtime_supervisor.InvocationResult {
	input := request.Input
	if len(input) == 0 {
		input = []byte(`{}`)
	}
	var invocationInput any
	if err := json.Unmarshal(input, &invocationInput); err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        fmt.Errorf("javascript_main: decode invocation input: %w", err),
		}
	}

	invCtx := ctx
	if !request.Deadline.IsZero() {
		var cancel context.CancelFunc
		invCtx, cancel = context.WithDeadline(ctx, request.Deadline)
		defer cancel()
	}

	output, err := m.host.Invoke(invCtx, request.Operation, invocationInput)
	if err != nil {
		return runtime_supervisor.InvocationResult{
			InvocationID: request.InvocationID,
			Status:       "failed",
			Error:        err,
		}
	}

	var outputBytes []byte
	if output != nil {
		outputBytes, err = json.Marshal(output)
		if err != nil {
			return runtime_supervisor.InvocationResult{
				InvocationID: request.InvocationID,
				Status:       "failed",
				Error:        fmt.Errorf("javascript_main: encode invocation output: %w", err),
			}
		}
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
