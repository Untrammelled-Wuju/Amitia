package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/mindruntime"
)

func TestModelUsageRecordsRealHTTPProviders(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	for _, provider := range []string{"openai-compatible", "anthropic", "gemini", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch provider {
				case "anthropic":
					fmt.Fprint(w, `{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":2}}`)
				case "gemini":
					fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`)
				case "ollama":
					fmt.Fprint(w, `{"message":{"content":"ok"},"prompt_eval_count":5,"eval_count":2}`)
				default:
					fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
				}
			}))
			defer server.Close()
			cfg := &ModelConfig{APIType: provider, BaseURL: server.URL, ModelName: provider, MaxTokens: 128, TimeoutSeconds: 5}
			text, tokens, err := f.model.callLLM(t.Context(), cfg, []map[string]interface{}{{"role": "user", "content": "test"}})
			if err != nil || text != "ok" || tokens != 7 {
				t.Fatalf("provider result: %s %d %v", text, tokens, err)
			}
		})
	}
	rows, _, err := store.Aggregate(t.Context(), mindruntime.UsageFilter{SpaceID: "space"}, "overview")
	if err != nil || rows[0].Calls != 4 || rows[0].Tokens != 28 || rows[0].Input != 20 || rows[0].Unknown != 0 {
		t.Fatalf("durable provider usage wrong: %+v %v", rows, err)
	}
}

func TestModelUsageStreamsDeduplicateFinalUsageAndKeepUnknown(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		n := calls.Add(1)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		if n == 1 {
			for i := 0; i < 2; i++ {
				fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":5,\"total_tokens\":105}}\n\n")
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	cfg := &ModelConfig{APIType: "openai-compatible", Protocol: "openai_chat", BaseURL: server.URL, ModelName: "stream", MaxTokens: 128, TimeoutSeconds: 5}
	for i := 0; i < 2; i++ {
		if _, err := f.model.callLLMStreamAdapter(context.Background(), cfg, []map[string]interface{}{{"role": "user", "content": "test"}}, nil, false, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := store.Aggregate(t.Context(), mindruntime.UsageFilter{SpaceID: "space"}, "overview")
	if err != nil || rows[0].Calls != 2 || rows[0].Tokens != 105 || rows[0].Unknown != 1 {
		t.Fatalf("stream usage duplicated or unknown hidden: %+v %v", rows, err)
	}
}

func TestModelUsageRecordsFailureAndCancellation(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"invalid request"}}`)
	}))
	defer failed.Close()
	cfg := &ModelConfig{APIType: "openai-compatible", BaseURL: failed.URL, ModelName: "failure", MaxTokens: 128, TimeoutSeconds: 5}
	if _, _, err := f.model.callLLM(t.Context(), cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); err == nil {
		t.Fatal("provider failure hidden")
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancelled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cancel() }))
	defer cancelled.Close()
	cfg.BaseURL = cancelled.URL
	if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); err == nil {
		t.Fatal("cancellation hidden")
	}
	rows, _, err := store.Aggregate(t.Context(), mindruntime.UsageFilter{SpaceID: "space"}, "overview")
	if err != nil || rows[0].Failed != 1 || rows[0].Cancelled != 1 || rows[0].Unknown != 2 || rows[0].Running != 0 {
		t.Fatalf("failure status wrong: %+v %v", rows, err)
	}
}
