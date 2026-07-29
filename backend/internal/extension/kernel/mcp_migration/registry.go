package mcp_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const SystemMCPExtensionID = "system/amitia-core"
const MCPModuleID = "mcp-servers"

type MCPContributionSpec struct {
	ServerID        string            `json:"serverId"`
	LegacyServerID  string            `json:"legacyServerId"`
	DisplayName     string            `json:"displayName"`
	Description     string            `json:"description"`
	Transport       string            `json:"transport"`
	Endpoint        string            `json:"endpoint,omitempty"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Tools           []string          `json:"tools,omitempty"`
	Resources       []string          `json:"resources,omitempty"`
	Prompts         []string          `json:"prompts,omitempty"`
	Permissions     []string          `json:"permissions,omitempty"`
	RuntimeBinding  string            `json:"runtimeBinding"`
	Scope           string            `json:"scope"`
	AutoStart       bool              `json:"autoStart"`
	Deprecated      bool              `json:"deprecated,omitempty"`
	DeprecationNote string            `json:"deprecationNote,omitempty"`
}

type MCPMigrationRegistry struct {
	mu            sync.RWMutex
	specs         map[string]*MCPContributionSpec
	specsByLegacy map[string]string
}

func NewMCPMigrationRegistry() *MCPMigrationRegistry {
	return &MCPMigrationRegistry{
		specs:         make(map[string]*MCPContributionSpec),
		specsByLegacy: make(map[string]string),
	}
}

func (r *MCPMigrationRegistry) Register(spec *MCPContributionSpec) error {
	if spec == nil || spec.ServerID == "" {
		return ErrInvalidSpec
	}
	if spec.LegacyServerID == "" {
		spec.LegacyServerID = spec.ServerID
	}
	if spec.RuntimeBinding == "" {
		spec.RuntimeBinding = "mcp_protocol"
	}
	if spec.Scope == "" {
		spec.Scope = "global"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.ServerID]; exists {
		return fmt.Errorf("%w: %s", ErrServerExists, spec.ServerID)
	}
	r.specs[spec.ServerID] = spec
	r.specsByLegacy[spec.LegacyServerID] = spec.ServerID
	return nil
}

func (r *MCPMigrationRegistry) GetByCanonicalID(serverID string) (*MCPContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[serverID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, serverID)
	}
	return spec, nil
}

func (r *MCPMigrationRegistry) GetByLegacyID(legacyID string) (*MCPContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonicalID, exists := r.specsByLegacy[legacyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLegacyServerNotFound, legacyID)
	}
	return r.specs[canonicalID], nil
}

func (r *MCPMigrationRegistry) List() []*MCPContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*MCPContributionSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

type LegacyMCPAdapter struct {
	registry *MCPMigrationRegistry
}

func NewLegacyMCPAdapter(registry *MCPMigrationRegistry) *LegacyMCPAdapter {
	return &LegacyMCPAdapter{registry: registry}
}

func (a *LegacyMCPAdapter) TranslateLegacyServerID(legacyID string) (string, error) {
	spec, err := a.registry.GetByLegacyID(legacyID)
	if err != nil {
		return "", err
	}
	return spec.ServerID, nil
}

type MCPMigrationReport struct {
	StartTime          time.Time            `json:"startTime"`
	EndTime            time.Time            `json:"endTime"`
	TotalLegacy        int                  `json:"totalLegacy"`
	MigratedCount      int                  `json:"migratedCount"`
	FailedEntries      []FailedMCPMigration `json:"failedEntries,omitempty"`
	TransportBreakdown map[string]int       `json:"transportBreakdown"`
	Status             string               `json:"status"`
}

type FailedMCPMigration struct {
	LegacyServerID string `json:"legacyServerId"`
	Reason         string `json:"reason"`
}

func RunMCPMigration(ctx context.Context, registry *MCPMigrationRegistry, legacyServerIDs []string) (*MCPMigrationReport, error) {
	report := &MCPMigrationReport{
		StartTime:          time.Now().UTC(),
		TransportBreakdown: make(map[string]int),
		Status:             "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()
	report.TotalLegacy = len(legacyServerIDs)
	specs := registry.List()
	specsByLegacy := make(map[string]bool, len(specs))
	for _, spec := range specs {
		specsByLegacy[spec.LegacyServerID] = true
		report.TransportBreakdown[spec.Transport]++
	}
	migrated := 0
	for _, legacyID := range legacyServerIDs {
		if specsByLegacy[legacyID] {
			migrated++
		} else {
			report.FailedEntries = append(report.FailedEntries, FailedMCPMigration{
				LegacyServerID: legacyID,
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
	ErrInvalidSpec          = errors.New("mcp_migration: invalid spec")
	ErrServerExists         = errors.New("mcp_migration: server exists")
	ErrServerNotFound       = errors.New("mcp_migration: server not found")
	ErrLegacyServerNotFound = errors.New("mcp_migration: legacy server not found")
)

var _ = json.Marshal
