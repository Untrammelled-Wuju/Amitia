package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type cloudRuntimeDispatcher struct {
	registry *capability.RuntimeAdapterRegistry
}

func NewCloudRuntimeDispatcher(registry *capability.RuntimeAdapterRegistry) *cloudRuntimeDispatcher {
	return &cloudRuntimeDispatcher{registry: registry}
}

func (d *cloudRuntimeDispatcher) Resolve(handlerName string) agent.RuntimeInvokeHandler {
	binding := capability.RuntimeBinding{
		RuntimeType: capability.RuntimeType(handlerName),
		HandlerName: handlerName,
	}
	adapter, ok := d.registry.Resolve(binding)
	if !ok {
		return nil
	}
	return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		invocation := capability.ToolInvocationContext{
			InvocationID: invoke.InvocationID,
			UserID:       string(invoke.DeviceID),
		}
		result := adapter.Execute(context.Background(), binding, invocation, invoke.Input)
		now := time.Now().UTC()
		if result.Status == capability.ToolResultStatusSuccess {
			output, _ := json.Marshal(result.Content)
			return &protocol.RuntimeResultPayload{
				InvocationID:         invoke.InvocationID,
				RuntimeSessionID:     invoke.RuntimeSessionID,
				ConnectionGeneration: invoke.ConnectionGeneration,
				DeviceID:             invoke.DeviceID,
				RuntimeID:            invoke.RuntimeID,
				Status:               string(result.Status),
				Result:               output,
				CompletedAt:          now,
			}, nil
		}
		errMsg := string(result.Status)
		if result.Error != nil {
			errMsg = result.Error.Message
		}
		return nil, errors.New(errMsg)
	}
}
