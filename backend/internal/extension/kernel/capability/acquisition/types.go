package acquisition

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CandidateKind string

const (
	CandidateInstalledExtension CandidateKind = "installed_extension"
	CandidateAgentSkill         CandidateKind = "agent_skill"
	CandidateMCP                CandidateKind = "mcp"
	CandidateExtensionPackage   CandidateKind = "extension_package"
	CandidateBuiltin            CandidateKind = "builtin"
	CandidateGeneratedSkill     CandidateKind = "generated_skill"
)

type TrustLevel string

const (
	TrustBuiltin    TrustLevel = "builtin"
	TrustTrusted    TrustLevel = "trusted"
	TrustVerified   TrustLevel = "verified"
	TrustUnverified TrustLevel = "unverified"
	TrustBlocked    TrustLevel = "blocked"
)

type InstallMethod string

const (
	InstallExtension      InstallMethod = "extension"
	InstallMCP            InstallMethod = "mcp"
	InstallSkill          InstallMethod = "skill"
	InstallGeneratedSkill InstallMethod = "generated_skill"
	InstallEnableExisting InstallMethod = "enable_existing"
)

type PolicyAction string

const (
	ActionAllowAuto       PolicyAction = "allow_auto"
	ActionRequireApproval PolicyAction = "require_approval"
	ActionDeny            PolicyAction = "deny"
)

type AcquisitionState string

const (
	StatePlanned          AcquisitionState = "planned"
	StateAwaitingApproval AcquisitionState = "awaiting_approval"
	StateInstalling       AcquisitionState = "installing"
	StateEnabling         AcquisitionState = "enabling"
	StateReconciling      AcquisitionState = "reconciling"
	StateReady            AcquisitionState = "ready"
	StateFailed           AcquisitionState = "failed"
	StateRolledBack       AcquisitionState = "rolled_back"
	StateCancelled        AcquisitionState = "cancelled"
	StateInstalledOnly    AcquisitionState = "installed_only"
	StateWaitingRuntime   AcquisitionState = "waiting_runtime"
	StateReconcileFailed  AcquisitionState = "reconcile_failed"
)

type ResumeState string

const (
	ResumePending    ResumeState = "pending"
	ResumeInProgress ResumeState = "in_progress"
	ResumeCompleted  ResumeState = "completed"
	ResumeFailed     ResumeState = "failed"
)

type CandidateSource struct {
	Provider  string `json:"provider,omitempty"`
	URI       string `json:"uri,omitempty"`
	Registry  string `json:"registry,omitempty"`
	Local     bool   `json:"local"`
	Verified  bool   `json:"verified"`
	Publisher string `json:"publisher,omitempty"`
}

type CandidateRuntimeSupport struct {
	Placements                []capability.ProviderPlacement `json:"placements"`
	Platforms                 []runtimeidentity.Platform     `json:"platforms,omitempty"`
	RequiresPersistentRuntime bool                           `json:"requiresPersistentRuntime"`
	RequiresDevice            bool                           `json:"requiresDevice"`
}

type CandidateTrust struct {
	Level             TrustLevel `json:"level"`
	SignatureVerified bool       `json:"signatureVerified"`
	PublisherVerified bool       `json:"publisherVerified"`
	SourceVerified    bool       `json:"sourceVerified"`
}

type CandidateDependency struct {
	Capability  capability.CapabilityID `json:"capability,omitempty"`
	PackageURI  string                  `json:"packageUri,omitempty"`
	MCPName     string                  `json:"mcpName,omitempty"`
	Description string                  `json:"description,omitempty"`
}

type ExtensionInstallDescriptor struct {
	PackageURI string `json:"packageUri"`
	Hash       string `json:"hash,omitempty"`
}

type MCPInstallDescriptor struct {
	ServerName string            `json:"serverName"`
	Transport  string            `json:"transport"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Registry   string            `json:"registry,omitempty"`
}

type SkillInstallDescriptor struct {
	SourceURI string `json:"sourceUri"`
	SkillName string `json:"skillName,omitempty"`
	Hash      string `json:"hash,omitempty"`
}

type GeneratedSkillDescriptor struct {
	PromptStub   string   `json:"promptStub"`
	ToolsNeeded  []string `json:"toolsNeeded"`
	StoresNeeded []string `json:"storesNeeded"`
	Description  string   `json:"description,omitempty"`
}

type CandidateInstallDescriptor struct {
	Method           InstallMethod               `json:"method"`
	ExtensionPackage *ExtensionInstallDescriptor `json:"extensionPackage,omitempty"`
	MCP              *MCPInstallDescriptor       `json:"mcp,omitempty"`
	Skill            *SkillInstallDescriptor     `json:"skill,omitempty"`
	GeneratedSkill   *GeneratedSkillDescriptor   `json:"generatedSkill,omitempty"`
}

type CandidateMatch struct {
	ExactCapabilities   int     `json:"exactCapabilities"`
	PartialCapabilities int     `json:"partialCapabilities"`
	SemanticScore       float64 `json:"semanticScore"`
}

type CapabilityCandidate struct {
	ID           string                     `json:"id"`
	Kind         CandidateKind              `json:"kind"`
	Name         string                     `json:"name"`
	Description  string                     `json:"description"`
	Version      string                     `json:"version"`
	Capabilities []capability.CapabilityID  `json:"capabilities"`
	Source       CandidateSource            `json:"source"`
	Runtime      CandidateRuntimeSupport    `json:"runtime"`
	Permissions  []string                   `json:"permissions,omitempty"`
	Dependencies []CandidateDependency      `json:"dependencies,omitempty"`
	Trust        CandidateTrust             `json:"trust"`
	Install      CandidateInstallDescriptor `json:"install"`
	Match        CandidateMatch             `json:"match,omitempty"`
	Metadata     map[string]any             `json:"metadata,omitempty"`
}

func (c CapabilityCandidate) IsGenerated() bool {
	return c.Kind == CandidateGeneratedSkill
}

func (c CapabilityCandidate) IsBlocked() bool {
	return c.Trust.Level == TrustBlocked
}

func (c CapabilityCandidate) HasExactCapability(id capability.CapabilityID) bool {
	for _, capID := range c.Capabilities {
		if capID == id {
			return true
		}
	}
	return false
}
