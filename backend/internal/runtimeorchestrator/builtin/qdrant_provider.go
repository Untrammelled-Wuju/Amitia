// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package builtin

import (
	"context"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/vectorstore/qdrantconfig"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/internal/vectorstore/qdrantlayout"
)

type QdrantDependency interface {
	StartQdrant() error
	WaitForQdrant(port int) error
	InitClient() error
	GetClient() *qdrant.Client
}

type QdrantProviderFactory struct {
	dep QdrantDependency
}

func NewQdrantProviderFactory() *QdrantProviderFactory {
	return &QdrantProviderFactory{}
}

func (f *QdrantProviderFactory) ProviderID() string { return "builtin.qdrant-process" }
func (f *QdrantProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotVectorStore
}
func (f *QdrantProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{ID: runtimehost.CapProcessSpawn, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessTreeControl, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapProcessRestart, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapFilesystemExecutable, Minimum: runtimehost.SupportSupported},
		{ID: runtimehost.CapNetworkLoopback, Minimum: runtimehost.SupportSupported},
	}
}

func (f *QdrantProviderFactory) Build(ctx runtimeorchestrator.ProviderBuildContext) (runtimeorchestrator.ProviderInstance, error) {
	if ctx.Config == nil {
		return nil, runtimeorchestrator.DescriptorFailure("", "nil config")
	}
	if ctx.Host == nil {
		return nil, runtimeorchestrator.DescriptorFailure("", "nil host")
	}

	envResolver, err := qdrantenv.NewResolver(qdrantenv.ResolveContext{
		Config: ctx.Config,
		Host:   ctx.Host,
	})
	if err != nil {
		return nil, runtimeorchestrator.DescriptorFailure("", err.Error())
	}

	layoutResolver, err := qdrantlayout.NewResolver(qdrantlayout.ResolveContext{
		Config: ctx.Config,
		Host:   ctx.Host,
	})
	if err != nil {
		return nil, runtimeorchestrator.DescriptorFailure("", err.Error())
	}

	return &qdrantProvider{
		config:           &ctx.Config.Providers.VectorStore,
		host:             ctx.Host,
		envResolver:      envResolver,
		layoutResolver:   layoutResolver,
		directoryManager: qdrantlayout.NewDirectoryManager(nil),
		configRenderer:   qdrantconfig.NewRenderer(),
		configWriter:     qdrantconfig.NewWriter(nil),
	}, nil
}

type qdrantProvider struct {
	config           *config.VectorStoreProviderConfig
	host             runtimehost.RuntimeHost
	layoutResolver   qdrantlayout.Resolver
	envResolver      qdrantenv.Resolver
	directoryManager qdrantlayout.DirectoryManager
	configRenderer   qdrantconfig.Renderer
	configWriter     qdrantconfig.Writer

	capabilityMu sync.RWMutex
	client       *qdrant.Client
	started      bool
	stopped      bool
}

func (p *qdrantProvider) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentVectorStore,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      p.config.Enabled,
		Required:     p.config.Required,
		Capabilities: []string{"storage.vector"},
	}
}

func (p *qdrantProvider) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotVectorStore
}

func (p *qdrantProvider) ProviderID() string { return "builtin.qdrant-process" }

func (p *qdrantProvider) Capability() any {
	p.capabilityMu.RLock()
	defer p.capabilityMu.RUnlock()
	return p.client
}

func (p *qdrantProvider) Start(ctx context.Context) error {
	if p.started {
		return nil
	}

	env, err := p.envResolver.Resolve(ctx)
	if err != nil {
		if qdrantErr, ok := unwrapNotInstalled(err); ok {
			return qdrantErr
		}
		return phaseError{phase: "qdrant:env-resolve", err: err}
	}

	layout, err := p.layoutResolver.Resolve(ctx, env)
	if err != nil {
		return phaseError{phase: "qdrant:layout-resolve", err: err}
	}

	if err := p.directoryManager.Ensure(ctx, layout); err != nil {
		return phaseError{phase: "qdrant:ensure-dirs", err: err}
	}

	doc := qdrantconfig.Document{
		HTTPPort:     p.config.Qdrant.Port,
		GRPCPort:     p.config.Qdrant.Port + 1,
		StoragePath:  layout.StorageDir,
		SnapshotPath: layout.SnapshotsDir,
	}

	configBytes, err := p.configRenderer.Render(doc)
	if err != nil {
		return phaseError{phase: "qdrant:render-config", err: err}
	}

	if err := p.configWriter.Write(ctx, layout.ConfigPath, configBytes); err != nil {
		return phaseError{phase: "qdrant:write-config", err: err}
	}

	p.started = true
	return nil
}

func (p *qdrantProvider) Ready(ctx context.Context) error {
	if p.client == nil {
		return runtimeorchestrator.DescriptorFailure("", "qdrant client not initialized")
	}
	p.capabilityMu.RLock()
	client := p.client
	p.capabilityMu.RUnlock()

	hCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := client.HealthCheck(hCtx); err != nil {
		return err
	}
	return nil
}

func (p *qdrantProvider) Stop(ctx context.Context) error {
	if p.stopped {
		return nil
	}
	p.stopped = true
	return nil
}

type phaseError struct {
	phase string
	err   error
}

func (e phaseError) Error() string { return e.phase + ": " + e.err.Error() }
func (e phaseError) Unwrap() error { return e.err }

func unwrapNotInstalled(err error) (error, bool) {
	for err != nil {
		if err == qdrantenv.ErrQdrantBinaryNotInstalled {
			return err, true
		}
		type unwrap interface{ Unwrap() error }
		if u, ok := err.(unwrap); ok {
			err = u.Unwrap()
		} else {
			return nil, false
		}
	}
	return nil, false
}
