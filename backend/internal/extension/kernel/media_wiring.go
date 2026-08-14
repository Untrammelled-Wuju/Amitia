package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/media"
)

func makeMediaCallFunc(svc *media.Service) capability.MediaCallFunc {
	return func(ctx context.Context, handlerName string, invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		if svc == nil {
			return nil, fmt.Errorf("media service not configured")
		}
		dispatcher := NewMediaToolDispatcher(svc)
		return dispatcher.Dispatch(ctx, handlerName, input)
	}
}

func makeMediaHealthFunc(svc *media.Service) capability.MediaHealthFunc {
	return func(ctx context.Context) capability.HealthStatus {
		if svc == nil {
			return capability.HealthUnknown
		}
		caps, err := svc.Capabilities(ctx)
		if err != nil || !caps.Available {
			return capability.HealthDegraded
		}
		return capability.HealthReady
	}
}
