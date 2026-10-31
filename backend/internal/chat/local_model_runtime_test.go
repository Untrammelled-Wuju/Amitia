package chat

import (
	"strings"
	"testing"
)

func TestFitEmbeddedModelMessagesPreservesSystemTriggerAndBoundedContext(t *testing.T) {
	messages := []map[string]interface{}{
		{"role": "system", "content": strings.Repeat("系统指令", 8000)},
		{"role": "user", "content": "<owned_context_data>" + strings.Repeat("上下文", 8000) + "</owned_context_data>"},
		{"role": "assistant", "content": strings.Repeat("旧回复", 2000)},
		{"role": "user", "content": "只回复 OK"},
	}
	fitted := fitEmbeddedModelMessages(messages, 2048)
	if estimateModelMessagesTokens(fitted) > 2048 {
		t.Fatalf("fitted messages exceed budget: %d", estimateModelMessagesTokens(fitted))
	}
	if len(fitted) < 2 || fitted[0]["role"] != "system" || fitted[len(fitted)-1]["content"] != "只回复 OK" {
		t.Fatalf("critical messages were not preserved: %#v", fitted)
	}
	joined := ""
	for _, message := range fitted {
		joined += messageStringContent(message)
	}
	if !strings.Contains(joined, "上下文") || !strings.Contains(joined, "因本地模型上下文限制已截断") {
		t.Fatalf("context or truncation marker missing: %q", joined)
	}
}

func TestEmbeddedModelInputLimitReadsProviderConfig(t *testing.T) {
	cfg := &ModelConfig{
		ContextWindow:      8192,
		MaxOutputTokens:    512,
		ProviderConfigJSON: `{"contextSize":32768}`,
	}
	if got := embeddedModelInputLimit(cfg); got != 22579 {
		t.Fatalf("input limit = %d", got)
	}
}

func messageStringContent(message map[string]interface{}) string {
	value, _ := message["content"].(string)
	return value
}
