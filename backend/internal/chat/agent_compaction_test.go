package chat

import (
	"context"
	"strings"
	"testing"
)

func TestAgentCompactionBoundaryPreservesMultiResultToolGroup(t *testing.T) {
	messages := []map[string]interface{}{
		{"role": "system", "content": "system"},
		{"role": "assistant", "tool_calls": []map[string]interface{}{{"id": "a"}}},
		{"role": "tool", "tool_call_id": "a", "content": "result-a"},
		{"role": "assistant", "content": "working"},
		{"role": "assistant", "tool_calls": []map[string]interface{}{{"id": "b"}, {"id": "c"}, {"id": "d"}}},
		{"role": "tool", "tool_call_id": "b", "content": "result-b"},
		{"role": "tool", "tool_call_id": "c", "content": "result-c"},
		{"role": "tool", "tool_call_id": "d", "content": "result-d"},
		{"role": "user", "content": "continue"},
	}
	for _, boundary := range []int{5, 6, 7} {
		if actual := agentCompactionBoundary(messages, 1, boundary); actual != 4 {
			t.Fatalf("boundary %d split a multi-result tool group; actual %d", boundary, actual)
		}
	}
	if actual := agentCompactionBoundary(messages, 1, 8); actual != 8 {
		t.Fatalf("user message should remain a valid boundary: %d", actual)
	}
	if actual := agentCompactionBoundary(messages, 1, 4); actual != 4 {
		t.Fatalf("assistant tool-call start should be a valid boundary: %d", actual)
	}
}

func TestAgentCompactionSummaryRecordsToolCallsAndResults(t *testing.T) {
	messages := []map[string]interface{}{{"role": "system", "content": "system"}}
	for i := 0; i < 8; i++ {
		id := string(rune('a' + i))
		messages = append(messages,
			map[string]interface{}{"role": "assistant", "tool_calls": []map[string]interface{}{{"id": id, "function": map[string]interface{}{"name": "workspace.read", "arguments": "{}"}}}},
			map[string]interface{}{"role": "tool", "tool_call_id": id, "content": "result-" + id},
		)
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": "continue"})
	compacted := (&service{}).compactAgentMessages(context.Background(), &ModelConfig{ContextWindow: 100}, messages, 1)
	if len(compacted) >= len(messages) {
		t.Fatal("expected compaction with a small model context")
	}
	summary, ok := compacted[1]["content"].(string)
	if compacted[1]["role"] != "user" || !strings.Contains(summary, "<untrusted_tool_history>") {
		t.Fatalf("tool history must not be promoted into high-priority system instructions: %#v", compacted[1])
	}
	if !ok || !strings.Contains(summary, "tool_calls=") || !strings.Contains(summary, "tool_call_id=") {
		t.Fatalf("tool invocation and output evidence was lost during compaction: %.200s", summary)
	}
	if len(compacted) > 2 && compacted[2]["role"] == "tool" {
		t.Fatal("compaction produced an orphan tool result")
	}
}
