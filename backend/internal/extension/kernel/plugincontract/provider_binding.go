package plugincontract

import (
	"fmt"
	"sort"
	"strings"
)

type ProviderBinding struct {
	ProviderID          string           `json:"providerId"`
	PluginID            string           `json:"pluginId"`
	ModuleID            string           `json:"moduleId"`
	ServiceID           string           `json:"serviceId"`
	Version             string           `json:"version,omitempty"`
	ContractHash        string           `json:"contractHash,omitempty"`
	Scopes              []ServiceScope   `json:"scopes"`
	Placement           ServicePlacement `json:"placement,omitempty"`
	Priority            int              `json:"priority,omitempty"`
	Enabled             bool             `json:"enabled"`
	Platforms           []string         `json:"platforms"`
	RequiredPermissions []string         `json:"requiredPermissions"`
	MinContractVersion  string           `json:"minContractVersion,omitempty"`
}

type bindingRequirements struct {
	Platforms           []string
	RequiredPermissions []string
	MinContractVersion  string
}

func newProviderBinding(module ModuleRef, offer ServiceOffer, enabled bool) ProviderBinding {
	return newProviderBindingWithRequirements(module, offer, enabled, bindingRequirements{})
}

func newProviderBindingWithRequirements(module ModuleRef, offer ServiceOffer, enabled bool, requirements bindingRequirements) ProviderBinding {
	return ProviderBinding{
		ProviderID:          composeProviderID(module, offer.ServiceID),
		PluginID:            strings.TrimSpace(module.PluginID),
		ModuleID:            strings.TrimSpace(module.ModuleID),
		ServiceID:           strings.TrimSpace(offer.ServiceID),
		Version:             strings.TrimSpace(offer.Version),
		ContractHash:        strings.TrimSpace(offer.ContractHash),
		Scopes:              append([]ServiceScope(nil), offer.Scopes...),
		Placement:           offer.Placement,
		Priority:            offer.Priority,
		Enabled:             enabled,
		Platforms:           append([]string(nil), requirements.Platforms...),
		RequiredPermissions: append([]string(nil), requirements.RequiredPermissions...),
		MinContractVersion:  requirements.MinContractVersion,
	}
}

func (b ProviderBinding) clone() ProviderBinding {
	b.Scopes = append([]ServiceScope(nil), b.Scopes...)
	b.Platforms = append([]string(nil), b.Platforms...)
	b.RequiredPermissions = append([]string(nil), b.RequiredPermissions...)
	return b
}

func moduleBindingRequirements(manifest Manifest, module Module) bindingRequirements {
	platforms := declarationPlatforms(module)
	return bindingRequirements{
		Platforms:           platforms,
		RequiredPermissions: requiredPermissionNames(manifest, module),
		MinContractVersion:  minContractVersion(manifest, module),
	}
}

func declarationPlatforms(module Module) []string {
	if module.DeviceRequirements != nil && len(module.DeviceRequirements.Platforms) > 0 {
		return normalizePlatforms(module.DeviceRequirements.Platforms)
	}
	if module.Compatibility != nil && len(module.Compatibility.Platforms) > 0 {
		return normalizePlatforms(module.Compatibility.Platforms)
	}
	return nil
}

func normalizePlatforms(platforms []string) []string {
	normalized := make([]string, 0, len(platforms))
	seen := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		value := strings.TrimSpace(platform)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func requiredPermissionNames(manifest Manifest, module Module) []string {
	names := make([]string, 0, len(module.Permissions)+len(manifest.Permissions))
	seen := make(map[string]bool, len(module.Permissions)+len(manifest.Permissions))
	for _, permissions := range [][]Permission{module.Permissions, manifest.Permissions} {
		for _, permission := range permissions {
			if !permission.Required {
				continue
			}
			name := strings.TrimSpace(permission.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func minContractVersion(manifest Manifest, module Module) string {
	if module.Compatibility != nil {
		if version := strings.TrimSpace(module.Compatibility.MinHostVersion); version != "" {
			return version
		}
	}
	return strings.TrimSpace(manifest.Compatibility.MinHostVersion)
}

func (m Manifest) CatalogModules(enabled bool) []CatalogModule {
	modules := make([]CatalogModule, 0, len(m.Modules))
	for _, module := range m.Modules {
		modules = append(modules, CatalogModule{
			Module:   ModuleRef{PluginID: m.Plugin.ID, ModuleID: module.ID},
			Services: module.Services,
			Enabled:  enabled,
		})
	}
	return modules
}

func (m Manifest) ProviderBindings() []ProviderBinding {
	bindings := make([]ProviderBinding, 0, len(m.Modules))
	for _, module := range m.Modules {
		moduleRef := ModuleRef{PluginID: m.Plugin.ID, ModuleID: module.ID}
		requirements := moduleBindingRequirements(m, module)
		for _, offer := range module.Services.Provides {
			bindings = append(bindings, newProviderBindingWithRequirements(moduleRef, offer, true, requirements))
		}
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ProviderID < bindings[j].ProviderID })
	return bindings
}

func (c *ServiceCatalog) ProviderBindings() []ProviderBinding {
	if c == nil {
		return nil
	}
	modules := c.Snapshot()
	bindings := make([]ProviderBinding, 0, len(modules))
	for _, module := range modules {
		for _, offer := range module.Services.Provides {
			if strings.TrimSpace(offer.ServiceID) == "" {
				continue
			}
			bindings = append(bindings, newProviderBinding(module.Module, offer, module.Enabled))
		}
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ProviderID < bindings[j].ProviderID })
	return bindings
}

func ServiceCatalogFromModules(modules []CatalogModule) (*ServiceCatalog, error) {
	catalog := NewServiceCatalog()
	for _, module := range modules {
		if err := catalog.Replace(module); err != nil {
			return nil, err
		}
	}
	if resolution := catalog.Resolve(); !resolution.Valid() {
		return nil, fmt.Errorf("plugin service graph is invalid: %+v", resolution.Issues)
	}
	return catalog, nil
}

type ProviderBindingDiff struct {
	Added   []ProviderBinding
	Removed []ProviderBinding
	Changed []ProviderBindingChange
}

type ProviderBindingChange struct {
	Binding ProviderBinding
	Fields  []string
}

func (d ProviderBindingDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

func (d ProviderBindingDiff) Touched() int {
	return len(d.Added) + len(d.Removed) + len(d.Changed)
}

func DiffProviderBindings(previous, next []ProviderBinding) ProviderBindingDiff {
	previousByID := make(map[string]ProviderBinding, len(previous))
	for _, binding := range previous {
		previousByID[binding.ProviderID] = binding
	}
	nextByID := make(map[string]ProviderBinding, len(next))
	for _, binding := range next {
		nextByID[binding.ProviderID] = binding
	}
	diff := ProviderBindingDiff{}
	for id, binding := range nextByID {
		old, exists := previousByID[id]
		if !exists {
			diff.Added = append(diff.Added, binding.clone())
			continue
		}
		if fields := changedBindingFields(old, binding); len(fields) > 0 {
			diff.Changed = append(diff.Changed, ProviderBindingChange{Binding: binding.clone(), Fields: fields})
		}
	}
	for id, binding := range previousByID {
		if _, exists := nextByID[id]; !exists {
			diff.Removed = append(diff.Removed, binding.clone())
		}
	}
	sort.Slice(diff.Added, func(i, j int) bool { return diff.Added[i].ProviderID < diff.Added[j].ProviderID })
	sort.Slice(diff.Removed, func(i, j int) bool { return diff.Removed[i].ProviderID < diff.Removed[j].ProviderID })
	sort.Slice(diff.Changed, func(i, j int) bool { return diff.Changed[i].Binding.ProviderID < diff.Changed[j].Binding.ProviderID })
	return diff
}

func changedBindingFields(previous, next ProviderBinding) []string {
	fields := make([]string, 0, 6)
	if previous.Version != next.Version {
		fields = append(fields, "version")
	}
	if previous.ContractHash != next.ContractHash {
		fields = append(fields, "contractHash")
	}
	if previous.Placement != next.Placement {
		fields = append(fields, "placement")
	}
	if !sameScopes(previous.Scopes, next.Scopes) {
		fields = append(fields, "scopes")
	}
	if previous.Priority != next.Priority {
		fields = append(fields, "priority")
	}
	if previous.Enabled != next.Enabled {
		fields = append(fields, "enabled")
	}
	if !sameStringSlices(previous.Platforms, next.Platforms) {
		fields = append(fields, "platforms")
	}
	if !sameStringSlices(previous.RequiredPermissions, next.RequiredPermissions) {
		fields = append(fields, "requiredPermissions")
	}
	if previous.MinContractVersion != next.MinContractVersion {
		fields = append(fields, "minContractVersion")
	}
	return fields
}

func sameStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameScopes(left, right []ServiceScope) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
