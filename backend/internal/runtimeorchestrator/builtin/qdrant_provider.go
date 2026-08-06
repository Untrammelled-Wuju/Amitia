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
	"github.com/u-ai/backend/internal/vectorstore/qdrantprocess"
	"github.com/u-ai/backend/internal/vectorstore/qdrantprofile"
	"github.com/u-ai/backend/pkg/platform"
)

type QdrantProviderFactory struct{}

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

type descriptorProviderHost struct {
	host runtimehost.RuntimeHost
}

func (d descriptorProviderHost) Descriptor() platform.RuntimeDescriptor {
	return d.host.Descriptor()
}

func (d descriptorProviderHost) Capabilities() *runtimehost.HostCapabilities {
	return d.host.Capabilities()
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

	ownershipRoot, err := qdrantprocess.ResolveOwnershipRoot(ctx.Host)
	if err != nil {
		return nil, runtimeorchestrator.DescriptorFailure("", err.Error())
	}

	ownershipReconciler, err := qdrantprocess.NewReconciler(
		ownershipRoot,
		qdrantprocess.NewFileSystem(),
		nil,
		nil,
		nil,
	)
	if err != nil {
		return nil, runtimeorchestrator.DescriptorFailure("", err.Error())
	}

	descProvider := descriptorProviderHost{host: ctx.Host}
	profileResolver, err := qdrantprofile.NewResolver(qdrantprofile.ResolveContext{
		DescriptorProvider: descProvider,
	})
	if err != nil {
		return nil, runtimeorchestrator.DescriptorFailure("", err.Error())
	}

	envSanitizer := qdrantprofile.NewEnvironmentSanitizer()

	return &qdrantProvider{
		config:               &ctx.Config.Providers.VectorStore,
		host:                 ctx.Host,
		envResolver:          envResolver,
		layoutResolver:       layoutResolver,
		directoryManager:     qdrantlayout.NewDirectoryManager(nil),
		configRenderer:       qdrantconfig.NewRenderer(),
		configWriter:         qdrantconfig.NewWriter(nil),
		ownershipReconciler:  ownershipReconciler,
		profileResolver:      profileResolver,
		environmentSanitizer: envSanitizer,
	}, nil
}

type qdrantProvider struct {
	config               *config.VectorStoreProviderConfig
	host                 runtimehost.RuntimeHost
	layoutResolver       qdrantlayout.Resolver
	envResolver          qdrantenv.Resolver
	directoryManager     qdrantlayout.DirectoryManager
	configRenderer       qdrantconfig.Renderer
	configWriter         qdrantconfig.Writer
	ownershipReconciler  qdrantprocess.Reconciler
	profileResolver      qdrantprofile.Resolver
	environmentSanitizer qdrantprofile.EnvironmentSanitizer

	capabilityMu sync.RWMutex
	client       *qdrant.Client
	started      bool
	stopped      bool

	lifecycleMu sync.Mutex
	activeLease qdrantprocess.Lease
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
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()

	if p.started && p.activeLease != nil {
		return qdrantprocess.ErrQdrantAlreadyRunning
	}
	if p.started {
		return nil
	}

	resolvedProfile, err := p.profileResolver.Resolve(ctx, p.config.Qdrant.ResourceProfile)
	if err != nil {
		return phaseError{phase: "qdrant:resolve-profile", err: err}
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

	expected := qdrantprocess.NewExpectedProcess(env.BinaryPath, layout.ConfigPath)
	lease, err := p.ownershipReconciler.Acquire(ctx, expected)
	if err != nil {
		return phaseError{phase: "qdrant:acquire-lease", err: err}
	}
	p.activeLease = lease

	if err := p.directoryManager.Ensure(ctx, layout); err != nil {
		p.releaseLeaseOnFailure(ctx, lease)
		return phaseError{phase: "qdrant:ensure-dirs", err: err}
	}

	doc := qdrantconfig.Document{
		HTTPPort:     p.config.Qdrant.Port,
		GRPCPort:     p.config.Qdrant.Port + 1,
		StoragePath:  layout.StorageDir,
		SnapshotPath: layout.SnapshotsDir,
	}
	if resolvedProfile.Mobile && resolvedProfile.Settings != nil {
		doc.ResourceProfile = resolvedProfile.Settings
	}

	configBytes, err := p.configRenderer.Render(doc)
	if err != nil {
		p.releaseLeaseOnFailure(ctx, lease)
		return phaseError{phase: "qdrant:render-config", err: err}
	}

	if err := p.configWriter.Write(ctx, layout.ConfigPath, configBytes); err != nil {
		p.releaseLeaseOnFailure(ctx, lease)
		return phaseError{phase: "qdrant:write-config", err: err}
	}

	p.started = true
	return nil
}

func (p *qdrantProvider) releaseLeaseOnFailure(ctx context.Context, lease qdrantprocess.Lease) {
	if lease != nil {
		_ = lease.Release(ctx)
		if p.activeLease == lease {
			p.activeLease = nil
		}
	}
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
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()

	if p.stopped {
		return nil
	}
	p.stopped = true

	if p.activeLease != nil {
		rec := p.activeLease.Record()
		_ = p.activeLease.MarkStopping(ctx)

		if rec.Child != nil && rec.Child.PID > 0 {
			supervisor := p.host.Processes()
			if supervisor != nil {
				_ = supervisor.Stop(ctx, runtimehost.ProcessIDQdrant)
			}
		}
		_ = p.activeLease.MarkExited(ctx)
		_ = p.activeLease.Release(ctx)
		p.activeLease = nil
	}
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
