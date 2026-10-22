package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/u-ai/backend/internal/mindruntime"
)

func TestModelCacheUsageActualHTTPInference(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"prompt_cache_hit_tokens\":60,\"total_tokens\":105}}\n\ndata: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":900,"prompt_tokens_details":{"cached_tokens":900},"total_tokens":905}}`)
	}))
	defer provider.Close()
	if err := f.model.db.Model(&ModelConfig{}).Where("id = 1").Updates(map[string]any{"base_url": provider.URL, "protocol": "openai_chat"}).Error; err != nil {
		t.Fatal(err)
	}
	collector := mindruntime.DefaultMetricsCollector
	before := collector.Snapshot()
	response, err := f.engine.Run(t.Context(), f.request("cache-usage-stream"))
	if err != nil || response.Generation.Text != "ok" {
		t.Fatalf("stream inference failed: %+v %v", response, err)
	}
	cfg, err := f.model.modelForScenario("chat", 1)
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := f.model.callLLM(t.Context(), cfg, []map[string]interface{}{{"role": "user", "content": "test"}})
	if err != nil || text != "ok" {
		t.Fatalf("nonstream inference failed: %s %v", text, err)
	}
	after := collector.Snapshot()
	if after.TotalModelCalls-before.TotalModelCalls != 2 || after.ModelInputTokens-before.ModelInputTokens != 1000 || after.CachedInputTokens-before.CachedInputTokens != 960 || after.CacheUsageSamples-before.CacheUsageSamples != 2 {
		t.Fatalf("inference did not aggregate provider usage once: before=%+v after=%+v", before, after)
	}
}
