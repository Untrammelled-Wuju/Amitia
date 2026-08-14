// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package builtin

import (
	"context"
	"fmt"
	"sync"

	"github.com/qdrant/go-client/qdrant"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/vectorstore/qdrantconfig"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/internal/vectorstore/qdranthealth"
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

	lifecycleMu  sync.Mutex
	activeLease  qdrantprocess.Lease
	coordinator  *qdranthealth.Coordinator
	processGuard *qdrantProcessGuard
}

type qdrantProcessGuard struct {
	mu      sync.Mutex
	started bool
	exited  bool
	pid     int
}

func (g *qdrantProcessGuard) IsStarted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.started
}

func (g *qdrantProcessGuard) IsExited() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.exited
}

func (g *qdrantProcessGuard) PID() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pid
}

func (g *qdrantProcessGuard) MarkStarted(pid int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.started = true
	g.pid = pid
}

func (g *qdrantProcessGuard) MarkExited() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.exited = true
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

	if err := p.startProcessWithHealthCheck(ctx, env, layout, lease); err != nil {
		p.releaseLeaseOnFailure(ctx, lease)
		return phaseError{phase: "qdrant:start-process-health", err: err}
	}

	return nil
}

func (p *qdrantProvider) startProcessWithHealthCheck(ctx context.Context, env qdrantenv.Environment, layout qdrantlayout.Layout, lease qdrantprocess.Lease) error {
	target := qdranthealth.NewTarget("127.0.0.1", p.config.Qdrant.Port)
	policy := p.selectHealthPolicy()

	if p.processGuard == nil {
		p.processGuard = &qdrantProcessGuard{}
	}

	controller := &qdrantProcessController{provider: p}
	p.coordinator = qdranthealth.NewCoordinator(target, policy, p.processGuard, controller)

	supervisor := p.host.Processes()
	processSpec := runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDQdrant,
		Executable: env.BinaryPath,
		Args:       buildProcessArgs(layout.ConfigPath),
		WorkingDir: layout.StorageDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyExplicit,
			Values: p.buildProcessEnv(),
		},
		Ports: []runtimehost.LoopbackPortClaim{
			{Host: "127.0.0.1", Port: p.config.Qdrant.Port, Protocol: "tcp"},
			{Host: "127.0.0.1", Port: p.config.Qdrant.Port + 1, Protocol: "tcp"},
		},
		StartupTimeout: policy.StartupTimeout,
	}

	if err := supervisor.Register(processSpec); err != nil {
		return fmt.Errorf("register qdrant process: %w", err)
	}

	if err := supervisor.Start(ctx, runtimehost.ProcessIDQdrant); err != nil {
		return fmt.Errorf("start qdrant process: %w", err)
	}

	snap, _ := supervisor.Snapshot(runtimehost.ProcessIDQdrant)
	if snap.PID > 0 {
		p.processGuard.MarkStarted(snap.PID)
	}

	_, err := p.coordinator.WaitReady(ctx)
	if err != nil {
		p.processGuard.MarkExited()
		return fmt.Errorf("qdrant ready check failed: %w", err)
	}

	client, clientErr := qdrant.NewClient(&qdrant.Config{
		Host: "127.0.0.1",
		Port: int(p.config.Qdrant.Port),
	})
	if clientErr != nil {
		return fmt.Errorf("create qdrant client: %w", clientErr)
	}

	p.capabilityMu.Lock()
	p.client = client
	p.capabilityMu.Unlock()

	return nil
}

func (p *qdrantProvider) selectHealthPolicy() qdranthealth.Policy {
	if resolvedProfile, err := p.profileResolver.Resolve(context.Background(), p.config.Qdrant.ResourceProfile); err == nil {
		if resolvedProfile.Settings != nil {
			switch resolvedProfile.Settings.ID {
			case qdrantprofile.ProfileMobileCompact:
				return qdranthealth.MobileCompactPolicy()
			case qdrantprofile.ProfileMobileBalanced:
				return qdranthealth.MobileBalancedPolicy()
			case qdrantprofile.ProfileMobilePerformance:
				return qdranthealth.MobilePerformancePolicy()
			}
		}
	}
	return qdranthealth.DesktopPolicy()
}

func buildProcessArgs(configPath string) []string {
	args := []string{"--config-path", configPath}
	return args
}

func (p *qdrantProvider) buildProcessEnv() map[string]string {
	values := make(map[string]string)
	return values
}

type qdrantProcessController struct {
	provider *qdrantProvider
}

func (c *qdrantProcessController) Stop(ctx context.Context) error {
	supervisor := c.provider.host.Processes()
	if supervisor != nil {
		return supervisor.Stop(ctx, runtimehost.ProcessIDQdrant)
	}
	return nil
}

func (c *qdrantProcessController) ReleaseLease(ctx context.Context) error {
	c.provider.lifecycleMu.Lock()
	lease := c.provider.activeLease
	if lease != nil {
		c.provider.activeLease = nil
	}
	c.provider.lifecycleMu.Unlock()

	if lease != nil {
		return lease.Release(ctx)
	}
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
	p.capabilityMu.RLock()
	client := p.client
	p.capabilityMu.RUnlock()

	if client == nil {
		return runtimeorchestrator.DescriptorFailure("", "qdrant client not initialized")
	}

	if p.coordinator != nil && !p.coordinator.IsReady() {
		return runtimeorchestrator.DescriptorFailure("", "qdrant coordinator reports not ready")
	}

	return nil
}

func (p *qdrantProvider) HealthCheck(ctx context.Context) qdranthealth.Snapshot {
	p.lifecycleMu.Lock()
	c := p.coordinator
	p.lifecycleMu.Unlock()

	if c == nil {
		return qdranthealth.NewSnapshot(qdranthealth.StateProcessNotStarted, qdranthealth.Target{})
	}
	return c.HealthCheck(ctx)
}

func (p *qdrantProvider) Stop(ctx context.Context) error {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()

	if p.stopped {
		return nil
	}
	p.stopped = true

	if p.coordinator != nil {
		p.coordinator.Stop()
	}

	if p.processGuard != nil {
		p.processGuard.MarkExited()
	}

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

	p.capabilityMu.Lock()
	p.client = nil
	p.capabilityMu.Unlock()

	p.coordinator = nil
	p.processGuard = nil
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
