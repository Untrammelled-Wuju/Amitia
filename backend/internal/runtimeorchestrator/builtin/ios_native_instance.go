//go:build ios
// +build ios

package builtin

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

const ComponentIDIOSNative runtimeorchestrator.ComponentID = "provider.ios-native"

type IOSNativeProviderCapability struct {
	ProviderID   string                    `json:"providerId"`
	Slot         string                    `json:"slot"`
	RuntimeID    string                    `json:"runtimeId"`
	HostPlatform string                    `json:"hostPlatform"`
	Healthy      bool                      `json:"healthy"`
	BridgeReady  bool                      `json:"bridgeReady"`
	Generation   nativebridge.HostGeneration `json:"generation"`
}

type iosNativeProviderInstance struct {
	mu         sync.RWMutex
	bridge     nativebridge.Bridge
	host       runtimehost.RuntimeHost
	orch       *runtimeorchestrator.RuntimeOrchestrator
	healthy    bool
	generation nativebridge.HostGeneration
}

func newIOSNativeProviderInstance(
	bridge nativebridge.Bridge,
	host runtimehost.RuntimeHost,
) *iosNativeProviderInstance {
	return &iosNativeProviderInstance{
		bridge:     bridge,
		host:       host,
		generation: nativebridge.HostGenerationZero,
	}
}

func (p *iosNativeProviderInstance) SetBridge(bridge nativebridge.Bridge) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bridge = bridge
	p.generation++
	if p.orch != nil {
		p.reportComponentStateLocked()
	}
}

func (p *iosNativeProviderInstance) SetOrchestrator(orch *runtimeorchestrator.RuntimeOrchestrator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.orch = orch
}

func (p *iosNativeProviderInstance) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:       ComponentIDIOSNative,
		Phase:    runtimeorchestrator.PhaseApplication,
		Enabled:  true,
		Required: false,
		Capabilities: []string{
			"platform/ios/native",
			"platform/ios/health",
		},
	}
}

func (p *iosNativeProviderInstance) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bridge != nil {
		p.healthy = p.bridge.Health(ctx) == nativebridge.HealthReady
	}

	p.reportComponentStateLocked()
	return nil
}

func (p *iosNativeProviderInstance) Ready(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bridge == nil {
		return &IOSNativeError{
			Code:  IOSNativeErrBridgeRequired,
			Cause: fmt.Errorf("native bridge is not available"),
		}
	}

	p.healthy = p.bridge.Health(ctx) == nativebridge.HealthReady
	return nil
}

func (p *iosNativeProviderInstance) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.healthy = false
	p.reportComponentStateLocked()
	return nil
}

func (p *iosNativeProviderInstance) Restart(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bridge != nil {
		p.healthy = p.bridge.Health(ctx) == nativebridge.HealthReady
	}
	p.reportComponentStateLocked()
	return nil
}

func (p *iosNativeProviderInstance) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSNative
}

func (p *iosNativeProviderInstance) ProviderID() string {
	return "ios-native"
}

func (p *iosNativeProviderInstance) Capability() any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	runtimeID := ""
	hostPlatform := ""
	if p.host != nil {
		runtimeID = p.host.RuntimeInstanceID()
		hostPlatform = string(p.host.Descriptor().Host)
	}

	return IOSNativeProviderCapability{
		ProviderID:   "ios-native",
		Slot:         string(runtimeorchestrator.ProviderSlotIOSNative),
		RuntimeID:    runtimeID,
		HostPlatform: hostPlatform,
		Healthy:      p.healthy,
		BridgeReady:  p.bridge != nil,
		Generation:   p.generation,
	}
}

func (p *iosNativeProviderInstance) ReportComponentState(state runtimeorchestrator.ComponentState, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reportComponentStateLocked()
}

func (p *iosNativeProviderInstance) reportComponentStateLocked() {
	if p.orch == nil {
		return
	}

	var state runtimeorchestrator.ComponentState
	if p.healthy {
		state = runtimeorchestrator.StateReady
	} else if p.bridge != nil {
		state = runtimeorchestrator.StateDegraded
	} else {
		state = runtimeorchestrator.StateRegistered
	}

	p.orch.ReportComponentState(ComponentIDIOSNative, state, nil)
}

var _ runtimeorchestrator.ProviderInstance = (*iosNativeProviderInstance)(nil)

var _ capability.IOSProvider = (*iosNativeProviderInstance)(nil)

func (p *iosNativeProviderInstance) Execute(ctx context.Context, request capability.IOSBridgeRequest) capability.IOSBridgeResponse {
	p.mu.RLock()
	bridge := p.bridge
	p.mu.RUnlock()

	if bridge == nil {
		return capability.IOSBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.IOSError{
				Code:    nativebridge.ErrProviderUnavailable,
				Message: "ios native bridge is not available",
			},
		}
	}

	req := nativebridge.Request{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Platform:        "ios",
		Operation:       request.Operation,
		Payload:         request.Payload,
	}

	resp, err := bridge.Execute(ctx, req)
	if err != nil {
		return capability.IOSBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.IOSError{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: err.Error(),
			},
		}
	}

	return capability.IOSBridgeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		RequestID:       resp.RequestID,
		Status:          resp.Status,
		Result:          resp.Result,
		Error: func() *capability.IOSError {
			if resp.Error == nil {
				return nil
			}
			return &capability.IOSError{
				Code:       resp.Error.Code,
				Message:    resp.Error.Message,
				DomainCode: resp.Error.DomainCode,
			}
		}(),
	}
}

func (p *iosNativeProviderInstance) Health(ctx context.Context) capability.HealthStatus {
	p.mu.RLock()
	bridge := p.bridge
	p.mu.RUnlock()

	if bridge == nil {
		return capability.HealthUnhealthy
	}

	h := bridge.Health(ctx)
	switch h {
	case nativebridge.HealthReady:
		return capability.HealthReady
	case nativebridge.HealthUnhealthy:
		return capability.HealthUnhealthy
	default:
		return capability.HealthUnknown
	}
}
