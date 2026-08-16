package manifest_v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/mcp_manifest"
)

const ManifestVersion = 2

type Manifest struct {
	ManifestVersion int             `json:"manifestVersion"`
	Placement       string          `json:"placement,omitempty"`
	Extension       ExtensionMeta   `json:"extension"`
	Publisher       PublisherMeta   `json:"publisher"`
	Compatibility   Compatibility   `json:"compatibility"`
	Modules         []ModuleMeta    `json:"modules"`
	Dependencies    []Dependency    `json:"dependencies,omitempty"`
	Permissions     []PermissionReq `json:"permissions,omitempty"`
	Resources       []ResourceMeta  `json:"resources,omitempty"`
	Lifecycle       LifecycleMeta   `json:"lifecycle,omitempty"`
	Integrity       IntegrityMeta   `json:"integrity"`
	Development     DevelopmentMeta `json:"development,omitempty"`
	SecretRefs      []SecretRefMeta `json:"secretRefs,omitempty"`
}

type ExtensionMeta struct {
	ID          string         `json:"id"`
	Name        LocalizedText  `json:"name"`
	Description LocalizedText  `json:"description"`
	Version     string         `json:"version"`
	License     string         `json:"license,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
	Repository  string         `json:"repository,omitempty"`
	Categories  []string       `json:"categories,omitempty"`
	Keywords    []string       `json:"keywords,omitempty"`
	Icon        string         `json:"icon,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PublisherMeta struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	TrustLevel  string `json:"trustLevel,omitempty"`
	Contact     string `json:"contact,omitempty"`
	Website     string `json:"website,omitempty"`
}

type Compatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	MaxHostVersion string   `json:"maxHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
	FeatureFlags   []string `json:"featureFlags,omitempty"`
}

type ModuleMeta struct {
	ID            string               `json:"id"`
	Name          LocalizedText        `json:"name"`
	Description   LocalizedText        `json:"description,omitempty"`
	Type          string               `json:"type"`
	Version       string               `json:"version,omitempty"`
	Runtime       *RuntimeMeta         `json:"runtime,omitempty"`
	Contributions []ContributionMeta   `json:"contributions,omitempty"`
	Dependencies  []Dependency         `json:"dependencies,omitempty"`
	Compatibility *ModuleCompatibility `json:"compatibility,omitempty"`
	Policies      *ModulePolicies      `json:"policies,omitempty"`

	Placement            string                   `json:"placement,omitempty"`
	DeviceRequirements   *DeviceRequirementsMeta  `json:"deviceRequirements,omitempty"`
	ProvidedCapabilities []ProvidedCapabilityMeta `json:"providedCapabilities,omitempty"`
	Provider             *ProviderMetadataMeta    `json:"provider,omitempty"`
}

type ModuleCompatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
}

type ModulePolicies struct {
	Isolation        string `json:"isolation,omitempty"`
	NetworkAccess    bool   `json:"networkAccess,omitempty"`
	FileSystemAccess bool   `json:"fileSystemAccess,omitempty"`
}

type RuntimeMeta struct {
	Type         string            `json:"type"`
	ServiceID    string            `json:"serviceId,omitempty"`
	EntryPoint   string            `json:"entryPoint,omitempty"`
	WorkerCount  int               `json:"workerCount,omitempty"`
	Timeout      string            `json:"timeout,omitempty"`
	Memory       int64             `json:"memory,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
	Capabilities map[string]bool   `json:"capabilities,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type ContributionMeta struct {
	ID                  string              `json:"id"`
	Kind                string              `json:"kind"`
	Name                LocalizedText       `json:"name"`
	Description         LocalizedText       `json:"description,omitempty"`
	Version             string              `json:"version,omitempty"`
	Spec                map[string]any      `json:"spec,omitempty"`
	RequiredPermissions []string            `json:"requiredPermissions,omitempty"`
	RequiredScope       []string            `json:"requiredScope,omitempty"`
	Exposure            *ExposureMeta       `json:"exposure,omitempty"`
	RuntimeBinding      *RuntimeBindingMeta `json:"runtimeBinding,omitempty"`
	Dependencies        []Dependency        `json:"dependencies,omitempty"`
}

type ExposureMeta struct {
	VisibleByDefault    bool     `json:"visibleByDefault,omitempty"`
	HiddenFromDiscovery bool     `json:"hiddenFromDiscovery,omitempty"`
	RequiredRoles       []string `json:"requiredRoles,omitempty"`
}

type RuntimeBindingMeta struct {
	RuntimeID  string `json:"runtimeId"`
	Generation int64  `json:"generation,omitempty"`
}

type Dependency struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type PermissionReq struct {
	Name     string `json:"name"`
	Reason   string `json:"reason,omitempty"`
	Required bool   `json:"required,omitempty"`
	Scope    string `json:"scope,omitempty"`
}

type ResourceMeta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type LifecycleMeta struct {
	AutoUpdate      bool   `json:"autoUpdate,omitempty"`
	BackgroundTasks bool   `json:"backgroundTasks,omitempty"`
	NetworkAccess   bool   `json:"networkAccess,omitempty"`
	Isolation       string `json:"isolation,omitempty"`
	Sandbox         bool   `json:"sandbox,omitempty"`
}

type IntegrityMeta struct {
	Algorithm       string            `json:"algorithm"`
	ContentTreeHash string            `json:"contentTreeHash"`
	FileHashes      map[string]string `json:"fileHashes,omitempty"`
}

type DevelopmentMeta struct {
	DeveloperMode bool     `json:"developerMode,omitempty"`
	HotReload     bool     `json:"hotReload,omitempty"`
	SourceMaps    bool     `json:"sourceMaps,omitempty"`
	TestEntry     string   `json:"testEntry,omitempty"`
	WatchPaths    []string `json:"watchPaths,omitempty"`
}

type DeviceRequirementsMeta struct {
	Platforms         []string `json:"platforms,omitempty"`
	Architectures     []string `json:"architectures,omitempty"`
	MinAppVersion     string   `json:"minAppVersion,omitempty"`
	MinRuntimeVersion string   `json:"minRuntimeVersion,omitempty"`
	RequiredFeatures  []string `json:"requiredFeatures,omitempty"`
}

type ProvidedCapabilityMeta struct {
	ID       string         `json:"id"`
	Version  string         `json:"version,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ProviderMetadataMeta struct {
	ID       string            `json:"id,omitempty"`
	Priority int               `json:"priority,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

type SecretRefMeta struct {
	Ref       string `json:"ref"`
	ServiceID string `json:"serviceId,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	Required  bool   `json:"required,omitempty"`
}

type LocalizedText struct {
	Default      string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

func (l LocalizedText) ToDomain() domain.LocalizedText {
	return domain.LocalizedText{
		Default:      l.Default,
		Translations: l.Translations,
	}
}

type ValidationReport struct {
	Errors   []ValidationError   `json:"errors"`
	Warnings []ValidationWarning `json:"warnings"`
}

type ValidationError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

type ValidationWarning struct {
	Path    string `json:"path"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

func (r *ValidationReport) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *ValidationReport) AddError(path, code, msg string) {
	r.Errors = append(r.Errors, ValidationError{Path: path, Code: code, Message: msg})
}

func (r *ValidationReport) AddErrorWithLocation(path, code, msg, file string, line, col int) {
	r.Errors = append(r.Errors, ValidationError{Path: path, Code: code, Message: msg, File: file, Line: line, Column: col})
}

func (r *ValidationReport) AddWarning(path, msg string) {
	r.Warnings = append(r.Warnings, ValidationWarning{Path: path, Message: msg})
}

func (r *ValidationReport) AddWarningWithLocation(path, msg, file string, line, col int) {
	r.Warnings = append(r.Warnings, ValidationWarning{Path: path, Message: msg, File: file, Line: line, Column: col})
}

func (r *ValidationReport) AddWarningCode(path, code, msg string) {
	r.Warnings = append(r.Warnings, ValidationWarning{Path: path, Code: code, Message: msg})
}

func (r *ValidationReport) Merge(other ValidationReport) {
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

func (r *ValidationReport) MergePtr(other *ValidationReport) {
	if other == nil {
		return
	}
	r.Merge(*other)
}

func (r *ValidationReport) AttachFile(file string) {
	for i := range r.Errors {
		if r.Errors[i].File == "" {
			r.Errors[i].File = file
		}
	}
	for i := range r.Warnings {
		if r.Warnings[i].File == "" {
			r.Warnings[i].File = file
		}
	}
}

var (
	ErrInvalidManifest         = errors.New("manifest_v2: invalid manifest")
	ErrUnsupportedVersion      = errors.New("manifest_v2: unsupported manifest version")
	ErrMissingField            = errors.New("manifest_v2: missing required field")
	ErrInvalidExtensionID      = errors.New("manifest_v2: invalid extension id")
	ErrInvalidVersion          = errors.New("manifest_v2: invalid version")
	ErrDuplicateModule         = errors.New("manifest_v2: duplicate module id")
	ErrDuplicateContribution   = errors.New("manifest_v2: duplicate contribution id")
	ErrUnknownModuleType       = errors.New("manifest_v2: unknown module type")
	ErrUnknownContributionKind = errors.New("manifest_v2: unknown contribution kind")
)

func Parse(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := rejectDuplicateJSONKeys(decoder); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return Manifest{}, fmt.Errorf("%w: trailing JSON content", ErrInvalidManifest)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: parse error: %v", ErrInvalidManifest, err)
	}
	return m, nil
}

func rejectDuplicateJSONKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if keys[key] {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			keys[key] = true
			if err := rejectDuplicateJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := rejectDuplicateJSONKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return nil
	}
}

func (m Manifest) Validate() ValidationReport {
	report := ValidationReport{}
	if m.ManifestVersion != ManifestVersion {
		report.AddError("manifestVersion", "unsupported_version",
			fmt.Sprintf("expected %d, got %d", ManifestVersion, m.ManifestVersion))
		return report
	}
	if m.Extension.ID == "" {
		report.AddError("extension.id", "missing", "extension id required")
	} else if err := domain.ValidateExtensionID(domain.ExtensionID(m.Extension.ID)); err != nil {
		report.AddError("extension.id", "invalid_id", err.Error())
	}
	if m.Extension.Name.Default == "" {
		report.AddError("extension.name.default", "missing", "extension name required")
	}
	if m.Extension.Version == "" {
		report.AddError("extension.version", "missing", "version required")
	} else if _, err := domain.ParseVersion(m.Extension.Version); err != nil {
		report.AddError("extension.version", "invalid_version", err.Error())
	}
	if m.Publisher.ID == "" {
		report.AddError("publisher.id", "missing", "publisher id required")
	}
	if len(m.Modules) == 0 {
		report.AddError("modules", "missing", "at least one module required")
	}
	moduleIDs := make(map[string]bool)
	moduleTypes := map[string]bool{
		"builtin": true, "javascript": true, "data_only": true, "wasm": true,
		"native": true, "service": true,
	}
	runtimeTypes := map[string]bool{
		"javascript": true, "mcp": true, "workflow": true, "static": true, "wasm": true,
		"service": true,
	}
	contributionKinds := map[string]bool{
		"tool": true, "agent_skill": true, "workflow": true,
		"mcp_server": true,
		"provider":   true, "hook": true, "event_subscription": true,
		"schedule": true, "background_task": true,
		"ui_page": true, "ui_panel": true, "ui_chat": true,
		"ui_context_action": true, "ui_desktop": true,
		"game_plugin": true, "desktop_pet_plugin": true,
	}
	contributionIDs := make(map[string]bool)
	for i, mod := range m.Modules {
		path := fmt.Sprintf("modules[%d]", i)
		if mod.ID == "" {
			report.AddError(path+".id", "missing", "module id required")
			continue
		}
		if moduleIDs[mod.ID] {
			report.AddError(path+".id", "duplicate", fmt.Sprintf("module %s already declared", mod.ID))
			continue
		}
		moduleIDs[mod.ID] = true
		if mod.Name.Default == "" {
			report.AddError(path+".name.default", "missing", "module name required")
		}
		if mod.Type == "" {
			report.AddError(path+".type", "missing", "module type required")
		} else if !moduleTypes[mod.Type] {
			report.AddError(path+".type", "unknown_type", fmt.Sprintf("unknown module type: %s", mod.Type))
		}
		if mod.Runtime != nil {
			if mod.Runtime.Type == "" {
				report.AddError(path+".runtime.type", "missing", "runtime type required")
			} else if !runtimeTypes[mod.Runtime.Type] {
				report.AddError(path+".runtime.type", "unsupported_runtime", fmt.Sprintf(`{"code":"unsupported_runtime","moduleId":"%s","runtimeType":"%s"}`, mod.ID, mod.Runtime.Type))
			}
		}
		for j, c := range mod.Contributions {
			cpath := fmt.Sprintf("%s.contributions[%d]", path, j)
			if c.ID == "" {
				report.AddError(cpath+".id", "missing", "contribution id required")
				continue
			}
			if contributionIDs[c.ID] {
				report.AddError(cpath+".id", "duplicate", fmt.Sprintf("contribution %s already declared", c.ID))
				continue
			}
			contributionIDs[c.ID] = true
			if c.Kind == "" {
				report.AddError(cpath+".kind", "missing", "contribution kind required")
			} else if !contributionKinds[c.Kind] {
				report.AddError(cpath+".kind", "unknown_kind", fmt.Sprintf("unknown contribution kind: %s", c.Kind))
			}
			if c.Name.Default == "" {
				report.AddError(cpath+".name.default", "missing", "contribution name required")
			}
			if c.Kind == "game_plugin" {
				if err := validateGamePluginContribution(c.Spec, cpath, moduleIDs); err != nil {
					report.AddError(cpath+".spec", "invalid_game_plugin", err.Error())
				}
			}
			if c.Kind == "desktop_pet_plugin" {
				if err := validateDesktopPetPluginContribution(c.Spec, cpath, moduleIDs); err != nil {
					report.AddError(cpath+".spec", "invalid_desktop_pet_plugin", err.Error())
				}
			}
			if c.Kind == "mcp_server" {
				if err := validateMCPServerContribution(c.Spec, cpath, mod.Runtime); err != nil {
					report.AddError(cpath+".spec", "invalid_mcp_server", err.Error())
				}
			}
		}
	}
	if m.Integrity.Algorithm == "" {
		report.AddError("integrity.algorithm", "missing", "integrity algorithm required")
	}
	if m.Integrity.ContentTreeHash == "" {
		report.AddError("integrity.contentTreeHash", "missing", "content tree hash required")
	}
	depTypes := map[string]bool{
		"extension": true, "module": true, "mcp": true,
		"provider": true, "host_api": true,
	}
	for i, dep := range m.Dependencies {
		path := fmt.Sprintf("dependencies[%d]", i)
		if dep.ID == "" {
			report.AddError(path+".id", "missing", "dependency id required")
		}
		if dep.Type == "" {
			report.AddError(path+".type", "missing", "dependency type required")
		} else if !depTypes[dep.Type] {
			report.AddError(path+".type", "unknown_type", fmt.Sprintf("unknown dependency type: %s", dep.Type))
		}
		if dep.Version != "" {
			if _, err := dependency.ParseRange(dep.Version); err != nil {
				report.AddError(path+".version", "invalid_constraint", err.Error())
			}
		}
	}
	permissionPattern := regexp.MustCompile(`^[a-z][a-z0-9_.:-]{1,127}$`)
	for i, permission := range m.Permissions {
		if !permissionPattern.MatchString(permission.Name) {
			report.AddError(fmt.Sprintf("permissions[%d].name", i), "invalid_permission", "permission name is invalid")
		}
	}
	for i, resource := range m.Resources {
		cleaned := path.Clean(strings.ReplaceAll(resource.Path, "\\", "/"))
		if resource.Path == "" || strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.HasPrefix(cleaned, "/") {
			report.AddError(fmt.Sprintf("resources[%d].path", i), "invalid_path", "resource path must stay inside package")
		}
	}
	validateExtensionPlacement(m.Placement, &report, "placement")
	checkExtensionModulePlacementConsistency(m, &report)
	for i, mod := range m.Modules {
		modPath := fmt.Sprintf("modules[%d]", i)
		validateModulePlacement(mod.Placement, &report, modPath+".placement")
		validateDeviceRequirements(mod, modPath, &report)
		validateProvidedCapabilities(mod, modPath, &report)
		validateProviderMetadata(mod, modPath, &report)
	}
	checkMetadataSize(m.Extension.Metadata, "extension.metadata", &report)
	return report
}

func (m Manifest) ToExtensionDefinition() (domain.ExtensionDefinition, error) {
	normalized, _ := m.NormalizeCompatibility()
	report := normalized.Validate()
	if report.HasErrors() {
		msgs := make([]string, len(report.Errors))
		for i, e := range report.Errors {
			msgs[i] = fmt.Sprintf("%s: %s", e.Path, e.Message)
		}
		return domain.ExtensionDefinition{}, fmt.Errorf("%w: %s", ErrInvalidManifest, strings.Join(msgs, "; "))
	}
	version, _ := domain.ParseVersion(normalized.Extension.Version)
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID(normalized.Extension.ID),
		Name:            normalized.Extension.Name.ToDomain(),
		Description:     normalized.Extension.Description.ToDomain(),
		Version:         version,
		ManifestVersion: normalized.ManifestVersion,
		Placement:       domain.ExtensionPlacement(normalized.Placement),
		Publisher: domain.PublisherReference{
			PublisherID: normalized.Publisher.ID,
			DisplayName: normalized.Publisher.DisplayName,
			TrustLevel:  normalized.Publisher.TrustLevel,
		},
		Compatibility: domain.ExtensionCompatibility{
			MinHostVersion: normalized.Compatibility.MinHostVersion,
			MaxHostVersion: normalized.Compatibility.MaxHostVersion,
			Platforms:      normalized.Compatibility.Platforms,
			FeatureFlags:   normalized.Compatibility.FeatureFlags,
		},
		Integrity: domain.ExtensionIntegrity{
			Algorithm:       normalized.Integrity.Algorithm,
			ContentTreeHash: normalized.Integrity.ContentTreeHash,
			FileHashes:      normalized.Integrity.FileHashes,
		},
		Policies: domain.ExtensionPolicies{
			AutoUpdate:      normalized.Lifecycle.AutoUpdate,
			BackgroundTasks: normalized.Lifecycle.BackgroundTasks,
			NetworkAccess:   normalized.Lifecycle.NetworkAccess,
			Isolation:       normalized.Lifecycle.Isolation,
			Sandbox:         normalized.Lifecycle.Sandbox,
		},
	}
	for _, mod := range normalized.Modules {
		moduleDef, err := mod.ToDomain(domain.ExtensionID(normalized.Extension.ID))
		if err != nil {
			return domain.ExtensionDefinition{}, err
		}
		def.Modules = append(def.Modules, moduleDef)
	}
	for _, dep := range normalized.Dependencies {
		def.Dependencies = append(def.Dependencies, domain.DependencyDefinition{
			Type:     domain.DependencyType(dep.Type),
			ID:       dep.ID,
			Version:  dep.Version,
			Optional: dep.Optional,
			Reason:   dep.Reason,
		})
	}
	for _, sr := range normalized.SecretRefs {
		def.SecretRefs = append(def.SecretRefs, domain.SecretRefDefinition{
			Ref:       sr.Ref,
			ServiceID: sr.ServiceID,
			Purpose:   sr.Purpose,
			Required:  sr.Required,
		})
	}
	return def, nil
}

func (m ModuleMeta) ToDomain(extID domain.ExtensionID) (domain.ModuleDefinition, error) {
	var runtime *domain.RuntimeDefinition
	if m.Runtime != nil {
		timeout, _ := time.ParseDuration(m.Runtime.Timeout)
		rd := domain.RuntimeDefinition{
			Type:         domain.RuntimeType(m.Runtime.Type),
			ServiceID:    m.Runtime.ServiceID,
			EntryPoint:   m.Runtime.EntryPoint,
			WorkerCount:  m.Runtime.WorkerCount,
			Timeout:      timeout,
			Memory:       m.Runtime.Memory,
			Permissions:  m.Runtime.Permissions,
			Capabilities: m.Runtime.Capabilities,
			Env:          m.Runtime.Env,
		}
		runtime = &rd
	}
	var compat *domain.ModuleCompatibility
	if m.Compatibility != nil {
		c := domain.ModuleCompatibility{
			MinHostVersion: m.Compatibility.MinHostVersion,
			Platforms:      m.Compatibility.Platforms,
		}
		compat = &c
	}
	var policies *domain.ModulePolicies
	if m.Policies != nil {
		p := domain.ModulePolicies{
			Isolation:        m.Policies.Isolation,
			NetworkAccess:    m.Policies.NetworkAccess,
			FileSystemAccess: m.Policies.FileSystemAccess,
		}
		policies = &p
	}
	mod := domain.ModuleDefinition{
		ID:            domain.ModuleID(m.ID),
		ExtensionID:   extID,
		Name:          m.Name.ToDomain(),
		Description:   m.Description.ToDomain(),
		Type:          domain.ModuleType(m.Type),
		Version:       m.Version,
		Runtime:       runtime,
		Compatibility: deref(compat),
		Policies:      deref(policies),
		Placement:     domain.ModulePlacement(m.Placement),
	}

	if m.DeviceRequirements != nil {
		mod.DeviceRequirements = &domain.DeviceRequirements{
			Platforms:         m.DeviceRequirements.Platforms,
			Architectures:     m.DeviceRequirements.Architectures,
			MinAppVersion:     m.DeviceRequirements.MinAppVersion,
			MinRuntimeVersion: m.DeviceRequirements.MinRuntimeVersion,
			RequiredFeatures:  m.DeviceRequirements.RequiredFeatures,
		}
	}

	for _, pc := range m.ProvidedCapabilities {
		mod.ProvidedCapabilities = append(mod.ProvidedCapabilities, domain.ProvidedCapability{
			ID:       pc.ID,
			Version:  pc.Version,
			Metadata: pc.Metadata,
		})
	}

	if m.Provider != nil {
		mod.Provider = &domain.ProviderMetadata{
			ID:       m.Provider.ID,
			Priority: m.Provider.Priority,
			Labels:   m.Provider.Labels,
			Metadata: m.Provider.Metadata,
		}
	}

	for _, c := range m.Contributions {
		cd, err := c.ToDomain(extID, domain.ModuleID(m.ID))
		if err != nil {
			return domain.ModuleDefinition{}, err
		}
		mod.Contributions = append(mod.Contributions, cd)
	}
	for _, dep := range m.Dependencies {
		mod.Dependencies = append(mod.Dependencies, domain.DependencyDefinition{
			Type:     domain.DependencyType(dep.Type),
			ID:       dep.ID,
			Version:  dep.Version,
			Optional: dep.Optional,
			Reason:   dep.Reason,
		})
	}
	return mod, nil
}

func (c ContributionMeta) ToDomain(extID domain.ExtensionID, modID domain.ModuleID) (domain.ContributionDefinition, error) {
	cd := domain.ContributionDefinition{
		ID:                  domain.ContributionID(c.ID),
		ModuleID:            modID,
		ExtensionID:         extID,
		Kind:                domain.ContributionKind(c.Kind),
		Name:                c.Name.ToDomain(),
		Description:         c.Description.ToDomain(),
		Version:             c.Version,
		Definition:          c.Spec,
		RequiredPermissions: c.RequiredPermissions,
		RequiredScope:       c.RequiredScope,
	}
	for _, dep := range c.Dependencies {
		cd.Dependencies = append(cd.Dependencies, domain.DependencyDefinition{Type: domain.DependencyType(dep.Type),
			ID: dep.ID, Version: dep.Version, Optional: dep.Optional, Reason: dep.Reason})
	}
	if c.Exposure != nil {
		cd.Exposure = domain.Exposure{
			VisibleByDefault:    c.Exposure.VisibleByDefault,
			HiddenFromDiscovery: c.Exposure.HiddenFromDiscovery,
			RequiredRoles:       c.Exposure.RequiredRoles,
		}
	}
	if c.RuntimeBinding != nil {
		cd.RuntimeBinding = &domain.RuntimeBinding{
			RuntimeID:  domain.RuntimeID(c.RuntimeBinding.RuntimeID),
			Generation: c.RuntimeBinding.Generation,
		}
	}
	return cd, nil
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func validateGamePluginContribution(spec map[string]any, cpath string, moduleIDs map[string]bool) error {
	if spec == nil {
		return fmt.Errorf("game_plugin requires spec with protocolVersion")
	}
	protocolVersion, ok := spec["protocolVersion"].(string)
	if !ok || protocolVersion == "" {
		return fmt.Errorf("game_plugin requires protocolVersion")
	}
	if runtimeModuleID, ok := spec["runtimeModuleId"].(string); ok && runtimeModuleID != "" {
		if !moduleIDs[runtimeModuleID] {
			return fmt.Errorf("game_plugin references unknown module: %s", runtimeModuleID)
		}
	}
	return nil
}

func validateDesktopPetPluginContribution(spec map[string]any, cpath string, moduleIDs map[string]bool) error {
	if spec == nil {
		return nil
	}
	if runtimeModuleID, ok := spec["runtimeModuleId"].(string); ok && runtimeModuleID != "" {
		if !moduleIDs[runtimeModuleID] {
			return fmt.Errorf("desktop_pet_plugin references unknown module: %s", runtimeModuleID)
		}
	}
	return nil
}

func validateMCPServerContribution(spec map[string]any, cpath string, runtime *RuntimeMeta) error {
	if spec == nil {
		return fmt.Errorf("mcp_server contribution requires spec")
	}
	parsed, err := mcp_manifest.ParseSpec(spec)
	if err != nil {
		return fmt.Errorf("mcp_server spec parse error: %w", err)
	}
	validationErrors := mcp_manifest.Validate(parsed, cpath+".spec")
	if len(validationErrors) > 0 {
		var msgs []string
		for _, ve := range validationErrors {
			msgs = append(msgs, fmt.Sprintf("%s: %s", ve.Path, ve.Code))
		}
		return fmt.Errorf("mcp_server spec validation failed: %s", strings.Join(msgs, "; "))
	}
	if runtime != nil && runtime.Type != "" && runtime.Type != "mcp" {
		return fmt.Errorf("mcp_server contribution requires module runtime.type='mcp', got '%s'", runtime.Type)
	}
	return nil
}
