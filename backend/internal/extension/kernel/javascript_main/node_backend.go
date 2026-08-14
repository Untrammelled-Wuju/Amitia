package javascript_main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type NodeProcessBackend struct {
	factory          *RuntimeFactory
	nodeResolver     script_host.NodeEnvironmentResolver
	artifactResolver script_host.ArtifactResolver
	capabilities     JavaScriptRuntimeCapabilities
}

func NewNodeProcessBackend(
	factory *RuntimeFactory,
	nodeResolver script_host.NodeEnvironmentResolver,
	artifactResolver script_host.ArtifactResolver,
) *NodeProcessBackend {
	caps := DefaultCapabilities()
	if nodeResolver != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if env, err := nodeResolver.Resolve(ctx); err == nil {
			caps.Architecture = env.Architecture
		}
	}
	return &NodeProcessBackend{
		factory:          factory,
		nodeResolver:     nodeResolver,
		artifactResolver: artifactResolver,
		capabilities:     caps,
	}
}

func (b *NodeProcessBackend) Capabilities() JavaScriptRuntimeCapabilities {
	return b.capabilities
}

func (b *NodeProcessBackend) Start(ctx context.Context, spec JavaScriptRuntimeSpec) (JavaScriptRuntimeInstance, error) {
	if spec.ExtensionID == "" {
		return nil, fmt.Errorf("node_backend: extension id required")
	}
	if spec.ModuleID == "" {
		return nil, fmt.Errorf("node_backend: module id required")
	}
	if spec.Entry == "" {
		return nil, fmt.Errorf("node_backend: entry required")
	}

	nodeEnv, err := b.nodeResolver.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("node_backend: resolve node: %w", err)
	}

	artifact, err := b.artifactResolver.Resolve(ctx, script_host.KindPluginHost)
	if err != nil {
		return nil, fmt.Errorf("node_backend: resolve plugin host: %w", err)
	}

	req := CreateHostRequest{
		ExtensionID:      spec.ExtensionID,
		ModuleID:         spec.ModuleID,
		Entry:            spec.Entry,
		DefinitionHash:   spec.DefinitionHash,
		Generation:       spec.Generation,
		HostAPIVersion:   spec.HostAPIVersion,
		SessionToken:     spec.SessionToken,
		NetworkDisabled:  spec.NetworkDisabled,
		Env:              spec.Env,
		NodePath:         nodeEnv.NodeBinary,
		PluginHostPath:   artifact.EntryPath,
		WorkingDirectory: artifact.DistributionRoot,
		ResourceLimits: runtime.ResourceLimits{
			MaxMemoryMB:        spec.ResourceLimits.MaxMemoryMB,
			MaxConcurrentCalls: spec.ResourceLimits.MaxConcurrentCalls,
			MaxQueueDepth:      spec.ResourceLimits.MaxQueueDepth,
			SingleCallTimeout:  spec.ResourceLimits.SingleCallTimeout,
			MaxOpenHandles:     spec.ResourceLimits.MaxOpenHandles,
		},
	}

	host, err := b.factory.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("node_backend: create host: %w", err)
	}

	result := host.Start(ctx)
	if !result.Success {
		return nil, fmt.Errorf("node_backend: start failed: %s", result.Reason)
	}

	return &nodeRuntimeInstance{host: host}, nil
}

type nodeRuntimeInstance struct {
	host *PluginHost
}

func (i *nodeRuntimeInstance) InstanceID() string {
	return i.host.InstanceID()
}

func (i *nodeRuntimeInstance) ExtensionID() string {
	return i.host.ExtensionID()
}

func (i *nodeRuntimeInstance) ModuleID() string {
	return i.host.ModuleID()
}

func (i *nodeRuntimeInstance) Generation() int {
	return i.host.spec.Generation
}

func (i *nodeRuntimeInstance) State() HostState {
	return i.host.State()
}

func (i *nodeRuntimeInstance) Health() HealthReport {
	return i.host.Health()
}

func (i *nodeRuntimeInstance) Invoke(ctx context.Context, contributionID string, input []byte, deadline int64) (json.RawMessage, error) {
	if len(input) == 0 {
		input = []byte(`{}`)
	}
	var cancel context.CancelFunc
	if deadline > 0 {
		dl := time.UnixMilli(deadline)
		ctx, cancel = context.WithDeadline(ctx, dl)
		defer cancel()
	}
	output, err := i.host.Invoke(ctx, contributionID, input)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(fmt.Sprintf("%v", output)), nil
}

func (i *nodeRuntimeInstance) Start(ctx context.Context) error {
	result := i.host.Start(ctx)
	if !result.Success {
		return fmt.Errorf("node_backend: start failed: %s", result.Reason)
	}
	return nil
}

func (i *nodeRuntimeInstance) Stop(ctx context.Context, reason string) error {
	return i.host.Stop(ctx, reason)
}
