package chat

import (
	"context"
	"encoding/json"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
)

type dynamicRoundToolRuntime struct {
	ModelToolRuntime
	calls atomic.Int32
}

func (r *dynamicRoundToolRuntime) ExecuteModelTool(context.Context, string, json.RawMessage, SkillScope, string) (ToolResult, bool) {
	r.calls.Add(1)
	return ToolResult{Status: "SUCCESS", VisibleText: "ok", Output: json.RawMessage(`{"ok":true}`)}, true
}

func TestInvokeLLMWithToolsTracksReasoningDuration(t *testing.T) {
	svc := &service{}
	svc.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		time.Sleep(30 * time.Millisecond)
		return "回复", "思考", nil, 12, nil
	}

	reply, reasoning, forceVoice, tokens, durationMS, err := svc.invokeLLMWithTools(
		context.Background(),
		&ModelConfig{},
		nil,
		applog.TraceFields{},
		nil,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		nil,
		nil,
		nil,
		context.Background(),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "回复" || reasoning != "思考" {
		t.Fatalf("unexpected reply or reasoning: %q %q", reply, reasoning)
	}
	if forceVoice {
		t.Fatal("expected forceVoice to be false")
	}
	if tokens != 12 {
		t.Fatalf("expected 12 tokens, got %d", tokens)
	}
	if durationMS <= 0 {
		t.Fatalf("expected positive reasoning duration, got %d", durationMS)
	}
}

func TestInvokeLLMWithToolsContinuesUntilFinalAnswer(t *testing.T) {
	runtime := &dynamicRoundToolRuntime{}
	svc := &service{toolRuntime: runtime}
	modelCalls := 0
	svc.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		modelCalls++
		if modelCalls <= 4 {
			return "", "", []map[string]interface{}{{
				"id":   "call-" + strconv.Itoa(modelCalls),
				"type": "function",
				"function": map[string]interface{}{
					"name":      "read_file",
					"arguments": `{"path":"file-` + strconv.Itoa(modelCalls) + `.txt"}`,
				},
			}}, 3, nil
		}
		return "最终回答", "", nil, 4, nil
	}

	reply, _, _, _, _, err := svc.invokeLLMWithTools(
		context.Background(),
		&ModelConfig{},
		nil,
		applog.TraceFields{},
		nil,
		"",
		"conv",
		"char",
		"web",
		"req",
		"",
		"",
		"request_approval",
		nil,
		nil,
		map[string]bool{},
		context.Background(),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "最终回答" {
		t.Fatalf("expected final answer, got %q", reply)
	}
	if modelCalls != 5 {
		t.Fatalf("expected five model rounds, got %d", modelCalls)
	}
	if runtime.calls.Load() != 4 {
		t.Fatalf("expected four tool executions, got %d", runtime.calls.Load())
	}
}
