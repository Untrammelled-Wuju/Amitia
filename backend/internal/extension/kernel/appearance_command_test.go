package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/sse"
)

func TestAppearanceOriginRejectsMissingAndAdministrativeIdentity(t *testing.T) {
	cases := []struct {
		name  string
		actor *auth.ActorContext
	}{
		{"missing", nil},
		{"admin", &auth.ActorContext{PrincipalType: auth.PrincipalSystemWorker, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a"), SessionID: "session"}},
		{"automation", &auth.ActorContext{PrincipalType: auth.PrincipalAutomation, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a"), SessionID: "session"}},
		{"missing_device", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space-a"), SessionID: "session"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.actor != nil {
				ctx = auth.WithActor(ctx, tc.actor)
			}
			if _, err := appearanceOrigin(ctx); err == nil {
				t.Fatal("expected origin rejection")
			}
		})
	}
}

func TestAppearanceOriginAcceptsVerifiedLocalDevice(t *testing.T) {
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a"), SessionID: "session"}
	resolved, err := appearanceOrigin(auth.WithActor(context.Background(), actor))
	if err != nil || resolved != actor {
		t.Fatalf("origin rejected: %v", err)
	}
}

func TestAppearanceOriginAcceptsVerifiedDeviceWithoutAuthSession(t *testing.T) {
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a")}
	resolved, err := appearanceOrigin(auth.WithActor(context.Background(), actor))
	if err != nil || resolved != actor {
		t.Fatalf("origin rejected: %v", err)
	}
}

func TestAppearanceFailsClosedWithoutAuthenticatedOrigin(t *testing.T) {
	notifier := NewSSEUIHostNotifier(nil)
	_, err := notifier.ExecuteAppearanceCommand(context.Background(), "reset", 0, nil)
	if err == nil || !strings.Contains(err.Error(), "authenticated device") {
		t.Fatalf("expected unauthorized rejection: %v", err)
	}
}

func TestBindAppearanceChangesToHostPlatform(t *testing.T) {
	changes := map[string]interface{}{
		"scope": map[string]interface{}{"page": "chat"},
		"targets": []interface{}{
			map[string]interface{}{
				"scope": map[string]interface{}{"platform": "desktop", "page": "chat"},
				"target": "chat.composer.input",
			},
			map[string]interface{}{
				"target": "chat.composer.send",
			},
		},
		"reset": map[string]interface{}{
			"scope": map[string]interface{}{"platform": "desktop", "page": "chat"},
		},
	}
	bindAppearanceChangesToHostPlatform(changes, runtimeidentity.PlatformAndroid)
	if got := changes["scope"].(map[string]interface{})["platform"]; got != "mobile" {
		t.Fatalf("top-level scope platform = %#v, want mobile", got)
	}
	targets := changes["targets"].([]interface{})
	firstScope := targets[0].(map[string]interface{})["scope"].(map[string]interface{})
	if firstScope["platform"] != "mobile" {
		t.Fatalf("first target scope platform = %#v, want mobile", firstScope["platform"])
	}
	secondScope := targets[1].(map[string]interface{})["scope"].(map[string]interface{})
	if secondScope["platform"] != "mobile" {
		t.Fatalf("second target scope platform = %#v, want mobile", secondScope["platform"])
	}
	resetScope := changes["reset"].(map[string]interface{})["scope"].(map[string]interface{})
	if resetScope["platform"] != "mobile" {
		t.Fatalf("reset scope platform = %#v, want mobile", resetScope["platform"])
	}
}

func TestAppearanceCommandRoutesStyleEventToOriginatingDevice(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE kernel_host_registry (
		host_client_id TEXT PRIMARY KEY,
		host_session_id TEXT NOT NULL,
		space_id TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL DEFAULT '',
		runtime_id TEXT NOT NULL DEFAULT '',
		window_id TEXT NOT NULL DEFAULT '',
		capabilities TEXT NOT NULL DEFAULT '[]',
		entry_kind TEXT NOT NULL DEFAULT 'ui_host',
		authenticated_at TEXT NOT NULL,
		last_heartbeat TEXT NOT NULL,
		connection_state TEXT NOT NULL DEFAULT 'disconnected',
		session_token TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		expires_at TEXT,
		runtime_session_id TEXT NOT NULL DEFAULT '',
		connection_generation INTEGER NOT NULL DEFAULT 0,
		entry_id TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	actor := &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       runtimeidentity.SpaceID("space-style"),
		DeviceID:      runtimeidentity.DeviceID("device-style"),
		SessionID:     "session-style",
	}
	if err := registry.RegisterHost(context.Background(), &host_registry.HostEntry{
		EntryID:       "host-style",
		Kind:          host_registry.RegistryEntryKindUIHost,
		SpaceID:       actor.SpaceID,
		DeviceID:      actor.DeviceID,
		Platform:      runtimeidentity.PlatformWindows,
		Features:      []host_registry.EndpointFeature{host_registry.EndpointFeatureUIStyle},
		PresenceState: host_registry.PresenceStateReady,
		HostClientID:  "host-style",
		HostSessionID: "host-session-style",
	}); err != nil {
		t.Fatal(err)
	}
	hub := sse.Global
	clientID := "host-style"
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)
	notifier := NewSSEUIHostNotifierWithRegistry(hub, registry)
	readyHosts, err := registry.ListReadyHosts(context.Background(), actor.SpaceID, host_registry.CapUIStyle)
	if err != nil || len(readyHosts) != 1 {
		t.Fatalf("ready ui.style hosts = %d, err = %v", len(readyHosts), err)
	}
	if readyHosts[0].HostClientID != "host-style" ||
		readyHosts[0].DeviceID != actor.DeviceID ||
		readyHosts[0].SpaceID != actor.SpaceID ||
		!hub.ClientExists(readyHosts[0].HostClientID) {
		t.Fatalf("ready host binding mismatch: %#v clientExists=%v", readyHosts[0], hub.ClientExists(readyHosts[0].HostClientID))
	}

	type commandOutcome struct {
		result map[string]interface{}
		err    error
	}
	outcomeCh := make(chan commandOutcome, 1)
	go func() {
		result, commandErr := notifier.ExecuteAppearanceCommand(
			auth.WithActor(context.Background(), actor),
			"apply",
			4,
			map[string]interface{}{"themeMode": "dark"},
		)
		outcomeCh <- commandOutcome{result: result, err: commandErr}
	}()

	var commandID string
	select {
	case message := <-client.Events:
		if eventName, _ := message["event"].(string); eventName != "ui_style_command" {
			t.Fatalf("event = %v, want ui_style_command", message["event"])
		}
		raw, _ := message["data"].(map[string]interface{})
		payload, _ := raw["payload"].(map[string]interface{})
		commandID, _ = payload["commandId"].(string)
		if commandID == "" {
			t.Fatalf("missing commandId in payload: %#v", raw)
		}
		if fmt.Sprint(payload["expectedRevision"]) != "4" {
			t.Fatalf("expectedRevision = %#v, want 4", payload["expectedRevision"])
		}
		changes, _ := payload["changes"].(map[string]interface{})
		if changes["themeMode"] != "dark" {
			t.Fatalf("changes = %#v", changes)
		}
	case outcome := <-outcomeCh:
		t.Fatalf("command ended before event: result=%#v err=%v", outcome.result, outcome.err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ui_style_command")
	}
	if !notifier.ResolveClientRuntimeCommandWithHost(
		commandID,
		"host-style",
		"host-session-style",
		map[string]interface{}{"ok": true, "revision": 5},
		"",
	) {
		t.Fatal("style response was not accepted")
	}
	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("command failed: %v", outcome.err)
		}
		if outcome.result["ok"] != true || fmt.Sprint(outcome.result["revision"]) != "5" {
			encoded, _ := json.Marshal(outcome.result)
			t.Fatalf("unexpected result: %s", encoded)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command completion")
	}
}

func TestAppearanceCommandForwardsResetAndInspectChanges(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE kernel_host_registry (
		host_client_id TEXT PRIMARY KEY,
		host_session_id TEXT NOT NULL,
		space_id TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL DEFAULT '',
		runtime_id TEXT NOT NULL DEFAULT '',
		window_id TEXT NOT NULL DEFAULT '',
		capabilities TEXT NOT NULL DEFAULT '[]',
		entry_kind TEXT NOT NULL DEFAULT 'ui_host',
		authenticated_at TEXT NOT NULL,
		last_heartbeat TEXT NOT NULL,
		connection_state TEXT NOT NULL DEFAULT 'disconnected',
		session_token TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		expires_at TEXT,
		runtime_session_id TEXT NOT NULL DEFAULT '',
		connection_generation INTEGER NOT NULL DEFAULT 0,
		entry_id TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	actor := &auth.ActorContext{
		PrincipalType: auth.PrincipalTrustedDevice,
		SpaceID:       runtimeidentity.SpaceID("space-filter"),
		DeviceID:      runtimeidentity.DeviceID("device-filter"),
		SessionID:     "session-filter",
	}
	if err := registry.RegisterHost(context.Background(), &host_registry.HostEntry{
		EntryID:       "host-filter",
		Kind:          host_registry.RegistryEntryKindUIHost,
		SpaceID:       actor.SpaceID,
		DeviceID:      actor.DeviceID,
		Platform:      runtimeidentity.PlatformWindows,
		Features:      []host_registry.EndpointFeature{host_registry.EndpointFeatureUIStyle},
		PresenceState: host_registry.PresenceStateReady,
		HostClientID:  "host-filter",
		HostSessionID: "host-session-filter",
	}); err != nil {
		t.Fatal(err)
	}
	hub := sse.Global
	clientID := "host-filter"
	client := hub.Subscribe(clientID)
	defer hub.Unsubscribe(clientID)
	notifier := NewSSEUIHostNotifierWithRegistry(hub, registry)

	cases := []struct {
		action  string
		changes map[string]interface{}
	}{
		{"reset", map[string]interface{}{"target": "chat.composer.input", "properties": []string{"backgroundColor"}}},
		{"inspect", map[string]interface{}{"target": "chat.composer.input", "limit": 10, "includeComputed": true}},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			type commandOutcome struct {
				result map[string]interface{}
				err    error
			}
			outcomeCh := make(chan commandOutcome, 1)
			go func() {
				result, commandErr := notifier.ExecuteAppearanceCommand(
					auth.WithActor(context.Background(), actor),
					tc.action,
					0,
					tc.changes,
				)
				outcomeCh <- commandOutcome{result: result, err: commandErr}
			}()

			var commandID string
			select {
			case message := <-client.Events:
				raw, _ := message["data"].(map[string]interface{})
				payload, _ := raw["payload"].(map[string]interface{})
				commandID, _ = payload["commandId"].(string)
				if commandID == "" {
					t.Fatalf("missing commandId in payload: %#v", raw)
				}
				if payload["action"] != tc.action {
					t.Fatalf("action = %#v, want %s", payload["action"], tc.action)
				}
				changes, ok := payload["changes"].(map[string]interface{})
				if !ok {
					t.Fatalf("changes must be an object: %#v", payload["changes"])
				}
				if changes["target"] != "chat.composer.input" {
					t.Fatalf("changes = %#v", changes)
				}
			case outcome := <-outcomeCh:
				t.Fatalf("command ended before event: result=%#v err=%v", outcome.result, outcome.err)
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for ui_style_command")
			}
			if !notifier.ResolveClientRuntimeCommandWithHost(
				commandID,
				"host-filter",
				"host-session-filter",
				map[string]interface{}{"ok": true},
				"",
			) {
				t.Fatal("style response was not accepted")
			}
			select {
			case outcome := <-outcomeCh:
				if outcome.err != nil {
					t.Fatalf("command failed: %v", outcome.err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for command completion")
			}
		})
	}
}
