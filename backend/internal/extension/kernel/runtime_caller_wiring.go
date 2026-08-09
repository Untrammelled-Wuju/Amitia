package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func makeWorkflowCallFunc(executor *workflow.WorkflowExecutor) capability.WorkflowCallFunc {
	return func(ctx context.Context, workflowID string, invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		if executor == nil {
			return nil, fmt.Errorf("workflow executor not configured")
		}

		inputPayload := input
		if len(inputPayload) == 0 {
			inputPayload = json.RawMessage(`{}`)
		}

		req := workflow.ExecuteRequest{
			WorkflowID: workflowID,
			Input:      inputPayload,
			Context: workflow.ExecutionContext{
				InvocationID:     invocation.InvocationID,
				CharacterID:      invocation.CharacterID,
				ConversationID:   invocation.ConversationID,
				OperationID:      invocation.OperationID,
				TraceID:          invocation.TraceID,
				ExtensionID:      invocation.ExtensionID,
				ModuleID:         invocation.ModuleID,
				Generation:       invocation.Generation,
				ScheduleID:       invocation.ScheduleID,
				TriggerID:        invocation.TriggerID,
				IdempotencyKey:   invocation.IdempotencyKey,
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

func makeWorkflowCancelFunc(executor *workflow.WorkflowExecutor) capability.WorkflowCancelFunc {
	return func(ctx context.Context, invocationID string, reason string) error {
		if executor == nil {
			return fmt.Errorf("workflow executor not configured")
		}
		executor.Cancel(invocationID)
		return nil
	}
}

func (c *Container) WireMCPAdapter(caller capability.MCPCallFunc, health capability.MCPHealthFunc, postProcessor capability.MCPPostProcessor) {
	if c == nil || c.AdapterRegistry == nil {
		return
	}
	mcpAdapter := capability.NewMCPRuntimeAdapter(caller, health)
	if postProcessor != nil {
		mcpAdapter.SetPostProcessor(postProcessor)
	}
	c.AdapterRegistry.Register(capability.RuntimeTypeMCP, mcpAdapter)
}

func (c *Container) WireAndroidPlatformAdapter(provider capability.AndroidProvider) {
	if c == nil || c.AdapterRegistry == nil {
		return
	}
	adapter := capability.NewAndroidRuntimeAdapter(provider)
	c.AdapterRegistry.Register(capability.RuntimeTypeAndroid_Native, adapter)
}

func (c *Container) WireIOSPlatformAdapter(provider capability.IOSProvider) {
	if c == nil || c.AdapterRegistry == nil {
		return
	}
	adapter := capability.NewIOSRuntimeAdapter(provider)
	c.AdapterRegistry.Register(capability.RuntimeTypeIOS_Native, adapter)
}
