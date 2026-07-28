package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func makeWorkflowCallFunc(executor *workflow.WorkflowExecutor) capability.WorkflowCallFunc {
	return func(ctx context.Context, workflowID string, input json.RawMessage) (json.RawMessage, error) {
		if executor == nil {
			return nil, fmt.Errorf("workflow executor not configured")
		}

		inputPayload := input
		if len(inputPayload) == 0 {
			inputPayload = json.RawMessage(`{}`)
		}

		invocationID := fmt.Sprintf("wf-tool-%s", uuid.NewString())

		req := workflow.ExecuteRequest{
			WorkflowID: workflowID,
			Input:      inputPayload,
			Context: workflow.ExecutionContext{
				InvocationID: invocationID,
			},
		}

		result, err := executor.Execute(ctx, req)
		if err != nil {
			return nil, err
		}

		if !result.Success {
			return nil, fmt.Errorf("workflow execution failed: %s", result.Error)
		}

		output := result.Output
		if len(output) == 0 {
			output = json.RawMessage(`{}`)
		}

		return output, nil
	}
}

func (c *Container) WireMCPAdapter(caller capability.MCPCallFunc, health capability.MCPHealthFunc) {
	if c == nil || c.AdapterRegistry == nil {
		return
	}
	mcpAdapter := capability.NewMCPRuntimeAdapter(caller, health)
	c.AdapterRegistry.Register(capability.RuntimeTypeMCP, mcpAdapter)
}
