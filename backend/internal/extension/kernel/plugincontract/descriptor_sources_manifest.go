package plugincontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	manifestSkillKind      = "agent_skill"
	manifestUIProviderKind = "ui_provider"
	providerIDSuffix       = ".provider"
)

var ErrInvalidManifestDescriptor = errors.New("plugincontract: invalid manifest descriptor")

type ManifestDescriptorError struct {
	Source   DescriptorSource
	PluginID string
	ModuleID string
	LocalID  string
	Cause    error
}

func (e *ManifestDescriptorError) Error() string {
	base := fmt.Sprintf("plugincontract: cannot derive %s descriptor for plugin %q module %q local id %q", e.Source, e.PluginID, e.ModuleID, e.LocalID)
	if e.Cause != nil {
		return base + ": " + e.Cause.Error()
	}
	return base
}

func (e *ManifestDescriptorError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrInvalidManifestDescriptor, e.Cause}
	}
	return []error{ErrInvalidManifestDescriptor}
}

func ManifestDescriptorCatalog(m Manifest) (DescriptorCatalog, error) {
	builder := newDescriptorCatalogBuilder()
	if err := builder.addManifestCapabilities(m); err != nil {
		return DescriptorCatalog{}, err
	}
	if err := builder.addManifestContributions(m, manifestSkillKind, DescriptorSourceManifestSkill); err != nil {
		return DescriptorCatalog{}, err
	}
	if err := builder.addManifestContributions(m, manifestUIProviderKind, DescriptorSourceManifestUIProvider); err != nil {
		return DescriptorCatalog{}, err
	}
	return builder.catalog()
}

func ManifestCapabilityDescriptorCatalog(m Manifest) (DescriptorCatalog, error) {
	builder := newDescriptorCatalogBuilder()
	if err := builder.addManifestCapabilities(m); err != nil {
		return DescriptorCatalog{}, err
	}
	return builder.catalog()
}

func ManifestSkillDescriptorCatalog(m Manifest) (DescriptorCatalog, error) {
	builder := newDescriptorCatalogBuilder()
	if err := builder.addManifestContributions(m, manifestSkillKind, DescriptorSourceManifestSkill); err != nil {
		return DescriptorCatalog{}, err
	}
	return builder.catalog()
}

func ManifestUIProviderDescriptorCatalog(m Manifest) (DescriptorCatalog, error) {
	builder := newDescriptorCatalogBuilder()
	if err := builder.addManifestContributions(m, manifestUIProviderKind, DescriptorSourceManifestUIProvider); err != nil {
		return DescriptorCatalog{}, err
	}
	return builder.catalog()
}

type descriptorCatalogBuilder struct {
	entries []DescriptorEntry
	schemas map[string]json.RawMessage
}

func newDescriptorCatalogBuilder() *descriptorCatalogBuilder {
	return &descriptorCatalogBuilder{schemas: make(map[string]json.RawMessage)}
}

func (b *descriptorCatalogBuilder) catalog() (DescriptorCatalog, error) {
	catalog := DescriptorCatalog{
		Entries: make([]DescriptorEntry, len(b.entries)),
		Schemas: make(map[string]json.RawMessage, len(b.schemas)),
	}
	copy(catalog.Entries, b.entries)
	sortDescriptorEntries(catalog.Entries)
	for hash, schema := range b.schemas {
		catalog.Schemas[hash] = schema
	}
	if err := catalog.Validate(); err != nil {
		return DescriptorCatalog{}, err
	}
	return catalog, nil
}

func sortDescriptorEntries(entries []DescriptorEntry) {
	sort.Slice(entries, func(left, right int) bool { return entries[left].CapabilityID < entries[right].CapabilityID })
}

func (b *descriptorCatalogBuilder) addManifestCapabilities(m Manifest) error {
	pluginID := strings.TrimSpace(m.Plugin.ID)
	for _, module := range m.Modules {
		moduleID := strings.TrimSpace(module.ID)
		placement := manifestPlacement(module.Placement)
		for _, capability := range module.ProvidedCapabilities {
			rawID := strings.TrimSpace(capability.ID)
			capabilityID, err := manifestCapabilityIdentity(pluginID, moduleID, rawID)
			if err != nil {
				return &ManifestDescriptorError{Source: DescriptorSourceManifestCapability, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
			version := firstNonEmpty(capability.Version, module.Version, m.Plugin.Version, CoreDescriptorVersion)
			inputSchema, err := manifestSchemaValue(capability.Metadata, "inputSchema")
			if err != nil {
				return &ManifestDescriptorError{Source: DescriptorSourceManifestCapability, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
			outputSchema, err := manifestSchemaValue(capability.Metadata, "outputSchema")
			if err != nil {
				return &ManifestDescriptorError{Source: DescriptorSourceManifestCapability, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
			spec := sourcedDescriptorSpec{
				Source:       DescriptorSourceManifestCapability,
				CapabilityID: capabilityID,
				Version:      version,
				Summary:      manifestSummary(manifestLocalID(capabilityID), module),
				Tags:         []string{"capability", "plugin"},
				Scopes:       manifestScopes(placement),
				Permissions:  manifestPermissionsFromDeclarations(module.Permissions),
				Placement:    placement,
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
			}
			if err := b.add(spec); err != nil {
				return &ManifestDescriptorError{Source: spec.Source, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
		}
	}
	return nil
}

func (b *descriptorCatalogBuilder) addManifestContributions(m Manifest, kind string, source DescriptorSource) error {
	pluginID := strings.TrimSpace(m.Plugin.ID)
	for _, module := range m.Modules {
		moduleID := strings.TrimSpace(module.ID)
		placement := manifestPlacement(module.Placement)
		for _, contribution := range module.Contributions {
			declaredKind := strings.TrimSpace(contribution.Kind)
			if declaredKind != kind {
				continue
			}
			rawID := strings.TrimSpace(contribution.ID)
			capabilityID, err := manifestContributionIdentity(pluginID, moduleID, rawID)
			if err != nil {
				return &ManifestDescriptorError{Source: source, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
			version := firstNonEmpty(contribution.Version, module.Version, m.Plugin.Version, CoreDescriptorVersion)
			scopes := manifestContributionScopes(placement, contribution.RequiredScope)
			spec := sourcedDescriptorSpec{
				Source:       source,
				CapabilityID: capabilityID,
				Version:      version,
				Summary:      manifestContributionSummary(source, contribution),
				Tags:         manifestContributionTags(source),
				Scopes:       scopes,
				Permissions:  manifestPermissions(contribution.RequiredPermissions),
				Placement:    placement,
				InputSchema:  cloneRawMessage(contribution.Schema),
			}
			if err := b.add(spec); err != nil {
				return &ManifestDescriptorError{Source: source, PluginID: pluginID, ModuleID: moduleID, LocalID: rawID, Cause: err}
			}
		}
	}
	return nil
}

type sourcedDescriptorSpec struct {
	Source       DescriptorSource
	CapabilityID string
	Version      string
	Summary      string
	Tags         []string
	Scopes       []ServiceScope
	Permissions  []DescriptorPermission
	Placement    ServicePlacement
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

func (b *descriptorCatalogBuilder) add(spec sourcedDescriptorSpec) error {
	capabilityID := strings.TrimSpace(spec.CapabilityID)
	if err := ValidateCapabilityIdentity(capabilityID); err != nil {
		return err
	}
	for _, entry := range b.entries {
		if entry.CapabilityID == capabilityID {
			return fmt.Errorf("duplicated capability %q in descriptor catalog", capabilityID)
		}
	}
	providerID, err := manifestProviderIdentity(capabilityID)
	if err != nil {
		return err
	}
	inputSchema, err := canonicalSchemaOrEmpty(spec.InputSchema)
	if err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	outputSchema, err := canonicalSchemaOrEmpty(spec.OutputSchema)
	if err != nil {
		return fmt.Errorf("output schema: %w", err)
	}
	var inputRef *SchemaRef
	if len(inputSchema) > 0 {
		ref, err := putCoreSchema(b.schemas, inputSchema)
		if err != nil {
			return err
		}
		inputRef = &ref
	}
	var outputRef *SchemaRef
	if len(outputSchema) > 0 {
		ref, err := putCoreSchema(b.schemas, outputSchema)
		if err != nil {
			return err
		}
		outputRef = &ref
	}
	placement := spec.Placement
	if !placement.Valid() {
		placement = ServicePlacementDevice
	}
	contractHash, err := CanonicalContractHash(ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       manifestLocalID(capabilityID),
		Version:         spec.Version,
		Placement:       placement,
		Scopes:          spec.Scopes,
		InputSchema:     inputSchema,
		OutputSchema:    outputSchema,
	})
	if err != nil {
		return err
	}
	descriptor, err := normalizeCoreDescriptor(Descriptor{
		ID:          capabilityID,
		Summary:     spec.Summary,
		Tags:        spec.Tags,
		Scopes:      spec.Scopes,
		Permissions: spec.Permissions,
		Placement:   placement,
		InputRef:    inputRef,
		OutputRef:   outputRef,
	})
	if err != nil {
		return err
	}
	b.entries = append(b.entries, DescriptorEntry{
		Source:       spec.Source,
		ProviderID:   providerID,
		CapabilityID: capabilityID,
		ServiceID:    manifestLocalID(capabilityID),
		Version:      spec.Version,
		ContractHash: contractHash,
		Descriptor:   descriptor,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	})
	return nil
}

func manifestCapabilityIdentity(pluginID, moduleID, rawID string) (string, error) {
	if err := ValidateCapabilityIdentity(rawID); err == nil {
		return rawID, nil
	}
	return manifestContributionIdentity(pluginID, moduleID, rawID)
}

func manifestContributionIdentity(pluginID, moduleID, rawID string) (string, error) {
	trimmed := strings.TrimSpace(rawID)
	plugin := strings.TrimSpace(pluginID)
	module := strings.TrimSpace(moduleID)
	if trimmed == "" {
		return "", fmt.Errorf("capability id is empty")
	}
	if plugin != "" && strings.HasPrefix(trimmed, plugin+"/") {
		if err := ValidateCapabilityIdentity(trimmed); err == nil {
			return trimmed, nil
		}
	}
	candidate := plugin + "/" + module + "/" + trimmed
	if err := ValidateCapabilityIdentity(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func manifestProviderIdentity(capabilityID string) (string, error) {
	index := strings.LastIndex(capabilityID, "/")
	if index <= 0 || index == len(capabilityID)-1 {
		return "", fmt.Errorf("capability id %q has no local segment", capabilityID)
	}
	candidate := capabilityID + providerIDSuffix
	if err := ValidateProviderID(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func manifestLocalID(capabilityID string) string {
	segments := strings.Split(capabilityID, "/")
	return segments[len(segments)-1]
}

func manifestPlacement(raw ServicePlacement) ServicePlacement {
	trimmed := ServicePlacement(strings.TrimSpace(string(raw)))
	if trimmed.Valid() {
		return trimmed
	}
	return ServicePlacementDevice
}

func manifestScopes(placement ServicePlacement) []ServiceScope {
	scopes := []ServiceScope{ServiceScopePlugin}
	if placement == ServicePlacementDevice {
		scopes = append(scopes, ServiceScopeDevice)
	}
	return scopes
}

func manifestContributionScopes(placement ServicePlacement, declared []string) []ServiceScope {
	scopes := manifestScopes(placement)
	for _, raw := range declared {
		scope := ServiceScope(strings.ToLower(strings.TrimSpace(raw)))
		if scope.Valid() {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func manifestPermissionsFromDeclarations(permissions []Permission) []DescriptorPermission {
	values := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		values = append(values, permission.Name)
	}
	return manifestPermissions(values)
}

func manifestPermissions(values []string) []DescriptorPermission {
	out := make([]DescriptorPermission, 0, len(values))
	for _, raw := range values {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "" {
			continue
		}
		out = append(out, DescriptorPermission{Capability: trimmed})
	}
	return out
}

func manifestSummary(localID string, module Module) string {
	name := strings.TrimSpace(module.Name.Default)
	if name == "" {
		name = strings.TrimSpace(module.ID)
	}
	return fmt.Sprintf("%s 提供能力 %s", name, localID)
}

func manifestContributionSummary(source DescriptorSource, contribution ContributionDescriptor) string {
	name := strings.TrimSpace(contribution.Name.Default)
	if name == "" {
		name = strings.TrimSpace(contribution.ID)
	}
	switch source {
	case DescriptorSourceManifestSkill:
		return "技能 " + name
	case DescriptorSourceManifestUIProvider:
		return "UI Provider " + name
	default:
		return name
	}
}

func manifestContributionTags(source DescriptorSource) []string {
	switch source {
	case DescriptorSourceManifestSkill:
		return []string{"skill"}
	case DescriptorSourceManifestUIProvider:
		return []string{"provider", "ui"}
	default:
		return []string{"capability", "plugin"}
	}
}

func manifestSchemaValue(metadata map[string]any, key string) (json.RawMessage, error) {
	if metadata == nil {
		return nil, nil
	}
	value, exists := metadata[key]
	if !exists || value == nil {
		return nil, nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		return cloneRawMessage(raw), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("metadata %s: %w", key, err)
	}
	return encoded, nil
}

func canonicalSchemaOrEmpty(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	canonical, err := CanonicalizeJSONValue(raw)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
