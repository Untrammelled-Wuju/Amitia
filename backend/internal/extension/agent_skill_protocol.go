package extension

import (
	"encoding/json"
	"time"
)

type AgentSkillSource string
type AgentSkillScope string
type AgentSkillResourceKind string
type AgentSkillCompatibilityStatus string

const (
	AgentSkillSourceBundled         AgentSkillSource              = "bundled"
	AgentSkillSourceDirectory       AgentSkillSource              = "local-directory"
	AgentSkillSourceZIP             AgentSkillSource              = "local-zip"
	AgentSkillSourceWorkshop        AgentSkillSource              = "workshop"
	AgentSkillScopeGlobal           AgentSkillScope               = "global"
	AgentSkillScopeCharacter        AgentSkillScope               = "character"
	AgentSkillResourceSkill         AgentSkillResourceKind        = "skill"
	AgentSkillResourceReference     AgentSkillResourceKind        = "reference"
	AgentSkillResourceAsset         AgentSkillResourceKind        = "asset"
	AgentSkillResourceScript        AgentSkillResourceKind        = "script"
	AgentSkillResourceAgentMetadata AgentSkillResourceKind        = "agent_metadata"
	AgentSkillResourceOther         AgentSkillResourceKind        = "other"
	AgentSkillCompatible            AgentSkillCompatibilityStatus = "compatible"
	AgentSkillCompatibleWarnings    AgentSkillCompatibilityStatus = "compatible_with_warnings"
	AgentSkillPartiallyCompatible   AgentSkillCompatibilityStatus = "partially_compatible"
	AgentSkillBlocked               AgentSkillCompatibilityStatus = "blocked"
)

const (
	ErrAgentSkillNotFound         = "AGENT_SKILL_NOT_FOUND"
	ErrAgentSkillDisabled         = "AGENT_SKILL_DISABLED"
	ErrAgentSkillBlocked          = "AGENT_SKILL_BLOCKED"
	ErrAgentSkillNotExecutable    = "AGENT_SKILL_NOT_EXECUTABLE"
	ErrAgentSkillInvalidArchive   = "AGENT_SKILL_INVALID_ARCHIVE"
	ErrAgentSkillPathTraversal    = "AGENT_SKILL_PATH_TRAVERSAL"
	ErrAgentSkillArchiveLimit     = "AGENT_SKILL_ARCHIVE_LIMIT_EXCEEDED"
	ErrAgentSkillMissingSkillMD   = "AGENT_SKILL_SKILL_MD_MISSING"
	ErrAgentSkillFrontmatter      = "AGENT_SKILL_FRONTMATTER_INVALID"
	ErrAgentSkillNameInvalid      = "AGENT_SKILL_NAME_INVALID"
	ErrAgentSkillNameMismatch     = "AGENT_SKILL_NAME_MISMATCH"
	ErrAgentSkillDescription      = "AGENT_SKILL_DESCRIPTION_INVALID"
	ErrAgentSkillNameConflict     = "AGENT_SKILL_NAME_CONFLICT"
	ErrAgentSkillResourceNotFound = "AGENT_SKILL_RESOURCE_NOT_FOUND"
	ErrAgentSkillResourceDenied   = "AGENT_SKILL_RESOURCE_ACCESS_DENIED"
	ErrAgentSkillResourceTooLarge = "AGENT_SKILL_RESOURCE_TOO_LARGE"
	ErrAgentSkillScriptDisabled   = "AGENT_SKILL_SCRIPT_EXECUTION_DISABLED"
	ErrAgentSkillToolUnsupported  = "AGENT_SKILL_TOOL_UNSUPPORTED"
	ErrAgentSkillActivationLimit  = "AGENT_SKILL_ACTIVATION_LIMIT"
	ErrAgentSkillPromptLimit      = "AGENT_SKILL_PROMPT_LIMIT"
	ErrAgentSkillScopeForbidden   = "AGENT_SKILL_SCOPE_FORBIDDEN"
	ErrAgentSkillArtifactInvalid  = "AGENT_SKILL_ARTIFACT_INVALID"
	ErrAgentSkillChecksumMismatch = "AGENT_SKILL_CHECKSUM_MISMATCH"
)

type AgentSkillLimits struct {
	MaxFiles             int
	MaxDepth             int
	MaxExpandedBytes     int64
	MaxSkillMDBytes      int64
	MaxTextResourceBytes int64
	MaxResourceBytes     int64
	MaxCompressionRatio  float64
	MaxFrontmatterBytes  int64
	MaxYAMLDepth         int
	MaxActivations       int
	MaxBodyTokens        int
	MaxPromptTokens      int
	MaxCatalogEntries    int
	MaxCatalogTokens     int
	MaxResourceReads     int
	MaxResourceReadBytes int64
}

func DefaultAgentSkillLimits() AgentSkillLimits {
	return AgentSkillLimits{MaxFiles: 500, MaxDepth: 12, MaxExpandedBytes: 50 << 20, MaxSkillMDBytes: 256 << 10, MaxTextResourceBytes: 2 << 20, MaxResourceBytes: 20 << 20, MaxCompressionRatio: 100, MaxFrontmatterBytes: 64 << 10, MaxYAMLDepth: 16, MaxActivations: 4, MaxBodyTokens: 5000, MaxPromptTokens: 10000, MaxCatalogEntries: 100, MaxCatalogTokens: 10000, MaxResourceReads: 20, MaxResourceReadBytes: 4 << 20}
}

type AgentSkillWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}
type AgentSkillError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}
type AgentSkillToolMapping struct {
	SourceTool    string `json:"sourceTool"`
	TargetSkillID string `json:"targetSkillId,omitempty"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
}
type AgentSkillResource struct {
	Path         string                 `json:"path"`
	Kind         AgentSkillResourceKind `json:"kind"`
	MIMEType     string                 `json:"mimeType"`
	Size         int64                  `json:"size"`
	SHA256       string                 `json:"sha256"`
	TextReadable bool                   `json:"textReadable"`
	Executable   bool                   `json:"executable"`
	Supported    bool                   `json:"supported"`
}
type AgentSkillMCPDependency struct {
	ID                         string   `json:"id"`
	Description                string   `json:"description,omitempty"`
	Required                   bool     `json:"required"`
	Transport                  string   `json:"transport"`
	URL                        string   `json:"url,omitempty"`
	Command                    string   `json:"command,omitempty"`
	Args                       []string `json:"args,omitempty"`
	AuthType                   string   `json:"authType"`
	ToolAllowlist              []string `json:"toolAllowlist"`
	DefaultScope               string   `json:"defaultScope"`
	AutoConfigure              bool     `json:"autoConfigure"`
	AutoEnable                 bool     `json:"autoEnable"`
	RequiresManualConfirmation bool     `json:"requiresManualConfirmation"`
}
type AgentSkillCompatibilityReport struct {
	Status             AgentSkillCompatibilityStatus `json:"status"`
	DetectedProfiles   []AgentSkillEcosystemProfile  `json:"detectedProfiles,omitempty"`
	FieldMappings      []AgentSkillFieldMapping      `json:"fieldMappings,omitempty"`
	ToolMappings       []AgentSkillToolMapping       `json:"toolMappings"`
	MappedFeatures     []AgentSkillFeatureResult     `json:"mappedFeatures,omitempty"`
	DependencyMappings []AgentSkillDependencyMapping `json:"dependencyMappings,omitempty"`
	RequiredScripts    []string                      `json:"requiredScripts"`
	MissingFiles       []string                      `json:"missingFiles"`
	Unsupported        []string                      `json:"unsupported"`
	Warnings           []AgentSkillWarning           `json:"warnings"`
	Errors             []AgentSkillError             `json:"errors"`
	Fingerprint        *AgentSkillCompatibilityFingerprint `json:"fingerprint,omitempty"`
	EvaluatedAt        time.Time                     `json:"evaluatedAt"`
}

type AgentSkillEcosystemProfile struct {
	ID       string   `json:"id"`
	Version  string   `json:"version"`
	Evidence []string `json:"evidence"`
}

type AgentSkillFieldMapping struct {
	Profile string `json:"profile"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
}

type AgentSkillFeatureResult struct {
	Profile string `json:"profile"`
	Feature string `json:"feature"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
}

type AgentSkillDependencyMapping struct {
	Profile    string `json:"profile"`
	ResolvedAs string `json:"resolvedAs"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
}

type AgentSkillCompatibilityFingerprint struct {
	ContentHash     string            `json:"contentHash"`
	AdapterVersions map[string]string `json:"adapterVersions"`
	CapabilityGen   int               `json:"capabilityGeneration"`
}

type AgentSkillDefinition struct {
	ExtensionID         string                        `json:"extensionId"`
	Name                string                        `json:"name"`
	Description         string                        `json:"description"`
	License             string                        `json:"license,omitempty"`
	Compatibility       string                        `json:"compatibility,omitempty"`
	Metadata            map[string]string             `json:"metadata"`
	AllowedTools        string                        `json:"allowedTools,omitempty"`
	DisplayName         string                        `json:"displayName,omitempty"`
	ShortDescription    string                        `json:"shortDescription,omitempty"`
	DefaultPrompt       string                        `json:"defaultPrompt,omitempty"`
	IconSmall           string                        `json:"iconSmall,omitempty"`
	IconLarge           string                        `json:"iconLarge,omitempty"`
	BrandColor          string                        `json:"brandColor,omitempty"`
	Source              AgentSkillSource              `json:"source"`
	Scope               AgentSkillScope               `json:"scope"`
	ScopeID             string                        `json:"scopeId,omitempty"`
	UserID              string                        `json:"userId"`
	ArtifactID          string                        `json:"artifactId"`
	ContentHash         string                        `json:"contentHash"`
	Body                string                        `json:"body"`
	RawSkillMD          string                        `json:"rawSkillMd"`
	RawFrontmatter      json.RawMessage               `json:"rawFrontmatter"`
	ExtraFrontmatter    json.RawMessage               `json:"extraFrontmatter"`
	OpenAIMetadata      json.RawMessage               `json:"openaiMetadata"`
	Resources           []AgentSkillResource          `json:"resources"`
	ToolMappings        []AgentSkillToolMapping       `json:"toolMappings"`
	CompatibilityStatus AgentSkillCompatibilityStatus `json:"compatibilityStatus"`
	Warnings            []AgentSkillWarning           `json:"warnings"`
	MCPDependencies     []AgentSkillMCPDependency     `json:"mcpDependencies"`
	Enabled             bool                          `json:"enabled"`
	CreatedAt           time.Time                     `json:"createdAt"`
	UpdatedAt           time.Time                     `json:"updatedAt"`
}

type AgentSkillImportPreview struct {
	PreviewID  string                        `json:"previewId"`
	Definition AgentSkillDefinition          `json:"definition"`
	Report     AgentSkillCompatibilityReport `json:"compatibilityReport"`
	Files      []AgentSkillResource          `json:"files"`
	ExpiresAt  time.Time                     `json:"expiresAt"`
}
type AgentSkillFilter struct {
	Query    string
	Status   AgentSkillCompatibilityStatus
	Scope    AgentSkillScope
	Page     int
	PageSize int
}
type PagedAgentSkills struct {
	Items    []AgentSkillDefinition `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}
type AgentSkillCatalogEntry struct {
	ExtensionID   string                        `json:"extensionId"`
	Name          string                        `json:"name"`
	Description   string                        `json:"description"`
	DisplayName   string                        `json:"displayName,omitempty"`
	Scope         AgentSkillScope               `json:"scope"`
	Source        AgentSkillSource              `json:"source"`
	Compatibility AgentSkillCompatibilityStatus `json:"compatibility"`
}
type ActivatedAgentSkill struct {
	ActivationID string               `json:"activationId"`
	Definition   AgentSkillDefinition `json:"definition"`
	Prompt       string               `json:"prompt"`
	BodyTokens   int                  `json:"bodyTokens"`
	Explicit     bool                 `json:"explicit"`
}
type AgentSkillActivation struct {
	ID                  string                        `json:"id"`
	ActivationID        string                        `json:"activationId"`
	ExtensionID         string                        `json:"extensionId"`
	AgentSkillName      string                        `json:"agentSkillName"`
	Source              AgentSkillSource              `json:"source"`
	Scope               AgentSkillScope               `json:"scope"`
	CompatibilityStatus AgentSkillCompatibilityStatus `json:"compatibilityStatus"`
	UserID              string                        `json:"userId"`
	CharacterID         string                        `json:"characterId"`
	ConversationID      string                        `json:"conversationId"`
	Channel             string                        `json:"channel"`
	TriggerType         string                        `json:"triggerType"`
	Explicit            bool                          `json:"explicit"`
	Status              string                        `json:"status"`
	LoadedTokens        int                           `json:"loadedTokens"`
	ResourceReads       int                           `json:"resourceReads"`
	ResourcePaths       []string                      `json:"resourcePaths"`
	ScriptsUsed         bool                          `json:"scriptsUsed"`
	ToolMappings        []AgentSkillToolMapping       `json:"toolMappings"`
	InstructionPosition string                        `json:"instructionPosition"`
	TokenLimitHit       bool                          `json:"tokenLimitHit"`
	TraceID             string                        `json:"traceId"`
	ErrorCode           string                        `json:"errorCode,omitempty"`
	CreatedAt           time.Time                     `json:"createdAt"`
}
type ActivateAgentSkillRequest struct {
	Scope    ExecutionScope
	NameOrID string
	Explicit bool
}
type ListAgentSkillResourcesRequest struct {
	Scope    ExecutionScope
	NameOrID string
	Kind     AgentSkillResourceKind
}
type ReadAgentSkillResourceRequest struct {
	Scope    ExecutionScope
	NameOrID string
	Path     string
}
type AgentSkillResourceContent struct {
	Path       string                 `json:"path"`
	Kind       AgentSkillResourceKind `json:"kind"`
	MIMEType   string                 `json:"mimeType"`
	Content    string                 `json:"content"`
	Size       int64                  `json:"size"`
	Executable bool                   `json:"executable"`
}
type InstallAgentSkillRequest struct {
	UserID      string
	CharacterID string
	PreviewID   string
	Scope       AgentSkillScope
	Enable      bool
}
