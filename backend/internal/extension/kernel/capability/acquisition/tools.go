package acquisition

// FindCapabilitiesTool 定义 find_capability 工具（ToolDefinition 兼容格式）
const FindCapabilitiesToolID = "find_capability"
const FindCapabilitiesToolDescription = "Search for available extensions/skills/MCPs that provide a capability the AI needs but does not currently have. Returns a ranked list of CapabilityCandidate items."

// AcquireCapabilityTool 定义 acquire_capability 工具
const AcquireCapabilityToolID = "acquire_capability"
const AcquireCapabilityToolDescription = "Install, enable, or configure a candidate capability so that the AI can use it. Returns an AcquisitionResult and suspends the current task until the capability becomes executable."

// FindCapabilitiesInput defines the input parameters for the find_capability tool.
type FindCapabilitiesInput struct {
	CapabilityID  string `json:"capabilityId" description:"The identifier of the capability needed (for example: browser.control, search.web, mcp.server.filesystem, chat.openai, skill.weather_query)"`
	Description   string `json:"description,omitempty" description:"Optional description of what the AI wants to use the capability for"`
	PreferredKind string `json:"preferredKind,omitempty" description:"Optional preferred kind filter: extension, mcp, skill, generated_skill"`
}

// FindCapabilitiesOutput defines the output returned by the find_capability tool.
type FindCapabilitiesOutput struct {
	Candidates   []CapabilityCandidate `json:"candidates"`
	TotalFound   int                   `json:"totalFound"`
	SearchTimeMs int64                 `json:"searchTimeMs"`
}

// AcquireInput defines the input parameters for the acquire_capability tool.
type AcquireInput struct {
	CapabilityID  string `json:"capabilityId" description:"The canonical capability identifier to acquire (for example: search.web, browser.control, github.issue.manage)"`
	CandidateID   string `json:"candidateId,omitempty" description:"Optional candidate id from find_capability result. If empty, the Planner selects the best candidate automatically."`
	Approval      bool   `json:"approval,omitempty" description:"Whether the AI should proceed with auto-install when user pre-approved"`
	UserConfirmed bool   `json:"userConfirmed,omitempty" description:"Whether the explicit user approval was already granted"`
}

// AcquireOutput defines the output returned by the acquire_capability tool.
type AcquireOutput struct {
	Success       bool             `json:"success"`
	State         AcquisitionState `json:"state"`
	CapabilityID  string           `json:"capabilityId,omitempty"`
	ResumeToken   string           `json:"resumeToken,omitempty"`
	NeedsApproval bool             `json:"needsApproval,omitempty"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	InstalledAt   string           `json:"installedAt,omitempty"`
}
