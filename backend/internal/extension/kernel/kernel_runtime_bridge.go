package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

// RuntimeStateServiceBridge 适配 deviceruntime.Service 到 runtimeorchestrator.RuntimeStatePort 接口
type RuntimeStateServiceBridge struct {
	sessionService *deviceruntime.Service
	deviceRegistry *host_registry.Registry
}

// NewRuntimeStateServiceBridge 创建 RuntimeStateServiceBridge 实例
func NewRuntimeStateServiceBridge(sessionService *deviceruntime.Service, deviceRegistry *host_registry.Registry) *RuntimeStateServiceBridge {
	return &RuntimeStateServiceBridge{
		sessionService: sessionService,
		deviceRegistry: deviceRegistry,
	}
}

func (b *RuntimeStateServiceBridge) IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error) {
	if b.sessionService == nil {
		return false, fmt.Errorf("session service not configured")
	}
	sessions, err := b.sessionService.ListActiveSessions(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range sessions {
		if s.RuntimeID == runtimeID && s.Status == protocol.SessionStatusReady {
			return true, nil
		}
	}
	return false, nil
}

func (b *RuntimeStateServiceBridge) GetRuntimeForDevice(ctx context.Context, deviceID runtimeidentity.DeviceID) (*runtimeorchestrator.RuntimeInfo, error) {
	if b.sessionService == nil {
		return nil, fmt.Errorf("session service not configured")
	}
	sessions, err := b.sessionService.ListActiveSessions(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sessions {
		if s.DeviceID == deviceID {
			state := runtimeorchestrator.RuntimeAvailabilityOffline
			if s.Status == protocol.SessionStatusReady {
				state = runtimeorchestrator.RuntimeAvailabilityOnline
			}
			return &runtimeorchestrator.RuntimeInfo{
				RuntimeID:    s.RuntimeID,
				DeviceID:     deviceID,
				Availability: state,
				Health:       string(s.Status),
			}, nil
		}
	}
	return &runtimeorchestrator.RuntimeInfo{
		DeviceID:     deviceID,
		Availability: runtimeorchestrator.RuntimeAvailabilityOffline,
	}, nil
}

func (b *RuntimeStateServiceBridge) GetRuntimeByID(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (*runtimeorchestrator.RuntimeInfo, error) {
	if b.sessionService == nil {
		return nil, fmt.Errorf("session service not configured")
	}
	sessions, err := b.sessionService.ListActiveSessions(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sessions {
		if s.RuntimeID == runtimeID {
			state := runtimeorchestrator.RuntimeAvailabilityOffline
			if s.Status == protocol.SessionStatusReady {
				state = runtimeorchestrator.RuntimeAvailabilityOnline
			}
			return &runtimeorchestrator.RuntimeInfo{
				RuntimeID:    runtimeID,
				DeviceID:     s.DeviceID,
				Availability: state,
				Health:       string(s.Status),
			}, nil
		}
	}
	return &runtimeorchestrator.RuntimeInfo{
		RuntimeID:    runtimeID,
		Availability: runtimeorchestrator.RuntimeAvailabilityOffline,
	}, nil
}

// ProviderInstanceBridge 适配 capability.ProviderRegistry 到 runtimeorchestrator.ProviderInstanceAvailabilityPort 接口
type ProviderInstanceBridge struct {
	registry *capability.ProviderRegistry
}

// NewProviderInstanceBridge 创建 ProviderInstanceBridge 实例
func NewProviderInstanceBridge(registry *capability.ProviderRegistry) *ProviderInstanceBridge {
	return &ProviderInstanceBridge{registry: registry}
}

func (b *ProviderInstanceBridge) GetInstanceRuntimeID(ctx context.Context, instanceID capability.ProviderInstanceID) (runtimeidentity.RuntimeID, bool) {
	if b.registry == nil {
		return "", false
	}
	inst, ok := b.registry.GetInstanceByID(instanceID)
	if !ok {
		return "", false
	}
	return inst.RuntimeID, inst.RuntimeID != ""
}

func (b *ProviderInstanceBridge) IsInstanceAvailable(ctx context.Context, instanceID capability.ProviderInstanceID) bool {
	if b.registry == nil {
		return false
	}
	inst, ok := b.registry.GetInstanceByID(instanceID)
	if !ok {
		return false
	}
	return inst.IsExecutable()
}
