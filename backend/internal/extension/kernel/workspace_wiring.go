package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/workspace"
)

func makeWorkspaceCallFunc(svc *workspace.Service) capability.WorkspaceCallFunc {
	return func(ctx context.Context, handlerName string, invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
		if svc == nil {
			return nil, fmt.Errorf("workspace service not configured")
		}
		scopedInput, err := scopeWorkspaceInvocationInput(invocation, input)
		if err != nil {
			return nil, err
		}
		dispatcher := workspace.NewToolDispatcher(svc)
		dispatcher.SetPreciseService(workspace.NewDefaultPreciseEditingService(svc))
		return dispatcher.Dispatch(ctx, handlerName, scopedInput)
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

func scopeWorkspaceInvocationInput(invocation capability.ToolInvocationContext, input json.RawMessage) (json.RawMessage, error) {
	if invocation.ExecContext == nil {
		return input, nil
	}
	bindings := append([]execution.WorkspaceBinding(nil), invocation.ExecContext.WorkspaceBindings...)
	if len(bindings) == 0 && strings.TrimSpace(invocation.ExecContext.WorkspaceID) != "" {
		workspaceID := strings.TrimSpace(invocation.ExecContext.WorkspaceID)
		binding := execution.WorkspaceBinding{ID: workspaceID, RootURI: workspace.MountURI(workspace.WorkspaceID(workspaceID))}
		if metadata := invocation.ExecContext.Metadata; metadata != nil {
			if value, ok := metadata["workspaceRootUri"].(string); ok && strings.TrimSpace(value) != "" {
				binding.RootURI = strings.TrimSpace(value)
			}
		}
		bindings = append(bindings, binding)
	}
	if len(bindings) == 0 {
		return input, nil
	}
	primaryID := strings.TrimSpace(invocation.ExecContext.PrimaryWorkspaceID)
	if primaryID == "" {
		primaryID = strings.TrimSpace(invocation.ExecContext.WorkspaceID)
	}
	if primaryID == "" {
		primaryID = bindings[0].ID
	}
	roots := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		if binding.ID != "" && binding.RootURI != "" {
			roots[binding.ID] = binding.RootURI
		}
	}
	if roots[primaryID] == "" {
		return nil, fmt.Errorf("WORKSPACE_NOT_BOUND: primary workspace is unavailable")
	}
	var payload map[string]any
	if len(input) == 0 {
		payload = map[string]any{}
	} else if err := json.Unmarshal(input, &payload); err != nil {
		return nil, fmt.Errorf("workspace input must be an object: %w", err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	workspaceID := primaryID
	if value, ok := payload["workspaceId"].(string); ok && strings.TrimSpace(value) != "" {
		workspaceID = strings.TrimSpace(value)
	}
	if roots[workspaceID] == "" {
		return nil, fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: requested=%s is outside the current project", workspaceID)
	}
	payload["workspaceId"] = workspaceID
	for _, key := range []string{"uri", "sourceUri", "destinationDirUri", "workspaceUri"} {
		value, ok := payload[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		normalized, err := bindProjectWorkspaceURI(roots, workspaceID, primaryID, value)
		if err != nil {
			return nil, err
		}
		payload[key] = normalized
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode scoped workspace input: %w", err)
	}
	return encoded, nil
}

func executionWorkspaceBindings(execCtx *execution.ExecutionContext) []execution.WorkspaceBinding {
	if execCtx == nil {
		return nil
	}
	bindings := append([]execution.WorkspaceBinding(nil), execCtx.WorkspaceBindings...)
	if len(bindings) == 0 && strings.TrimSpace(execCtx.WorkspaceID) != "" {
		workspaceID := strings.TrimSpace(execCtx.WorkspaceID)
		binding := execution.WorkspaceBinding{ID: workspaceID, RootURI: workspace.MountURI(workspace.WorkspaceID(workspaceID))}
		if metadata := execCtx.Metadata; metadata != nil {
			if value, ok := metadata["workspaceRootUri"].(string); ok && strings.TrimSpace(value) != "" {
				binding.RootURI = strings.TrimSpace(value)
			}
			if value, ok := metadata["workspaceName"].(string); ok {
				binding.Name = strings.TrimSpace(value)
			}
			if value, ok := metadata["workspaceKind"].(string); ok {
				binding.Kind = strings.TrimSpace(value)
			}
			if value, ok := metadata["workspaceDeviceId"].(string); ok {
				binding.DeviceID = strings.TrimSpace(value)
			}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func executionWorkspaceRoot(execCtx *execution.ExecutionContext, workspaceID string) string {
	bindings := executionWorkspaceBindings(execCtx)
	if len(bindings) == 0 {
		return ""
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" && execCtx != nil {
		workspaceID = strings.TrimSpace(execCtx.PrimaryWorkspaceID)
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(execCtx.WorkspaceID)
		}
	}
	if workspaceID == "" {
		workspaceID = bindings[0].ID
	}
	for _, binding := range bindings {
		if binding.ID == workspaceID {
			return binding.RootURI
		}
	}
	return ""
}

func bindProjectWorkspaceURI(roots map[string]string, workspaceID, primaryID, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "amitia://workspace/") {
		const prefix = "amitia://workspace/@"
		if !strings.HasPrefix(value, prefix) {
			return "", fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: uri %s is outside the current project", value)
		}
		remainder := strings.TrimPrefix(value, prefix)
		parts := strings.SplitN(remainder, "/", 2)
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" || roots[parts[0]] == "" {
			return "", fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: uri %s is outside the current project", value)
		}
		rootURI := roots[parts[0]]
		if value != strings.TrimSuffix(rootURI, "/") && !strings.HasPrefix(value, rootURI) {
			return "", fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: uri %s is outside workspace %s", value, parts[0])
		}
		return value, nil
	}
	rootURI := roots[workspaceID]
	if rootURI == "" {
		rootURI = roots[primaryID]
	}
	return bindWorkspaceURI(rootURI, value)
}

func bindWorkspaceURI(rootURI, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "amitia://workspace/") {
		if value == strings.TrimSuffix(rootURI, "/") || strings.HasPrefix(value, rootURI) {
			return value, nil
		}
		return "", fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: uri %s is outside current workspace", value)
	}
	clean := path.Clean(strings.TrimPrefix(strings.ReplaceAll(value, "\\", "/"), "/"))
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: relative path escapes current workspace")
	}
	if clean == "" {
		return rootURI, nil
	}
	return rootURI + clean, nil
}
