package protocol

import "fmt"

const (
	WorkflowProtocolVersion = "1"
	WorkflowSchemaVersion   = "workflow-v2"
	ToolProtocolVersion     = "1"
)

func WorkflowProtocolCapability() string {
	return "workflow.protocol.v" + WorkflowProtocolVersion
}

func WorkflowSchemaCapability(schemaVersion string) string {
	if schemaVersion == "" {
		schemaVersion = WorkflowSchemaVersion
	}
	return fmt.Sprintf("workflow.schema.%s", schemaVersion)
}

func ToolProtocolCapability() string {
	return "tool.protocol.v" + ToolProtocolVersion
}

func RequiredWorkflowRuntimeCapabilities(schemaVersion string) []string {
	if schemaVersion == "" {
		schemaVersion = WorkflowSchemaVersion
	}
	return []string{
		WorkflowProtocolCapability(),
		WorkflowSchemaCapability(schemaVersion),
		ToolProtocolCapability(),
	}
}
