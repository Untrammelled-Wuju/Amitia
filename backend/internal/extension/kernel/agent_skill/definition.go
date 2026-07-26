package agent_skill

import "crypto/sha256"
import "encoding/hex"
import "encoding/json"

type AgentSkillScope string

const (
	AgentSkillScopeGlobal    AgentSkillScope = "global"
	AgentSkillScopeCharacter AgentSkillScope = "character"
)

type ActivationMode string

const (
	ActivationManual  ActivationMode = "manual"
	ActivationAuto    ActivationMode = "auto"
	ActivationExplicit ActivationMode = "explicit"
)

type ActivationRule struct {
	Mode     ActivationMode `json:"mode"`
	Keywords []string       `json:"keywords,omitempty"`
	Priority int            `json:"priority,omitempty"`
	MaxPerRound int         `json:"maxPerRound,omitempty"`
}

type ToolReference struct {
	ToolID     string `json:"toolId"`
	Required   bool   `json:"required"`
	Constraint string `json:"constraint,omitempty"`
}

type MCPReference struct {
	ServerID     string `json:"serverId"`
	DependencyName string `json:"dependencyName,omitempty"`
	Optional      bool   `json:"optional"`
	AutoInstall   bool   `json:"autoInstall,omitempty"`
}

type SkillResourceKind string

const (
	KindReference  SkillResourceKind = "reference"
	KindTemplate   SkillResourceKind = "template"
	KindAsset      SkillResourceKind = "asset"
	KindScript     SkillResourceKind = "script"
	KindConfig     SkillResourceKind = "config"
	KindData       SkillResourceKind = "data"
)

type SkillResourceDescriptor struct {
	Path         string            `json:"path"`
	Kind         SkillResourceKind `json:"kind"`
	MIMEType      string            `json:"mimeType,omitempty"`
	Size          int64             `json:"size"`
	SHA256        string            `json:"sha256,omitempty"`
	TextReadable  bool              `json:"textReadable"`
	TokenEstimate int               `json:"tokenEstimate,omitempty"`
	Description   string            `json:"description,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

type SkillTokenPolicy struct {
	MaxInstructionTokens    int `json:"maxInstructionTokens"`
	MaxResourceTokensPerTurn int `json:"maxResourceTokensPerTurn,omitempty"`
	MaxTotalResources        int `json:"maxTotalResources,omitempty"`
	TruncationStrategy       string `json:"truncationStrategy,omitempty"`
}

type SkillCompatibility struct {
	MinHostVersion string   `json:"minHostVersion,omitempty"`
	MaxHostVersion  string   `json:"maxHostVersion,omitempty"`
	Platforms       []string `json:"platforms,omitempty"`
	FeatureFlags    []string `json:"featureFlags,omitempty"`
	SchemaVersion   int      `json:"schemaVersion"`
	Status          string   `json:"status"`
	Messages        []string `json:"messages,omitempty"`
}

type SkillIntegrity struct {
	Algorithm       string            `json:"algorithm"`
	ContentHash     string            `json:"contentHash"`
	ResourceHashes  map[string]string `json:"resourceHashes,omitempty"`
	FrontmatterHash string            `json:"frontmatterHash"`
	VerifiedAt      string            `json:"verifiedAt,omitempty"`
}

type SkillInstructionRef struct {
	Text         string `json:"text"`
	TokenCount   int    `json:"tokenCount"`
	ContentHash  string `json:"contentHash"`
	Truncated    bool   `json:"truncated,omitempty"`
}

type AgentSkillDefinition struct {
	ID               string                    `json:"id"`
	ExtensionID      string                    `json:"extensionId"`
	ModuleID         string                    `json:"moduleId,omitempty"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	DisplayName      string                    `json:"displayName,omitempty"`
	Version          string                    `json:"version"`
	SchemaVersion    int                       `json:"schemaVersion"`

	Instructions     SkillInstructionRef       `json:"instructions"`
	Activation       ActivationRule            `json:"activation"`
	Resources        []SkillResourceDescriptor `json:"resources,omitempty"`
	RequiredTools    []ToolReference           `json:"requiredTools,omitempty"`
	RequiredMCP      []MCPReference            `json:"requiredMCP,omitempty"`
	TokenPolicy      SkillTokenPolicy          `json:"tokenPolicy,omitempty"`
	Compatibility    SkillCompatibility        `json:"compatibility,omitempty"`
	Integrity        SkillIntegrity            `json:"integrity,omitempty"`

	Scope            AgentSkillScope           `json:"scope"`
	ScopeID          string                    `json:"scopeId,omitempty"`
	Enabled          bool                      `json:"enabled"`
	Compatible       bool                      `json:"compatible,omitempty"`
	Source           string                    `json:"source"`
	License          string                    `json:"license,omitempty"`
	Author           string                    `json:"author,omitempty"`

	ToolMappings     []map[string]any          `json:"toolMappings,omitempty"`
	Metadata         map[string]any            `json:"metadata,omitempty"`
}

type AgentSkillCatalogEntry struct {
	ExtensionID   string           `json:"extensionId"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	DisplayName   string           `json:"displayName,omitempty"`
	Version       string           `json:"version,omitempty"`
	Scope         AgentSkillScope  `json:"scope"`
	Compatibility string           `json:"compatibility,omitempty"`
	Source        string           `json:"source,omitempty"`
	Enabled       bool             `json:"enabled"`
	Keywords      []string         `json:"keywords,omitempty"`
	ResourceCount int              `json:"resourceCount"`
	TokenBudget   int              `json:"tokenBudget,omitempty"`
}

func (d AgentSkillDefinition) ToCatalogEntry() AgentSkillCatalogEntry {
	return AgentSkillCatalogEntry{
		ExtensionID:   d.ExtensionID,
		Name:          d.Name,
		Description:   d.Description,
		DisplayName:   d.DisplayName,
		Version:       d.Version,
		Scope:         d.Scope,
		Compatibility: d.Compatibility.Status,
		Source:        d.Source,
		Enabled:       d.Enabled,
		Keywords:      d.Activation.Keywords,
		ResourceCount: len(d.Resources),
		TokenBudget:   d.TokenPolicy.MaxInstructionTokens,
	}
}

func (d AgentSkillDefinition) HasActiveResources() bool {
	return len(d.Resources) > 0
}

func (d AgentSkillDefinition) IsGloballyActive() bool {
	return d.Scope == AgentSkillScopeGlobal && d.Enabled
}

func (d AgentSkillDefinition) NeedsAnyTool() bool {
	for _, ref := range d.RequiredTools {
		if ref.Required {
			return true
		}
	}
	return false
}

func (d AgentSkillDefinition) MissingToolIDs(available map[string]bool) []string {
	var missing []string
	for _, ref := range d.RequiredTools {
		if ref.Required && !available[ref.ToolID] {
			missing = append(missing, ref.ToolID)
		}
	}
	return missing
}

func ComputeResourceHash(resource SkillResourceDescriptor) string {
	payload, _ := json.Marshal(resource)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
