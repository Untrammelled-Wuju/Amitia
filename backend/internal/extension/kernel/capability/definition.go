package capability

import (
	"encoding/json"
	"time"
)

type ToolVersion struct {
	SchemaVersion int    `json:"schemaVersion"`
	Revision      string `json:"revision"`
}

type RetryPolicy struct {
	MaxRetries  int           `json:"maxRetries"`
	BackoffBase time.Duration `json:"backoffBase"`
}

type ResourceLimits struct {
	MaxMemoryBytes int64                    `json:"maxMemoryBytes,omitempty"`
	MaxCPUPercent  int                      `json:"maxCpuPercent,omitempty"`
	Requirement    ResourceLimitRequirement `json:"requirement,omitempty"`
}

type ToolExecutionPolicy struct {
	Timeout          time.Duration  `json:"timeout"`
	MaxConcurrency   int            `json:"maxConcurrency"`
	RetryPolicy      RetryPolicy    `json:"retryPolicy,omitempty"`
	Idempotent       bool           `json:"idempotent"`
	ApprovalRequired bool           `json:"approvalRequired"`
	AllowBackground  bool           `json:"allowBackground"`
	MaxDepth         int            `json:"maxDepth"`
	ResourceLimits   ResourceLimits `json:"resourceLimits,omitempty"`
}

type ModelExposureRule struct {
	ExposedByDefault   bool     `json:"exposedByDefault"`
	RequiresActivation bool     `json:"requiresActivation"`
	Categories         []string `json:"categories,omitempty"`
	MaxPromptTokens    int      `json:"maxPromptTokens"`
	Priority           int      `json:"priority"`
}

type AvailabilityRule struct {
	Platforms     []string `json:"platforms,omitempty"`
	RequiresRoles []string `json:"requiresRoles,omitempty"`
}

type ToolStreamingPolicy struct {
	Enabled       bool  `json:"enabled"`
	MaxEventBytes int   `json:"maxEventBytes,omitempty"`
	MaxEvents     int   `json:"maxEvents,omitempty"`
	MaxTotalBytes int64 `json:"maxTotalBytes,omitempty"`
}

const (
	DefaultMaxEventBytes = 64 * 1024
	DefaultMaxEvents     = 4096
	DefaultMaxTotalBytes = 8 * 1024 * 1024
)

func (p ToolStreamingPolicy) EffectiveMaxEventBytes() int {
	if p.MaxEventBytes > 0 {
		return p.MaxEventBytes
	}
	return DefaultMaxEventBytes
}

func (p ToolStreamingPolicy) EffectiveMaxEvents() int {
	if p.MaxEvents > 0 {
		return p.MaxEvents
	}
	return DefaultMaxEvents
}

func (p ToolStreamingPolicy) EffectiveMaxTotalBytes() int64 {
	if p.MaxTotalBytes > 0 {
		return p.MaxTotalBytes
	}
	return DefaultMaxTotalBytes
}

func (p ToolStreamingPolicy) LimitsEnabled() bool {
	return p.Enabled
}

type ToolResultPolicy struct {
	SanitizeError  bool                `json:"sanitizeError"`
	MaxOutputBytes int                 `json:"maxOutputBytes,omitempty"`
	Streaming      ToolStreamingPolicy `json:"streaming,omitempty"`
}

type CapabilityDefinition struct {
	ID              CapabilityID            `json:"id"`
	Type            CapabilityType          `json:"type"`
	Owner           ResourceOwner           `json:"owner"`
	Source          CapabilitySource        `json:"source"`
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	InputSchema     json.RawMessage         `json:"inputSchema"`
	OutputSchema    json.RawMessage         `json:"outputSchema"`
	Permissions     []PermissionRequirement `json:"permissions,omitempty"`
	ScopeRule       ScopeRule               `json:"scopeRule,omitempty"`
	RiskLevel       RiskLevel               `json:"riskLevel,omitempty"`
	SideEffectLevel SideEffectLevel         `json:"sideEffectLevel,omitempty"`
	Runtime         RuntimeBinding          `json:"runtime"`
	Availability    AvailabilityRule        `json:"availability,omitempty"`
	Metadata        map[string]any          `json:"metadata,omitempty"`
}

type CapabilityID string

type ToolDefinition struct {
	ID               string                  `json:"id"`
	ModelName        string                  `json:"modelName"`
	CapabilityID     CapabilityID            `json:"capabilityId"`
	ExtensionID      string                  `json:"extensionId,omitempty"`
	ModuleID         string                  `json:"moduleId,omitempty"`
	Source           ToolSource              `json:"source"`
	Name             string                  `json:"name"`
	Description      string                  `json:"description"`
	Version          string                  `json:"version,omitempty"`
	InputSchema      json.RawMessage         `json:"inputSchema"`
	OutputSchema     json.RawMessage         `json:"outputSchema"`
	Permissions      []PermissionRequirement `json:"permissions,omitempty"`
	SecretReferences []string                `json:"secretReferences,omitempty"`
	RiskLevel        RiskLevel               `json:"riskLevel,omitempty"`
	SideEffect       SideEffectLevel         `json:"sideEffect,omitempty"`
	Scope            ScopeRule               `json:"scope,omitempty"`
	Enabled          bool                    `json:"enabled"`
	Compatible       bool                    `json:"compatible,omitempty"`
	Internal         bool                    `json:"internal,omitempty"`
	HasSideEffects   bool                    `json:"hasSideEffects"`
	Idempotent       bool                    `json:"idempotent"`
	Retryable        bool                    `json:"retryable"`
	TimeoutMS        int64                   `json:"timeoutMs"`
	Metadata         map[string]any          `json:"metadata,omitempty"`

	ToolVersion        ToolVersion         `json:"toolVersion"`
	State              ToolState           `json:"state"`
	ModelExposure      ModelExposureRule   `json:"modelExposure"`
	ExecutionPolicy    ToolExecutionPolicy `json:"executionPolicy"`
	ResultPolicy       ToolResultPolicy    `json:"resultPolicy,omitempty"`
	Runtime            RuntimeBinding      `json:"runtime"`
	RoutingMode        ProviderRoutingMode `json:"routingMode,omitempty"`
	ProviderID         string              `json:"providerId,omitempty"`
}

func (td ToolDefinition) CapabilitySource() CapabilitySource {
	return CapabilitySource(td.Source)
}

func (td ToolDefinition) ToCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		ID:              CapabilityID(td.ID),
		Type:            CapabilityTypeTool,
		Owner:           td.Owner(),
		Source:          td.CapabilitySource(),
		Name:            td.Name,
		Description:     td.Description,
		InputSchema:     td.InputSchema,
		OutputSchema:    td.OutputSchema,
		Permissions:     td.Permissions,
		ScopeRule:       td.Scope,
		RiskLevel:       td.RiskLevel,
		SideEffectLevel: td.SideEffect,
		Runtime:         td.Runtime,
		Metadata:        td.Metadata,
	}
}

func (td ToolDefinition) Owner() ResourceOwner {
	if td.ExtensionID != "" {
		return ResourceOwner{
			OwnerType:   OwnerTypeExtension,
			OwnerID:     td.ExtensionID,
			ExtensionID: td.ExtensionID,
			ModuleID:    td.ModuleID,
		}
	}
	switch td.Source {
	case ToolSourceBuiltin:
		return ResourceOwner{OwnerType: OwnerTypeSystem, OwnerID: "core"}
	case ToolSourceInternal:
		return ResourceOwner{OwnerType: OwnerTypeSystem, OwnerID: "core"}
	default:
		return ResourceOwner{OwnerType: OwnerTypeSystem, OwnerID: "core"}
	}
}

type ModelToolView struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	CapabilityID string          `json:"capabilityId"`
}

func (td ToolDefinition) ModelToolView() ModelToolView {
	return ModelToolView{
		Name:         td.ModelName,
		Description:  td.Description,
		InputSchema:  td.InputSchema,
		CapabilityID: td.ID,
	}
}

func (td ToolDefinition) IsEnabled() bool {
	return td.Enabled
}

func (td ToolDefinition) ComputedState() ToolState {
	state := td.State
	state.Installed = true

	if !state.ModuleEnabled && !state.CapabilityEnabled && !state.PermissionGranted && !state.RuntimeReady && !state.DependencyReady {
		state = ToolState{
			Installed:         true,
			ModuleEnabled:     td.Enabled,
			CapabilityEnabled: td.Enabled,
			ScopeAllowed:      true,
			PermissionGranted: true,
			RuntimeReady:      true,
			DependencyReady:   true,
			Health:            HealthReady,
		}
	}
	return state
}
