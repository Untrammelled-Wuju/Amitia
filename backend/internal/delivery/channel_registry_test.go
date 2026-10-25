package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	basechannel "github.com/u-ai/backend/internal/channel"
)

func TestPluginChannelAdapterPassesVoiceContentAndAsset(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/send-voice" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	provider := basechannel.NewHTTPProvider(basechannel.HTTPProviderOptions{
		BaseURL:    server.URL,
		Definition: basechannel.Definition{ID: "qq_personal"},
	})
	adapter := &pluginChannelAdapter{name: "qq_personal", provider: provider}
	payload, err := json.Marshal(map[string]any{
		"content":      "角色语音",
		"originalPath": "/audio/role.mp3",
		"mimeType":     "audio/mpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Deliver(DeliveryIntent{
		ID:            "delivery-1",
		InteractionID: "conversation-1",
		Channel:       "qq_personal",
		PeerID:        "20002",
		ContentType:   "voice",
		Payload:       payload,
	}); err != nil {
		t.Fatal(err)
	}
	if body["text"] != "角色语音" || body["assetUrl"] != "/audio/role.mp3" {
		t.Fatalf("voice delivery payload incomplete: %#v", body)
	}
}
