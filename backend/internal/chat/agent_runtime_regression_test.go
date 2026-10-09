package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
)

type agentRegressionRuntime struct {
	ModelToolRuntime
	lastName string
	lastKey  string
}

func (r *agentRegressionRuntime) ExecuteModelTool(_ context.Context, name string, _ json.RawMessage, _ SkillScope, idempotencyKey string) (ToolResult, bool) {
	r.lastName = name
	r.lastKey = idempotencyKey
	return ToolResult{Status: "SUCCESS", VisibleText: "file checked"}, true
}

func TestAgentModelToolAliasesPreserveRegisteredNames(t *testing.T) {
	parameters, err := tool.ParseParametersSchema(json.RawMessage(`{"type":"object","properties":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	registered := []tool.Tool{
		{Function: tool.Function{Name: "workspace.read", Parameters: parameters}},
		{Function: tool.Function{Name: "workspace_read", Parameters: parameters}},
		{Function: tool.Function{Name: "normal-tool", Parameters: parameters}},
	}
	modelTools, aliases := prepareAgentModelTools(registered)
	if len(modelTools) != len(registered) {
		t.Fatalf("model tools = %d; want %d", len(modelTools), len(registered))
	}
	if modelTools[0].Function.Name == "workspace.read" {
		t.Fatal("invalid dotted name was exposed to an OpenAI-compatible provider")
	}
	if !validAgentModelToolName(modelTools[0].Function.Name) {
		t.Fatal("mapped name must be accepted by the model API")
	}
	if aliases[modelTools[0].Function.Name] != "workspace.read" {
		t.Fatal("model alias must resolve to original registered workspace tool")
	}
	if aliases["workspace_read"] != "workspace_read" || aliases["normal-tool"] != "normal-tool" {
		t.Fatal("valid tool names should remain unchanged")
	}
	if registered[0].Function.Name != "workspace.read" {
		t.Fatal("original registry definitions must not be mutated")
	}
}

func TestAgentLoopDispatchesMappedWorkspaceToolWithIdempotency(t *testing.T) {
	parameters, err := tool.ParseParametersSchema(json.RawMessage(`{"type":"object","properties":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	definitions := []tool.Tool{{Function: tool.Function{Name: "workspace.read", Parameters: parameters}}}
	runtime := &agentRegressionRuntime{}
	svc := &service{toolRuntime: runtime}
	calls := 0
	svc.llmWithTools = func(_ context.Context, _ *ModelConfig, _ []map[string]interface{}, modelTools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		calls++
		if calls == 1 {
			if len(modelTools) != 1 || !validAgentModelToolName(modelTools[0].Function.Name) {
				t.Fatalf("invalid model tools: %#v", modelTools)
			}
			return "", "", []map[string]interface{}{{
				"id": "call-123", "type": "function",
				"function": map[string]interface{}{"name": modelTools[0].Function.Name, "arguments": `{"uri":"workspace://README.md"}`},
			}}, 7, nil
		}
		return "文件已检查", "", nil, 11, nil
	}
	reply, _, _, tokens, _, err := svc.invokeLLMWithTools(
		context.Background(), &ModelConfig{}, nil, applog.TraceFields{}, nil,
		"", "conv", "char", "web", "req-123", "", "", "request_approval",
		nil, definitions, map[string]bool{}, context.Background(), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "文件已检查" || runtime.lastName != "workspace.read" {
		t.Fatalf("reply = %q, executed tool = %q", reply, runtime.lastName)
	}
	if runtime.lastKey != "conv:req-123:call-123" {
		t.Fatalf("idempotency key = %q", runtime.lastKey)
	}
	if tokens != 18 {
		t.Fatalf("total tokens = %d, expected sum 18", tokens)
	}
}

func TestToolResultFailureStatusWithoutStructuredError(t *testing.T) {
	for _, status := range []string{"FAILED", "TIMED_OUT", "DENIED", "CANCELLED"} {
		outcome := toolResultToOutcome(ToolResult{Status: status, VisibleText: "operation failed"}, true)
		if !outcome.HasError || !strings.HasPrefix(outcome.ErrorCode, "TOOL_") {
			t.Fatalf("status %s was reported as success: %#v", status, outcome)
		}
	}
	success := toolResultToOutcome(ToolResult{Status: "SUCCESS"}, true)
	if success.HasError {
		t.Fatalf("successful tool unexpectedly failed: %#v", success)
	}
	missing := toolResultToOutcome(ToolResult{Status: "SUCCESS"}, false)
	if !missing.HasError {
		t.Fatal("tool lookup failure must not count as success")
	}
}

func TestAgentCompactionKeepsToolCallsAndResultsTogether(t *testing.T) {
	messages := []map[string]interface{}{{"role": "system", "content": "system"}}
	for i := 1; i <= 5; i++ {
		id := string(rune('0' + i))
		messages = append(messages,
			map[string]interface{}{"role": "assistant", "content": "", "tool_calls": []map[string]interface{}{{"id": id}}},
			map[string]interface{}{"role": "tool", "tool_call_id": id, "content": strings.Repeat("output", 100)},
		)
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": "continue"})
	compacted := (&service{}).compactAgentMessages(context.Background(), &ModelConfig{ContextWindow: 100}, messages, 1)
	if len(compacted) >= len(messages) {
		t.Fatal("expected compaction on an oversized tool history")
	}
	for i, message := range compacted {
		if message["role"] != "tool" {
			continue
		}
		if i == 0 || compacted[i-1]["role"] != "assistant" && compacted[i-1]["role"] != "tool" {
			t.Fatalf("orphaned tool result at index %d", i)
		}
	}
	if len(compacted) > 2 && compacted[2]["role"] == "tool" {
		t.Fatal("compacted tail cannot begin with an orphan tool result")
	}
}
