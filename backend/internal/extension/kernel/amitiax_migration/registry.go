package amitiax_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type AmitiaxMigrationSpec struct {
	LegacyPackageID    string                `json:"legacyPackageId"`
	NewExtensionID     string                `json:"newExtensionId"`
	NewModuleID        string                `json:"newModuleId"`
	DisplayName        string                `json:"displayName"`
	Publisher          string                `json:"publisher"`
	Version            string                `json:"version"`
	ManifestVersion    string                `json:"manifestVersion"`
	Modules            []ModuleMigrationSpec `json:"modules"`
	Permissions        []string              `json:"permissions,omitempty"`
	TrustLevel         string                `json:"trustLevel"`
	RequiresReapproval bool                  `json:"requiresReapproval"`
	Deprecated         bool                  `json:"deprecated,omitempty"`
	DeprecationNote    string                `json:"deprecationNote,omitempty"`
}

type ModuleMigrationSpec struct {
	LegacyModuleID  string   `json:"legacyModuleId"`
	NewModuleID     string   `json:"newModuleId"`
	Type            string   `json:"type"`
	Entry           string   `json:"entry"`
	Tools           []string `json:"tools,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	Workflows       []string `json:"workflows,omitempty"`
	MCPServers      []string `json:"mcpServers,omitempty"`
	UIContributions []string `json:"uiContributions,omitempty"`
}

type AmitiaxMigrationRegistry struct {
	mu    sync.RWMutex
	specs map[string]*AmitiaxMigrationSpec
}

func NewAmitiaxMigrationRegistry() *AmitiaxMigrationRegistry {
	return &AmitiaxMigrationRegistry{
		specs: make(map[string]*AmitiaxMigrationSpec),
	}
}

func (r *AmitiaxMigrationRegistry) Register(spec *AmitiaxMigrationSpec) error {
	if spec == nil || spec.LegacyPackageID == "" || spec.NewExtensionID == "" {
		return ErrInvalidSpec
	}
	if spec.TrustLevel == "" {
		spec.TrustLevel = "user_installed"
	}
	if spec.ManifestVersion == "" {
		spec.ManifestVersion = "v2"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.LegacyPackageID]; exists {
		return fmt.Errorf("%w: %s", ErrPackageExists, spec.LegacyPackageID)
	}
	r.specs[spec.LegacyPackageID] = spec
	return nil
}

func (r *AmitiaxMigrationRegistry) Get(legacyPackageID string) (*AmitiaxMigrationSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[legacyPackageID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPackageNotFound, legacyPackageID)
	}
	return spec, nil
}

func (r *AmitiaxMigrationRegistry) List() []*AmitiaxMigrationSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*AmitiaxMigrationSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

type AmitiaxMigrationReport struct {
	StartTime      time.Time                `json:"startTime"`
	EndTime        time.Time                `json:"endTime"`
	TotalLegacy    int                      `json:"totalLegacy"`
	MigratedCount  int                      `json:"migratedCount"`
	FailedEntries  []FailedPackageMigration `json:"failedEntries,omitempty"`
	TrustBreakdown map[string]int           `json:"trustBreakdown"`
	Status         string                   `json:"status"`
}

type FailedPackageMigration struct {
	LegacyPackageID string `json:"legacyPackageId"`
	Reason          string `json:"reason"`
}

func RunAmitiaxMigration(ctx context.Context, registry *AmitiaxMigrationRegistry, legacyPackageIDs []string) (*AmitiaxMigrationReport, error) {
	report := &AmitiaxMigrationReport{
		StartTime:      time.Now().UTC(),
		TrustBreakdown: make(map[string]int),
		Status:         "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()
	report.TotalLegacy = len(legacyPackageIDs)
	specsByLegacy := make(map[string]bool)
	for _, spec := range registry.List() {
		specsByLegacy[spec.LegacyPackageID] = true
		report.TrustBreakdown[spec.TrustLevel]++
	}
	migrated := 0
	for _, legacyID := range legacyPackageIDs {
		if specsByLegacy[legacyID] {
			migrated++
		} else {
			report.FailedEntries = append(report.FailedEntries, FailedPackageMigration{
				LegacyPackageID: legacyID,
				Reason:          "no migration spec",
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
	ErrInvalidSpec     = errors.New("amitiax_migration: invalid spec")
	ErrPackageExists   = errors.New("amitiax_migration: package exists")
	ErrPackageNotFound = errors.New("amitiax_migration: package not found")
)

var _ = json.Marshal
