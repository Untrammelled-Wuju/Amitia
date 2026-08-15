package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/workspace"
)

func makeWorkspaceCallFunc(svc *workspace.Service) capability.WorkspaceCallFunc {
	return func(ctx context.Context, handlerName string, invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		if svc == nil {
			return nil, fmt.Errorf("workspace service not configured")
		}
		dispatcher := workspace.NewToolDispatcher(svc)
		return dispatcher.Dispatch(ctx, handlerName, input)
	}
}

func makeWorkspaceHealthFunc(svc *workspace.Service) capability.WorkspaceHealthFunc {
	return func(ctx context.Context) capability.HealthStatus {
		if svc == nil {
			return capability.HealthUnknown
		}
		if !svc.HasBackend(workspace.WorkspaceKindLocal) {
			return capability.HealthUnhealthy
		}
		mounts, err := svc.ListMounts(ctx)
		if err != nil {
			return capability.HealthDegraded
		}
		available := 0
		for _, m := range mounts {
			if m.Available {
				available++
			}
		}
		if available == 0 && len(mounts) > 0 {
			return capability.HealthDegraded
		}
		return capability.HealthReady
	}
}
