package tool

import (
	"context"
	"strings"
	"testing"
)

func setupSummaryTestDB(t *testing.T) func() {
	t.Helper()
	_, _, cleanup := setupToolTestDB(t)
	for _, row := range []struct {
		role, key, value string
		importance       int
	}{
		{"char-1", "颜色", "最喜欢的颜色是蓝色", 9}, {"char-1", "音乐", "喜欢古典音乐和爵士", 7}, {"char-1", "食物", "喜欢吃辣", 6}, {"char-2", "颜色", "最喜欢绿色", 8}, {"char-1", "运动", "偶尔跑步但不常去", 4}, {"char-1", "工作", "是一名软件工程师", 9}, {"char-1", "旅行计划", "计划去日本旅行", 6},
	} {
		execution := ToolExecutionContext{CharacterID: row.role, ConversationID: "seed-chat", RequestID: "seed-" + row.role + row.key, ToolCallID: "call"}
		if _, err := currentToolMemoryTestRuntime.ExecuteMemory(t.Context(), execution, "save_memory", map[string]interface{}{"key": row.key, "value": row.value, "importance": row.importance}); err != nil {
			t.Fatal(err)
		}
	}
	return cleanup
}

func TestSummarizeMemoriesReturnsResults(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-1",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-1",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "颜色") {
		t.Fatalf("expected topic in result, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "蓝色") {
		t.Fatalf("expected memory value in result, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesHonorsScopeIsolation(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-2",
		CharacterID:    "char-2",
		Channel:        "wechat",
		RequestID:      "req-summary-2",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if strings.Contains(result.Content, "蓝色") {
		t.Fatalf("char-2 should not see char-1 memory, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "绿色") {
		t.Fatalf("char-2 should see own memory, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesEmptyTopicReturnsError(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-3",
		CharacterID:    "char-1",
		Channel:        "web",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":""}`)
	if !ok {
		if !strings.Contains(result.Content, "required") {
			t.Fatalf("expected error about required, got: %s", result.Content)
		}
	}
}

func TestSummarizeMemoriesRespectsLimit(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-4",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-4",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"喜欢","limit":2}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "找到") {
		t.Fatalf("expected summary, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesNoResultsReturnsEmptyMessage(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-5",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-5",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"不存在的主题xyz"}`)
	if !ok {
		t.Fatalf("summarize_memories should succeed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "未找到") {
		t.Fatalf("expected no results message, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesCancelledContext(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	callCtx, cancel := context.WithCancel(context.Background())
	cancel()

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-6",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-6",
	}

	result, ok := ExecuteMemoryWithContextAndCancel(callCtx, execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatal("cancelled tool should return true")
	}
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected cancelled status, got: %s", result.Status)
	}
}
