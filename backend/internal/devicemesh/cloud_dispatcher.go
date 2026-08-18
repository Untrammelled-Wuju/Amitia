package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type cloudRuntimeDispatcher struct {
	registry *capability.RuntimeAdapterRegistry
	mu       sync.Mutex
	cancels  map[string]context.CancelFunc
}

func NewCloudRuntimeDispatcher(registry *capability.RuntimeAdapterRegistry) *cloudRuntimeDispatcher {
	return &cloudRuntimeDispatcher{registry: registry, cancels: make(map[string]context.CancelFunc)}
}

func (d *cloudRuntimeDispatcher) Resolve(_ string) agent.RuntimeInvokeHandler {
	if d == nil || d.registry == nil {
		return nil
	}
	return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		binding := capability.RuntimeBinding{
			RuntimeType: capability.RuntimeType(invoke.RuntimeType),
			HandlerName: invoke.Handler,
			ProviderID:  invoke.ProviderID,
		}
		adapter, ok := d.registry.Resolve(binding)
		if !ok || adapter == nil {
			return nil, errors.New("runtime adapter not found: " + invoke.RuntimeType)
		}

		ctx := context.Background()
		var cancel context.CancelFunc
		if invoke.DeadlineMs > 0 {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(invoke.DeadlineMs)*time.Millisecond)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		d.mu.Lock()
		d.cancels[invoke.InvocationID] = cancel
		d.mu.Unlock()
		defer func() {
			cancel()
			d.mu.Lock()
			delete(d.cancels, invoke.InvocationID)
			d.mu.Unlock()
		}()

		invocation := capability.ToolInvocationContext{
			InvocationID: invoke.InvocationID,
			UserID:       string(invoke.UserID),
		}
		result := adapter.Execute(ctx, binding, invocation, invoke.Input)
		now := time.Now().UTC()
		if result.Status == capability.ToolResultStatusSuccess {
			output := result.Structured
			if len(output) == 0 {
				var marshalErr error
				output, marshalErr = json.Marshal(result.Content)
				if marshalErr != nil {
					return nil, fmt.Errorf("marshal runtime result: %w", marshalErr)
				}
			}
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

func (d *cloudRuntimeDispatcher) CancelInvocation(invocationID string) bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	cancel, ok := d.cancels[invocationID]
	d.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}
