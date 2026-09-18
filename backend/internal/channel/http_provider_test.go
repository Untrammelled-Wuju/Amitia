package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPProviderDefaultHeaders(t *testing.T) {
	const token = "test-sidecar-token"
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
