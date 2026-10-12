package kernel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func appearanceOrigin(ctx context.Context) (*auth.ActorContext, error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil {
		return nil, fmt.Errorf("appearance modification requires authenticated device context")
	}
	switch actor.PrincipalType {
	case auth.PrincipalLocalUI, auth.PrincipalTrustedDevice, auth.PrincipalDeviceRuntime:
	default:
		return nil, fmt.Errorf("appearance modification is forbidden for non-device principals")
	}
	if strings.TrimSpace(string(actor.SpaceID)) == "" || strings.TrimSpace(string(actor.DeviceID)) == "" {
		return nil, fmt.Errorf("appearance modification requires authenticated originating device")
	}
	return actor, nil
}

func (n *SSEUIHostNotifier) ExecuteAppearanceCommand(ctx context.Context, action string, expectedRevision int64, changes map[string]interface{}) (map[string]interface{}, error) {
	actor, err := appearanceOrigin(ctx)
	if err != nil {
		return nil, err
	}
	switch action {
	case "inspect", "apply", "reset", "undo":
	default:
		return nil, fmt.Errorf("unsupported appearance action %q", action)
	}
	if n.hostRegistry == nil || n.hub == nil {
		return nil, ErrUIHostUnavailable
	}
	hosts, err := n.hostRegistry.ListReadyHostsString(ctx, string(actor.SpaceID), host_registry.CapUIStyle)
	if err != nil {
		return nil, err
	}
	var selected *host_registry.HostEntry
	for _, host := range hosts {
		if host == nil || host.DeviceID != actor.DeviceID || host.SpaceID != actor.SpaceID ||
			!n.hub.ClientExists(host.HostClientID) {
			continue
		}
		if selected != nil {
			return nil, fmt.Errorf("multiple UI host sessions exist for this device; refuse ambiguous appearance target")
		}
		selected = host
	}
	if selected == nil {
		return nil, fmt.Errorf("the originating device does not have an active UI style host session")
	}
	bindAppearanceChangesToHostPlatform(changes, selected.Platform)
	commandID := "ui-style-" + uuid.NewString()
	pending := &pendingClientRuntimeCommand{
		responseCh:   make(chan clientRuntimeHostResponse, 1),
		allowedHosts: map[string]string{selected.HostClientID: selected.HostSessionID},
		responded:    make(map[string]struct{}),
	}
	n.mu.Lock()
	n.pendingClientRuntimeCommands[commandID] = pending
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.pendingClientRuntimeCommands, commandID)
		n.mu.Unlock()
	}()
	if changes == nil {
		changes = map[string]interface{}{}
	}
	payload := map[string]interface{}{
		"schemaVersion":    1,
		"commandId":        commandID,
		"hostClientId":     selected.HostClientID,
		"hostSessionId":    selected.HostSessionID,
		"action":           action,
		"expectedRevision": expectedRevision,
		"changes":          changes,
		"platform":         string(selected.Platform),
		"deviceId":         string(actor.DeviceID),
	}
	envelope := NewSSEEventEnvelope("ui_style_command", "com.amitia.builtin.uiagent", payload, 90*time.Second)
	envelopeMap := envelope.ToMap()
	envelopeMap["hostClientId"] = selected.HostClientID
	envelopeMap["hostSessionId"] = selected.HostSessionID
	n.hub.SendToClient(selected.HostClientID, "ui_style_command", envelopeMap)
	wait, cancel := context.WithTimeout(ctx, 85*time.Second)
	defer cancel()
	select {
	case response := <-pending.responseCh:
		if response.Error != "" {
			return nil, fmt.Errorf("appearance command failed: %s", response.Error)
		}
		if response.Result == nil || response.Result["ok"] != true {
			return nil, fmt.Errorf("appearance command was not applied")
		}
		return response.Result, nil
	case <-wait.Done():
		return nil, fmt.Errorf("appearance command response unavailable: %w", wait.Err())
	}
}

func bindAppearanceChangesToHostPlatform(changes map[string]interface{}, platform runtimeidentity.Platform) {
	if changes == nil {
		return
	}
	stylePlatform := stylePlatformForRuntime(platform)
	if stylePlatform == "" {
		return
	}
	bindScopePlatform := func(container map[string]interface{}) {
		scope, ok := container["scope"].(map[string]interface{})
		if !ok {
			scope = map[string]interface{}{}
			container["scope"] = scope
		}
		scope["platform"] = stylePlatform
	}
	if scope, ok := changes["scope"].(map[string]interface{}); ok {
		scope["platform"] = stylePlatform
	}
	for _, key := range []string{"targets", "rules"} {
		items, _ := changes[key].([]interface{})
		for _, item := range items {
			target, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			bindScopePlatform(target)
		}
	}
	if reset, ok := changes["reset"].(map[string]interface{}); ok {
		if scope, exists := reset["scope"].(map[string]interface{}); exists {
			scope["platform"] = stylePlatform
		}
	}
}

func stylePlatformForRuntime(platform runtimeidentity.Platform) string {
	switch platform {
	case runtimeidentity.PlatformAndroid, runtimeidentity.PlatformIOS:
		return "mobile"
	case runtimeidentity.PlatformWindows, runtimeidentity.PlatformLinux, runtimeidentity.PlatformDarwin:
		return "desktop"
	case runtimeidentity.PlatformWeb:
		return "web"
	default:
		return ""
	}
}
