// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/internal/scriptruntime/sidecar"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	surrealdbDB "github.com/u-ai/backend/pkg/database/surrealdb"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type runtimeBootstrap struct {
	host             runtimehost.RuntimeHost
	orchestrator     *runtimeorchestrator.RuntimeOrchestrator
	providerRegistry *runtimeorchestrator.ProviderRegistry
	resources        *util.RuntimePaths
	graphMu          sync.RWMutex
	graphSvc         graph.Service
	stopOnce         sync.Once
	runError         error
	nodeEnvironment  nodeenv.Resolver
	artifactResolver sidecar.ArtifactResolver

	profile runtimeprofile.Profile
	policy  runtimeprofile.Policy

	iosSandboxProvider runtimeorchestrator.ProviderInstance
	iosNativeProvider  runtimeorchestrator.ProviderInstance
	iosNativeBridge    nativebridge.Bridge

	androidNativeBridge nativebridge.Bridge
}

func newRuntimeBootstrap(paths *util.RuntimePaths, profile runtimeprofile.Profile) (*runtimeBootstrap, error) {
	descriptor := platform.Get().Descriptor()
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{
		Descriptor: descriptor,
		Paths:      *paths,
	})
	if err != nil {
		return nil, fmt.Errorf("create runtime host: %w", err)
	}

	policy := runtimeprofile.PolicyFor(profile)

	orch := runtimeorchestrator.NewWithProfile(descriptor, profile)

	providerRegistry := runtimeorchestrator.NewProviderRegistry()

	nodeResolver, err := nodeenv.NewResolver(nodeenv.ResolveContext{
		Config: config.AppCfg,
		Host:   host,
	})
	if err != nil {
		return nil, fmt.Errorf("create node environment resolver: %w", err)
	}

	artifactResolver, err := sidecar.NewArtifactResolver(sidecar.ResolveContext{
		Host: host,
	})
	if err != nil {
		return nil, fmt.Errorf("create sidecar artifact resolver: %w", err)
	}

	bootstrap := &runtimeBootstrap{
		host:             host,
		orchestrator:     orch,
		providerRegistry: providerRegistry,
		resources:        paths,
		nodeEnvironment:  nodeResolver,
		artifactResolver: artifactResolver,
		profile:          profile,
		policy:           policy,
	}

	androidBridge := nativebridge.NewAndroidTransportBridge()
	bootstrap.SetAndroidNativeBridge(androidBridge)

	if err := bootstrap.registerProviderFactories(); err != nil {
		return nil, err
	}

	if err := bootstrap.buildPlatformProviders(); err != nil {
		return nil, err
	}

	return bootstrap, nil
}

func (b *runtimeBootstrap) RuntimeHost() runtimehost.RuntimeHost {
	return b.host
}

func (b *runtimeBootstrap) RuntimeProfile() runtimeprofile.Profile {
	if b == nil {
		return runtimeprofile.ProfileLocal
	}
	return b.profile
}

func (b *runtimeBootstrap) RuntimePolicy() runtimeprofile.Policy {
	if b == nil {
		return runtimeprofile.PolicyFor(runtimeprofile.ProfileLocal)
	}
	return b.policy
}

func (b *runtimeBootstrap) ProcessSupervisor() runtimehost.ProcessSupervisor {
	return b.host.Processes()
}

func (b *runtimeBootstrap) registerProviderFactories() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	return b.registerProviderFactoriesIOS()
}

func (b *runtimeBootstrap) buildPlatformProviders() error {
	if b == nil {
		return fmt.Errorf("nil runtime bootstrap")
	}

	return b.buildPlatformProvidersIOS()
}

func (b *runtimeBootstrap) RegisterInfrastructure(sqlDB *sql.DB, graphSvc graph.Service) error {
	vectorProvider := &vectorStoreProviderAdapter{host: b.host}
	graphProvider := &graphStoreProviderAdapter{host: b.host, graphSvc: graphSvc}

	if err := b.orchestrator.Register(vectorProvider); err != nil {
		return fmt.Errorf("register vector store: %w", err)
	}
	if err := b.orchestrator.Register(graphProvider); err != nil {
		return fmt.Errorf("register graph store: %w", err)
	}
	if err := b.orchestrator.Register(&sqliteComponent{db: sqlDB}); err != nil {
		return fmt.Errorf("register sqlite: %w", err)
	}
	if err := b.orchestrator.Register(newSidecarComponent(b.host, b.nodeEnvironment, b.artifactResolver)); err != nil {
		return fmt.Errorf("register sidecars: %w", err)
	}
	return nil
}

func (b *runtimeBootstrap) RegisterApplication(services *AppServices) error {
	if err := b.orchestrator.Register(newExtensionKernelComponent(services)); err != nil {
		return fmt.Errorf("register extension kernel: %w", err)
	}
	if err := b.orchestrator.Register(newTaskRuntimeComponent(services)); err != nil {
		return fmt.Errorf("register task runtime: %w", err)
	}
	if err := b.orchestrator.Register(newDesktopPetComponent(services)); err != nil {
		return fmt.Errorf("register desktop pet: %w", err)
	}
	if err := b.orchestrator.Register(newBrowserComponent(services, b.host)); err != nil {
		return fmt.Errorf("register browser: %w", err)
	}
	return nil
}

func (b *runtimeBootstrap) StartPhase(ctx context.Context, phase runtimeorchestrator.ComponentPhase) error {
	return b.orchestrator.StartPhase(ctx, phase)
}

func (b *runtimeBootstrap) StopAll(ctx context.Context) error {
	b.stopOnce.Do(func() {
		b.runError = b.orchestrator.StopAll(ctx)
		b.host.Processes().StopAll(ctx)
	})
	return b.runError
}

func (b *runtimeBootstrap) Snapshot() runtimeorchestrator.RuntimeSnapshot {
	return b.orchestrator.Snapshot()
}

func (b *runtimeBootstrap) GraphService() graph.Service {
	b.graphMu.RLock()
	defer b.graphMu.RUnlock()
	return b.graphSvc
}

func (b *runtimeBootstrap) SetGraphService(svc graph.Service) {
	b.graphMu.Lock()
	b.graphSvc = svc
	b.graphMu.Unlock()
}

func (b *runtimeBootstrap) NodeEnvironmentResolver() nodeenv.Resolver {
	if b == nil {
		return nil
	}
	return b.nodeEnvironment
}

func (b *runtimeBootstrap) SidecarArtifactResolver() sidecar.ArtifactResolver {
	if b == nil {
		return nil
	}
	return b.artifactResolver
}

func (b *runtimeBootstrap) IOSSandboxProvider() runtimeorchestrator.ProviderInstance {
	return b.iosSandboxProvider
}

func (b *runtimeBootstrap) IOSNativeProvider() runtimeorchestrator.ProviderInstance {
	return b.iosNativeProvider
}

func (b *runtimeBootstrap) AndroidNativeBridge() nativebridge.Bridge {
	if b == nil {
		return nil
	}
	return b.androidNativeBridge
}

func (b *runtimeBootstrap) SetAndroidNativeBridge(bridge nativebridge.Bridge) error {
	if b == nil {
		return nil
	}
	b.androidNativeBridge = bridge
	return nil
}

type vectorStoreProviderAdapter struct {
	host runtimehost.RuntimeHost
}

func (a *vectorStoreProviderAdapter) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentVectorStore,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      true,
		Required:     false,
		Capabilities: []string{"storage.vector"},
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentSQLite},
		Profiles:     profilesCore,
	}
}

func (a *vectorStoreProviderAdapter) Start(ctx context.Context) error {
	spec, err := qdrantDB.BuildQdrantProcessSpec(a.host)
	if err != nil {
		return fmt.Errorf("build qdrant spec: %w", err)
	}
	supervisor := a.host.Processes()
	if err := supervisor.Register(spec); err != nil {
		return fmt.Errorf("register qdrant process: %w", err)
	}
	if err := supervisor.Start(ctx, spec.ID); err != nil {
		return fmt.Errorf("start qdrant process: %w", err)
	}
	if err := supervisor.WaitReady(ctx, spec.ID); err != nil {
		return fmt.Errorf("wait for qdrant ready: %w", err)
	}
	if err := qdrantDB.InitClient(); err != nil {
		return fmt.Errorf("init qdrant client: %w", err)
	}
	if err := qdrantDB.EnsureCollections(); err != nil {
		return fmt.Errorf("ensure qdrant collections: %w", err)
	}
	return nil
}

func (a *vectorStoreProviderAdapter) Ready(ctx context.Context) error {
	if qdrantDB.Client == nil {
		return fmt.Errorf("qdrant client not initialized")
	}
	return nil
}

func (a *vectorStoreProviderAdapter) Stop(ctx context.Context) error {
	_ = a.host.Processes().Stop(ctx, runtimehost.ProcessIDQdrant)
	return nil
}

type graphStoreProviderAdapter struct {
	host     runtimehost.RuntimeHost
	graphSvc graph.Service
}

func (a *graphStoreProviderAdapter) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentGraphStore,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      true,
		Required:     false,
		Capabilities: []string{"storage.graph"},
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentSQLite},
		Profiles:     profilesCore,
	}
}

func (a *graphStoreProviderAdapter) Start(ctx context.Context) error {
	spec, err := surrealdbDB.BuildSurrealProcessSpec(a.host.RuntimeInstanceID())
	if err != nil {
		return fmt.Errorf("build surrealdb spec: %w", err)
	}
	supervisor := a.host.Processes()
	if regErr := supervisor.Register(spec); regErr != nil {
		return fmt.Errorf("register surrealdb: %w", regErr)
	}
	return supervisor.Start(ctx, spec.ID)
}

func (a *graphStoreProviderAdapter) Ready(ctx context.Context) error {
	if a.graphSvc == nil {
		return fmt.Errorf("graph service not ready")
	}
	return nil
}

func (a *graphStoreProviderAdapter) Stop(ctx context.Context) error {
	return a.host.Processes().Stop(ctx, runtimehost.ProcessIDSurrealDB)
}
