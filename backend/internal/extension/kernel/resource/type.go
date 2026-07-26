package resource

type ResourceType string

const (
	ResourceExtensionPackage   ResourceType = "extension_package"
	ResourceExtensionModule    ResourceType = "extension_module"
	ResourceTool               ResourceType = "tool"
	ResourceAgentSkill         ResourceType = "agent_skill"
	ResourceMCPServer          ResourceType = "mcp_server"
	ResourceMCPTool            ResourceType = "mcp_tool"
	ResourceWorkflow           ResourceType = "workflow"
	ResourceUIContribution     ResourceType = "ui_contribution"
	ResourceHook               ResourceType = "hook"
	ResourceBackgroundTask     ResourceType = "background_task"
	ResourceSchedule           ResourceType = "schedule"
	ResourceEventSubscription  ResourceType = "event_subscription"
	ResourceProvider           ResourceType = "provider"
	ResourceSecret             ResourceType = "secret"
	ResourceStorageNamespace   ResourceType = "storage_namespace"
	ResourceFile               ResourceType = "file"
	ResourceArtifact           ResourceType = "artifact"
	ResourceCache              ResourceType = "cache"
	ResourceProcess            ResourceType = "process"
	ResourceConnection         ResourceType = "connection"
	ResourceTemporaryDirectory ResourceType = "temporary_directory"
	ResourceWindow             ResourceType = "window"
	ResourceTrayAction         ResourceType = "tray_action"
)

func (rt ResourceType) IsValid() bool {
	switch rt {
	case ResourceExtensionPackage, ResourceExtensionModule, ResourceTool,
		ResourceAgentSkill, ResourceMCPServer, ResourceMCPTool,
		ResourceWorkflow, ResourceUIContribution, ResourceHook,
		ResourceBackgroundTask, ResourceSchedule, ResourceEventSubscription,
		ResourceProvider, ResourceSecret, ResourceStorageNamespace,
		ResourceFile, ResourceArtifact, ResourceCache,
		ResourceProcess, ResourceConnection, ResourceTemporaryDirectory,
		ResourceWindow, ResourceTrayAction:
		return true
	}
	return false
}
