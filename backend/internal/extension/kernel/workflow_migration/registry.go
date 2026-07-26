package workflow_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const SystemWorkflowExtensionID = "system/amitia-core"
const WorkflowModuleID = "workflows"

type WorkflowContributionSpec struct {
	WorkflowID      string          `json:"workflowId"`
	LegacyWorkflowID string         `json:"legacyWorkflowId"`
	DisplayName     string          `json:"displayName"`
	Description     string          `json:"description"`
	Category        string          `json:"category"`
	Steps           []WorkflowStep  `json:"steps"`
	InputSchema     json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema    json.RawMessage `json:"outputSchema,omitempty"`
	RequiredTools   []string        `json:"requiredTools,omitempty"`
	Permissions     []string        `json:"permissions,omitempty"`
	RuntimeBinding  string          `json:"runtimeBinding"`
	RiskLevel       string          `json:"riskLevel,omitempty"`
	MaxConcurrency  int             `json:"maxConcurrency,omitempty"`
	Timeout         time.Duration   `json:"timeout,omitempty"`
	Deprecated      bool            `json:"deprecated,omitempty"`
	DeprecationNote string          `json:"deprecationNote,omitempty"`
}

type WorkflowStep struct {
	StepID    string          `json:"stepId"`
	Type      string          `json:"type"`
	ToolID    string          `json:"toolId,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Condition string          `json:"condition,omitempty"`
	OnError   string          `json:"onError,omitempty"`
}

type WorkflowMigrationRegistry struct {
	mu            sync.RWMutex
	specs         map[string]*WorkflowContributionSpec
	specsByLegacy map[string]string
}

func NewWorkflowMigrationRegistry() *WorkflowMigrationRegistry {
	return &WorkflowMigrationRegistry{
		specs:         make(map[string]*WorkflowContributionSpec),
		specsByLegacy: make(map[string]string),
	}
}

func (r *WorkflowMigrationRegistry) Register(spec *WorkflowContributionSpec) error {
	if spec == nil || spec.WorkflowID == "" {
		return ErrInvalidSpec
	}
	if spec.LegacyWorkflowID == "" {
		spec.LegacyWorkflowID = spec.WorkflowID
	}
	if spec.RuntimeBinding == "" {
		spec.RuntimeBinding = "host_internal"
	}
	if spec.MaxConcurrency <= 0 {
		spec.MaxConcurrency = 1
	}
	if spec.Timeout <= 0 {
		spec.Timeout = 5 * time.Minute
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.WorkflowID]; exists {
		return fmt.Errorf("%w: %s", ErrWorkflowExists, spec.WorkflowID)
	}
	r.specs[spec.WorkflowID] = spec
	r.specsByLegacy[spec.LegacyWorkflowID] = spec.WorkflowID
	return nil
}

func (r *WorkflowMigrationRegistry) GetByCanonicalID(workflowID string) (*WorkflowContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[workflowID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrWorkflowNotFound, workflowID)
	}
	return spec, nil
}

func (r *WorkflowMigrationRegistry) List() []*WorkflowContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*WorkflowContributionSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

type LegacyWorkflowAdapter struct {
	registry *WorkflowMigrationRegistry
}

func NewLegacyWorkflowAdapter(registry *WorkflowMigrationRegistry) *LegacyWorkflowAdapter {
	return &LegacyWorkflowAdapter{registry: registry}
}

func (a *LegacyWorkflowAdapter) TranslateLegacyWorkflowID(legacyID string) (string, error) {
	r := a.registry
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonicalID, exists := r.specsByLegacy[legacyID]
	if !exists {
		return "", fmt.Errorf("%w: %s", ErrLegacyWorkflowNotFound, legacyID)
	}
	return canonicalID, nil
}

type WorkflowMigrationReport struct {
	StartTime        time.Time              `json:"startTime"`
	EndTime          time.Time              `json:"endTime"`
	TotalLegacy      int                    `json:"totalLegacy"`
	MigratedCount    int                    `json:"migratedCount"`
	FailedEntries    []FailedWorkflowMigration `json:"failedEntries,omitempty"`
	CategorySummary  map[string]int         `json:"categorySummary"`
	Status           string                 `json:"status"`
}

type FailedWorkflowMigration struct {
	LegacyWorkflowID string `json:"legacyWorkflowId"`
	Reason           string `json:"reason"`
}

func RunWorkflowMigration(ctx context.Context, registry *WorkflowMigrationRegistry, legacyWorkflowIDs []string) (*WorkflowMigrationReport, error) {
	report := &WorkflowMigrationReport{
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
	report.TotalLegacy = len(legacyWorkflowIDs)
	specs := registry.List()
	specsByLegacy := make(map[string]bool, len(specs))
	for _, spec := range specs {
		specsByLegacy[spec.LegacyWorkflowID] = true
		report.CategorySummary[spec.Category]++
	}
	migrated := 0
	for _, legacyID := range legacyWorkflowIDs {
		if specsByLegacy[legacyID] {
			migrated++
		} else {
			report.FailedEntries = append(report.FailedEntries, FailedWorkflowMigration{
				LegacyWorkflowID: legacyID,
				Reason:           "no canonical mapping",
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
	ErrInvalidSpec           = errors.New("workflow_migration: invalid spec")
	ErrWorkflowExists        = errors.New("workflow_migration: workflow exists")
	ErrWorkflowNotFound      = errors.New("workflow_migration: workflow not found")
	ErrLegacyWorkflowNotFound = errors.New("workflow_migration: legacy workflow not found")
)
