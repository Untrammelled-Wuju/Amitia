package manifest_v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const ManifestVersion = 2

type Manifest struct {
	ManifestVersion int             `json:"manifestVersion"`
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
}

type ExtensionMeta struct {
	ID         string                 `json:"id"`
	Name       LocalizedText          `json:"name"`
	Description LocalizedText         `json:"description"`
	Version    string                 `json:"version"`
	License    string                 `json:"license,omitempty"`
	Homepage   string                 `json:"homepage,omitempty"`
	Repository string                 `json:"repository,omitempty"`
	Categories []string               `json:"categories,omitempty"`
	Keywords   []string               `json:"keywords,omitempty"`
	Icon       string                 `json:"icon,omitempty"`
	Metadata   map[string]any         `json:"metadata,omitempty"`
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
	ID            string                  `json:"id"`
	Name          LocalizedText           `json:"name"`
	Description   LocalizedText           `json:"description,omitempty"`
	Type          string                  `json:"type"`
	Version       string                  `json:"version,omitempty"`
	Runtime       *RuntimeMeta            `json:"runtime,omitempty"`
	Contributions []ContributionMeta      `json:"contributions,omitempty"`
	Dependencies  []Dependency            `json:"dependencies,omitempty"`
	Compatibility *ModuleCompatibility    `json:"compatibility,omitempty"`
	Policies      *ModulePolicies         `json:"policies,omitempty"`
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
	Type        string            `json:"type"`
	EntryPoint  string            `json:"entryPoint,omitempty"`
	WorkerCount int               `json:"workerCount,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	Memory      int64             `json:"memory,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Capabilities map[string]bool  `json:"capabilities,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

type ContributionMeta struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"`
	Name        LocalizedText          `json:"name"`
	Description LocalizedText          `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Spec        map[string]any         `json:"spec,omitempty"`
	RequiredPermissions []string       `json:"requiredPermissions,omitempty"`
	RequiredScope []string             `json:"requiredScope,omitempty"`
	Exposure    *ExposureMeta          `json:"exposure,omitempty"`
	RuntimeBinding *RuntimeBindingMeta `json:"runtimeBinding,omitempty"`
	Dependencies []Dependency          `json:"dependencies,omitempty"`
}

type ExposureMeta struct {
	VisibleByDefault   bool     `json:"visibleByDefault,omitempty"`
	HiddenFromDiscovery bool    `json:"hiddenFromDiscovery,omitempty"`
	RequiredRoles      []string `json:"requiredRoles,omitempty"`
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
	Name      string `json:"name"`
	Reason    string `json:"reason,omitempty"`
	Required  bool   `json:"required,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

type ResourceMeta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"`
	Size int64 `json:"size,omitempty"`
}

type LifecycleMeta struct {
	AutoUpdate      bool `json:"autoUpdate,omitempty"`
	BackgroundTasks bool `json:"backgroundTasks,omitempty"`
	NetworkAccess   bool `json:"networkAccess,omitempty"`
	Isolation       string `json:"isolation,omitempty"`
	Sandbox         bool `json:"sandbox,omitempty"`
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

type LocalizedText struct {
	Default    string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

func (l LocalizedText) ToDomain() domain.LocalizedText {
	return domain.LocalizedText{
		Default:      l.Default,
		Translations: l.Translations,
	}
}

type ValidationReport struct {
	Errors []ValidationError
	Warnings []ValidationWarning
}

type ValidationError struct {
	Path    string
	Code    string
	Message string
}

type ValidationWarning struct {
	Path    string
	Message string
}

func (r *ValidationReport) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *ValidationReport) AddError(path, code, msg string) {
	r.Errors = append(r.Errors, ValidationError{Path: path, Code: code, Message: msg})
}

func (r *ValidationReport) AddWarning(path, msg string) {
	r.Warnings = append(r.Warnings, ValidationWarning{Path: path, Message: msg})
}

var (
	ErrInvalidManifest      = errors.New("manifest_v2: invalid manifest")
	ErrUnsupportedVersion   = errors.New("manifest_v2: unsupported manifest version")
	ErrMissingField         = errors.New("manifest_v2: missing required field")
	ErrInvalidExtensionID   = errors.New("manifest_v2: invalid extension id")
	ErrInvalidVersion       = errors.New("manifest_v2: invalid version")
	ErrDuplicateModule      = errors.New("manifest_v2: duplicate module id")
	ErrDuplicateContribution = errors.New("manifest_v2: duplicate contribution id")
	ErrUnknownModuleType    = errors.New("manifest_v2: unknown module type")
	ErrUnknownContributionKind = errors.New("manifest_v2: unknown contribution kind")
)

func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: parse error: %v", ErrInvalidManifest, err)
	}
	return m, nil
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
		"builtin": true, "javascript": true, "data_only": true,
	}
	runtimeTypes := map[string]bool{
		"javascript": true, "mcp": true, "workflow": true, "static": true,
	}
	contributionKinds := map[string]bool{
		"tool": true, "agent_skill": true, "workflow": true,
		"mcp_server": true,
	}
	unsupportedContributionKinds := map[string]bool{
		"provider": true, "hook": true, "event_subscription": true,
		"schedule": true, "background_task": true,
		"ui_page": true, "ui_panel": true, "ui_chat": true,
		"ui_context_action": true, "ui_desktop": true, "resource": true,
	}
	unsupportedModuleTypes := map[string]bool{
		"native": true, "wasm": true, "service": true,
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
		} else if unsupportedModuleTypes[mod.Type] {
			report.AddError(path+".type", "unsupported_runtime", fmt.Sprintf(`{"code":"unsupported_runtime","moduleId":"%s","runtimeType":"%s"}`, mod.ID, mod.Type))
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
			} else if unsupportedContributionKinds[c.Kind] {
				report.AddError(cpath+".kind", "unsupported_contribution", fmt.Sprintf(`{"code":"unsupported_contribution","moduleId":"%s","contributionId":"%s","kind":"%s"}`, mod.ID, c.ID, c.Kind))
			} else if !contributionKinds[c.Kind] {
				report.AddError(cpath+".kind", "unknown_kind", fmt.Sprintf("unknown contribution kind: %s", c.Kind))
			}
			if c.Name.Default == "" {
				report.AddError(cpath+".name.default", "missing", "contribution name required")
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
	}
	return report
}

func (m Manifest) ToExtensionDefinition() (domain.ExtensionDefinition, error) {
	report := m.Validate()
	if report.HasErrors() {
	 msgs := make([]string, len(report.Errors))
	 for i, e := range report.Errors {
		msgs[i] = fmt.Sprintf("%s: %s", e.Path, e.Message)
	 }
	 return domain.ExtensionDefinition{}, fmt.Errorf("%w: %s", ErrInvalidManifest, strings.Join(msgs, "; "))
	}
	version, _ := domain.ParseVersion(m.Extension.Version)
	def := domain.ExtensionDefinition{
		ID:              domain.ExtensionID(m.Extension.ID),
		Name:            m.Extension.Name.ToDomain(),
		Description:     m.Extension.Description.ToDomain(),
		Version:         version,
		ManifestVersion: m.ManifestVersion,
		Publisher: domain.PublisherReference{
			PublisherID: m.Publisher.ID,
			DisplayName: m.Publisher.DisplayName,
			TrustLevel:  m.Publisher.TrustLevel,
		},
		Compatibility: domain.ExtensionCompatibility{
			MinHostVersion: m.Compatibility.MinHostVersion,
			MaxHostVersion: m.Compatibility.MaxHostVersion,
			Platforms:      m.Compatibility.Platforms,
			FeatureFlags:   m.Compatibility.FeatureFlags,
		},
		Integrity: domain.ExtensionIntegrity{
			Algorithm:       m.Integrity.Algorithm,
			ContentTreeHash: m.Integrity.ContentTreeHash,
			FileHashes:      m.Integrity.FileHashes,
		},
		Policies: domain.ExtensionPolicies{
			AutoUpdate:      m.Lifecycle.AutoUpdate,
			BackgroundTasks: m.Lifecycle.BackgroundTasks,
			NetworkAccess:   m.Lifecycle.NetworkAccess,
			Isolation:       m.Lifecycle.Isolation,
			Sandbox:         m.Lifecycle.Sandbox,
		},
	}
	for _, mod := range m.Modules {
		moduleDef, err := mod.ToDomain(domain.ExtensionID(m.Extension.ID))
		if err != nil {
			return domain.ExtensionDefinition{}, err
		}
		def.Modules = append(def.Modules, moduleDef)
	}
	for _, dep := range m.Dependencies {
		def.Dependencies = append(def.Dependencies, domain.DependencyDefinition{
			Type:     domain.DependencyType(dep.Type),
			ID:       dep.ID,
			Version:  dep.Version,
			Optional: dep.Optional,
			Reason:   dep.Reason,
		})
	}
	return def, nil
}

func (m ModuleMeta) ToDomain(extID domain.ExtensionID) (domain.ModuleDefinition, error) {
	var runtime *domain.RuntimeDefinition
	if m.Runtime != nil {
		timeout, _ := time.ParseDuration(m.Runtime.Timeout)
		rd := domain.RuntimeDefinition{
			Type:        domain.RuntimeType(m.Runtime.Type),
			EntryPoint:  m.Runtime.EntryPoint,
			WorkerCount: m.Runtime.WorkerCount,
			Timeout:     timeout,
			Memory:      m.Runtime.Memory,
			Permissions: m.Runtime.Permissions,
			Capabilities: m.Runtime.Capabilities,
			Env:         m.Runtime.Env,
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
		ID:           domain.ModuleID(m.ID),
		ExtensionID:  extID,
		Name:         m.Name.ToDomain(),
		Description:  m.Description.ToDomain(),
		Type:         domain.ModuleType(m.Type),
		Version:      m.Version,
		Runtime:      runtime,
		Compatibility: deref(compat),
		Policies:     deref(policies),
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
		ID:                domain.ContributionID(c.ID),
		ModuleID:          modID,
		ExtensionID:       extID,
		Kind:              domain.ContributionKind(c.Kind),
		Name:              c.Name.ToDomain(),
		Description:       c.Description.ToDomain(),
		Version:           c.Version,
		Definition:        c.Spec,
		RequiredPermissions: c.RequiredPermissions,
		RequiredScope:     c.RequiredScope,
	}
	if c.Exposure != nil {
		cd.Exposure = domain.Exposure{
			VisibleByDefault:   c.Exposure.VisibleByDefault,
			HiddenFromDiscovery: c.Exposure.HiddenFromDiscovery,
			RequiredRoles:      c.Exposure.RequiredRoles,
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
