package javascript_main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

type RuntimeFactory struct {
	mu      sync.Mutex
	hosts   map[string]*PluginHost
	history []*PluginHost
	hostAPI host_api.Gateway
}

func NewRuntimeFactory() *RuntimeFactory {
	return &RuntimeFactory{
		hosts:   make(map[string]*PluginHost),
		history: make([]*PluginHost, 0),
	}
}

func (f *RuntimeFactory) SetHostAPI(gateway host_api.Gateway) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hostAPI = gateway
}

type CreateHostRequest struct {
	ExtensionID          string
	ModuleID             string
	Entry                string
	DefinitionHash       string
	HostAPIVersion       string
	SessionToken         string
	Generation           int
	AllowedContributions []AllowedContribution
	ResourceLimits       runtime.ResourceLimits
}

func (f *RuntimeFactory) Create(ctx context.Context, req CreateHostRequest) (*PluginHost, error) {
	if req.ExtensionID == "" {
		return nil, errors.New("javascript_main: extension id required")
	}
	if req.ModuleID == "" {
		return nil, errors.New("javascript_main: module id required")
	}
	if req.Entry == "" {
		return nil, errors.New("javascript_main: entry required")
	}

	instanceID := fmt.Sprintf("inst-%s-%s-%d", req.ExtensionID, req.ModuleID, time.Now().UnixNano())

	f.mu.Lock()
	if existing, exists := f.hosts[instanceID]; exists {
		f.mu.Unlock()
		return existing, nil
	}
	f.mu.Unlock()

	spec := runtime.BootstrapSpec{
		InstanceID:           instanceID,
		ExtensionID:          req.ExtensionID,
		ModuleID:             req.ModuleID,
		DefinitionHash:       req.DefinitionHash,
		Generation:           req.Generation,
		Entry:                req.Entry,
		HostAPIVersion:       req.HostAPIVersion,
		ResourceLimits:       req.ResourceLimits,
		SessionToken:         req.SessionToken,
		AllowedContributions: nil,
	}

	boundary := runtime.DefaultProcessBoundary()
	boundary.ResourceLimits = req.ResourceLimits

	f.mu.Lock()
	gateway := f.hostAPI
	f.mu.Unlock()

	host, err := NewPluginHost(PluginHostConfig{
		InstanceID:           instanceID,
		ExtensionID:          req.ExtensionID,
		ModuleID:             req.ModuleID,
		BootstrapSpec:        spec,
		ProcessBoundary:      boundary,
		DefinitionHash:       req.DefinitionHash,
		HostAPIVersion:       req.HostAPIVersion,
		AllowedContributions: req.AllowedContributions,
		HostAPI:              gateway,
	})
	if err != nil {
		return nil, fmt.Errorf("javascript_main: create host failed: %w", err)
	}

	f.mu.Lock()
	f.hosts[instanceID] = host
	f.history = append(f.history, host)
	f.mu.Unlock()

	return host, nil
}

func (f *RuntimeFactory) Get(instanceID string) (*PluginHost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	host, exists := f.hosts[instanceID]
	if !exists {
		return nil, fmt.Errorf("javascript_main: host %s not found", instanceID)
	}
	return host, nil
}

func (f *RuntimeFactory) List() []*PluginHost {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*PluginHost, 0, len(f.hosts))
	for _, h := range f.hosts {
		result = append(result, h)
	}
	return result
}

func (f *RuntimeFactory) Remove(instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.hosts[instanceID]; !exists {
		return fmt.Errorf("javascript_main: host %s not found", instanceID)
	}
	delete(f.hosts, instanceID)
	return nil
}

func (f *RuntimeFactory) ActiveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, h := range f.hosts {
		if h.State() == HostStateReady {
			count++
		}
	}
	return count
}

func (f *RuntimeFactory) StopAll(ctx context.Context, reason string) []error {
	f.mu.Lock()
	hosts := make([]*PluginHost, 0, len(f.hosts))
	for _, h := range f.hosts {
		hosts = append(hosts, h)
	}
	f.mu.Unlock()

	var errs []error
	for _, h := range hosts {
		if err := h.Stop(ctx, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (f *RuntimeFactory) History() []*PluginHost {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*PluginHost, len(f.history))
	copy(out, f.history)
	return out
}
