package plugincontract

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type CatalogModule struct {
	Module   ModuleRef
	Services ServiceDeclaration
	Enabled  bool
}

type ServiceCatalog struct {
	mu        sync.RWMutex
	modules   map[string]CatalogModule
	providers map[string]string
	resolver  ServiceGraphResolver
}

func NewServiceCatalog() *ServiceCatalog {
	return &ServiceCatalog{
		modules:   make(map[string]CatalogModule),
		providers: make(map[string]string),
	}
}

func (c *ServiceCatalog) Replace(module CatalogModule) error {
	if c == nil {
		return fmt.Errorf("plugincontract: service catalog is nil")
	}
	module.Module.PluginID = strings.TrimSpace(module.Module.PluginID)
	module.Module.ModuleID = strings.TrimSpace(module.Module.ModuleID)
	key := module.Module.Key()
	if module.Module.PluginID == "" || module.Module.ModuleID == "" {
		return fmt.Errorf("plugincontract: catalog module identity is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nextProviders := cloneStringMap(c.providers)
	moduleProviderIDs := make(map[string]bool)
	for _, offer := range module.Services.Provides {
		serviceID := strings.TrimSpace(offer.ServiceID)
		if serviceID == "" {
			return fmt.Errorf("plugincontract: service id is required")
		}
		providerID := key + "/" + serviceID
		if moduleProviderIDs[providerID] {
			return fmt.Errorf("plugincontract: provider %s is duplicated", providerID)
		}
		moduleProviderIDs[providerID] = true
		if owner, exists := nextProviders[providerID]; exists && owner != key {
			return fmt.Errorf("plugincontract: provider %s already owned by %s", providerID, owner)
		}
		nextProviders[providerID] = key
	}
	for providerID, owner := range c.providers {
		if owner == key {
			delete(nextProviders, providerID)
		}
	}
	c.providers = nextProviders
	c.modules[key] = cloneCatalogModule(module)
	return nil
}

func (c *ServiceCatalog) Unregister(module ModuleRef) bool {
	if c == nil {
		return false
	}
	key := module.Key()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.modules[key]; !exists {
		return false
	}
	delete(c.modules, key)
	for providerID, owner := range c.providers {
		if owner == key {
			delete(c.providers, providerID)
		}
	}
	return true
}

func (c *ServiceCatalog) SetEnabled(module ModuleRef, enabled bool) error {
	if c == nil {
		return fmt.Errorf("plugincontract: service catalog is nil")
	}
	key := module.Key()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, exists := c.modules[key]
	if !exists {
		return fmt.Errorf("plugincontract: module %s not found", key)
	}
	entry.Enabled = enabled
	c.modules[key] = entry
	return nil
}

func (c *ServiceCatalog) Get(module ModuleRef) (CatalogModule, bool) {
	if c == nil {
		return CatalogModule{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.modules[module.Key()]
	return cloneCatalogModule(entry), exists
}

func (c *ServiceCatalog) Snapshot() []CatalogModule {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]CatalogModule, 0, len(c.modules))
	for _, entry := range c.modules {
		result = append(result, cloneCatalogModule(entry))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Module.Key() < result[j].Module.Key()
	})
	return result
}

func (c *ServiceCatalog) Clone() *ServiceCatalog {
	clone := NewServiceCatalog()
	if c == nil {
		return clone
	}
	for _, module := range c.Snapshot() {
		_ = clone.Replace(module)
	}
	return clone
}

func (c *ServiceCatalog) Resolve() ServiceGraphResolution {
	if c == nil {
		return ServiceGraphResolution{Issues: []ServiceGraphIssue{{Code: "catalog_unavailable", Detail: "service catalog is nil"}}}
	}
	c.mu.RLock()
	request := c.graphRequestUnsafe()
	c.mu.RUnlock()
	return c.resolver.Resolve(request)
}

func (c *ServiceCatalog) Bindings(module ModuleRef) []ServiceBinding {
	resolution := c.Resolve()
	if !resolution.Valid() {
		return nil
	}
	return append([]ServiceBinding(nil), resolution.Graph.Bindings[module.Key()]...)
}

func (c *ServiceCatalog) graphRequestUnsafe() ServiceGraphRequest {
	request := ServiceGraphRequest{}
	keys := make([]string, 0, len(c.modules))
	for key := range c.modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry, exists := c.modules[key]
		if !exists || !entry.Enabled {
			continue
		}
		module := entry.Module
		for _, offer := range entry.Services.Provides {
			serviceID := strings.TrimSpace(offer.ServiceID)
			request.Providers = append(request.Providers, ServiceDescriptor{
				ProviderID:   key + "/" + serviceID,
				Module:       module,
				ServiceID:    serviceID,
				Version:      strings.TrimSpace(offer.Version),
				ContractHash: strings.TrimSpace(offer.ContractHash),
				Scopes:       append([]ServiceScope(nil), offer.Scopes...),
				Placement:    offer.Placement,
				Priority:     offer.Priority,
			})
		}
		if len(entry.Services.Requires) > 0 {
			request.Consumers = append(request.Consumers, ServiceConsumer{
				Module:       module,
				Requirements: append([]ServiceRequirement(nil), entry.Services.Requires...),
			})
		}
	}
	return request
}

func cloneCatalogModule(module CatalogModule) CatalogModule {
	module.Services.Provides = append([]ServiceOffer(nil), module.Services.Provides...)
	module.Services.Requires = append([]ServiceRequirement(nil), module.Services.Requires...)
	for index := range module.Services.Provides {
		module.Services.Provides[index].Scopes = append([]ServiceScope(nil), module.Services.Provides[index].Scopes...)
	}
	return module
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
