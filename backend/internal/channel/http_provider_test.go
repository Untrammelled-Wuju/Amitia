package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPProviderDefaultHeaders(t *testing.T) {
	const token = "test-provider-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "connected", "connected": true}})
		case "/api/send":
			_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		BaseURL:    server.URL,
		Definition: Definition{ID: "test"},
		DefaultHeaders: map[string]string{
			"Authorization": "Bearer " + token,
		},
	})
	if _, err := provider.Status(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Send(context.Background(), SendRequest{PeerID: "peer", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPProviderRoutesAllMediaAndPreservesPayload(t *testing.T) {
	type captured struct {
		Path string
		Body map[string]any
	}
	requests := []captured{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, captured{Path: request.URL.Path, Body: body})
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		BaseURL:    server.URL,
		Definition: Definition{ID: "test"},
	})
	payload := func(original, content string) []byte {
		data, err := json.Marshal(map[string]any{
			"originalPath": original,
			"fallbackPath": "/fallback.bin",
			"content":      content,
			"mimeType":     "application/octet-stream",
			"altText":      "附件",
			"messageId":    "message-1",
		})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, request := range []SendRequest{
		{PeerID: "peer", ConversationID: "conversation", Text: "正文", ContentType: "text", Payload: payload("", "正文")},
		{PeerID: "peer", ContentType: "image", Payload: payload("/image.png", "")},
		{PeerID: "peer", ContentType: "voice", Payload: payload("/voice.mp3", "语音正文")},
		{PeerID: "peer", ContentType: "video", Payload: payload("/video.mp4", "")},
		{PeerID: "peer", ContentType: "file", Payload: payload("/report.pdf", "")},
	} {
		if _, err := provider.Send(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	expectedPaths := []string{"/api/send", "/api/send-image", "/api/send-voice", "/api/send-video", "/api/send-file"}
	for index, expected := range expectedPaths {
		if requests[index].Path != expected {
			t.Fatalf("request %d path = %s, want %s", index, requests[index].Path, expected)
		}
	}
	if requests[0].Body["content"] != "正文" || requests[0].Body["messageId"] != "message-1" {
		t.Fatalf("text payload not preserved: %#v", requests[0].Body)
	}
	if requests[2].Body["text"] != "语音正文" || requests[2].Body["assetUrl"] != "/voice.mp3" {
		t.Fatalf("voice payload not preserved: %#v", requests[2].Body)
	}
	if requests[4].Body["assetUrl"] != "/report.pdf" || requests[4].Body["mimeType"] != "application/octet-stream" {
		t.Fatalf("file payload not preserved: %#v", requests[4].Body)
	}
}

func TestHTTPProviderRejectsUnsupportedContentType(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{
		BaseURL:    "http://127.0.0.1:1",
		Definition: Definition{ID: "test"},
	})
	if _, err := provider.Send(context.Background(), SendRequest{
		PeerID:      "peer",
		ContentType: "unknown",
	}); err == nil {
		t.Fatal("expected unsupported content type error")
	}
}
