package execution

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ApprovalPersistence interface {
	SaveApprovalRequest(context.Context, ApprovalRequest) error
	SaveApprovalResolution(context.Context, ApprovalRequest, ApprovalActor) error
	FindApprovedApproval(context.Context, ApprovalRequest) (ApprovalRequest, bool, error)
}

type AgentExecutionObserver interface {
	BeforeToolExecution(context.Context, capability.ToolInvocationContext, capability.ToolDefinition, []byte) error
	AfterToolExecution(context.Context, capability.ToolInvocationContext, capability.ToolDefinition, capability.UnifiedToolResult, int64)
	RecordUnknownOutcome(context.Context, capability.ToolInvocationContext, capability.ToolDefinition, capability.UnifiedToolResult, string) error
}
