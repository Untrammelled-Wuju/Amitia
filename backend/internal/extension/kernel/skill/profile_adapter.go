package skill

import (
	"context"
	"time"
)

const (
	ProfileIDAgentSkills          = "agent-skills"
	ProfileIDClaudeCode           = "claude-code"
	ProfileIDClaudeCommandLegacy  = "claude-command-legacy"
	ProfileIDOpenAI               = "openai"

	AdapterVersionAgentSkills  = "agent-skills@1"
	AdapterVersionClaudeCode   = "claude-code@1"
	AdapterVersionClaudeLegacy = "claude-command-legacy@1"
	AdapterVersionOpenAI       = "openai@1"

	FeatureStateMapped             = "mapped"
	FeatureStateMappedWithPolicy   = "mapped_with_policy"
	FeatureStateIgnoredDisplayOnly = "ignored_display_only"
	FeatureStateDegraded           = "degraded"
	FeatureStateUnsupported        = "unsupported"
	FeatureStateBlocked            = "blocked"

	ExecutionModeInline     = "inline"
	ExecutionModeIsolated   = "isolated"
	ExecutionModeBackground = "background"

	CapabilityGeneration = 1
)

type SkillPackageView struct {
	RootURI    string
	SourceFile string
	Files      map[string][]byte
	Parsed     ParsedSkill
	ContentHash string
	Source     string
}

type SkillArgumentSchema struct {
	Raw        string            `json:"raw"`
	Positional []string          `json:"positional,omitempty"`
	Named      map[string]string `json:"named,omitempty"`
}

type SkillUIHints struct {
	ArgumentHint     string `json:"argumentHint,omitempty"`
	IconSmall        string `json:"iconSmall,omitempty"`
	IconLarge        string `json:"iconLarge,omitempty"`
	BrandColor       string `json:"brandColor,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	DefaultPrompt    string `json:"defaultPrompt,omitempty"`
}

type SkillInvocationPolicy struct {
	UserInvocationAllowed      bool `json:"userInvocationAllowed"`
	ImplicitInvocationAllowed  bool `json:"implicitInvocationAllowed"`
	ScheduledInvocationAllowed bool `json:"scheduledInvocationAllowed"`
	BackgroundAllowed          bool `json:"backgroundAllowed"`
	IsolatedExecutionRequested bool `json:"isolatedExecutionRequested"`
}

type SkillEcosystemProfile struct {
	ID       string   `json:"id"`
	Version  string   `json:"version"`
	Evidence []string `json:"evidence"`
}

type SkillProfileDetection struct {
	Detected []SkillEcosystemProfile `json:"detected"`
}

type SkillFieldMapping struct {
	Profile string `json:"profile"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
}

type SkillFeatureResult struct {
	Profile string `json:"profile"`
	Feature string `json:"feature"`
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
}

type SkillDependencyMapping struct {
	Profile    string `json:"profile"`
	ResolvedAs string `json:"resolvedAs"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
}

type SkillMCPDependency struct {
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

type SkillToolMapping struct {
	SourceTool    string `json:"sourceTool"`
	TargetSkillID string `json:"targetSkillId,omitempty"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
}

type SkillWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type SkillError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type CanonicalSkillCompatibility struct {
	InvocationPolicy      SkillInvocationPolicy `json:"invocationPolicy"`
	ArgumentSchema        *SkillArgumentSchema  `json:"argumentSchema,omitempty"`
	ActivationHints       []string              `json:"activationHints,omitempty"`
	PreferredModelHint    string                `json:"preferredModelHint,omitempty"`
	PreferredEffort       string                `json:"preferredEffort,omitempty"`
	WorkspacePathPatterns []string              `json:"workspacePathPatterns,omitempty"`
	ExecutionMode         string                `json:"executionMode,omitempty"`
	ToolAllowHints        []string              `json:"toolAllowHints,omitempty"`
	ToolDenyRules         []string              `json:"toolDenyRules,omitempty"`
	MCPDependencies       []SkillMCPDependency  `json:"mcpDependencies,omitempty"`
	UI                    SkillUIHints          `json:"ui,omitempty"`
	UnsupportedFeatures   []string              `json:"unsupportedFeatures,omitempty"`
}

type SkillCompatibilityOverlay struct {
	Profile               string                   `json:"profile"`
	AdapterVersion        string                   `json:"adapterVersion"`
	InvocationPolicy      *SkillInvocationPolicy   `json:"invocationPolicy,omitempty"`
	ArgumentSchema        *SkillArgumentSchema     `json:"argumentSchema,omitempty"`
	ActivationHints       []string                 `json:"activationHints,omitempty"`
	PreferredModelHint    string                   `json:"preferredModelHint,omitempty"`
	PreferredEffort       string                   `json:"preferredEffort,omitempty"`
	WorkspacePathPatterns []string                 `json:"workspacePathPatterns,omitempty"`
	ExecutionMode         string                   `json:"executionMode,omitempty"`
	ToolAllowHints        []string                 `json:"toolAllowHints,omitempty"`
	ToolDenyRules         []string                 `json:"toolDenyRules,omitempty"`
	MCPDependencies       []SkillMCPDependency     `json:"mcpDependencies,omitempty"`
	UI                    *SkillUIHints            `json:"ui,omitempty"`
	FieldMappings         []SkillFieldMapping      `json:"fieldMappings,omitempty"`
	Features              []SkillFeatureResult     `json:"features,omitempty"`
	DependencyMappings    []SkillDependencyMapping `json:"dependencyMappings,omitempty"`
	UnsupportedFeatures   []string                 `json:"unsupportedFeatures,omitempty"`
	Warnings              []SkillWarning           `json:"warnings,omitempty"`
	Errors                []SkillError             `json:"errors,omitempty"`
}

type CompatibilityFingerprint struct {
	ContentHash     string            `json:"contentHash"`
	AdapterVersions map[string]string `json:"adapterVersions"`
	CapabilityGen   int               `json:"capabilityGeneration"`
}

type SkillCompatibilityReport struct {
	Status             string                   `json:"status"`
	Detected           []SkillEcosystemProfile  `json:"detectedProfiles"`
	FieldMappings      []SkillFieldMapping      `json:"fieldMappings"`
	ToolMappings       []SkillToolMapping       `json:"toolMappings"`
	DependencyMappings []SkillDependencyMapping `json:"dependencyMappings"`
	RequiredScripts    []string                 `json:"requiredScripts"`
	MissingFiles       []string                 `json:"missingFiles"`
	MappedFeatures     []SkillFeatureResult     `json:"mappedFeatures"`
	Unsupported        []string                 `json:"unsupported"`
	Warnings           []SkillWarning           `json:"warnings"`
	Errors             []SkillError             `json:"errors"`
	Fingerprint        *CompatibilityFingerprint `json:"fingerprint,omitempty"`
	EvaluatedAt        time.Time                `json:"evaluatedAt"`
}

type SkillProfileAdapter interface {
	ID() string
	Version() string
	Detect(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillProfileDetection, error)
	Analyze(ctx context.Context, pkg SkillPackageView, parsed ParsedSkill) (SkillCompatibilityOverlay, error)
}

var DefaultInvocationPolicy = SkillInvocationPolicy{
	UserInvocationAllowed:     true,
	ImplicitInvocationAllowed: true,
}

func ComputeFingerprint(contentHash string, adapterVersions map[string]string) CompatibilityFingerprint {
	versions := make(map[string]string, len(adapterVersions))
	for k, v := range adapterVersions {
		versions[k] = v
	}
	return CompatibilityFingerprint{
		ContentHash:     contentHash,
		AdapterVersions: versions,
		CapabilityGen:   CapabilityGeneration,
	}
}

func MergeInvocationPolicy(base, overlay SkillInvocationPolicy) SkillInvocationPolicy {
	return SkillInvocationPolicy{
		UserInvocationAllowed:      base.UserInvocationAllowed && overlay.UserInvocationAllowed,
		ImplicitInvocationAllowed:  base.ImplicitInvocationAllowed && overlay.ImplicitInvocationAllowed,
		ScheduledInvocationAllowed: base.ScheduledInvocationAllowed && overlay.ScheduledInvocationAllowed,
		BackgroundAllowed:          base.BackgroundAllowed && overlay.BackgroundAllowed,
		IsolatedExecutionRequested: base.IsolatedExecutionRequested || overlay.IsolatedExecutionRequested,
	}
}
