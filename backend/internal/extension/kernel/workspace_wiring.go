package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

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
	workspaceID := strings.TrimSpace(invocation.ExecContext.WorkspaceID)
	if workspaceID == "" {
		return input, nil
	}
	rootURI := workspace.MountURI(workspace.WorkspaceID(workspaceID))
	if metadata := invocation.ExecContext.Metadata; metadata != nil {
		if value, ok := metadata["workspaceRootUri"].(string); ok && strings.TrimSpace(value) != "" {
			rootURI = strings.TrimSpace(value)
		}
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
	if value, ok := payload["workspaceId"].(string); ok && strings.TrimSpace(value) != "" && strings.TrimSpace(value) != workspaceID {
		return nil, fmt.Errorf("WORKSPACE_SCOPE_MISMATCH: current=%s requested=%s", workspaceID, strings.TrimSpace(value))
	}
	payload["workspaceId"] = workspaceID
	for _, key := range []string{"uri", "sourceUri", "destinationDirUri", "workspaceUri"} {
		value, ok := payload[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		normalized, err := bindWorkspaceURI(rootURI, value)
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
