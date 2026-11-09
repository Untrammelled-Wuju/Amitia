package plugincontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var (
	pluginIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(\.[a-z0-9-]+)+(/[a-z0-9][a-z0-9-]+)?$`)
	moduleIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	serviceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	adapterIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	kindPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	hashPattern      = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
)

func Parse(data []byte) (*Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("plugincontract: decode manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("plugincontract: manifest contains multiple JSON values")
		}
		return nil, fmt.Errorf("plugincontract: decode trailing data: %w", err)
	}
	return &manifest, nil
}

func (m *Manifest) Validate() ValidationReport {
	var report ValidationReport
	if m == nil {
		report.Add("", "manifest_required", "manifest is required")
		return report
	}
	if m.SchemaVersion != SchemaVersion {
		report.Add("schemaVersion", "schema_version", fmt.Sprintf("expected schema version %d", SchemaVersion))
	}
	validateLocalized("plugin.name", m.Plugin.Name, true, &report)
	validateLocalized("plugin.description", m.Plugin.Description, false, &report)
	validateReservedMetadata("plugin.metadata", m.Plugin.Metadata, &report)
	if !pluginIDPattern.MatchString(m.Plugin.ID) {
		report.Add("plugin.id", "plugin_id", "plugin id must match reverse-domain/name")
	}
	if _, err := domain.ParseVersion(m.Plugin.Version); err != nil {
		report.Add("plugin.version", "version", err.Error())
	}
	if strings.TrimSpace(m.Plugin.Publisher.ID) == "" {
		report.Add("plugin.publisher.id", "publisher_id", "publisher id is required")
	}
	if strings.TrimSpace(m.Plugin.Publisher.DisplayName) == "" {
		report.Add("plugin.publisher.displayName", "publisher_name", "publisher display name is required")
	}
	if _, err := domain.ParseVersion(m.Compatibility.MinHostVersion); err != nil {
		report.Add("compatibility.minHostVersion", "version", err.Error())
	}
	if m.Compatibility.MaxHostVersion != "" {
		if _, err := domain.ParseVersion(m.Compatibility.MaxHostVersion); err != nil {
			report.Add("compatibility.maxHostVersion", "version", err.Error())
		}
	}
	validateUniqueStrings("compatibility.platforms", m.Compatibility.Platforms, &report)
	if len(m.Modules) == 0 {
		report.Add("modules", "modules_required", "at least one module is required")
	}
	moduleIDs := make(map[string]bool)
	contributionIDs := make(map[string]bool)
	for moduleIndex := range m.Modules {
		module := &m.Modules[moduleIndex]
		modulePath := fmt.Sprintf("modules[%d]", moduleIndex)
		if !moduleIDPattern.MatchString(module.ID) {
			report.Add(modulePath+".id", "module_id", "module id is required and must be lowercase")
		}
		if moduleIDs[module.ID] {
			report.Add(modulePath+".id", "duplicate_module", "module id is duplicated")
		}
		moduleIDs[module.ID] = true
		validateLocalized(modulePath+".name", module.Name, true, &report)
		if module.Version != "" {
			if _, err := domain.ParseVersion(module.Version); err != nil {
				report.Add(modulePath+".version", "version", err.Error())
			}
		}
		if module.Runtime != nil {
			validateRuntime(modulePath+".runtime", module.Runtime, &report)
		}
		if module.Compatibility != nil {
			if module.Compatibility.MinHostVersion != "" {
				if _, err := domain.ParseVersion(module.Compatibility.MinHostVersion); err != nil {
					report.Add(modulePath+".compatibility.minHostVersion", "version", err.Error())
				}
			}
			validateUniqueStrings(modulePath+".compatibility.platforms", module.Compatibility.Platforms, &report)
		}
		if module.Policies != nil && module.Policies.Isolation != "" {
			switch module.Policies.Isolation {
			case "none", "process", "sandbox":
			default:
				report.Add(modulePath+".policies.isolation", "module_policy", "isolation must be none, process or sandbox")
			}
		}
		if module.DeviceRequirements != nil {
			validateUniqueStrings(modulePath+".deviceRequirements.platforms", module.DeviceRequirements.Platforms, &report)
			validateUniqueStrings(modulePath+".deviceRequirements.architectures", module.DeviceRequirements.Architectures, &report)
			validateUniqueStrings(modulePath+".deviceRequirements.requiredFeatures", module.DeviceRequirements.RequiredFeatures, &report)
		}
		providedCapabilityIDs := make(map[string]bool)
		for capabilityIndex := range module.ProvidedCapabilities {
			capability := module.ProvidedCapabilities[capabilityIndex]
			if !serviceIDPattern.MatchString(capability.ID) {
				report.Add(fmt.Sprintf("%s.providedCapabilities[%d].id", modulePath, capabilityIndex), "capability_id", "capability id is invalid")
			}
			if providedCapabilityIDs[capability.ID] {
				report.Add(fmt.Sprintf("%s.providedCapabilities[%d].id", modulePath, capabilityIndex), "duplicate_capability", "capability id is duplicated")
			}
			providedCapabilityIDs[capability.ID] = true
			if capability.Version != "" {
				if _, err := domain.ParseVersion(capability.Version); err != nil {
					report.Add(fmt.Sprintf("%s.providedCapabilities[%d].version", modulePath, capabilityIndex), "version", err.Error())
				}
			}
		}
		if module.Provider != nil && strings.TrimSpace(module.Provider.ID) == "" {
			report.Add(modulePath+".provider.id", "provider_id", "provider id cannot be empty when provider metadata is declared")
		}
		if module.Provider != nil {
			validateReservedMetadata(modulePath+".provider.metadata", module.Provider.Metadata, &report)
		}
		validateServices(modulePath+".services", module.Services, &report)
		validateDynamic(modulePath+".dynamic", module.Dynamic, &report)
		for contributionIndex := range module.Contributions {
			contribution := &module.Contributions[contributionIndex]
			contributionPath := fmt.Sprintf("%s.contributions[%d]", modulePath, contributionIndex)
			validateContribution(contributionPath, contribution, &report)
			validateReservedMetadata(contributionPath+".metadata", contribution.Metadata, &report)
			key := m.Plugin.ID + "/" + contribution.ID
			if contributionIDs[key] {
				report.Add(contributionPath+".id", "duplicate_contribution", "contribution id is duplicated")
			}
			contributionIDs[key] = true
		}
		validatePermissions(modulePath+".permissions", module.Permissions, &report)
		validateResources(modulePath+".resources", module.Resources, &report)
		for dependencyIndex := range module.Dependencies {
			validateDependency(fmt.Sprintf("%s.dependencies[%d]", modulePath, dependencyIndex), module.Dependencies[dependencyIndex], &report)
		}
		if module.Placement != "" && !module.Placement.Valid() {
			report.Add(modulePath+".placement", "placement", "invalid placement")
		}
	}
	validatePermissions("permissions", m.Permissions, &report)
	validateResources("resources", m.Resources, &report)
	for dependencyIndex := range m.Dependencies {
		validateDependency(fmt.Sprintf("dependencies[%d]", dependencyIndex), m.Dependencies[dependencyIndex], &report)
	}
	for secretIndex := range m.SecretRefs {
		secret := m.SecretRefs[secretIndex]
		if strings.TrimSpace(secret.Ref) == "" {
			report.Add(fmt.Sprintf("secretRefs[%d].ref", secretIndex), "secret_ref", "secret ref is required")
		}
	}
	providerServices := make(map[string]bool)
	providerAdapters := make(map[string]bool)
	for index := range m.AdapterProviders {
		provider := m.AdapterProviders[index]
		providerPath := fmt.Sprintf("adapterProviders[%d]", index)
		if strings.TrimSpace(provider.ServiceID) == "" {
			report.Add(providerPath+".serviceId", "adapter_provider", "service id is required")
		} else if providerServices[provider.ServiceID] {
			report.Add(providerPath+".serviceId", "adapter_provider", "adapter provider service id is duplicated")
		}
		providerServices[provider.ServiceID] = true
		if !adapterIDPattern.MatchString(provider.AdapterID) {
			report.Add(providerPath+".adapterId", "adapter_provider", "adapter id is invalid")
		}
		adapterKey := fmt.Sprintf("%s@%d", provider.AdapterID, provider.ContractVersion)
		if providerAdapters[adapterKey] {
			report.Add(providerPath+".adapterId", "adapter_provider", "adapter version is duplicated")
		}
		providerAdapters[adapterKey] = true
		if provider.ContractVersion <= 0 {
			report.Add(providerPath+".contractVersion", "adapter_provider", "contract version must be positive")
		}
		if len(provider.SupportedKinds) == 0 {
			report.Add(providerPath+".supportedKinds", "adapter_provider", "supported kinds are required")
		}
		for kindIndex, kind := range provider.SupportedKinds {
			if !kindPattern.MatchString(kind) {
				report.Add(fmt.Sprintf("%s.supportedKinds[%d]", providerPath, kindIndex), "adapter_provider", "kind is invalid")
			}
		}
		operationSeen := make(map[string]bool)
		for operationIndex, operation := range provider.Operations {
			if !kindPattern.MatchString(operation) {
				report.Add(fmt.Sprintf("%s.operations[%d]", providerPath, operationIndex), "adapter_provider", "operation is invalid")
			}
			if operationSeen[operation] {
				report.Add(fmt.Sprintf("%s.operations[%d]", providerPath, operationIndex), "adapter_provider", "operation is duplicated")
			}
			operationSeen[operation] = true
		}
		var schema map[string]any
		if len(provider.ContributionSchema) == 0 || json.Unmarshal(provider.ContributionSchema, &schema) != nil || schema == nil {
			report.Add(providerPath+".contributionSchema", "adapter_provider", "contribution schema must be a JSON object")
		}
	}
	if m.Data != nil {
		if m.Data.SchemaVersion <= 0 {
			report.Add("data.schemaVersion", "data_version", "data schema version must be positive")
		}
		switch m.Data.Uninstall {
		case "", "retain", "delete", "export":
		default:
			report.Add("data.uninstall", "data_uninstall", "uninstall policy must be retain, delete or export")
		}
	}
	if m.Integrity.Algorithm != "sha256" {
		report.Add("integrity.algorithm", "integrity_algorithm", "integrity algorithm must be sha256")
	}
	for file, hash := range m.Integrity.FileHashes {
		if !safeRelativePath(file) {
			report.Add("integrity.fileHashes", "unsafe_path", "file hash path must be relative and contained")
		}
		if !hashPattern.MatchString(hash) {
			report.Add("integrity.fileHashes", "integrity_hash", "file hash must be a 64 character hexadecimal SHA-256")
		}
	}
	return report
}

func validateRuntime(path string, runtime *RuntimeSpec, report *ValidationReport) {
	if !adapterIDPattern.MatchString(runtime.Adapter) {
		report.Add(path+".adapter", "adapter_required", "runtime adapter is required and must be valid")
	}
	if runtime.Entry != "" && !safeRelativePath(runtime.Entry) {
		report.Add(path+".entry", "unsafe_path", "runtime entry must be relative and contained")
	}
	if runtime.MemoryBytes < 0 || runtime.CPUPercent < 0 || runtime.MaxProcesses < 0 {
		report.Add(path, "runtime_limit", "runtime limits cannot be negative")
	}
	if runtime.WorkerCount < 0 {
		report.Add(path+".workerCount", "runtime_limit", "worker count cannot be negative")
	}
	if runtime.HostAccess != nil {
		validateHostAccess(path+".hostAccess", runtime.HostAccess, report)
	}
	for index := range runtime.NativeArtifacts {
		artifact := runtime.NativeArtifacts[index]
		artifactPath := fmt.Sprintf("%s.nativeArtifacts[%d]", path, index)
		if strings.TrimSpace(artifact.ID) == "" {
			report.Add(artifactPath+".id", "artifact_id", "artifact id is required")
		}
		if strings.TrimSpace(artifact.Platform) == "" {
			report.Add(artifactPath+".platform", "artifact_platform", "artifact platform is required")
		}
		if !safeRelativePath(artifact.Path) {
			report.Add(artifactPath+".path", "unsafe_path", "artifact path must be relative and contained")
		}
		if !hashPattern.MatchString(artifact.SHA256) {
			report.Add(artifactPath+".sha256", "artifact_hash", "artifact hash must be a 64 character hexadecimal SHA-256")
		}
	}
}

func validateHostAccess(path string, access *HostAccess, report *ValidationReport) {
	if access.Filesystem != nil {
		switch access.Filesystem.Mode {
		case "none", "read", "read_write":
		default:
			report.Add(path+".filesystem.mode", "host_access", "filesystem mode must be none, read or read_write")
		}
		for _, root := range access.Filesystem.Roots {
			switch root {
			case "package", "plugin_data":
			default:
				report.Add(path+".filesystem.roots", "host_access", "filesystem root must be package or plugin_data")
			}
		}
	}
	if access.Network != nil {
		switch access.Network.Mode {
		case "none", "host":
		default:
			report.Add(path+".network.mode", "host_access", "network mode must be none or host")
		}
	}
	if access.Process != nil {
		switch access.Process.Mode {
		case "none", "direct":
		default:
			report.Add(path+".process.mode", "host_access", "process mode must be none or direct")
		}
		if access.Process.MaxProcesses < 0 {
			report.Add(path+".process.maxProcesses", "host_access", "max processes cannot be negative")
		}
	}
}

func validateServices(path string, services ServiceDeclaration, report *ValidationReport) {
	offerIDs := make(map[string]bool)
	for index := range services.Provides {
		offer := services.Provides[index]
		offerPath := fmt.Sprintf("%s.provides[%d]", path, index)
		if !serviceIDPattern.MatchString(offer.ServiceID) {
			report.Add(offerPath+".serviceId", "service_id", "service id is required and must be valid")
		}
		if offerIDs[offer.ServiceID] {
			report.Add(offerPath+".serviceId", "duplicate_service", "service is offered more than once by the module")
		}
		offerIDs[offer.ServiceID] = true
		if _, err := domain.ParseVersion(offer.Version); err != nil {
			report.Add(offerPath+".version", "version", err.Error())
		}
		if offer.ContractHash != "" && !hashPattern.MatchString(offer.ContractHash) {
			report.Add(offerPath+".contractHash", "contract_hash", "contract hash must be a 64 character hexadecimal SHA-256")
		}
		if len(offer.Scopes) == 0 {
			report.Add(offerPath+".scopes", "service_scope", "at least one service scope is required")
		}
		for scopeIndex := range offer.Scopes {
			if !offer.Scopes[scopeIndex].Valid() {
				report.Add(fmt.Sprintf("%s.scopes[%d]", offerPath, scopeIndex), "service_scope", "invalid service scope")
			}
		}
		if !offer.Placement.Valid() {
			report.Add(offerPath+".placement", "placement", "invalid service placement")
		}
	}
	for index := range services.Requires {
		requirement := services.Requires[index]
		requirementPath := fmt.Sprintf("%s.requires[%d]", path, index)
		if !serviceIDPattern.MatchString(requirement.ServiceID) {
			report.Add(requirementPath+".serviceId", "service_id", "service id is required and must be valid")
		}
		if _, err := dependency.ParseRange(requirement.VersionRange); err != nil {
			report.Add(requirementPath+".versionRange", "version_range", err.Error())
		}
		if requirement.ContractHash != "" && !hashPattern.MatchString(requirement.ContractHash) {
			report.Add(requirementPath+".contractHash", "contract_hash", "contract hash must be a 64 character hexadecimal SHA-256")
		}
		selection := requirement.Selection
		if selection == "" {
			selection = ServiceSelectionSingle
		}
		if !selection.Valid() {
			report.Add(requirementPath+".selection", "service_selection", "invalid service selection")
		}
	}
}

func validateDynamic(path string, dynamic DynamicCapability, report *ValidationReport) {
	validateUniqueStrings(path+".allowedAdapters", dynamic.AllowedAdapters, report)
	validateUniqueStrings(path+".allowedKinds", dynamic.AllowedKinds, report)
	for index, adapter := range dynamic.AllowedAdapters {
		if !adapterIDPattern.MatchString(adapter) {
			report.Add(fmt.Sprintf("%s.allowedAdapters[%d]", path, index), "adapter_id", "adapter id is invalid")
		}
	}
	for index, kind := range dynamic.AllowedKinds {
		if !kindPattern.MatchString(kind) {
			report.Add(fmt.Sprintf("%s.allowedKinds[%d]", path, index), "kind_id", "kind is invalid")
		}
	}
	if dynamic.MaxContributions < 0 || dynamic.MaxTools < 0 || dynamic.MaxRoutes < 0 || dynamic.MaxServices < 0 {
		report.Add(path, "dynamic_limit", "dynamic limits cannot be negative")
	}
}

func validateReservedMetadata(path string, metadata map[string]any, report *ValidationReport) {
	for _, key := range []string{
		"system.managed",
		"system.builtin",
		"system.required",
		"system.disableAllowed",
		"system.bootstrapRevision",
		"system.component",
	} {
		if _, exists := metadata[key]; exists {
			report.Add(path+"."+key, "reserved_metadata", "metadata key is reserved for built-in definitions")
		}
	}
}

func validateContribution(path string, contribution *ContributionDescriptor, report *ValidationReport) {
	if strings.TrimSpace(contribution.ID) == "" {
		report.Add(path+".id", "contribution_id", "contribution id is required")
	}
	if !kindPattern.MatchString(contribution.Kind) {
		report.Add(path+".kind", "contribution_kind", "contribution kind is required and must be valid")
	}
	if !adapterIDPattern.MatchString(contribution.Adapter) {
		report.Add(path+".adapter", "adapter_required", "contribution adapter is required and must be valid")
	}
	if contribution.ContractVersion <= 0 {
		report.Add(path+".contractVersion", "contract_version", "contract version must be positive")
	}
	validateLocalized(path+".name", contribution.Name, true, report)
	if contribution.Version != "" {
		if _, err := domain.ParseVersion(contribution.Version); err != nil {
			report.Add(path+".version", "version", err.Error())
		}
	}
	if contribution.Exposure != nil {
		validateUniqueStrings(path+".exposure.requiredRoles", contribution.Exposure.RequiredRoles, report)
	}
	if contribution.RuntimeBinding != nil && strings.TrimSpace(contribution.RuntimeBinding.RuntimeID) == "" {
		report.Add(path+".runtimeBinding.runtimeId", "runtime_binding", "runtime binding id is required")
	}
	for dependencyIndex := range contribution.Dependencies {
		validateDependency(fmt.Sprintf("%s.dependencies[%d]", path, dependencyIndex), contribution.Dependencies[dependencyIndex], report)
	}
	if len(contribution.Schema) > 0 {
		var schema map[string]any
		if err := json.Unmarshal(contribution.Schema, &schema); err != nil {
			report.Add(path+".schema", "schema_invalid", "contribution schema must be a JSON object")
		}
	}
}

func validatePermissions(path string, permissions []Permission, report *ValidationReport) {
	seen := make(map[string]bool)
	for index := range permissions {
		permission := permissions[index]
		permissionPath := fmt.Sprintf("%s[%d]", path, index)
		if strings.TrimSpace(permission.Name) == "" {
			report.Add(permissionPath+".name", "permission_name", "permission name is required")
		}
		if seen[permission.Name] {
			report.Add(permissionPath+".name", "duplicate_permission", "permission is duplicated")
		}
		seen[permission.Name] = true
	}
}

func validateResources(path string, resources []Resource, report *ValidationReport) {
	seen := make(map[string]bool)
	for index := range resources {
		resource := resources[index]
		resourcePath := fmt.Sprintf("%s[%d]", path, index)
		if strings.TrimSpace(resource.ID) == "" {
			report.Add(resourcePath+".id", "resource_id", "resource id is required")
		}
		if seen[resource.ID] {
			report.Add(resourcePath+".id", "duplicate_resource", "resource id is duplicated")
		}
		seen[resource.ID] = true
		if !safeRelativePath(resource.Path) {
			report.Add(resourcePath+".path", "unsafe_path", "resource path must be relative and contained")
		}
		if resource.Size < 0 {
			report.Add(resourcePath+".size", "resource_size", "resource size cannot be negative")
		}
	}
}

func validateDependency(path string, dep PluginDependency, report *ValidationReport) {
	if strings.TrimSpace(dep.Type) == "" {
		report.Add(path+".type", "dependency_type", "dependency type is required")
	}
	if strings.TrimSpace(dep.ID) == "" {
		report.Add(path+".id", "dependency_id", "dependency id is required")
	}
	if dep.Version != "" {
		if _, err := dependency.ParseRange(dep.Version); err != nil {
			report.Add(path+".version", "version_range", err.Error())
		}
	}
}

func validateLocalized(path string, value LocalizedText, required bool, report *ValidationReport) {
	if strings.TrimSpace(value.Default) == "" {
		if required {
			report.Add(path, "localized_required", "localized text is required")
		} else if value.Translations != nil {
			report.Add(path+".default", "localized_default", "default localization is required")
		}
		return
	}
	for locale := range value.Translations {
		if strings.TrimSpace(locale) == "" {
			report.Add(path+".translations", "localized_locale", "translation locale cannot be empty")
		}
	}
}

func validateUniqueStrings(path string, values []string, report *ValidationReport) {
	seen := make(map[string]bool)
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			report.Add(fmt.Sprintf("%s[%d]", path, index), "empty_value", "value cannot be empty")
			continue
		}
		if seen[value] {
			report.Add(fmt.Sprintf("%s[%d]", path, index), "duplicate_value", "value is duplicated")
		}
		seen[value] = true
	}
}

func safeRelativePath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func (s ServiceScope) Valid() bool {
	switch s {
	case ServiceScopeGlobal, ServiceScopeCharacter, ServiceScopeConversation, ServiceScopeDevice, ServiceScopePlugin:
		return true
	default:
		return false
	}
}

func (s ServiceSelection) Valid() bool {
	switch s {
	case ServiceSelectionSingle, ServiceSelectionAll, ServiceSelectionExclusive:
		return true
	default:
		return false
	}
}

func (p ServicePlacement) Valid() bool {
	switch p {
	case ServicePlacementCore, ServicePlacementDevice, ServicePlacementCloud, ServicePlacementHybrid:
		return true
	default:
		return false
	}
}
