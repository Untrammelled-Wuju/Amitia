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

func TestMessagesToModelRequestFiltersInvalidOpenAIToolNames(t *testing.T) {
	cfg := &ModelConfig{
		APIType:   "openai-compatible",
		ModelName: "test-model",
	}
	parameters, err := tool.ParseParametersSchema(json.RawMessage(`{"type":"object","additionalProperties":true}`))
	if err != nil {
		t.Fatalf("parse parameters: %v", err)
	}
	tools := []tool.Tool{
		{Function: tool.Function{Name: "valid_tool", Parameters: parameters}},
		{Function: tool.Function{Name: "invalid.tool", Parameters: parameters}},
		{Function: tool.Function{Name: "valid-tool", Parameters: parameters}},
	}

	request := messagesToModelRequest(cfg, nil, tools, false)
	if len(request.Tools) != 2 {
		t.Fatalf("tool count = %d, want 2", len(request.Tools))
	}
	if request.Tools[0].Name != "valid_tool" || request.Tools[1].Name != "valid-tool" {
		t.Fatalf("tool names = %q, want valid_tool and valid-tool", []string{request.Tools[0].Name, request.Tools[1].Name})
	}
	if request.Tools[0].Parameters["additionalProperties"] != true {
		t.Fatalf("additionalProperties = %v, want true", request.Tools[0].Parameters["additionalProperties"])
	}
	if _, ok := request.Tools[0].Parameters["properties"]; !ok {
		t.Fatal("properties schema is missing")
	}
}

func TestCfgToProviderConfigFallsBackToMaxTokens(t *testing.T) {
	cfg := &ModelConfig{MaxTokens: 4096}
	if got := cfgToProviderConfig(cfg).MaxOutputTokens; got != 4096 {
		t.Fatalf("max output tokens = %d, want 4096", got)
	}

	cfg = &ModelConfig{}
	if got := cfgToProviderConfig(cfg).MaxOutputTokens; got != 4096 {
		t.Fatalf("default max output tokens = %d, want 4096", got)
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
