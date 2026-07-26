package skill_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const SystemSkillExtensionID = "system/amitia-core"
const SkillModuleID = "agent-skills"

type SkillContributionSpec struct {
	SkillID         string          `json:"skillId"`
	LegacySkillID   string          `json:"legacySkillId"`
	Module          string          `json:"module"`
	Category        string          `json:"category"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	TriggerPatterns []string        `json:"triggerPatterns,omitempty"`
	InputSchema     json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema    json.RawMessage `json:"outputSchema,omitempty"`
	RequiredTools   []string        `json:"requiredTools,omitempty"`
	Permissions     []string        `json:"permissions,omitempty"`
	RuntimeBinding  string          `json:"runtimeBinding"`
	RiskLevel       string          `json:"riskLevel,omitempty"`
	Deprecated      bool            `json:"deprecated,omitempty"`
	DeprecationNote string          `json:"deprecationNote,omitempty"`
}

type SkillMigrationRegistry struct {
	mu              sync.RWMutex
	specs           map[string]*SkillContributionSpec
	specsByLegacy   map[string]string
	categoriesByMod map[string]map[string]int
}

func NewSkillMigrationRegistry() *SkillMigrationRegistry {
	return &SkillMigrationRegistry{
		specs:           make(map[string]*SkillContributionSpec),
		specsByLegacy:   make(map[string]string),
		categoriesByMod: make(map[string]map[string]int),
	}
}

func (r *SkillMigrationRegistry) Register(spec *SkillContributionSpec) error {
	if spec == nil || spec.SkillID == "" {
		return ErrInvalidSpec
	}
	if spec.LegacySkillID == "" {
		spec.LegacySkillID = spec.SkillID
	}
	if spec.RuntimeBinding == "" {
		spec.RuntimeBinding = "host_internal"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.SkillID]; exists {
		return fmt.Errorf("%w: %s", ErrSkillExists, spec.SkillID)
	}
	r.specs[spec.SkillID] = spec
	r.specsByLegacy[spec.LegacySkillID] = spec.SkillID
	if r.categoriesByMod[spec.Module] == nil {
		r.categoriesByMod[spec.Module] = make(map[string]int)
	}
	r.categoriesByMod[spec.Module][spec.Category]++
	return nil
}

func (r *SkillMigrationRegistry) GetByCanonicalID(skillID string) (*SkillContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[skillID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, skillID)
	}
	return spec, nil
}

func (r *SkillMigrationRegistry) GetByLegacyID(legacyID string) (*SkillContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonicalID, exists := r.specsByLegacy[legacyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLegacySkillNotFound, legacyID)
	}
	return r.specs[canonicalID], nil
}

func (r *SkillMigrationRegistry) List() []*SkillContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SkillContributionSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

func (r *SkillMigrationRegistry) ListByModule(module string) []*SkillContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SkillContributionSpec, 0)
	for _, spec := range r.specs {
		if spec.Module == module {
			out = append(out, spec)
		}
	}
	return out
}

type LegacySkillAdapter struct {
	registry *SkillMigrationRegistry
}

func NewLegacySkillAdapter(registry *SkillMigrationRegistry) *LegacySkillAdapter {
	return &LegacySkillAdapter{registry: registry}
}

func (a *LegacySkillAdapter) TranslateLegacySkillID(legacyID string) (string, error) {
	spec, err := a.registry.GetByLegacyID(legacyID)
	if err != nil {
		return "", err
	}
	return spec.SkillID, nil
}

type SkillMigrationReport struct {
	StartTime        time.Time             `json:"startTime"`
	EndTime          time.Time             `json:"endTime"`
	TotalLegacy      int                   `json:"totalLegacy"`
	MigratedCount    int                   `json:"migratedCount"`
	FailedEntries    []FailedSkillMigration `json:"failedEntries,omitempty"`
	CategorySummary  map[string]int        `json:"categorySummary"`
	Status           string                `json:"status"`
}

type FailedSkillMigration struct {
	LegacySkillID string `json:"legacySkillId"`
	Reason        string `json:"reason"`
}

func RunSkillMigration(ctx context.Context, registry *SkillMigrationRegistry, legacySkillIDs []string) (*SkillMigrationReport, error) {
	report := &SkillMigrationReport{
		StartTime:       time.Now().UTC(),
		CategorySummary: make(map[string]int),
		Status:          "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()
	report.TotalLegacy = len(legacySkillIDs)
	migrated := 0
	specs := registry.List()
	specsByLegacy := make(map[string]bool, len(specs))
	for _, spec := range specs {
		specsByLegacy[spec.LegacySkillID] = true
		report.CategorySummary[spec.Category]++
	}
	for _, legacyID := range legacySkillIDs {
		if specsByLegacy[legacyID] {
			migrated++
		} else {
			report.FailedEntries = append(report.FailedEntries, FailedSkillMigration{
				LegacySkillID: legacyID,
				Reason:        "no canonical mapping",
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
	ErrInvalidSpec        = errors.New("skill_migration: invalid spec")
	ErrSkillExists        = errors.New("skill_migration: skill exists")
	ErrSkillNotFound      = errors.New("skill_migration: skill not found")
	ErrLegacySkillNotFound = errors.New("skill_migration: legacy skill not found")
)
