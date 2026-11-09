package plugincontract

import (
	"context"
	"fmt"
	"sync"
)

type ContributionRegistration interface {
	ID() string
}

type ContributionHandler interface {
	Build(context.Context, ModuleRef, ContributionDescriptor) (ContributionRegistration, error)
	Activate(context.Context, ContributionRegistration) error
	Deactivate(context.Context, ContributionRegistration) error
	Discard(context.Context, ContributionRegistration) error
}

type ContributionRouter struct {
	mu       sync.RWMutex
	adapters *AdapterRegistry
	routes   map[string]ContributionHandler
}

func NewContributionRouter(adapters *AdapterRegistry) *ContributionRouter {
	return &ContributionRouter{
		adapters: adapters,
		routes:   make(map[string]ContributionHandler),
	}
}

func (r *ContributionRouter) Register(adapterID string, contractVersion int, handler ContributionHandler) error {
	if r == nil || r.adapters == nil {
		return fmt.Errorf("plugincontract: adapter registry is unavailable")
	}
	if handler == nil {
		return fmt.Errorf("plugincontract: contribution handler is required")
	}
	descriptor, ok := r.adapters.Get(adapterID, contractVersion)
	if !ok {
		return fmt.Errorf("plugincontract: adapter %s v%d is not registered", adapterID, contractVersion)
	}
	key := contributionRouteKey(descriptor.ID, descriptor.ContractVersion)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.routes[key]; exists {
		return fmt.Errorf("plugincontract: adapter route %s already registered", key)
	}
	r.routes[key] = handler
	return nil
}

func (r *ContributionRouter) Unregister(adapterID string, contractVersion int) bool {
	if r == nil {
		return false
	}
	key := contributionRouteKey(adapterID, contractVersion)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.routes[key]; !exists {
		return false
	}
	delete(r.routes, key)
	return true
}

func (r *ContributionRouter) Has(adapterID string, contractVersion int) bool {
	if r == nil {
		return false
	}
	key := contributionRouteKey(adapterID, contractVersion)
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.routes[key]
	return exists
}

func (r *ContributionRouter) Resolve(contribution ContributionDescriptor) (ContributionHandler, error) {
	if r == nil || r.adapters == nil {
		return nil, fmt.Errorf("plugincontract: contribution router is unavailable")
	}
	descriptor, ok := r.adapters.Get(contribution.Adapter, contribution.ContractVersion)
	if !ok {
		return nil, fmt.Errorf("plugincontract: adapter %s v%d is not registered", contribution.Adapter, contribution.ContractVersion)
	}
	supported := false
	for _, kind := range descriptor.SupportedKinds {
		if kind == contribution.Kind {
			supported = true
			break
		}
	}
	if !supported {
		return nil, fmt.Errorf("plugincontract: adapter %s v%d does not support kind %s", contribution.Adapter, contribution.ContractVersion, contribution.Kind)
	}
	r.mu.RLock()
	handler := r.routes[contributionRouteKey(contribution.Adapter, contribution.ContractVersion)]
	r.mu.RUnlock()
	if handler == nil {
		return nil, fmt.Errorf("plugincontract: contribution handler %s v%d is not registered", contribution.Adapter, contribution.ContractVersion)
	}
	return handler, nil
}

func (r *ContributionRouter) BuildAndActivate(ctx context.Context, module ModuleRef, contribution ContributionDescriptor) (ContributionRegistration, error) {
	handler, err := r.Resolve(contribution)
	if err != nil {
		return nil, err
	}
	registration, err := handler.Build(ctx, module, contribution)
	if err != nil {
		return nil, err
	}
	if registration == nil {
		return nil, fmt.Errorf("plugincontract: adapter %s returned nil registration", contribution.Adapter)
	}
	if err := handler.Activate(ctx, registration); err != nil {
		_ = handler.Discard(ctx, registration)
		return nil, err
	}
	return registration, nil
}

func (r *ContributionRouter) DeactivateAndDiscard(ctx context.Context, module ModuleRef, contribution ContributionDescriptor, registration ContributionRegistration) error {
	handler, err := r.Resolve(contribution)
	if err != nil {
		return err
	}
	if err := handler.Deactivate(ctx, registration); err != nil {
		_ = handler.Discard(ctx, registration)
		return err
	}
	return handler.Discard(ctx, registration)
}

func contributionRouteKey(adapterID string, contractVersion int) string {
	return fmt.Sprintf("%s@%d", adapterID, contractVersion)
}
