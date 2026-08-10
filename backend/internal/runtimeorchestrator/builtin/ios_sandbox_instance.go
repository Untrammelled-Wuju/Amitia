//go:build ios
// +build ios

package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

const ComponentIDIOSSandbox runtimeorchestrator.ComponentID = "provider.ios-sandbox"

type IOSSandboxProviderCapability struct {
	ProviderID          string `json:"providerId"`
	Slot                string `json:"slot"`
	RuntimeID           string `json:"runtimeId"`
	HostPlatform        string `json:"hostPlatform"`
	Availability        string `json:"availability"`
	LifecycleState      string `json:"lifecycleState"`
	Generation          uint64 `json:"generation"`
	DesiredRunning      bool   `json:"desiredRunning"`
	Healthy             bool   `json:"healthy"`
	ISHInitialized      bool   `json:"ishInitialized"`
	RootfsInstalled     bool   `json:"rootfsInstalled"`
	RestartRequired     bool   `json:"restartRequired"`
	RecoveryPending     bool   `json:"recoveryPending"`
	ActiveExecutionID   string `json:"activeExecutionId,omitempty"`
	ActiveRootfsVersion string `json:"activeRootfsVersion"`
	RunningRootfsVersion string `json:"runningRootfsVersion"`
	LastErrorCode       string `json:"lastErrorCode"`
	LastStartedAt       string `json:"lastStartedAt,omitempty"`
	LastStoppedAt       string `json:"lastStoppedAt,omitempty"`
}

type iosSandboxProviderInstance struct {
	mu         sync.RWMutex
	backend    sandbox.SandboxBackend
	host       runtimehost.RuntimeHost
	config     IOSSandboxProviderConfig
	orch       *runtimeorchestrator.RuntimeOrchestrator

	lifecycleState sandbox.SandboxLifecycleState
	desiredRunning bool
	started        bool
	stopped        bool

	cachedHealth sandbox.SandboxHealth
	cachedAvail  sandbox.BackendAvailability
}

func newIOSSandboxProviderInstance(
	backend sandbox.SandboxBackend,
	host runtimehost.RuntimeHost,
	config IOSSandboxProviderConfig,
) *iosSandboxProviderInstance {
	return &iosSandboxProviderInstance{
		backend:        backend,
		host:           host,
		config:         config,
		lifecycleState: sandbox.SandboxStateIdle,
		cachedAvail:    sandbox.BackendUnavailable,
	}
}

func (p *iosSandboxProviderInstance) SetOrchestrator(orch *runtimeorchestrator.RuntimeOrchestrator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.orch = orch
}

func (p *iosSandboxProviderInstance) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:       ComponentIDIOSSandbox,
		Phase:    runtimeorchestrator.PhaseApplication,
		Enabled:  p.config.Enabled,
		Required: false,
		Capabilities: []string{
			"platform/ios/sandbox",
			"platform/ios/ish",
			"sandbox/execute",
		},
	}
}

func (p *iosSandboxProviderInstance) Start(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started && p.lifecycleState == sandbox.SandboxStateRunning {
		return nil
	}

	if p.host == nil {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("runtime host missing"),
		}
	}

	backend := p.backend
	host := p.host
	cfg := sandbox.SandboxConfig{
		RuntimeID:    host.RuntimeInstanceID(),
		WorkspaceURI: p.config.WorkspaceURI,
		RootfsURI:    p.config.RootfsURI,
		Environment:  cloneStringMap(p.config.Environment),
	}

	p.lifecycleState = sandbox.SandboxStateStarting
	p.desiredRunning = true

	if err := backend.Start(ctx, cfg); err != nil {
		p.lifecycleState = sandbox.SandboxStateFailed
		p.reportComponentStateLocked()
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend start: %w", err),
		}
	}

	p.started = true
	p.stopped = false
	p.lifecycleState = sandbox.SandboxStateRunning
	p.cachedHealth = backend.Health(ctx)
	p.cachedAvail = backend.Availability(ctx)
	p.reportComponentStateLocked()

	return nil
}

func (p *iosSandboxProviderInstance) Ready(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.RLock()
	localStarted := p.started
	p.mu.RUnlock()

	if !localStarted {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("provider not started"),
		}
	}

	health := p.backend.Health(ctx)

	p.mu.Lock()
	p.cachedHealth = health
	p.mu.Unlock()

	if !health.Healthy {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("provider not ready: %s", health.Message),
		}
	}

	if health.LifecycleState != string(sandbox.SandboxStateRunning) {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("sandbox not in running state: %s", health.LifecycleState),
		}
	}

	if !health.ISHInitialized {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("iSH runtime not fully initialized"),
		}
	}

	if !health.RootfsInstalled {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("rootfs not installed"),
		}
	}

	if health.RunningRootfsVersion == "" {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("no running rootfs version"),
		}
	}

	return nil
}

func (p *iosSandboxProviderInstance) Stop(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped || !p.started {
		return nil
	}

	reason := sandbox.StopReasonApplicationShutdown
	if !p.desiredRunning {
		reason = sandbox.StopReasonUser
	}

	backend := p.backend
	p.lifecycleState = sandbox.SandboxStateStopping

	if err := backend.Stop(ctx, reason); err != nil {
		p.lifecycleState = sandbox.SandboxStateFailed
		p.reportComponentStateLocked()
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend stop: %w", err),
		}
	}

	p.stopped = true
	p.started = false
	p.lifecycleState = sandbox.SandboxStateIdle
	p.desiredRunning = false
	p.cachedHealth = backend.Health(ctx)
	p.cachedAvail = backend.Availability(ctx)
	p.reportComponentStateLocked()

	return nil
}

func (p *iosSandboxProviderInstance) Restart(ctx context.Context) error {
	if !p.config.Enabled {
		return &SandboxError{
			Code:  SandboxErrDisabled,
			Cause: fmt.Errorf("provider disabled"),
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("provider not started"),
		}
	}

	reason := sandbox.RestartReasonManual
	if len(p.cachedHealth.ActiveRootfsVersion) > 0 && p.cachedHealth.ActiveRootfsVersion != p.cachedHealth.RunningRootfsVersion {
		reason = sandbox.RestartReasonRootfsChanged
	}

	if err := p.backend.Restart(ctx, reason); err != nil {
		p.lifecycleState = sandbox.SandboxStateFailed
		p.reportComponentStateLocked()
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend restart: %w", err),
		}
	}

	p.lifecycleState = sandbox.SandboxStateRunning
	p.cachedHealth = p.backend.Health(ctx)
	p.cachedAvail = p.backend.Availability(ctx)
	p.reportComponentStateLocked()

	return nil
}

func (p *iosSandboxProviderInstance) Quiesce(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lifecycleState != sandbox.SandboxStateRunning {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("can only quiesce when running"),
		}
	}

	if err := p.backend.Quiesce(ctx); err != nil {
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend quiesce: %w", err),
		}
	}

	p.lifecycleState = sandbox.SandboxStateQuiesced
	return nil
}

func (p *iosSandboxProviderInstance) Resume(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lifecycleState != sandbox.SandboxStateQuiesced {
		return &SandboxError{
			Code:  SandboxErrNotReady,
			Cause: fmt.Errorf("can only resume when quiesced"),
		}
	}

	if err := p.backend.Resume(ctx); err != nil {
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend resume: %w", err),
		}
	}

	p.lifecycleState = sandbox.SandboxStateRunning
	p.cachedHealth = p.backend.Health(ctx)
	return nil
}

func (p *iosSandboxProviderInstance) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSSandbox
}

func (p *iosSandboxProviderInstance) ProviderID() string {
	return sandbox.ProviderIDIOSSandbox
}

func (p *iosSandboxProviderInstance) Capability() any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	runtimeID := ""
	hostPlatform := ""

	if p.host != nil {
		runtimeID = p.host.RuntimeInstanceID()
		hostPlatform = string(p.host.Descriptor().Host)
	}

	health := p.cachedHealth
	result := health.LastStartedAt
	if result.IsZero() && p.started {
		result = time.Now()
	}
	lastStarted := ""
	if !result.IsZero() {
		lastStarted = result.Format(time.RFC3339)
	}

	return IOSSandboxProviderCapability{
		ProviderID:           sandbox.ProviderIDIOSSandbox,
		Slot:                 string(runtimeorchestrator.ProviderSlotIOSSandbox),
		RuntimeID:            runtimeID,
		HostPlatform:         hostPlatform,
		Availability:         p.cachedAvail.String(),
		LifecycleState:       string(p.lifecycleState),
		Generation:           health.Generation,
		DesiredRunning:       p.desiredRunning,
		Healthy:              health.Healthy,
		ISHInitialized:       health.ISHInitialized,
		RootfsInstalled:      health.RootfsInstalled,
		RestartRequired:      health.RestartRequired,
		RecoveryPending:      health.RecoveryPending,
		ActiveExecutionID:    health.ActiveExecutionID,
		ActiveRootfsVersion:  health.ActiveRootfsVersion,
		RunningRootfsVersion: health.RunningRootfsVersion,
		LastErrorCode:        health.LastErrorCode,
		LastStartedAt:        lastStarted,
	}
}

func (p *iosSandboxProviderInstance) ReportComponentState(state runtimeorchestrator.ComponentState, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reportComponentStateLocked()
}

func (p *iosSandboxProviderInstance) reportComponentStateLocked() {
	if p.orch == nil {
		return
	}

	var state runtimeorchestrator.ComponentState
	switch p.lifecycleState {
	case sandbox.SandboxStateRunning:
		state = runtimeorchestrator.StateReady
	case sandbox.SandboxStateFailed, sandbox.SandboxStateRecoveryPending:
		state = runtimeorchestrator.StateDegraded
	case sandbox.SandboxStateIdle:
		if p.stopped {
			state = runtimeorchestrator.StateStopped
		} else {
			state = runtimeorchestrator.StateRegistered
		}
	default:
		state = runtimeorchestrator.StateDegraded
	}

	p.orch.ReportComponentState(ComponentIDIOSSandbox, state, nil)
}

func (p *iosSandboxProviderInstance) PrepareShutdown(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.RLock()
	wasRunning := p.lifecycleState == sandbox.SandboxStateRunning
	p.mu.RUnlock()

	if wasRunning {
		return p.backend.Quiesce(ctx)
	}
	return nil
}

func (p *iosSandboxProviderInstance) RecoverySnapshot(ctx context.Context) sandbox.SandboxRecoverySnapshot {
	p.mu.RLock()
	localBackend := p.backend
	p.mu.RUnlock()
	return localBackend.RecoverySnapshot(ctx)
}

func (p *iosSandboxProviderInstance) Recover(ctx context.Context) error {
	if !p.config.Enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.desiredRunning || !p.started {
		return nil
	}

	if err := p.backend.Recover(ctx); err != nil {
		p.lifecycleState = sandbox.SandboxStateFailed
		p.reportComponentStateLocked()
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend recover: %w", err),
		}
	}

	p.lifecycleState = sandbox.SandboxStateRunning
	p.cachedHealth = p.backend.Health(ctx)
	p.cachedAvail = p.backend.Availability(ctx)
	p.reportComponentStateLocked()
	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}

	dst := make(map[string]string, len(src))

	for k, v := range src {
		dst[k] = v
	}

	return dst
}
