package acquisition

// FindCapabilitiesTool 定义 find_capability 工具（ToolDefinition 兼容格式）
const FindCapabilitiesToolID = "find_capability"
const FindCapabilitiesToolDescription = "Discover installed and local capabilities, GitHub Agent Skills, official MCP Registry servers, and native Amitia plugin packages. Use query for search words and preferredKind for skill, mcp, or extension. Use sourceUri for a verified local skill directory or GitHub skill directory; an MCP requires its actual install descriptor. Inspect source errors and missing configuration; never invent endpoints, credentials, or package identities. Returns ranked candidates with their original source and install descriptor."

// AcquireCapabilityTool 定义 acquire_capability 工具
const AcquireCapabilityToolID = "acquire_capability"
const AcquireCapabilityToolDescription = "Acquire a candidate returned by find_capability using the same capabilityId and candidateId, or an explicit verified source and install descriptor. Skills retain their resources and persist; MCP configurations persist and require a successful connection before becoming ready. Only native Amitia packages are supported as plugins. Respect approval requirements and report missing credentials or configuration without inventing values. Check installed, enabled, state, and warnings: an installed package may still need configuration before it can be used."

// FindCapabilitiesInput defines the input parameters for the find_capability tool.
type FindCapabilitiesInput struct {
	Query         string                      `json:"query,omitempty"`
	SourceURI     string                      `json:"sourceUri,omitempty"`
	Install       *CandidateInstallDescriptor `json:"install,omitempty"`
	ExtensionID   string                      `json:"extensionId,omitempty"`
	Version       string                      `json:"version,omitempty"`
	CapabilityID  string                      `json:"capabilityId" description:"The identifier of the capability needed (for example: browser.control, search.web, mcp.server.filesystem, chat.openai, skill.weather_query)"`
	Description   string                      `json:"description,omitempty" description:"Optional description of what the AI wants to use the capability for"`
	PreferredKind string                      `json:"preferredKind,omitempty" description:"Optional preferred kind filter: extension, mcp, skill, generated_skill"`
}

// FindCapabilitiesOutput defines the output returned by the find_capability tool.
type FindCapabilitiesOutput struct {
	Errors       []SourceError         `json:"errors,omitempty"`
	Candidates   []CapabilityCandidate `json:"candidates"`
	TotalFound   int                   `json:"totalFound"`
	SearchTimeMs int64                 `json:"searchTimeMs"`
}

// AcquireInput defines the input parameters for the acquire_capability tool.
type AcquireInput struct {
	Query         string                      `json:"query,omitempty"`
	SourceURI     string                      `json:"sourceUri,omitempty"`
	Install       *CandidateInstallDescriptor `json:"install,omitempty"`
	ExtensionID   string                      `json:"extensionId,omitempty"`
	Version       string                      `json:"version,omitempty"`
	PreferredKind string                      `json:"preferredKind,omitempty"`
	CapabilityID  string                      `json:"capabilityId,omitempty" description:"The canonical capability identifier to acquire (for example: search.web, browser.control, github.issue.manage). Required for a new acquisition."`
	CandidateID   string                      `json:"candidateId,omitempty" description:"Optional candidate id from find_capability result. If empty, the Planner selects the best candidate automatically."`
	ResumeToken   string                      `json:"resumeToken,omitempty" description:"Resume token returned by a previous approval-required acquisition. When supplied, the existing acquisition transaction is resumed instead of creating a new one."`
	Approval      bool                        `json:"approval,omitempty" description:"Whether the AI should proceed with auto-install when user pre-approved"`
	UserConfirmed bool                        `json:"userConfirmed,omitempty" description:"Whether the explicit user approval was already granted"`
}

// AcquireOutput defines the output returned by the acquire_capability tool.
type AcquireOutput struct {
	Installed     bool             `json:"installed"`
	Enabled       bool             `json:"enabled"`
	Warnings      []string         `json:"warnings,omitempty"`
	Success       bool             `json:"success"`
	State         AcquisitionState `json:"state"`
	CapabilityID  string           `json:"capabilityId,omitempty"`
	ResumeToken   string           `json:"resumeToken,omitempty"`
	NeedsApproval bool             `json:"needsApproval,omitempty"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	InstalledAt   string           `json:"installedAt,omitempty"`
}
