package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestToolPlannerModelRequestStopsWhenOwnedContextIsCancelled(t *testing.T) {
	for _, apiType := range []string{"openai-compatible", "ollama"} {
		t.Run(apiType, func(t *testing.T) {
			entered := make(chan struct{})
			stopServer := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(entered)
				select {
				case <-r.Context().Done():
				case <-stopServer:
				}
			}))
			defer server.Close()
			defer close(stopServer)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				_, _, err := (&service{}).callLLM(ctx, map[string]string{"apiType": apiType, "baseUrl": server.URL, "modelName": "test"}, []map[string]interface{}{{"role": "user", "content": "plan"}})
				finished <- err
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("model request did not reach provider")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("provider request ignored cancellation: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("provider request kept running after cancellation")
			}
		})
	}
}
