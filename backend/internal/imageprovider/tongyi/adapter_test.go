package tongyi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
)

func TestSubmitUsesDashScopeAsyncProtocol(t *testing.T) {
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/services/aigc/text2image/image-synthesis":
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("unexpected authorization: %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("X-DashScope-Async") != "enable" {
				t.Fatal("missing async header")
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["model"] != DefaultModel {
				t.Fatalf("unexpected model: %#v", body["model"])
			}
			writeTestJSON(t, w, map[string]any{
				"request_id": "submit-request",
				"output":     map[string]any{"task_id": "task-1", "task_status": "PENDING"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/task-1":
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("unexpected authorization: %s", r.Header.Get("Authorization"))
			}
			writeTestJSON(t, w, map[string]any{
				"request_id": "query-request",
				"output": map[string]any{
					"task_id":     "task-1",
					"task_status": "SUCCEEDED",
					"results":     []map[string]any{{"url": server.URL + "/images/1"}},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/images/1":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(pngData); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewWithClient(server.Client())
	adapter.pollInterval = time.Millisecond
	submission, err := adapter.Submit(context.Background(), imageprovider.ImageModelConfig{
		ApiKey:    "test-key",
		ModelName: DefaultModel,
		BaseUrl:   server.URL,
	}, imageprovider.ImageGenerationRequest{
		Prompt: "draw a tree",
		Width:  1024,
		Height: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if submission.OperationID != "task-1" || submission.Result == nil || len(submission.Result.Images) != 1 {
		t.Fatalf("unexpected submission: %#v", submission)
	}
	if submission.Result.Provider != ProviderName || submission.Result.Images[0].MimeType != "image/png" {
		t.Fatalf("unexpected result: %#v", submission.Result)
	}
}

func TestQueryPreservesDashScopeFailureMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeTestJSON(t, w, map[string]any{
			"request_id": "query-request",
			"output": map[string]any{
				"task_id":     "task-1",
				"task_status": "FAILED",
				"message":     "content policy rejected",
			},
		})
	}))
	defer server.Close()
	_, err := New().Query(context.Background(), imageprovider.ImageModelConfig{
		ApiKey:    "test-key",
		ModelName: DefaultModel,
		BaseUrl:   server.URL,
	}, "task-1")
	if err == nil || !strings.Contains(err.Error(), "content policy rejected") {
		t.Fatalf("expected original provider error, got %v", err)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Error(err)
	}
}
