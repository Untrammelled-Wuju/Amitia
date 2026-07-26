package capability

type CapabilitySource string

const (
	CapabilitySourceBuiltin     CapabilitySource = "builtin"
	CapabilitySourcePlugin      CapabilitySource = "plugin"
	CapabilitySourceMCP         CapabilitySource = "mcp"
	CapabilitySourceWorkflow    CapabilitySource = "workflow"
	CapabilitySourceComputerUse CapabilitySource = "computer_use"
	CapabilitySourceProvider    CapabilitySource = "provider"
	CapabilitySourceInternal    CapabilitySource = "internal"
	CapabilitySourceLegacy      CapabilitySource = "legacy"
)

type CapabilityType string

const (
	CapabilityTypeTool           CapabilityType = "tool"
	CapabilityTypeWorkflowEntry  CapabilityType = "workflow_entry"
	CapabilityTypeProviderAction CapabilityType = "provider_action"
	CapabilityTypeDesktopAction  CapabilityType = "desktop_action"
	CapabilityTypeInternalAction CapabilityType = "internal_action"
)

func CapabilitySourceToToolSource(s CapabilitySource) ToolSource {
	switch s {
	case CapabilitySourceBuiltin:
		return ToolSourceBuiltin
	case CapabilitySourcePlugin:
		return ToolSourcePlugin
	case CapabilitySourceMCP:
		return ToolSourceMCP
	case CapabilitySourceWorkflow:
		return ToolSourceWorkflow
	case CapabilitySourceInternal:
		return ToolSourceInternal
	case CapabilitySourceLegacy:
		return ToolSourceLegacy
	default:
		return ToolSource(string(s))
	}
}
