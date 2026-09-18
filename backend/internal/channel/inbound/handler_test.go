package inbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/serviceauth"
	"github.com/u-ai/backend/internal/interaction"
)

type staticProviderDefinitionSource struct {
	definitions []*capability.CapabilityProviderDefinition
}

func (s staticProviderDefinitionSource) ListByCapability(capabilityID capability.CapabilityID) []*capability.CapabilityProviderDefinition {
	if capabilityID != inboundProviderCapability {
		return nil
	}
	return s.definitions
}

type captureUnifiedEntry struct {
	request *interaction.UnifiedEntryRequest
}

func (e *captureUnifiedEntry) Handle(_ context.Context, request *interaction.UnifiedEntryRequest) (*interaction.OrchestrationResult, error) {
	e.request = request
	return &interaction.OrchestrationResult{
		InteractionID: "interaction-1",
		Outcome:       interaction.OutcomeCompleted,
	}, nil
}

func TestInboundHandlerRoutesGenericTextMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entry := &captureUnifiedEntry{}
	router := gin.New()
	RegisterInboundRouter(router, staticProviderDefinitionSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/channel",
				ModuleID:     "channel-service",
				CapabilityID: inboundProviderCapability,
				Metadata: map[string]any{
					"labels": map[string]any{"channelId": "telegram"},
				},
			},
		},
	}, entry)

	response := performInboundRequest(t, router, "com.example/channel", "channel-service", map[string]any{
		"channelId":      "telegram",
		"accountId":      "bot-1",
		"conversationId": "chat-1",
		"peerId":         "user-1",
		"messageId":      "message-1",
		"contentType":    "text",
		"text":           "hello",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if entry.request == nil {
		t.Fatal("unified entry was not called")
	}
	if entry.request.Channel != "telegram" || entry.request.PeerID != "user-1" || entry.request.Message != "hello" {
		t.Fatalf("unexpected unified entry request: %#v", entry.request)
	}
	if entry.request.ConversationID != "chat-1" || entry.request.SessionID != "bot-1" {
		t.Fatalf("unexpected conversation routing: %#v", entry.request)
	}
	if entry.request.RequestID != "com.example/channel:channel-service:telegram:message-1" {
		t.Fatalf("unexpected request id: %s", entry.request.RequestID)
	}
}

func TestInboundHandlerMapsMediaMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entry := &captureUnifiedEntry{}
	router := gin.New()
	RegisterInboundRouter(router, staticProviderDefinitionSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/channel",
				ModuleID:     "channel-service",
				CapabilityID: inboundProviderCapability,
				Metadata:     map[string]any{"channelId": "feishu"},
			},
		},
	}, entry)

	response := performInboundRequest(t, router, "com.example/channel", "channel-service", map[string]any{
		"channelId":   "feishu",
		"peerId":      "user-2",
		"messageId":   "message-2",
		"contentType": "audio",
		"audioUrl":    "https://example.invalid/voice.ogg",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if entry.request.AudioUrl != "https://example.invalid/voice.ogg" || !entry.request.VoiceMessage {
		t.Fatalf("unexpected media request: %#v", entry.request)
	}
}

func TestInboundHandlerRejectsInvalidCredentialAndProviderIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providers := staticProviderDefinitionSource{
		definitions: []*capability.CapabilityProviderDefinition{
			{
				ExtensionID:  "com.example/channel",
				ModuleID:     "channel-service",
				CapabilityID: inboundProviderCapability,
				Metadata:     map[string]any{"channelId": "telegram"},
			},
		},
	}
	router := gin.New()
	RegisterInboundRouter(router, providers, &captureUnifiedEntry{})

	request := httptest.NewRequest(http.MethodPost, "/api/channels/inbound", strings.NewReader(`{"channelId":"telegram","peerId":"user-1","text":"hello"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Authorization", "Bearer invalid")
	request.Header.Set("X-Amitia-Extension-ID", "com.example/channel")
	request.Header.Set("X-Amitia-Module-ID", "channel-service")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected invalid credential status: %d", response.Code)
	}

	response = performInboundRequest(t, router, "com.example/other", "channel-service", map[string]any{
		"channelId": "telegram",
		"peerId":    "user-1",
		"text":      "hello",
	})
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected provider mismatch status: %d", response.Code)
	}
}

func TestInboundHandlerRejectsNonLoopbackCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := serviceauth.Token("com.example/channel", "channel-service")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterInboundRouter(router, staticProviderDefinitionSource{}, &captureUnifiedEntry{})
	request := httptest.NewRequest(http.MethodPost, "/api/channels/inbound", strings.NewReader(`{"channelId":"telegram","peerId":"user-1","text":"hello"}`))
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Amitia-Extension-ID", "com.example/channel")
	request.Header.Set("X-Amitia-Module-ID", "channel-service")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected non-loopback status: %d", response.Code)
	}
}

func performInboundRequest(t *testing.T, router http.Handler, extensionID, moduleID string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	token, err := serviceauth.Token(extensionID, moduleID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/channels/inbound", strings.NewReader(string(body)))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Amitia-Extension-ID", extensionID)
	request.Header.Set("X-Amitia-Module-ID", moduleID)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
