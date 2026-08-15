package runtimeorchestrator

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimeAvailabilityState string

const (
	RuntimeAvailabilityOnline   RuntimeAvailabilityState = "online"
	RuntimeAvailabilityOffline  RuntimeAvailabilityState = "offline"
	RuntimeAvailabilityUnknown  RuntimeAvailabilityState = "unknown"
	RuntimeAvailabilityDisabled RuntimeAvailabilityState = "disabled"
)

type RuntimeInfo struct {
	RuntimeID        runtimeidentity.RuntimeID
	DeviceID         runtimeidentity.DeviceID
	Availability     RuntimeAvailabilityState
	Health           string
	ProviderInstance capability.ProviderInstanceID
}

type RuntimeStatePort interface {
	IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error)
	GetRuntimeForDevice(ctx context.Context, deviceID runtimeidentity.DeviceID) (*RuntimeInfo, error)
	GetRuntimeByID(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (*RuntimeInfo, error)
}

type ProviderInstanceAvailabilityPort interface {
	GetInstanceRuntimeID(ctx context.Context, instanceID capability.ProviderInstanceID) (runtimeidentity.RuntimeID, bool)
	IsInstanceAvailable(ctx context.Context, instanceID capability.ProviderInstanceID) bool
}

type RuntimeStateService struct {
	mu              sync.RWMutex
	runtimePort     RuntimeStatePort
	instancePort    ProviderInstanceAvailabilityPort
	offlineRuntimes map[runtimeidentity.RuntimeID]*RuntimeInfo
}

func NewRuntimeStateService(runtimePort RuntimeStatePort, instancePort ProviderInstanceAvailabilityPort) *RuntimeStateService {
	return &RuntimeStateService{
		runtimePort:     runtimePort,
		instancePort:    instancePort,
		offlineRuntimes: make(map[runtimeidentity.RuntimeID]*RuntimeInfo),
	}
}

func (s *RuntimeStateService) IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error) {
	if s.runtimePort == nil {
		return true, nil
	}
	return s.runtimePort.IsRuntimeOnline(ctx, runtimeID)
}

func (s *RuntimeStateService) IsDeviceOffline(ctx context.Context, deviceID runtimeidentity.DeviceID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.runtimePort == nil {
		return false, nil
	}
	rtInfo, err := s.runtimePort.GetRuntimeForDevice(ctx, deviceID)
	if err != nil || rtInfo == nil {
		return true, err
	}
	return rtInfo.Availability == RuntimeAvailabilityOffline, nil
}

func (s *RuntimeStateService) MarkRuntimeOffline(runtimeID runtimeidentity.RuntimeID, info *RuntimeInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offlineRuntimes[runtimeID] = info
}

func (s *RuntimeStateService) MarkRuntimeOnline(runtimeID runtimeidentity.RuntimeID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.offlineRuntimes, runtimeID)
}

func (s *RuntimeStateService) IsInstanceOnOfflineRuntime(ctx context.Context, instanceID capability.ProviderInstanceID) (bool, error) {
	if s.instancePort == nil {
		return false, nil
	}
	runtimeID, ok := s.instancePort.GetInstanceRuntimeID(ctx, instanceID)
	if !ok || runtimeID == "" {
		return false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, offline := s.offlineRuntimes[runtimeID]
	return offline, nil
}
