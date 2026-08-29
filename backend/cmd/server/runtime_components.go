package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/internal/scriptruntime/sidecar"
)

var (
	profilesAll = []runtimeprofile.Profile{
		runtimeprofile.ProfileLocal,
		runtimeprofile.ProfileCloudCore,
		runtimeprofile.ProfileDeviceAgent,
	}
	profilesCore = []runtimeprofile.Profile{
		runtimeprofile.ProfileLocal,
		runtimeprofile.ProfileCloudCore,
	}
	profilesLocalOnly = []runtimeprofile.Profile{
		runtimeprofile.ProfileLocal,
	}
	profilesDeviceLocal = []runtimeprofile.Profile{
		runtimeprofile.ProfileLocal,
		runtimeprofile.ProfileDeviceAgent,
	}
)

type sqliteComponent struct {
	db *sql.DB
}

func (c *sqliteComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentSQLite,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Capabilities: []string{"storage.relational"},
		Profiles:     profilesAll,
	}
}

func (c *sqliteComponent) Start(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *sqliteComponent) Ready(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.db.PingContext(hctx)
}

func (c *sqliteComponent) Stop(ctx context.Context) error {
	return nil
}

type sidecarComponent struct {
	host             runtimehost.RuntimeHost
	nodeResolver     nodeenv.Resolver
	artifactResolver sidecar.ArtifactResolver
	mu               sync.Mutex
}

func newSidecarComponent(host runtimehost.RuntimeHost, nodeResolver nodeenv.Resolver, artifactResolver sidecar.ArtifactResolver) *sidecarComponent {
	return &sidecarComponent{host: host, nodeResolver: nodeResolver, artifactResolver: artifactResolver}
}

func (s *sidecarComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	wechatEnabled := config.AppCfg.Components.Sidecars.Wechat.Enabled
	qqEnabled := config.AppCfg.Components.Sidecars.QQ.Enabled
	enabled := wechatEnabled || qqEnabled
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentSidecars,
		Phase:        runtimeorchestrator.PhaseInfrastructure,
		Enabled:      enabled,
		Required:     false,
		Capabilities: []string{"channel.sidecar"},
		Profiles:     profilesCore,
	}
}

func (s *sidecarComponent) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	supervisor := s.host.Processes()

	wechatEnabled := config.AppCfg.Components.Sidecars.Wechat.Enabled
	qqEnabled := config.AppCfg.Components.Sidecars.QQ.Enabled

	if wechatEnabled {
		spec, err := buildWeChatSidecarSpec(s.host.RuntimeInstanceID(), s.nodeResolver, s.artifactResolver)
		if err != nil {
			return fmt.Errorf("wechat spec: %w", err)
		}
		if err := supervisor.Register(spec); err != nil {
			return fmt.Errorf("register wechat: %w", err)
		}
		if err := supervisor.Start(ctx, spec.ID); err != nil {
			return fmt.Errorf("start wechat: %w", err)
		}
	}
	if qqEnabled {
		spec, err := buildQQSidecarSpec(s.host.RuntimeInstanceID(), s.nodeResolver, s.artifactResolver)
		if err != nil {
			if wechatEnabled {
				supervisor.Stop(ctx, runtimehost.ProcessIDSidecarWeChat)
			}
			return fmt.Errorf("qq spec: %w", err)
		}
		if err := supervisor.Register(spec); err != nil {
			if wechatEnabled {
				supervisor.Stop(ctx, runtimehost.ProcessIDSidecarWeChat)
			}
			return fmt.Errorf("register qq: %w", err)
		}
		if err := supervisor.Start(ctx, spec.ID); err != nil {
			if wechatEnabled {
				supervisor.Stop(ctx, runtimehost.ProcessIDSidecarWeChat)
			}
			return fmt.Errorf("start qq: %w", err)
		}
	}
	return nil
}

func (s *sidecarComponent) Ready(ctx context.Context) error {
	supervisor := s.host.Processes()
	var firstErr error
	if config.AppCfg.Components.Sidecars.Wechat.Enabled {
		if err := supervisor.WaitReady(ctx, runtimehost.ProcessIDSidecarWeChat); err != nil {
			firstErr = err
		}
	}
	if config.AppCfg.Components.Sidecars.QQ.Enabled {
		if err := supervisor.WaitReady(ctx, runtimehost.ProcessIDSidecarQQ); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *sidecarComponent) Stop(ctx context.Context) error {
	supervisor := s.host.Processes()
	var lastErr error
	if config.AppCfg.Components.Sidecars.QQ.Enabled {
		if err := supervisor.Stop(ctx, runtimehost.ProcessIDSidecarQQ); err != nil {
			lastErr = err
		}
	}
	if config.AppCfg.Components.Sidecars.Wechat.Enabled {
		if err := supervisor.Stop(ctx, runtimehost.ProcessIDSidecarWeChat); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type extensionKernelComponent struct {
	services *AppServices
	mu       sync.Mutex
	started  bool
	stopped  bool
}

func newExtensionKernelComponent(services *AppServices) *extensionKernelComponent {
	return &extensionKernelComponent{services: services}
}

func (c *extensionKernelComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentExtensionKernel,
		Phase:        runtimeorchestrator.PhaseApplication,
		Enabled:      true,
		Required:     true,
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentSQLite},
		Capabilities: []string{"extension.kernel"},
		Profiles:     profilesAll,
	}
}

func (c *extensionKernelComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	svc := c.services
	if svc == nil || svc.KernelContainer == nil {
		return fmt.Errorf("kernel container not initialized")
	}
	container := svc.KernelContainer
	policy := svc.RuntimePolicy
	if policy.DurableEvents && container.EventService != nil {
		if err := container.EventService.Start(ctx); err != nil {
			return fmt.Errorf("event service: %w", err)
		}
	}
	if policy.TaskRuntime && container.ScheduleService != nil {
		if err := container.ScheduleService.Start(ctx); err != nil {
			if policy.DurableEvents && container.EventService != nil {
				container.EventService.Stop()
			}
			return fmt.Errorf("schedule service: %w", err)
		}
	}
	if container.ArtifactMaintenance != nil {
		if err := container.ArtifactMaintenance.Start(ctx); err != nil {
			if policy.TaskRuntime && container.ScheduleService != nil {
				container.ScheduleService.Shutdown(ctx)
			}
			if policy.DurableEvents && container.EventService != nil {
				container.EventService.Stop()
			}
			return fmt.Errorf("artifact maintenance: %w", err)
		}
	}
	c.started = true
	return nil
}

func (c *extensionKernelComponent) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.services == nil || c.services.KernelContainer == nil {
		return fmt.Errorf("kernel container not initialized")
	}
	container := c.services.KernelContainer
	policy := c.services.RuntimePolicy
	if policy.DurableEvents && container.EventService == nil {
		return fmt.Errorf("event service not ready")
	}
	if policy.TaskRuntime && container.ScheduleService == nil {
		return fmt.Errorf("schedule service not ready")
	}
	if container.ArtifactMaintenance == nil {
		return fmt.Errorf("artifact maintenance not ready")
	}
	return nil
}

func (c *extensionKernelComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return nil
	}
	svc := c.services
	if svc != nil && svc.KernelContainer != nil {
		container := svc.KernelContainer
		if container.ArtifactMaintenance != nil {
			container.ArtifactMaintenance.Stop()
		}
		if container.ScheduleService != nil {
			container.ScheduleService.Shutdown(ctx)
		}
		if container.EventService != nil {
			container.EventService.Stop()
		}
	}
	if svc != nil && svc.Extension != nil {
		_ = svc.Extension.Close(ctx)
	}
	c.stopped = true
	return nil
}

type taskRuntimeComponent struct {
	services *AppServices
	mu       sync.Mutex
	started  bool
	enabled  bool
}

func newTaskRuntimeComponent(services *AppServices) *taskRuntimeComponent {
	return &taskRuntimeComponent{
		services: services,
		enabled:  config.AppCfg.Components.TaskHost.Enabled,
	}
}

func (c *taskRuntimeComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentTaskRuntime,
		Phase:        runtimeorchestrator.PhaseApplication,
		Enabled:      c.enabled,
		Required:     false,
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentExtensionKernel},
		Capabilities: []string{"task.runtime"},
		Profiles:     profilesAll,
	}
}

func (c *taskRuntimeComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	svc := c.services
	if svc == nil || svc.KernelContainer == nil || svc.KernelContainer.TaskRuntimeService == nil {
		return fmt.Errorf("task runtime service not available")
	}
	if err := svc.KernelContainer.TaskRuntimeService.StartupRecovery(ctx); err != nil {
		return fmt.Errorf("task recovery: %w", err)
	}
	svc.KernelContainer.TaskRuntimeService.Start(ctx)
	c.started = true
	return nil
}

func (c *taskRuntimeComponent) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	svc := c.services
	if svc == nil || svc.KernelContainer == nil || svc.KernelContainer.TaskRuntimeService == nil {
		return fmt.Errorf("task runtime not ready")
	}
	return nil
}

func (c *taskRuntimeComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	svc := c.services
	if svc == nil || svc.KernelContainer == nil || svc.KernelContainer.TaskRuntimeService == nil {
		return nil
	}
	svc.KernelContainer.TaskRuntimeService.Shutdown(ctx)
	return nil
}

type desktopPetComponent struct {
	services *AppServices
	mu       sync.Mutex
	started  bool
	state    workerStartState
}

type workerStartState struct {
	releaseRecoveryWorkerOk bool
	behaviorOk              bool
}

func newDesktopPetComponent(services *AppServices) *desktopPetComponent {
	return &desktopPetComponent{services: services}
}

func (c *desktopPetComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:       runtimeorchestrator.ComponentDesktopPet,
		Phase:    runtimeorchestrator.PhaseApplication,
		Enabled:  config.AppCfg.DesktopPetRuntime.Enabled,
		Required: false,
		Dependencies: []runtimeorchestrator.ComponentID{
			runtimeorchestrator.ComponentSQLite,
			runtimeorchestrator.ComponentExtensionKernel,
		},
		Capabilities: []string{"desktop-pet.runtime"},
		// Desktop-pet packages and the renderer runtime are device-local in both
		// standalone and cloud deployments. CloudCore never hosts the pet body.
		Profiles: profilesDeviceLocal,
	}
}

func (c *desktopPetComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("desktop pet start context: %w", err)
	}
	if c.started {
		return nil
	}
	svc := c.services
	if svc == nil {
		return fmt.Errorf("services nil")
	}
	if svc.Readiness == nil {
		return fmt.Errorf("readiness not initialized")
	}
	snap := svc.Readiness.Snapshot()
	if snap.OverallStatus == readiness.StatusBlocked {
		if svc.SafeMode != nil {
			svc.SafeMode.Enter("readiness blocked")
		}
		return fmt.Errorf("readiness blocked: %d blocking", snap.BlockingCount)
	}
	workerStarts := []struct {
		name  string
		start func()
	}{
		{name: "generation", start: func() {
			if svc.DesktopPetWorker != nil {
				svc.DesktopPetWorker.Start(ctx)
			}
		}},
		{name: "processing", start: func() {
			if svc.ProcessingWorker != nil {
				svc.ProcessingWorker.Start(ctx)
			}
		}},
		{name: "quality", start: func() {
			if svc.QualityWorker != nil {
				svc.QualityWorker.Start(ctx)
			}
		}},
		{name: "regeneration", start: func() {
			if svc.RegenerationWorker != nil {
				svc.RegenerationWorker.Start(ctx)
			}
		}},
		{name: "revision-bridge-recovery", start: func() {
			if svc.BridgeRecoveryWorker != nil {
				svc.BridgeRecoveryWorker.Start(ctx)
			}
		}},
	}
	for _, item := range workerStarts {
		if err := runDesktopPetWorkerStart(item.name, item.start); err != nil {
			c.stopAllLocked(ctx, svc)
			if svc.SafeMode != nil {
				svc.SafeMode.Enter("desktop pet worker start failed")
			}
			return err
		}
	}
	if svc.ReleaseRecoveryWorker != nil {
		if err := runDesktopPetWorkerStart("release-recovery", func() { svc.ReleaseRecoveryWorker.Start(ctx) }); err != nil {
			c.stopAllLocked(ctx, svc)
			return err
		}
		c.state.releaseRecoveryWorkerOk = true
	}
	if svc.BehaviorService != nil {
		if err := svc.BehaviorService.Start(ctx); err != nil {
			c.stopAllLocked(ctx, svc)
			if svc.SafeMode != nil {
				svc.SafeMode.Enter("behavior start failed")
			}
			return fmt.Errorf("behavior start: %w", err)
		}
		c.state.behaviorOk = true
	}
	if svc.DesktopPetRuntimeV2 != nil {
		if err := svc.DesktopPetRuntimeV2.Start(ctx); err != nil {
			c.stopAllLocked(ctx, svc)
			if svc.SafeMode != nil {
				svc.SafeMode.Enter("pet runtime v2 start failed")
			}
			return fmt.Errorf("runtime v2 start: %w", err)
		}
	}
	if svc.RuntimeDomainEventConsumer != nil {
		svc.RuntimeDomainEventConsumer.Start(ctx)
	}
	if err := ctx.Err(); err != nil {
		c.stopAllLocked(ctx, svc)
		return fmt.Errorf("desktop pet start context cancelled: %w", err)
	}
	c.started = true
	if err := c.readyLocked(svc); err != nil {
		c.started = false
		c.stopAllLocked(ctx, svc)
		if svc.SafeMode != nil {
			svc.SafeMode.Enter("desktop pet runtime start incomplete")
		}
		return err
	}
	return nil
}

func runDesktopPetWorkerStart(name string, start func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("desktop pet %s worker start panic: %v", name, r)
		}
	}()
	start()
	return nil
}

func (c *desktopPetComponent) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	svc := c.services
	if !c.started || svc == nil {
		return fmt.Errorf("desktop pet not ready")
	}
	return c.readyLocked(svc)
}

func (c *desktopPetComponent) readyLocked(svc *AppServices) error {
	if svc == nil || svc.DesktopPetRuntimeV2 == nil || svc.Readiness == nil {
		return fmt.Errorf("desktop pet not ready")
	}
	if svc.Readiness.Snapshot().OverallStatus == readiness.StatusBlocked {
		return fmt.Errorf("readiness blocked")
	}
	if svc.DesktopPetWorker == nil || !svc.DesktopPetWorker.IsRunning() {
		return fmt.Errorf("desktop pet generation worker not running")
	}
	if svc.ProcessingWorker == nil || !svc.ProcessingWorker.IsRunning() {
		return fmt.Errorf("desktop pet processing worker not running")
	}
	if svc.QualityWorker == nil || !svc.QualityWorker.IsRunning() {
		return fmt.Errorf("desktop pet quality worker not running")
	}
	if svc.RegenerationWorker == nil || !svc.RegenerationWorker.IsRunning() {
		return fmt.Errorf("desktop pet regeneration worker not running")
	}
	if svc.BridgeRecoveryWorker == nil || !svc.BridgeRecoveryWorker.IsRunning() {
		return fmt.Errorf("desktop pet revision bridge recovery worker not running")
	}
	if svc.ReleaseRecoveryWorker == nil || !svc.ReleaseRecoveryWorker.IsRunning() {
		return fmt.Errorf("desktop pet release recovery worker not running")
	}
	if svc.BehaviorService == nil || !svc.BehaviorService.IsRunning() {
		return fmt.Errorf("desktop pet behavior service not running")
	}
	if !svc.DesktopPetRuntimeV2.IsStarted() {
		return fmt.Errorf("desktop pet runtime v2 not running")
	}
	if svc.RuntimeDomainEventConsumer == nil || !svc.RuntimeDomainEventConsumer.IsRunning() {
		return fmt.Errorf("desktop pet runtime domain event consumer not running")
	}
	return nil
}

func (c *desktopPetComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	svc := c.services
	if svc == nil {
		return nil
	}
	c.stopAllLocked(ctx, svc)
	c.started = false
	c.state = workerStartState{}
	return nil
}

func (c *desktopPetComponent) stopAllLocked(ctx context.Context, svc *AppServices) {
	// Stop event ingress before the behavior engine so no new domain event can
	// race into a service that is already draining. Runtime v2 is closed next,
	// followed by behavior and the background workers in reverse dependency order.
	if svc.RuntimeDomainEventConsumer != nil {
		svc.RuntimeDomainEventConsumer.Stop()
	}
	if svc.DesktopPetRuntimeV2 != nil {
		_ = svc.DesktopPetRuntimeV2.Close(ctx)
	}
	if c.state.behaviorOk && svc.BehaviorService != nil {
		_ = svc.BehaviorService.Stop()
	}
	if c.state.releaseRecoveryWorkerOk && svc.ReleaseRecoveryWorker != nil {
		svc.ReleaseRecoveryWorker.Stop()
	}
	if svc.BridgeRecoveryWorker != nil {
		svc.BridgeRecoveryWorker.Stop()
	}
	if svc.RegenerationWorker != nil {
		svc.RegenerationWorker.Stop()
	}
	if svc.QualityWorker != nil {
		svc.QualityWorker.Stop()
	}
	if svc.ProcessingWorker != nil {
		svc.ProcessingWorker.Stop()
	}
	if svc.DesktopPetWorker != nil {
		svc.DesktopPetWorker.Stop()
	}
}

type browserComponent struct {
	services *AppServices
	host     runtimehost.RuntimeHost
	mu       sync.Mutex
	started  bool
	enabled  bool
}

func newBrowserComponent(services *AppServices, host runtimehost.RuntimeHost) *browserComponent {
	return &browserComponent{
		services: services,
		host:     host,
		enabled:  config.AppCfg != nil && config.AppCfg.Providers.Browser.Enabled,
	}
}

func (c *browserComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentBrowser,
		Phase:        runtimeorchestrator.PhaseApplication,
		Enabled:      c.enabled,
		Required:     false,
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentExtensionKernel},
		Capabilities: []string{"browser.runtime"},
		Profiles:     profilesLocalOnly,
	}
}

func (c *browserComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started || !c.enabled {
		return nil
	}
	svc := c.services
	if svc == nil || svc.Browser == nil {
		return nil
	}
	info, err := svc.Browser.Runtime().Start(ctx)
	if err != nil {
		return fmt.Errorf("start browser: %w", err)
	}
	if info == nil {
		return fmt.Errorf("start browser: nil runtime info")
	}
	c.started = true
	return nil
}

func (c *browserComponent) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return nil
	}
	svc := c.services
	if svc == nil || svc.Browser == nil {
		return nil
	}
	health := svc.Browser.Runtime().Health(ctx)
	if health != browser.BrowserHealthHealthy {
		return fmt.Errorf("browser runtime not healthy: %s", string(health))
	}
	return nil
}

func (c *browserComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	if err := svcStopBrowser(c.services); err != nil {
		return fmt.Errorf("stop browser: %w", err)
	}
	c.started = false
	return nil
}

func svcStopBrowser(services *AppServices) error {
	if services == nil || services.Browser == nil {
		return nil
	}
	return services.Browser.Runtime().Stop(context.Background())
}
