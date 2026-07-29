package plugin_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type PluginContributionSpec struct {
	PluginID        string   `json:"pluginId"`
	LegacyPluginID  string   `json:"legacyPluginId"`
	ExtensionID     string   `json:"extensionId"`
	ModuleID        string   `json:"moduleId"`
	DisplayName     string   `json:"displayName"`
	Description     string   `json:"description"`
	Version         string   `json:"version"`
	Category        string   `json:"category"`
	EntryKind       string   `json:"entryKind"`
	EntryPath       string   `json:"entryPath"`
	SchemaPath      string   `json:"schemaPath,omitempty"`
	Tools           []string `json:"tools,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	Workflows       []string `json:"workflows,omitempty"`
	Permissions     []string `json:"permissions,omitempty"`
	RuntimeBinding  string   `json:"runtimeBinding"`
	TrustLevel      string   `json:"trustLevel"`
	Publisher       string   `json:"publisher"`
	Deprecated      bool     `json:"deprecated,omitempty"`
	DeprecationNote string   `json:"deprecationNote,omitempty"`
}

type PluginMigrationRegistry struct {
	mu            sync.RWMutex
	specs         map[string]*PluginContributionSpec
	specsByLegacy map[string]string
}

func NewPluginMigrationRegistry() *PluginMigrationRegistry {
	return &PluginMigrationRegistry{
		specs:         make(map[string]*PluginContributionSpec),
		specsByLegacy: make(map[string]string),
	}
}

func (r *PluginMigrationRegistry) Register(spec *PluginContributionSpec) error {
	if spec == nil || spec.PluginID == "" {
		return ErrInvalidSpec
	}
	if spec.LegacyPluginID == "" {
		spec.LegacyPluginID = spec.PluginID
	}
	if spec.RuntimeBinding == "" {
		spec.RuntimeBinding = "javascript_main"
	}
	if spec.TrustLevel == "" {
		spec.TrustLevel = "user_installed"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.PluginID]; exists {
		return fmt.Errorf("%w: %s", ErrPluginExists, spec.PluginID)
	}
	r.specs[spec.PluginID] = spec
	r.specsByLegacy[spec.LegacyPluginID] = spec.PluginID
	return nil
}

func (r *PluginMigrationRegistry) Get(pluginID string) (*PluginContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[pluginID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, pluginID)
	}
	return spec, nil
}

func (r *PluginMigrationRegistry) List() []*PluginContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*PluginContributionSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

type LegacyPluginAdapter struct {
	registry *PluginMigrationRegistry
}

func NewLegacyPluginAdapter(registry *PluginMigrationRegistry) *LegacyPluginAdapter {
	return &LegacyPluginAdapter{registry: registry}
}

func (a *LegacyPluginAdapter) TranslateLegacyPluginID(legacyID string) (string, error) {
	a.registry.mu.RLock()
	defer a.registry.mu.RUnlock()
	canonicalID, exists := a.registry.specsByLegacy[legacyID]
	if !exists {
		return "", fmt.Errorf("%w: %s", ErrLegacyPluginNotFound, legacyID)
	}
	return canonicalID, nil
}

type PluginMigrationReport struct {
	StartTime       time.Time               `json:"startTime"`
	EndTime         time.Time               `json:"endTime"`
	TotalLegacy     int                     `json:"totalLegacy"`
	MigratedCount   int                     `json:"migratedCount"`
	FailedEntries   []FailedPluginMigration `json:"failedEntries,omitempty"`
	CategorySummary map[string]int          `json:"categorySummary"`
	TrustBreakdown  map[string]int          `json:"trustBreakdown"`
	Status          string                  `json:"status"`
}

type FailedPluginMigration struct {
	LegacyPluginID string `json:"legacyPluginId"`
	Reason         string `json:"reason"`
}

func RunPluginMigration(ctx context.Context, registry *PluginMigrationRegistry, legacyPluginIDs []string) (*PluginMigrationReport, error) {
	report := &PluginMigrationReport{
		StartTime:       time.Now().UTC(),
		CategorySummary: make(map[string]int),
		TrustBreakdown:  make(map[string]int),
		Status:          "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()
	report.TotalLegacy = len(legacyPluginIDs)
	specs := registry.List()
	specsByLegacy := make(map[string]bool, len(specs))
	for _, spec := range specs {
		specsByLegacy[spec.LegacyPluginID] = true
		report.CategorySummary[spec.Category]++
		report.TrustBreakdown[spec.TrustLevel]++
	}
	migrated := 0
	for _, legacyID := range legacyPluginIDs {
		if specsByLegacy[legacyID] {
			migrated++
		} else {
			report.FailedEntries = append(report.FailedEntries, FailedPluginMigration{
				LegacyPluginID: legacyID,
				Reason:         "no canonical mapping",
			})
		}
	}
	report.MigratedCount = migrated
	if len(report.FailedEntries) > 0 {
		report.Status = "completed_with_warnings"
	}
	return report, nil
}

var (
	ErrInvalidSpec          = errors.New("plugin_migration: invalid spec")
	ErrPluginExists         = errors.New("plugin_migration: plugin exists")
	ErrPluginNotFound       = errors.New("plugin_migration: plugin not found")
	ErrLegacyPluginNotFound = errors.New("plugin_migration: legacy plugin not found")
)

var _ = json.Marshal
