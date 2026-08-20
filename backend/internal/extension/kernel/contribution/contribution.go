package contribution

type ContributionType string

const (
	ContributionTypeTool              ContributionType = "tool"
	ContributionTypeAgentSkill        ContributionType = "agent_skill"
	ContributionTypeWorkflow          ContributionType = "workflow"
	ContributionTypeMCP               ContributionType = "mcp"
	ContributionTypeUI                ContributionType = "ui"
	ContributionTypeUIProvider        ContributionType = "ui_provider"
	ContributionTypeHook              ContributionType = "hook"
	ContributionTypeEventSubscription ContributionType = "event_subscription"
	ContributionTypeBackgroundTask    ContributionType = "background_task"
	ContributionTypeProvider          ContributionType = "provider"
	ContributionTypeAsset             ContributionType = "asset"
	ContributionTypeSchedule          ContributionType = "schedule"
)

type Contribution interface {
	ContributionID() string
	ContributionType() ContributionType
	ExtensionID() string
	ModuleID() string
}

type BaseContribution struct {
	ID        string           `json:"id"`
	Type      ContributionType `json:"type"`
	Extension string           `json:"extensionId"`
	Module    string           `json:"moduleId,omitempty"`
	Enabled   bool             `json:"enabled"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
}

func (c *BaseContribution) ContributionID() string             { return c.ID }
func (c *BaseContribution) ContributionType() ContributionType { return c.Type }
func (c *BaseContribution) ExtensionID() string                { return c.Extension }
func (c *BaseContribution) ModuleID() string                   { return c.Module }

type ToolContribution struct {
	BaseContribution
	ToolID string `json:"toolId"`
}

type AgentSkillContribution struct {
	BaseContribution
	AgentSkillID string `json:"agentSkillId"`
}

type WorkflowContribution struct {
	BaseContribution
	WorkflowID string `json:"workflowId"`
}

type MCPContribution struct {
	BaseContribution
	ServerID   string         `json:"serverId"`
	Descriptor map[string]any `json:"descriptor,omitempty"`
}

type UIContribution struct {
	BaseContribution
	SurfaceID string `json:"surfaceId"`
}

type HookContribution struct {
	BaseContribution
	Event   string `json:"event"`
	Handler string `json:"handler"`
}

type EventSubscriptionContribution struct {
	BaseContribution
	EventType string   `json:"eventType"`
	Filter    string   `json:"filter,omitempty"`
	Handler   string   `json:"handler"`
	SourceIDs []string `json:"sourceIds,omitempty"`
}

type ProviderContribution struct {
	BaseContribution
	ProviderID string `json:"providerId"`
}

type AssetContribution struct {
	BaseContribution
	Path string `json:"path"`
}

type BackgroundTaskContribution struct {
	BaseContribution
	TaskID string `json:"taskId"`
}

type ScheduleContribution struct {
	BaseContribution
	ScheduleID string `json:"scheduleId"`
}
