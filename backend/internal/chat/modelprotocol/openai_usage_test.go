package modelprotocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChatCacheUsage(t *testing.T) {
	for _, tc := range []struct {
		name, usage   string
		input, cached int
		reported      bool
	}{
		{"deepseek", `{"prompt_tokens":100,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}`, 100, 80, true},
		{"openai", `{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":40}}`, 100, 40, true},
		{"zero", `{"prompt_tokens":100,"prompt_cache_hit_tokens":0}`, 100, 0, true},
		{"missing", `{"prompt_tokens":100}`, 100, 0, false},
		{"null", `{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":null}}`, 100, 0, false},
		{"invalid", `{"prompt_tokens":100,"prompt_cache_hit_tokens":101}`, 100, 0, false},
		{"negative", `{"prompt_tokens":100,"prompt_cache_hit_tokens":-1}`, 100, 0, false},
		{"deepseek_total", `{"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}`, 100, 80, true},
		{"precedence", `{"prompt_tokens":100,"prompt_cache_hit_tokens":80,"prompt_tokens_details":{"cached_tokens":40}}`, 100, 80, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := (&OpenAIChatAdapter{}).parseResponse([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":` + tc.usage + `}`))
			if err != nil {
				t.Fatal(err)
			}
			if result.Usage.InputTokens != tc.input || result.Usage.CachedInputTokens != tc.cached || result.Usage.CacheUsageReported != tc.reported {
				t.Fatalf("unexpected usage: %+v", result.Usage)
			}
		})
	}
}

func TestOpenAIChatStreamCacheUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			StreamOptions struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.StreamOptions.IncludeUsage {
			t.Error("stream usage was not requested")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":5,\"total_tokens\":105,\"prompt_cache_hit_tokens\":60}}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	sink := &streamEventSink{}
	result, err := (&OpenAIChatAdapter{}).Stream(context.Background(), ProviderConfig{BaseURL: server.URL, TimeoutSeconds: 5}, ModelRequest{}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ok" || result.Usage.TotalTokens != 105 || result.Usage.CachedInputTokens != 60 || !result.Usage.CacheUsageReported {
		t.Fatalf("lost stream usage: %+v", result)
	}
	usageEvents := 0
	for _, event := range sink.events {
		if event.Type == ModelEventUsage {
			usageEvents++
			if event.Usage.CachedInputTokens != 60 {
				t.Fatalf("wrong usage event: %+v", event)
			}
		}
	}
	if usageEvents != 1 || sink.events[len(sink.events)-1].Type != ModelEventCompleted {
		t.Fatalf("unexpected events: %+v", sink.events)
	}
}

func TestOpenAIChatStreamMissingUsageIsUnknown(t *testing.T) {
	result, err := (&OpenAIChatAdapter{}).parseStream(context.Background(), strings.NewReader("data: [DONE]\n\n"), &streamEventSink{})
	if err != nil || result.Usage.CacheUsageReported {
		t.Fatalf("missing usage was treated as a miss: %+v %v", result, err)
	}
}

func TestOpenAIChatStreamFinalUsageWithoutNewline(t *testing.T) {
	body := strings.NewReader(`data: {"choices":[],"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":20}}}`)
	result, err := (&OpenAIChatAdapter{}).parseStream(context.Background(), body, &streamEventSink{})
	if err != nil || !result.Usage.CacheUsageReported || result.Usage.CachedInputTokens != 20 {
		t.Fatalf("last usage was dropped: %+v %v", result, err)
	}
}
