package workflow

import runtimeprotocol "github.com/u-ai/backend/internal/deviceruntime/protocol"

const (
	WorkflowProtocolVersion = runtimeprotocol.WorkflowProtocolVersion
	ToolProtocolVersion     = runtimeprotocol.ToolProtocolVersion
)

func WorkflowProtocolCapability() string {
	return runtimeprotocol.WorkflowProtocolCapability()
}

func WorkflowSchemaCapability(schemaVersion string) string {
	return runtimeprotocol.WorkflowSchemaCapability(schemaVersion)
}

func ToolProtocolCapability() string {
	return runtimeprotocol.ToolProtocolCapability()
}

func RequiredDeviceRuntimeCapabilities(schemaVersion string) []string {
	if schemaVersion == "" {
		schemaVersion = UserWorkflowSchemaVersion
	}
	return runtimeprotocol.RequiredWorkflowRuntimeCapabilities(schemaVersion)
}
