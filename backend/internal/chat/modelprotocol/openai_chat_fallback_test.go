package modelprotocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIChatStreamingAdapterAcceptsCompleteJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": "已读取文件",
					"tool_calls": []map[string]any{{
						"id":       "call_read",
						"type":     "function",
						"function": map[string]any{"name": "read_file", "arguments": `{"path":"README.md"}`},
					}},
				},
				"finish_reason": "tool_calls",
			}},
			"usage": map[string]any{"total_tokens": 42},
		})
	}))
	defer server.Close()
	sink := &streamEventSink{}
	adapter := &OpenAIChatAdapter{}
	result, err := adapter.Stream(context.Background(), ProviderConfig{
		BaseURL: server.URL, ModelName: "test", APIKey: "mock",
	}, ModelRequest{}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "已读取文件" || len(result.ToolCalls) != 1 || result.ToolCalls[0].ID != "call_read" || result.Usage.TotalTokens != 42 {
		t.Fatalf("fallback result was incomplete: %#v", result)
	}
	if len(sink.events) < 5 {
		t.Fatalf("model event stream lost text or tool call events: %#v", sink.events)
	}
	if sink.events[len(sink.events)-1].Type != ModelEventCompleted {
		t.Fatalf("model did not report completion: %#v", sink.events)
	}
}

func TestOpenAIChatStreamingAdapterKeepsSSEStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	sink := &streamEventSink{}
	result, err := (&OpenAIChatAdapter{}).Stream(context.Background(), ProviderConfig{
		BaseURL: server.URL, ModelName: "test", APIKey: "mock",
	}, ModelRequest{}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "hello" {
		t.Fatalf("normal SSE event stream changed: %#v", result)
	}
}

func TestOpenAIChatStreamSendsSystemInstructionsToProvider(t *testing.T) {
	seenInstructions := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode provider request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(request.Messages) > 0 && request.Messages[0].Role == "system" && request.Messages[0].Content == "require verified actions" {
			seenInstructions = true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer server.Close()
	_, err := (&OpenAIChatAdapter{}).Stream(context.Background(), ProviderConfig{
		BaseURL: server.URL, ModelName: "test", APIKey: "mock",
	}, ModelRequest{Instructions: []string{"require verified actions"}}, &streamEventSink{})
	if err != nil {
		t.Fatal(err)
	}
	if !seenInstructions {
		t.Fatal("system instructions were not included in the provider request")
	}
}

func TestOpenAIChatStreamingAdapterAcceptsWhitespacePrefixedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("  \r\n\t{\"choices\":[{\"message\":{\"content\":\"whitespace response\"}}]}"))
	}))
	defer server.Close()
	sink := &streamEventSink{}
	result, err := (&OpenAIChatAdapter{}).Stream(context.Background(), ProviderConfig{
		BaseURL: server.URL, ModelName: "test", APIKey: "mock",
	}, ModelRequest{}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "whitespace response" {
		t.Fatalf("expected JSON fallback, got %#v", result)
	}
	if len(sink.events) == 0 || sink.events[len(sink.events)-1].Type != ModelEventCompleted {
		t.Fatal("normal completion event missing")
	}
}
