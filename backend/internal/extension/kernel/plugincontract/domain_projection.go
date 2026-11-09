package plugincontract

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (m Manifest) ToExtensionDefinition() (domain.ExtensionDefinition, error) {
	report := m.Validate()
	if report.HasErrors() {
		details := make([]string, 0, len(report.Issues))
		for _, issue := range report.Issues {
			details = append(details, fmt.Sprintf("%s: %s", issue.Path, issue.Message))
		}
		return domain.ExtensionDefinition{}, fmt.Errorf("%w: %s", ErrInvalidManifest, strings.Join(details, "; "))
	}
	version, _ := domain.ParseVersion(m.Plugin.Version)
	definition := domain.ExtensionDefinition{
		ID:              domain.ExtensionID(m.Plugin.ID),
		Name:            m.Plugin.Name.ToDomain(),
		Description:     m.Plugin.Description.ToDomain(),
		Version:         version,
		ManifestVersion: m.SchemaVersion,
		Placement:       domain.ExtensionPlacement(sharedPlacement(m.Modules)),
		Publisher: domain.PublisherReference{
			PublisherID: m.Plugin.Publisher.ID,
			DisplayName: m.Plugin.Publisher.DisplayName,
			TrustLevel:  "unknown",
		},
		Compatibility: domain.ExtensionCompatibility{
			MinHostVersion: m.Compatibility.MinHostVersion,
			MaxHostVersion: m.Compatibility.MaxHostVersion,
			Platforms:      append([]string(nil), m.Compatibility.Platforms...),
			FeatureFlags:   append([]string(nil), m.Compatibility.FeatureFlags...),
		},
		Integrity: domain.ExtensionIntegrity{
			Algorithm:       m.Integrity.Algorithm,
			ContentTreeHash: m.Integrity.ContentTreeHash,
			FileHashes:      cloneStringMap(m.Integrity.FileHashes),
		},
		Policies: domain.ExtensionPolicies{
			BackgroundTasks: hasBackgroundContribution(m.Modules),
			NetworkAccess:   hasNetworkAccess(m.Modules),
			Sandbox:         hasSandboxRuntime(m.Modules),
		},
	}
	if m.Data != nil {
		definition.Metadata = map[string]any{
			"data.schemaVersion": m.Data.SchemaVersion,
			"data.scopes":        append([]string(nil), m.Data.Scopes...),
			"data.backup":        m.Data.Backup,
			"data.export":        m.Data.Export,
			"data.uninstall":     m.Data.Uninstall,
		}
	}
	for index := range m.Modules {
		moduleDefinition, err := m.Modules[index].ToDomain(domain.ExtensionID(m.Plugin.ID))
		if err != nil {
			return domain.ExtensionDefinition{}, err
		}
		for contributionIndex := range moduleDefinition.Contributions {
			contribution := &moduleDefinition.Contributions[contributionIndex]
			if contribution.EffectiveAdapter() != domain.ContributionAdapterUIProvider {
				continue
			}
			if contribution.Definition == nil {
				contribution.Definition = map[string]any{}
			}
			contribution.Definition["trustLevel"] = "unknown"
			placement := string(moduleDefinition.Placement)
			if placement == "" {
				placement = string(ServicePlacementCloud)
			}
			contribution.Definition["placement"] = placement
			if moduleDefinition.DeviceRequirements != nil {
				contribution.Definition["deviceRequirements"] = map[string]any{
					"platforms":         append([]string(nil), moduleDefinition.DeviceRequirements.Platforms...),
					"architectures":     append([]string(nil), moduleDefinition.DeviceRequirements.Architectures...),
					"minAppVersion":     moduleDefinition.DeviceRequirements.MinAppVersion,
					"minRuntimeVersion": moduleDefinition.DeviceRequirements.MinRuntimeVersion,
					"requiredFeatures":  append([]string(nil), moduleDefinition.DeviceRequirements.RequiredFeatures...),
				}
			}
			if len(contribution.RequiredPermissions) > 0 {
				contribution.Definition["permissions"] = append([]string(nil), contribution.RequiredPermissions...)
			}
		}
		definition.Modules = append(definition.Modules, moduleDefinition)
	}
	for _, dependency := range m.Dependencies {
		definition.Dependencies = append(definition.Dependencies, dependencyDefinition(dependency))
	}
	for _, secret := range m.SecretRefs {
		definition.SecretRefs = append(definition.SecretRefs, domain.SecretRefDefinition{
			Ref:       secret.Ref,
			ServiceID: secret.ServiceID,
			Purpose:   secret.Purpose,
			Required:  secret.Required,
		})
	}
	resolvedDomain, err := domain.ResolveExtensionDomain(definition.AllContributions())
	if err != nil {
		return domain.ExtensionDefinition{}, err
	}
	definition.Domain = resolvedDomain
	return definition, nil
}

func (m Module) ToDomain(extensionID domain.ExtensionID) (domain.ModuleDefinition, error) {
	var runtimeDefinition *domain.RuntimeDefinition
	if m.Runtime != nil {
		timeout, _ := time.ParseDuration(m.Runtime.Timeout)
		nativeCompanions := make([]domain.NativeCompanionDefinition, 0, len(m.Runtime.NativeArtifacts))
		for _, artifact := range m.Runtime.NativeArtifacts {
			nativeCompanions = append(nativeCompanions, domain.NativeCompanionDefinition{
				ID:           artifact.ID,
				Platform:     artifact.Platform,
				Architecture: artifact.Architecture,
				Path:         artifact.Path,
				SHA256:       artifact.SHA256,
				Executable:   artifact.Executable,
				Args:         append([]string(nil), artifact.Arguments...),
			})
		}
		runtimeType := normalizeRuntimeAdapter(m.Runtime.Adapter)
		runtimeDefinition = &domain.RuntimeDefinition{
			Type:             domain.RuntimeType(runtimeType),
			ServiceID:        m.Runtime.ServiceID,
			EntryPoint:       m.Runtime.Entry,
			WorkerCount:      m.Runtime.WorkerCount,
			Timeout:          timeout,
			Memory:           m.Runtime.MemoryBytes,
			CPUPercent:       m.Runtime.CPUPercent,
			MaxSubprocesses:  m.Runtime.MaxProcesses,
			Permissions:      append([]string(nil), m.Runtime.Permissions...),
			Capabilities:     cloneBoolMap(m.Runtime.Capabilities),
			Env:              cloneStringMap(m.Runtime.Environment),
			HostAccess:       m.Runtime.HostAccess.toDomain(),
			NativeCompanions: nativeCompanions,
		}
	}
	moduleDefinition := domain.ModuleDefinition{
		ID:                 domain.ModuleID(m.ID),
		ExtensionID:        extensionID,
		Name:               m.Name.ToDomain(),
		Description:        m.Description.ToDomain(),
		Type:               domain.ModuleType(moduleTypeForRuntime(runtimeDefinition)),
		Version:            m.Version,
		Runtime:            runtimeDefinition,
		Compatibility:      deref(m.Compatibility.toDomain()),
		Policies:           deref(m.Policies.toDomain()),
		Placement:          domain.ModulePlacement(m.Placement),
		DeviceRequirements: m.DeviceRequirements.toDomain(),
	}
	for _, capability := range m.ProvidedCapabilities {
		moduleDefinition.ProvidedCapabilities = append(moduleDefinition.ProvidedCapabilities, domain.ProvidedCapability{
			ID:       capability.ID,
			Version:  capability.Version,
			Metadata: cloneMap(capability.Metadata),
		})
	}
	for _, service := range m.Services.Provides {
		moduleDefinition.ProvidedCapabilities = append(moduleDefinition.ProvidedCapabilities, domain.ProvidedCapability{
			ID:      service.ServiceID,
			Version: service.Version,
		})
	}
	if m.Provider != nil {
		moduleDefinition.Provider = &domain.ProviderMetadata{
			ID:       m.Provider.ID,
			Priority: m.Provider.Priority,
			Labels:   cloneStringMap(m.Provider.Labels),
			Metadata: cloneMap(m.Provider.Metadata),
		}
	}
	for _, contribution := range m.Contributions {
		contributionDefinition, err := contribution.ToDomain(extensionID, domain.ModuleID(m.ID))
		if err != nil {
			return domain.ModuleDefinition{}, err
		}
		moduleDefinition.Contributions = append(moduleDefinition.Contributions, contributionDefinition)
	}
	for _, dependency := range m.Dependencies {
		moduleDefinition.Dependencies = append(moduleDefinition.Dependencies, dependencyDefinition(dependency))
	}
	return moduleDefinition, nil
}

func (c ContributionDescriptor) ToDomain(extensionID domain.ExtensionID, moduleID domain.ModuleID) (domain.ContributionDefinition, error) {
	definition := domain.ContributionDefinition{
		ID:                  domain.ContributionID(c.ID),
		ModuleID:            moduleID,
		ExtensionID:         extensionID,
		Kind:                domain.ContributionKind(c.Kind),
		Adapter:             domain.ContributionAdapter(strings.TrimSpace(c.Adapter)),
		ContractVersion:     c.ContractVersion,
		Schema:              append(json.RawMessage(nil), c.Schema...),
		Name:                c.Name.ToDomain(),
		Description:         c.Description.ToDomain(),
		Version:             c.Version,
		Definition:          cloneMap(c.Spec),
		RequiredPermissions: append([]string(nil), c.RequiredPermissions...),
		RequiredScope:       append([]string(nil), c.RequiredScope...),
	}
	for _, dependency := range c.Dependencies {
		definition.Dependencies = append(definition.Dependencies, dependencyDefinition(dependency))
	}
	if c.Exposure != nil {
		definition.Exposure = domain.Exposure{
			VisibleByDefault:    c.Exposure.VisibleByDefault,
			HiddenFromDiscovery: c.Exposure.HiddenFromDiscovery,
			RequiredRoles:       append([]string(nil), c.Exposure.RequiredRoles...),
		}
	}
	if c.RuntimeBinding != nil {
		definition.RuntimeBinding = &domain.RuntimeBinding{
			RuntimeID:  domain.RuntimeID(c.RuntimeBinding.RuntimeID),
			Generation: c.RuntimeBinding.Generation,
		}
	}
	return definition, nil
}

func (c ContributionDescriptor) EffectiveAdapter() domain.ContributionAdapter {
	return domain.ContributionAdapter(strings.TrimSpace(c.Adapter))
}

func (m Module) EffectiveType() string {
	if m.Runtime == nil {
		return "data_only"
	}
	switch normalizeRuntimeAdapter(m.Runtime.Adapter) {
	case "builtin", "javascript", "wasm", "native", "service":
		return normalizeRuntimeAdapter(m.Runtime.Adapter)
	default:
		return "data_only"
	}
}

func IsSupportedModuleType(value string) bool {
	switch value {
	case "builtin", "javascript", "data_only", "wasm", "native", "service":
		return true
	default:
		return false
	}
}

func IsSupportedRuntimeAdapter(value string) bool {
	switch normalizeRuntimeAdapter(value) {
	case "javascript", "mcp", "workflow", "static", "wasm", "service", "native", "builtin":
		return true
	default:
		return false
	}
}

func (t LocalizedText) ToDomain() domain.LocalizedText {
	return domain.LocalizedText{
		Default:      t.Default,
		Translations: cloneStringMap(t.Translations),
	}
}

func (a *HostAccess) toDomain() domain.RuntimeHostAccess {
	if a == nil {
		return domain.RuntimeHostAccess{}
	}
	result := domain.RuntimeHostAccess{}
	if a.Filesystem != nil {
		result.Filesystem = domain.RuntimeFilesystemAccess{
			Mode:  strings.TrimSpace(a.Filesystem.Mode),
			Roots: append([]string(nil), a.Filesystem.Roots...),
		}
	}
	if a.Network != nil {
		result.Network = domain.RuntimeNetworkAccess{Mode: strings.TrimSpace(a.Network.Mode)}
	}
	if a.Process != nil {
		result.Process = domain.RuntimeProcessAccess{
			Mode:         strings.TrimSpace(a.Process.Mode),
			MaxProcesses: a.Process.MaxProcesses,
		}
	}
	return result
}

func (c *ModuleCompatibility) toDomain() *domain.ModuleCompatibility {
	if c == nil {
		return nil
	}
	return &domain.ModuleCompatibility{
		MinHostVersion: c.MinHostVersion,
		Platforms:      append([]string(nil), c.Platforms...),
	}
}

func (p *ModulePolicies) toDomain() *domain.ModulePolicies {
	if p == nil {
		return nil
	}
	return &domain.ModulePolicies{
		Isolation:        p.Isolation,
		NetworkAccess:    p.NetworkAccess,
		FileSystemAccess: p.FileSystemAccess,
	}
}

func (r *DeviceRequirements) toDomain() *domain.DeviceRequirements {
	if r == nil {
		return nil
	}
	return &domain.DeviceRequirements{
		Platforms:         append([]string(nil), r.Platforms...),
		Architectures:     append([]string(nil), r.Architectures...),
		MinAppVersion:     r.MinAppVersion,
		MinRuntimeVersion: r.MinRuntimeVersion,
		RequiredFeatures:  append([]string(nil), r.RequiredFeatures...),
	}
}

func dependencyDefinition(dependency PluginDependency) domain.DependencyDefinition {
	return domain.DependencyDefinition{
		Type:     domain.DependencyType(dependency.Type),
		ID:       dependency.ID,
		Version:  dependency.Version,
		Optional: dependency.Optional,
		Reason:   dependency.Reason,
	}
}

func sharedPlacement(modules []Module) string {
	if len(modules) == 0 {
		return ""
	}
	placement := string(modules[0].Placement)
	if placement == "" {
		return ""
	}
	for _, module := range modules[1:] {
		if string(module.Placement) != placement {
			return ""
		}
	}
	return placement
}

func moduleTypeForRuntime(runtime *domain.RuntimeDefinition) string {
	if runtime == nil {
		return "data_only"
	}
	switch runtime.Type {
	case domain.RuntimeTypeJavaScript, domain.RuntimeTypeService, domain.RuntimeTypeWASM, domain.RuntimeTypeBuiltin:
		return string(runtime.Type)
	case domain.RuntimeType("native"):
		return "native"
	default:
		return "data_only"
	}
}

func normalizeRuntimeAdapter(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimPrefix(normalized, "amitia.")
	switch normalized {
	case "javascript", "service", "wasm", "native", "builtin", "static":
		return normalized
	case "", "data_only":
		return "static"
	default:
		return normalized
	}
}

func hasBackgroundContribution(modules []Module) bool {
	for _, module := range modules {
		for _, contribution := range module.Contributions {
			if contribution.Adapter == "background" || contribution.Adapter == "schedule" {
				return true
			}
		}
	}
	return false
}

func hasNetworkAccess(modules []Module) bool {
	for _, module := range modules {
		if module.Runtime != nil && module.Runtime.HostAccess != nil &&
			module.Runtime.HostAccess.Network != nil &&
			module.Runtime.HostAccess.Network.Mode == "host" {
			return true
		}
	}
	return false
}

func hasSandboxRuntime(modules []Module) bool {
	for _, module := range modules {
		if module.Policies != nil && (module.Policies.Isolation == "sandbox" || module.Policies.Isolation == "process") {
			return true
		}
		if module.Runtime != nil && module.Runtime.HostAccess != nil {
			return true
		}
	}
	return false
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func cloneBoolMap(value map[string]bool) map[string]bool {
	if value == nil {
		return nil
	}
	result := make(map[string]bool, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func deref[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
