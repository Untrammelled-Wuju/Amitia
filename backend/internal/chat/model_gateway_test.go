package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newModelGatewayService(t *testing.T, baseURL string) *service {
	t.Helper()
	svc := setupScopedChatService(t)
	if err := svc.db.AutoMigrate(&ModelConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&ModelConfig{
		Name:            "gateway",
		APIType:         "openai",
		Protocol:        string(ProtocolOpenAIChat),
		BaseURL:         baseURL,
		APIKey:          "test-key",
		ModelName:       "active-model",
		Temperature:     0.3,
		MaxTokens:       512,
		MaxOutputTokens: 256,
		TimeoutSeconds:  30,
		IsActive:        1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestInvokeModelGatewayTranslatesMessagesAndAggregatesStream(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected authorization %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("decode body: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"choices":[{"delta":{"reasoning_content":"思考"}}]}`,
			`data: {"choices":[{"delta":{"content":"你好"}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_file","arguments":"{\"path\":"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.txt\"}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: {"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`,
			`data: [DONE]`,
		}
		if _, err := io.WriteString(w, strings.Join(chunks, "\n\n")+"\n\n"); err != nil {
			t.Errorf("write stream: %v", err)
		}
	}))
	defer server.Close()

	svc := newModelGatewayService(t, server.URL)
	temperature := 0.9
	result, err := svc.InvokeModelGateway(context.Background(), ModelGatewayRequest{
		Provider:        "amitia",
		Model:           "dsh-model",
		System:          "系统提示",
		Temperature:     &temperature,
		MaxOutputTokens: 128,
		Messages: []ModelGatewayMessage{
			{Role: "user", Content: "读文件"},
			{Role: "assistant", Content: "", ToolCalls: []ModelGatewayToolCall{{ID: "call_0", Name: "read_file", Arguments: `{"path":"旧.txt"}`}}},
			{Role: "tool", Content: "旧内容", ToolCallID: "call_0"},
		},
		Tools: []ModelGatewayTool{{Name: "read_file", Description: "读取文件", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}},
	})
	if err != nil {
		t.Fatalf("invoke model gateway: %v", err)
	}
	if result.Model != "dsh-model" {
		t.Fatalf("expected model override, got %q", result.Model)
	}
	if result.Text != "你好" {
		t.Fatalf("expected text 你好, got %q", result.Text)
	}
	if result.Reasoning != "思考" {
		t.Fatalf("expected reasoning 思考, got %q", result.Reasoning)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one aggregated tool call, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].ID != "call_1" || result.ToolCalls[0].Name != "read_file" || result.ToolCalls[0].Arguments != `{"path":"a.txt"}` {
		t.Fatalf("unexpected streamed tool call %+v", result.ToolCalls[0])
	}
	if result.FinishReason != "tool_calls" {
		t.Fatalf("expected finish reason tool_calls, got %q", result.FinishReason)
	}
	if result.Usage.TotalTokens != 18 || result.Usage.InputTokens != 11 || result.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage %+v", result.Usage)
	}

	if captured["model"] != "dsh-model" {
		t.Fatalf("expected model dsh-model in request, got %v", captured["model"])
	}
	if captured["max_tokens"] != float64(128) {
		t.Fatalf("expected max_tokens override 128, got %v", captured["max_tokens"])
	}
	if captured["temperature"] != 0.9 {
		t.Fatalf("expected temperature override 0.9, got %v", captured["temperature"])
	}
	messages, ok := captured["messages"].([]any)
	if !ok || len(messages) != 4 {
		t.Fatalf("unexpected request messages %v", captured["messages"])
	}
	first, _ := messages[0].(map[string]any)
	if first["role"] != "system" || first["content"] != "系统提示" {
		t.Fatalf("expected leading system message, got %v", first)
	}
	assistant, _ := messages[2].(map[string]any)
	calls, _ := assistant["tool_calls"].([]any)
	if len(calls) != 1 {
		t.Fatalf("expected assistant tool_calls in request, got %v", assistant)
	}
	toolMessage, _ := messages[3].(map[string]any)
	if toolMessage["tool_call_id"] != "call_0" {
		t.Fatalf("expected tool_call_id in request, got %v", toolMessage)
	}
	tools, _ := captured["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected one tool definition, got %v", captured["tools"])
	}
}

func TestInvokeModelGatewayPreservesProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := io.WriteString(w, `{"error":{"message":"invalid api key"}}`); err != nil {
			t.Errorf("write error body: %v", err)
		}
	}))
	defer server.Close()

	svc := newModelGatewayService(t, server.URL)
	_, err := svc.InvokeModelGateway(context.Background(), ModelGatewayRequest{
		Messages: []ModelGatewayMessage{{Role: "user", Content: "你好"}},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if !strings.Contains(err.Error(), "API 返回 401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("expected original provider error, got %v", err)
	}
}

func TestInvokeModelGatewayRejectsUnsupportedRole(t *testing.T) {
	svc := newModelGatewayService(t, "http://127.0.0.1:1")
	_, err := svc.InvokeModelGateway(context.Background(), ModelGatewayRequest{
		Messages: []ModelGatewayMessage{{Role: "observer", Content: "你好"}},
	})
	if err == nil || !strings.Contains(err.Error(), "observer") {
		t.Fatalf("expected unsupported role error, got %v", err)
	}
}

func TestInvokeModelGatewayRejectsToolMessageWithoutCallID(t *testing.T) {
	svc := newModelGatewayService(t, "http://127.0.0.1:1")
	_, err := svc.InvokeModelGateway(context.Background(), ModelGatewayRequest{
		Messages: []ModelGatewayMessage{{Role: "tool", Content: "结果"}},
	})
	if err == nil || !strings.Contains(err.Error(), "toolCallId") {
		t.Fatalf("expected missing toolCallId error, got %v", err)
	}
}
