package builtin

import (
	"context"
	"fmt"
	"sync"

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
	Healthy             bool   `json:"healthy"`
	ISHInitialized      bool   `json:"ishInitialized"`
	RootfsInstalled     bool   `json:"rootfsInstalled"`
	RestartRequired     bool   `json:"restartRequired"`
	RecoveryPending     bool   `json:"recoveryPending"`
	RunningRootfsVersion string `json:"runningRootfsVersion"`
	ActiveRootfsVersion  string `json:"activeRootfsVersion"`
	ActiveRootfsDigest   string `json:"activeRootfsDigest"`
	LastErrorCode       string `json:"lastErrorCode"`
}

type iosSandboxProviderInstance struct {
	mu      sync.RWMutex
	backend sandbox.SandboxBackend
	host    runtimehost.RuntimeHost
	config  IOSSandboxProviderConfig

	started bool
	stopped bool

	cachedHealth sandbox.SandboxHealth
	cachedAvail  sandbox.BackendAvailability
}

func newIOSSandboxProviderInstance(
	backend sandbox.SandboxBackend,
	host runtimehost.RuntimeHost,
	config IOSSandboxProviderConfig,
) *iosSandboxProviderInstance {
	return &iosSandboxProviderInstance{
		backend:     backend,
		host:        host,
		config:      config,
		cachedAvail: sandbox.BackendUnavailable,
	}
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

	if p.started {
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

	if err := backend.Start(ctx, cfg); err != nil {
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend start: %w", err),
		}
	}

	p.started = true
	p.stopped = false
	p.cachedHealth = backend.Health(ctx)
	p.cachedAvail = backend.Availability(ctx)

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

	backend := p.backend

	if err := backend.Stop(ctx); err != nil {
		return &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend stop: %w", err),
		}
	}

	p.stopped = true
	p.started = false
	p.cachedHealth = backend.Health(ctx)
	p.cachedAvail = backend.Availability(ctx)

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

	return IOSSandboxProviderCapability{
		ProviderID:           sandbox.ProviderIDIOSSandbox,
		Slot:                 string(runtimeorchestrator.ProviderSlotIOSSandbox),
		RuntimeID:            runtimeID,
		HostPlatform:         hostPlatform,
		Availability:         p.cachedAvail.String(),
		LifecycleState:       p.lifecycleStateLocked(),
		Generation:           p.cachedHealth.Generation,
		Healthy:              p.cachedHealth.Healthy,
		ISHInitialized:       p.cachedHealth.ISHInitialized,
		RootfsInstalled:      p.cachedHealth.RootfsInstalled,
		RestartRequired:      p.cachedHealth.RestartRequired,
		RecoveryPending:      p.cachedHealth.RecoveryPending,
		RunningRootfsVersion: p.cachedHealth.RunningRootfsVersion,
		ActiveRootfsVersion:  p.cachedHealth.ActiveRootfsVersion,
		ActiveRootfsDigest:   p.cachedHealth.ActiveRootfsDigest,
		LastErrorCode:        p.cachedHealth.LastErrorCode,
	}
}

func (p *iosSandboxProviderInstance) lifecycleStateLocked() string {
	if p.stopped {
		return "stopped"
	}
	if p.started {
		if p.cachedHealth.LifecycleState != "" {
			return p.cachedHealth.LifecycleState
		}
		return "started"
	}
	return "not_started"
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
