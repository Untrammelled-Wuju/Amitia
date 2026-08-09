package execution

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ExecutionStatus string

const (
	StatusQueued           ExecutionStatus = "queued"
	StatusAwaitingApproval ExecutionStatus = "awaiting_approval"
	StatusRunning          ExecutionStatus = "running"
	StatusRetrying         ExecutionStatus = "retrying"
	StatusSucceeded        ExecutionStatus = "succeeded"
	StatusFailed           ExecutionStatus = "failed"
	StatusCancelled        ExecutionStatus = "cancelled"
	StatusTimedOut         ExecutionStatus = "timed_out"
	StatusDenied           ExecutionStatus = "denied"
	StatusRateLimited      ExecutionStatus = "rate_limited"
	StatusCircuitOpen      ExecutionStatus = "circuit_open"
	StatusInvalid          ExecutionStatus = "invalid"
)

type ToolExecutionRequest struct {
	ToolID     capability.CapabilityID          `json:"toolId"`
	Input      json.RawMessage                  `json:"input"`
	Invocation capability.ToolInvocationContext `json:"invocation"`
}

type ExecutionSecurityKernel interface {
	Execute(ctx context.Context, request ToolExecutionRequest) capability.UnifiedToolResult
}

type StreamingExecutionSecurityKernel interface {
	ExecutionSecurityKernel

	ExecuteStream(
		ctx context.Context,
		request ToolExecutionRequest,
		sink capability.ToolStreamSink,
	) (
		capability.UnifiedToolResult,
		error,
	)
}
