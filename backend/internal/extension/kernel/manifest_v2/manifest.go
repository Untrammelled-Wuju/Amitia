package manifest_v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
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
		"ui_context_action": true, "ui_desktop": true, "resource": true,
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
			Type:         domain.RuntimeType(m.Runtime.Type),
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

const manifestV2Schema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["manifestVersion", "extension", "publisher"],
  "properties": {
    "manifestVersion": {"type": "integer", "const": 2},
    "extension": {
      "type": "object",
      "required": ["id", "name", "version"],
      "properties": {
        "id": {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]*(\\.[a-z0-9-]+)+/[a-z0-9][a-z0-9-]*$"},
        "name": {
          "type": "object",
          "required": ["default"],
          "properties": {
            "default": {"type": "string", "minLength": 1},
            "translations": {"type": "object"}
          }
        },
        "description": {
          "type": "object",
          "properties": {
            "default": {"type": "string"},
            "translations": {"type": "object"}
          }
        },
        "version": {"type": "string", "pattern": "^(\\d+)\\.(\\d+)\\.(\\d+)(?:-[0-9A-Za-z-.]+)?(?:\\+[0-9A-Za-z-.]+)?$"},
        "license": {"type": "string"},
        "homepage": {"type": "string"},
        "repository": {"type": "string"},
        "categories": {"type": "array", "items": {"type": "string"}},
        "keywords": {"type": "array", "items": {"type": "string"}},
        "icon": {"type": "string"},
        "metadata": {"type": "object"}
      }
    },
    "publisher": {
      "type": "object",
      "required": ["id", "displayName"],
      "properties": {
        "id": {"type": "string", "minLength": 1},
        "displayName": {"type": "string", "minLength": 1},
        "trustLevel": {"type": "string", "enum": ["untrusted", "trusted", "verified", "official"]},
        "contact": {"type": "string"},
        "website": {"type": "string"}
      }
    },
    "compatibility": {
      "type": "object",
      "properties": {
        "minHostVersion": {"type": "string"},
        "maxHostVersion": {"type": "string"},
        "platforms": {"type": "array", "items": {"type": "string"}},
        "featureFlags": {"type": "array", "items": {"type": "string"}}
      }
    },
    "modules": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "name", "type"],
        "properties": {
          "id": {"type": "string", "minLength": 1},
          "name": {
            "type": "object",
            "required": ["default"],
            "properties": {
              "default": {"type": "string", "minLength": 1},
              "translations": {"type": "object"}
            }
          },
          "description": {
            "type": "object",
            "properties": {
              "default": {"type": "string"},
              "translations": {"type": "object"}
            }
          },
          "type": {"type": "string", "enum": ["builtin", "javascript", "data_only", "wasm", "native", "service"]},
          "version": {"type": "string"},
          "runtime": {
            "type": "object",
            "required": ["type"],
            "properties": {
              "type": {"type": "string", "enum": ["javascript", "mcp", "workflow", "static", "wasm", "service"]},
              "entryPoint": {"type": "string"},
              "workerCount": {"type": "integer"},
              "timeout": {"type": "string"},
              "memory": {"type": "integer"},
              "permissions": {"type": "array", "items": {"type": "string"}},
              "capabilities": {"type": "object"},
              "env": {"type": "object"}
            }
          },
          "contributions": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["id", "kind", "name"],
              "properties": {
                "id": {"type": "string", "minLength": 1},
                "kind": {"type": "string", "enum": ["tool", "agent_skill", "workflow", "mcp_server", "provider", "hook", "event_subscription", "schedule", "background_task", "ui_page", "ui_panel", "ui_chat", "ui_context_action", "ui_desktop", "resource", "game_plugin", "desktop_pet_plugin"]},
                "name": {
                  "type": "object",
                  "required": ["default"],
                  "properties": {
                    "default": {"type": "string", "minLength": 1},
                    "translations": {"type": "object"}
                  }
                },
                "description": {
                  "type": "object",
                  "properties": {
                    "default": {"type": "string"},
                    "translations": {"type": "object"}
                  }
                },
                "version": {"type": "string"},
                "spec": {"type": "object"},
                "requiredPermissions": {"type": "array", "items": {"type": "string"}},
                "requiredScope": {"type": "array", "items": {"type": "string"}},
                "exposure": {
                  "type": "object",
                  "properties": {
                    "visibleByDefault": {"type": "boolean"},
                    "hiddenFromDiscovery": {"type": "boolean"},
                    "requiredRoles": {"type": "array", "items": {"type": "string"}}
                  }
                },
                "runtimeBinding": {
                  "type": "object",
                  "required": ["runtimeId"],
                  "properties": {
                    "runtimeId": {"type": "string", "minLength": 1},
                    "generation": {"type": "integer"}
                  }
                },
                "dependencies": {"type": "array", "items": {"type": "object"}}
              }
            }
          },
          "dependencies": {"type": "array", "items": {"type": "object"}},
          "compatibility": {
            "type": "object",
            "properties": {
              "minHostVersion": {"type": "string"},
              "platforms": {"type": "array", "items": {"type": "string"}}
            }
          },
          "policies": {
            "type": "object",
            "properties": {
              "isolation": {"type": "string"},
              "networkAccess": {"type": "boolean"},
              "fileSystemAccess": {"type": "boolean"}
            }
          }
        }
      }
    },
    "dependencies": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "id"],
        "properties": {
          "type": {"type": "string", "enum": ["extension", "module", "mcp", "provider", "host_api"]},
          "id": {"type": "string", "minLength": 1},
          "version": {"type": "string"},
          "optional": {"type": "boolean"},
          "reason": {"type": "string"}
        }
      }
    },
    "permissions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string", "minLength": 1},
          "reason": {"type": "string"},
          "required": {"type": "boolean"},
          "scope": {"type": "string"}
        }
      }
    },
    "resources": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "type", "path"],
        "properties": {
          "id": {"type": "string", "minLength": 1},
          "type": {"type": "string", "minLength": 1},
          "path": {"type": "string", "minLength": 1},
          "hash": {"type": "string"},
          "size": {"type": "integer"}
        }
      }
    },
    "lifecycle": {
      "type": "object",
      "properties": {
        "autoUpdate": {"type": "boolean"},
        "backgroundTasks": {"type": "boolean"},
        "networkAccess": {"type": "boolean"},
        "isolation": {"type": "string"},
        "sandbox": {"type": "boolean"}
      }
    },
    "integrity": {
      "type": "object",
      "required": ["algorithm", "contentTreeHash"],
      "properties": {
        "algorithm": {"type": "string", "minLength": 1},
        "contentTreeHash": {"type": "string", "minLength": 1},
        "fileHashes": {"type": "object"}
      }
    },
    "development": {
      "type": "object",
      "properties": {
        "developerMode": {"type": "boolean"},
        "hotReload": {"type": "boolean"},
        "sourceMaps": {"type": "boolean"},
        "testEntry": {"type": "string"},
        "watchPaths": {"type": "array", "items": {"type": "string"}}
      }
    }
  }
}`

func schemaValueEqual(a, b any) bool {
	if af, ok := a.(float64); ok {
		if bf, ok2 := b.(float64); ok2 {
			return af == bf
		}
	}
	return a == b
}

func checkSchemaType(value any, t string) bool {
	switch t {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		f, ok := value.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	}
	return true
}

func validateSchemaValue(value any, schema map[string]any, path string, report *ValidationReport) {
	if t, ok := schema["type"].(string); ok {
		if !checkSchemaType(value, t) {
			report.AddError(path, "schema_type", fmt.Sprintf("expected type %s, got %T", t, value))
			return
		}
	}
	if c, ok := schema["const"]; ok {
		if !schemaValueEqual(value, c) {
			report.AddError(path, "schema_const", fmt.Sprintf("expected const %v, got %v", c, value))
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if schemaValueEqual(value, e) {
				found = true
				break
			}
		}
		if !found {
			report.AddError(path, "schema_enum", fmt.Sprintf("value %v not in enum %v", value, enum))
		}
	}
	if pattern, ok := schema["pattern"].(string); ok {
		if s, ok2 := value.(string); ok2 {
			if re, err := regexp.Compile(pattern); err == nil {
				if !re.MatchString(s) {
					report.AddError(path, "schema_pattern", fmt.Sprintf("value %q does not match pattern %s", s, pattern))
				}
			}
		}
	}
	if minLen, ok := schema["minLength"].(float64); ok {
		if s, ok2 := value.(string); ok2 {
			if len(s) < int(minLen) {
				report.AddError(path, "schema_minLength", fmt.Sprintf("string length %d less than minLength %d", len(s), int(minLen)))
			}
		}
	}
	switch t := schema["type"].(string); t {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return
		}
		if required, ok := schema["required"].([]any); ok {
			for _, r := range required {
				if rs, ok2 := r.(string); ok2 {
					if _, exists := obj[rs]; !exists {
						report.AddError(path+"."+rs, "schema_required", fmt.Sprintf("required field %s missing", rs))
					}
				}
			}
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			for key, val := range obj {
				if ps, ok2 := props[key].(map[string]any); ok2 {
					validateSchemaValue(val, ps, path+"."+key, report)
				}
			}
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return
		}
		if minItems, ok := schema["minItems"].(float64); ok {
			if len(arr) < int(minItems) {
				report.AddError(path, "schema_minItems", fmt.Sprintf("array length %d less than minItems %d", len(arr), int(minItems)))
			}
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range arr {
				validateSchemaValue(item, items, fmt.Sprintf("%s[%d]", path, i), report)
			}
		}
	}
}

func (m Manifest) ValidateWithSchema() *ValidationReport {
	report := &ValidationReport{}
	data, err := json.Marshal(m)
	if err != nil {
		report.AddError("", "schema_marshal", fmt.Sprintf("failed to marshal manifest: %v", err))
		return report
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		report.AddError("", "schema_parse", fmt.Sprintf("failed to parse manifest: %v", err))
		return report
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(manifestV2Schema), &schema); err != nil {
		report.AddError("", "schema_parse", fmt.Sprintf("failed to parse schema: %v", err))
		return report
	}
	validateSchemaValue(raw, schema, "", report)
	return report
}

func ValidateFile(filePath string) (*Manifest, *ValidationReport) {
	report := &ValidationReport{}
	data, err := os.ReadFile(filePath)
	if err != nil {
		report.AddErrorWithLocation("", "file_read", fmt.Sprintf("failed to read file: %v", err), filePath, 0, 0)
		return nil, report
	}
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if err := dec.Decode(&m); err != nil {
		offset := dec.InputOffset()
		if se, ok := err.(*json.SyntaxError); ok {
			offset = se.Offset
		} else if ute, ok := err.(*json.UnmarshalTypeError); ok {
			offset = ute.Offset
		}
		line, col := offsetToLineColumn(data, offset)
		report.AddErrorWithLocation("", "parse_error", fmt.Sprintf("failed to parse manifest: %v", err), filePath, line, col)
		return nil, report
	}
	r := m.Validate()
	r.AttachFile(filePath)
	return &m, &r
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

func offsetToLineColumn(data []byte, offset int64) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line := 1
	col := 1
	for i := int64(0); i < offset; i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
