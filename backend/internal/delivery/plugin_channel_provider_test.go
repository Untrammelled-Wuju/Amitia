package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/serviceauth"
)

type staticChannelProviderSource struct {
	definitions []*capability.CapabilityProviderDefinition
}

func (s staticChannelProviderSource) ListByCapability(capabilityID capability.CapabilityID) []*capability.CapabilityProviderDefinition {
	if capabilityID != channelProviderCapability {
		return nil
	}
	return s.definitions
}

func TestPluginChannelProviderRegistryUsesGenericProviderMetadata(t *testing.T) {
	token, err := serviceauth.Token("com.example/channel", "channel-service")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": map[string]any{"status": "connected"}})
		case "/api/connect":
			_ = json.NewEncoder(writer).Encode(map[string]any{"connected": true})
		case "/api/messages":
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": map[string]any{"messages": []any{map[string]any{"id": "m1"}}}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	registry := NewPluginChannelProviderRegistry(staticChannelProviderSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/channel",
				ModuleID:     "channel-service",
				CapabilityID: channelProviderCapability,
				Metadata: map[string]any{
					"id": "demo-channel",
					"transport": map[string]any{
						"type":        "http",
						"defaultPort": port,
					},
				},
			},
		},
	})

	status, err := registry.StatusData(context.Background(), "demo-channel")
	if err != nil {
		t.Fatal(err)
	}
	if status["status"] != "connected" {
		t.Fatalf("unexpected status: %#v", status)
	}
	messages, err := registry.Messages(context.Background(), "demo-channel", "", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages["messages"].([]any)) != 1 {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	connected, err := registry.Connect(context.Background(), "demo-channel", nil)
	if err != nil {
		t.Fatal(err)
	}
	if connected["connected"] != true {
		t.Fatalf("unexpected connect result: %#v", connected)
	}
}

func TestPluginChannelProviderRegistryFiltersInactiveExtensions(t *testing.T) {
	registry := NewPluginChannelProviderRegistry(staticChannelProviderSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/active",
				ModuleID:     "channel-service",
				CapabilityID: channelProviderCapability,
				Metadata:     map[string]any{"channelId": "active_channel"},
			},
			{
				ExtensionID:  "com.example/inactive",
				ModuleID:     "channel-service",
				CapabilityID: channelProviderCapability,
				Metadata:     map[string]any{"channelId": "inactive_channel"},
			},
		},
	})
	registry.SetExtensionActiveChecker(func(extensionID string) bool {
		return extensionID == "com.example/active"
	})
	if !registry.Has("active_channel") {
		t.Fatal("expected active channel provider")
	}
	if registry.Has("inactive_channel") {
		t.Fatal("inactive channel provider must not be exposed")
	}
	if channels := registry.Channels(); len(channels) != 1 || channels[0] != "active_channel" {
		t.Fatalf("unexpected active channels: %#v", channels)
	}
}

func TestResolveTransportBaseURLUsesDynamicPort(t *testing.T) {
	t.Setenv("AMITIA_TEST_CHANNEL_PORT", "19880")
	baseURL, err := resolveTransportBaseURL(map[string]any{
		"type":    "http",
		"host":    "127.0.0.1",
		"portEnv": "AMITIA_TEST_CHANNEL_PORT",
	})
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != "http://127.0.0.1:19880" {
		t.Fatalf("unexpected base url: %s", baseURL)
	}
}

func TestResolveTransportBaseURLRejectsNonLoopbackHost(t *testing.T) {
	_, err := resolveTransportBaseURL(map[string]any{
		"type":        "http",
		"host":        "192.0.2.10",
		"defaultPort": 19880,
	})
	if err == nil {
		t.Fatal("expected non-loopback transport host to be rejected")
	}
}
