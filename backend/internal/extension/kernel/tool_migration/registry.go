package tool_migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
)

const SystemExtensionID = "system/amitia-core"
const SystemPublisher = "amitia"

type ModuleID string

const (
	ModuleCoreTools       ModuleID = "core-tools"
	ModuleMemoryTools     ModuleID = "memory-tools"
	ModuleMessageTools    ModuleID = "message-tools"
	ModuleMediaTools      ModuleID = "media-tools"
	ModuleDesktopTools    ModuleID = "desktop-tools"
	ModuleProviderTools   ModuleID = "provider-tools"
	ModuleDiagnosticTools ModuleID = "diagnostic-tools"
)

type ToolContributionSpec struct {
	ToolID          string          `json:"toolId"`
	LegacyToolID    string          `json:"legacyToolId"`
	ModelToolName   string          `json:"modelToolName"`
	Module          ModuleID        `json:"module"`
	Namespace       string          `json:"namespace"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Parameters      json.RawMessage `json:"parameters,omitempty"`
	Permissions     []string        `json:"permissions,omitempty"`
	RiskLevel       string          `json:"riskLevel,omitempty"`
	RuntimeBinding  string          `json:"runtimeBinding"`
	Deprecated      bool            `json:"deprecated,omitempty"`
	DeprecationNote string          `json:"deprecationNote,omitempty"`
}

type ToolMigrationRegistry struct {
	mu            sync.RWMutex
	specs         map[string]*ToolContributionSpec
	specsByLegacy map[string]string
	extensionSpec *SystemExtensionSpec
}

type SystemExtensionSpec struct {
	ExtensionID  string
	Publisher    string
	Version      string
	Generation   int64
	TrustLevel   string
	Modules      []ModuleID
	BuiltInTools []string
}

func NewToolMigrationRegistry() *ToolMigrationRegistry {
	return &ToolMigrationRegistry{
		specs:         make(map[string]*ToolContributionSpec),
		specsByLegacy: make(map[string]string),
		extensionSpec: &SystemExtensionSpec{
			ExtensionID: SystemExtensionID,
			Publisher:   SystemPublisher,
			Version:     "1.0.0",
			Generation:  1,
			TrustLevel:  "platform",
			Modules: []ModuleID{
				ModuleCoreTools, ModuleMemoryTools, ModuleMessageTools,
				ModuleMediaTools, ModuleDesktopTools, ModuleProviderTools,
				ModuleDiagnosticTools,
			},
		},
	}
}

func (r *ToolMigrationRegistry) Register(spec *ToolContributionSpec) error {
	if spec == nil || spec.ToolID == "" {
		return ErrInvalidSpec
	}
	if spec.LegacyToolID == "" {
		spec.LegacyToolID = spec.ToolID
	}
	if spec.ModelToolName == "" {
		spec.ModelToolName = spec.ToolID
	}
	if spec.RuntimeBinding == "" {
		spec.RuntimeBinding = "host_internal"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.specs[spec.ToolID]; exists {
		return fmt.Errorf("%w: %s", ErrToolExists, spec.ToolID)
	}
	r.specs[spec.ToolID] = spec
	if spec.LegacyToolID != "" {
		r.specsByLegacy[spec.LegacyToolID] = spec.ToolID
	}
	return nil
}

func (r *ToolMigrationRegistry) GetByCanonicalID(toolID string) (*ToolContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, exists := r.specs[toolID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, toolID)
	}
	return spec, nil
}

func (r *ToolMigrationRegistry) GetByLegacyID(legacyID string) (*ToolContributionSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonicalID, exists := r.specsByLegacy[legacyID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrLegacyToolNotFound, legacyID)
	}
	return r.specs[canonicalID], nil
}

func (r *ToolMigrationRegistry) List() []*ToolContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ToolContributionSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec)
	}
	return out
}

func (r *ToolMigrationRegistry) ListByModule(module ModuleID) []*ToolContributionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ToolContributionSpec, 0)
	for _, spec := range r.specs {
		if spec.Module == module {
			out = append(out, spec)
		}
	}
	return out
}

func (r *ToolMigrationRegistry) ExtensionSpec() *SystemExtensionSpec {
	return r.extensionSpec
}

func DefaultMigrationRegistry() *ToolMigrationRegistry {
	r := NewToolMigrationRegistry()
	for _, spec := range DefaultToolSpecs() {
		_ = r.Register(spec)
	}
	return r
}

func DefaultToolSpecs() []*ToolContributionSpec {
	voicePerms := []string{"voice.generate"}
	memReadPerms := []string{"memory.read"}
	schedulePerms := []string{"schedule.create"}
	return []*ToolContributionSpec{
		{
			ToolID:         "builtin/core/get_current_time",
			LegacyToolID:   "get_current_time",
			ModelToolName:  "get_current_time",
			Module:         ModuleCoreTools,
			Namespace:      "core",
			Title:          "获取当前时间",
			Description:    "获取用户和角色当地时间及UTC时间",
			RuntimeBinding: "host_internal",
			RiskLevel:      "low",
		},
		{
			ToolID:         "builtin/memory/query",
			LegacyToolID:   "query_memory",
			ModelToolName:  "query_memory",
			Module:         ModuleMemoryTools,
			Namespace:      "memory",
			Title:          "查询记忆",
			Description:    "根据条件查询角色记忆",
			RuntimeBinding: "host_internal",
			RiskLevel:      "medium",
			Permissions:    memReadPerms,
		},
		{
			ToolID:         "builtin/memory/summary",
			LegacyToolID:   "memory_summary",
			ModelToolName:  "memory_summary",
			Module:         ModuleMemoryTools,
			Namespace:      "memory",
			Title:          "生成记忆摘要",
			Description:    "生成指定角色的记忆摘要",
			RuntimeBinding: "host_internal",
			RiskLevel:      "medium",
			Permissions:    memReadPerms,
		},
		{
			ToolID:         "builtin/state/read_need",
			LegacyToolID:   "read_need_state",
			ModelToolName:  "read_need_state",
			Module:         ModuleCoreTools,
			Namespace:      "state",
			Title:          "读取需求状态",
			Description:    "读取角色当前需求状态",
			RuntimeBinding: "host_internal",
			RiskLevel:      "low",
		},
		{
			ToolID:         "builtin/state/read_psyche",
			LegacyToolID:   "read_psyche_state",
			ModelToolName:  "read_psyche_state",
			Module:         ModuleCoreTools,
			Namespace:      "state",
			Title:          "读取心理状态",
			Description:    "读取角色当前心理状态",
			RuntimeBinding: "host_internal",
			RiskLevel:      "low",
		},
		{
			ToolID:         "builtin/message/voice_reply",
			LegacyToolID:   "voice_reply",
			ModelToolName:  "voice_reply",
			Module:         ModuleMessageTools,
			Namespace:      "message",
			Title:          "语音回复",
			Description:    "生成语音回复",
			RuntimeBinding: "host_internal",
			RiskLevel:      "medium",
			Permissions:    voicePerms,
		},
		{
			ToolID:         "builtin/core/schedule",
			LegacyToolID:   "schedule",
			ModelToolName:  "schedule",
			Module:         ModuleCoreTools,
			Namespace:      "core",
			Title:          "调度任务",
			Description:    "调度定时任务",
			RuntimeBinding: "host_internal",
			RiskLevel:      "high",
			Permissions:    schedulePerms,
		},
	}
}

type LegacyToolAdapter struct {
	registry *ToolMigrationRegistry
}

func NewLegacyToolAdapter(registry *ToolMigrationRegistry) *LegacyToolAdapter {
	return &LegacyToolAdapter{registry: registry}
}

func (a *LegacyToolAdapter) TranslateLegacyToolName(legacyName string) (string, error) {
	spec, err := a.registry.GetByLegacyID(legacyName)
	if err != nil {
		return "", err
	}
	return spec.ToolID, nil
}

func (a *LegacyToolAdapter) TranslateCanonicalToModel(canonicalID string) (string, error) {
	spec, err := a.registry.GetByCanonicalID(canonicalID)
	if err != nil {
		return "", err
	}
	return spec.ModelToolName, nil
}

func (a *LegacyToolAdapter) IsDeprecated(legacyName string) (bool, string) {
	spec, err := a.registry.GetByLegacyID(legacyName)
	if err != nil {
		return false, ""
	}
	return spec.Deprecated, spec.DeprecationNote
}

func (a *LegacyToolAdapter) ListAllLegacyTools() []string {
	specs := a.registry.List()
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.LegacyToolID)
	}
	return out
}

type MigrationReport struct {
	StartTime        time.Time         `json:"startTime"`
	EndTime          time.Time         `json:"endTime"`
	TotalLegacyTools int               `json:"totalLegacyTools"`
	MigratedTools    int               `json:"migratedTools"`
	FailedTools      []FailedMigration `json:"failedTools,omitempty"`
	OrphanedTools    []string          `json:"orphanedTools,omitempty"`
	ModuleBreakdown  map[ModuleID]int  `json:"moduleBreakdown"`
	Status           string            `json:"status"`
}

type FailedMigration struct {
	LegacyToolID string `json:"legacyToolId"`
	Reason       string `json:"reason"`
}

func RunMigration(ctx context.Context, registry *ToolMigrationRegistry) (*MigrationReport, error) {
	report := &MigrationReport{
		StartTime:       time.Now().UTC(),
		ModuleBreakdown: make(map[ModuleID]int),
		Status:          "running",
	}
	defer func() {
		report.EndTime = time.Now().UTC()
		if report.Status == "running" {
			report.Status = "completed"
		}
	}()

	legacyTools := tool.GetAll()
	legacyNames := make(map[string]bool, len(legacyTools))
	for _, t := range legacyTools {
		legacyNames[t.Function.Name] = true
	}
	memTools := tool.GetMemoryTools()
	for _, t := range memTools {
		legacyNames[t.Function.Name] = true
	}
	report.TotalLegacyTools = len(legacyNames)

	migratedCount := 0
	specs := registry.List()
	for _, spec := range specs {
		if legacyNames[spec.LegacyToolID] {
			migratedCount++
			report.ModuleBreakdown[spec.Module]++
		} else if !spec.Deprecated {
			report.OrphanedTools = append(report.OrphanedTools, spec.LegacyToolID)
		}
	}
	report.MigratedTools = migratedCount
	if len(report.OrphanedTools) > 0 {
		report.Status = "completed_with_warnings"
	}
	return report, nil
}

var (
	ErrInvalidSpec        = errors.New("tool_migration: invalid spec")
	ErrToolExists         = errors.New("tool_migration: tool exists")
	ErrToolNotFound       = errors.New("tool_migration: tool not found")
	ErrLegacyToolNotFound = errors.New("tool_migration: legacy tool not found")
)
