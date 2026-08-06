package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
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
	host runtimehost.RuntimeHost
	mu   sync.Mutex
}

func newSidecarComponent(host runtimehost.RuntimeHost) *sidecarComponent {
	return &sidecarComponent{host: host}
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
	}
}

func (s *sidecarComponent) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	supervisor := s.host.Processes()

	wechatEnabled := config.AppCfg.Components.Sidecars.Wechat.Enabled
	qqEnabled := config.AppCfg.Components.Sidecars.QQ.Enabled

	if wechatEnabled {
		spec, err := buildWeChatSidecarSpec(s.host.RuntimeInstanceID())
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
		spec, err := buildQQSidecarSpec(s.host.RuntimeInstanceID())
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
	if container.EventService != nil {
		if err := container.EventService.Start(ctx); err != nil {
			return fmt.Errorf("event service: %w", err)
		}
	}
	if container.ScheduleService != nil {
		if err := container.ScheduleService.Start(ctx); err != nil {
			if container.EventService != nil {
				container.EventService.Stop()
			}
			return fmt.Errorf("schedule service: %w", err)
		}
	}
	if container.ArtifactMaintenance != nil {
		if err := container.ArtifactMaintenance.Start(ctx); err != nil {
			if container.ScheduleService != nil {
				container.ScheduleService.Shutdown(ctx)
			}
			if container.EventService != nil {
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
	if container.EventService == nil || container.ScheduleService == nil || container.ArtifactMaintenance == nil {
		return fmt.Errorf("kernel sub-services not ready")
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
	runtimeOk               bool
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
	}
}

func (c *desktopPetComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	if svc.DesktopPetRuntime != nil {
		if err := svc.DesktopPetRuntime.Start(ctx); err != nil {
			if svc.SafeMode != nil {
				svc.SafeMode.Enter("pet runtime start failed")
			}
			return fmt.Errorf("runtime start: %w", err)
		}
		c.state.runtimeOk = true
	}
	startWorkerAsync(svc.DesktopPetWorker)
	startWorkerAsync(svc.ProcessingWorker)
	startWorkerAsync(svc.QualityWorker)
	startWorkerAsync(svc.RegenerationWorker)
	startWorkerAsync(svc.BridgeRecoveryWorker)
	if svc.ReleaseRecoveryWorker != nil {
		svc.ReleaseRecoveryWorker.Start(ctx)
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
	c.started = true
	return nil
}

func startWorkerAsync(ctx context.Context, w interface{ Start(ctx context.Context) }, onPanic func(name string, r any)) {
	if w == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(fmt.Sprintf("%T", w), r)
				}
			}
		}()
		w.Start(ctx)
	}()
}

func (c *desktopPetComponent) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	svc := c.services
	if svc == nil || svc.DesktopPetRuntime == nil || svc.Readiness == nil {
		return fmt.Errorf("desktop pet not ready")
	}
	if svc.Readiness.Snapshot().OverallStatus == readiness.StatusBlocked {
		return fmt.Errorf("readiness blocked")
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
	if c.state.runtimeOk && svc.DesktopPetRuntime != nil {
		_ = svc.DesktopPetRuntime.Close(ctx)
	}
}
