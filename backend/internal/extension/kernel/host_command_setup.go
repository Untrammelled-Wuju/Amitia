package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func SetupDefaultHostCommands(registry *HostCommandRegistry, gateway *host_api.DefaultGateway) error {
	if registry == nil {
		return fmt.Errorf("host command setup: registry is nil")
	}
	if gateway == nil {
		return fmt.Errorf("host command setup: gateway is nil")
	}

	commands := []HostCommandDefinition{
		{
			CommandID:   "app.open.settings",
			Description: "Open the application settings page",
			Permission:  "ui.navigate",
			Scope:       HostCommandScopeGlobal,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			Handler:     makeNavigateHandler(gateway, "/settings"),
		},
		{
			CommandID:   "app.open.extension.detail",
			Description: "Open the extension detail page for a specific extension",
			Permission:  "ui.navigate",
			Scope:       HostCommandScopeGlobal,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{"extensionId":{"type":"string","description":"The extension ID to view details for"}}}`),
			Handler:     makeExtensionDetailHandler(gateway),
		},
		{
			CommandID:   "app.refresh.current_view",
			Description: "Refresh the current view",
			Permission:  "ui.notify",
			Scope:       HostCommandScopeGlobal,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			Handler:     makeNotifyHandler(gateway, "refresh_view"),
		},
		{
			CommandID:   "app.show.notification_center",
			Description: "Show the notification center",
			Permission:  "ui.notify",
			Scope:       HostCommandScopeGlobal,
			Risk:        host_api.RiskLow,
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			Handler:     makeNotifyHandler(gateway, "show_notification_center"),
		},
	}

	for _, cmd := range commands {
		if err := registry.Register(cmd); err != nil {
			return fmt.Errorf("host command setup: failed to register %s: %w", cmd.CommandID, err)
		}
	}
	return nil
}

func makeNavigateHandler(gateway *host_api.DefaultGateway, route string) HostCommandHandler {
	return func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
		navInput, _ := json.Marshal(map[string]any{
			"target": route,
		})
		callReq := host_api.CallRequest{
			CallID:               fmt.Sprintf("host-cmd-nav-%s", uuid.NewString()),
			RuntimeIdentity:      buildIdentity(execCtx),
			Method:               host_api.MethodUINavigate,
			Version:              1,
			Input:                navInput,
			ScopeSnapshotID:      execCtx.ScopeSnapshotID,
			PermissionSnapshotID: execCtx.PermissionSnapshotID,
		}
		result := gateway.Call(ctx, callReq)
		if result.Error != nil {
			return nil, fmt.Errorf("host command navigate to %s failed: %s", route, result.Error.Message)
		}
		output := result.Output
		if len(output) == 0 {
			output = json.RawMessage(`{"navigated":true,"route":"` + route + `"}`)
		}
		return output, nil
	}
}

func makeExtensionDetailHandler(gateway *host_api.DefaultGateway) HostCommandHandler {
	return func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
		var p struct {
			ExtensionID string `json:"extensionId"`
		}
		if len(input) > 0 {
			if err := json.Unmarshal(input, &p); err != nil {
				return nil, NewHostCommandError(
					ErrCodeHostCommandInputInvalid,
					"invalid input for app.open.extension.detail",
					err,
				)
			}
		}
		targetExtID := p.ExtensionID
		if targetExtID == "" {
			targetExtID = execCtx.ExtensionID
		}
		route := fmt.Sprintf("/extensions/%s", targetExtID)
		navInput, _ := json.Marshal(map[string]any{
			"target": route,
		})
		callReq := host_api.CallRequest{
			CallID:               fmt.Sprintf("host-cmd-ext-detail-%s", uuid.NewString()),
			RuntimeIdentity:      buildIdentity(execCtx),
			Method:               host_api.MethodUINavigate,
			Version:              1,
			Input:                navInput,
			ScopeSnapshotID:      execCtx.ScopeSnapshotID,
			PermissionSnapshotID: execCtx.PermissionSnapshotID,
		}
		result := gateway.Call(ctx, callReq)
		if result.Error != nil {
			return nil, fmt.Errorf("host command open extension detail failed: %s", result.Error.Message)
		}
		output := result.Output
		if len(output) == 0 {
			output = json.RawMessage(`{"navigated":true,"extensionId":"` + targetExtID + `"}`)
		}
		return output, nil
	}
}

func makeNotifyHandler(gateway *host_api.DefaultGateway, action string) HostCommandHandler {
	return func(ctx context.Context, execCtx HostCommandExecContext, input []byte) ([]byte, error) {
		notifyInput, _ := json.Marshal(map[string]any{
			"type":   action,
			"action": action,
		})
		callReq := host_api.CallRequest{
			CallID:               fmt.Sprintf("host-cmd-notify-%s-%s", action, uuid.NewString()),
			RuntimeIdentity:      buildIdentity(execCtx),
			Method:               host_api.MethodUINotify,
			Version:              1,
			Input:                notifyInput,
			ScopeSnapshotID:      execCtx.ScopeSnapshotID,
			PermissionSnapshotID: execCtx.PermissionSnapshotID,
		}
		result := gateway.Call(ctx, callReq)
		if result.Error != nil {
			return nil, fmt.Errorf("host command %s failed: %s", action, result.Error.Message)
		}
		output := result.Output
		if len(output) == 0 {
			output = json.RawMessage(`{"notified":true,"action":"` + action + `"}`)
		}
		return output, nil
	}
}

func buildIdentity(execCtx HostCommandExecContext) runtime_supervisor.RuntimeIdentity {
	return runtime_supervisor.RuntimeIdentity{
		InstanceID:  execCtx.SessionID,
		ExtensionID: domain.ExtensionID(execCtx.ExtensionID),
		ModuleID:    domain.ModuleID(execCtx.ModuleID),
		Generation:  execCtx.Generation,
	}
}
