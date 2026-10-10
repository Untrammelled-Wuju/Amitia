package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	applog "github.com/u-ai/backend/log"
)

func TestAgentToolArgumentValidation(t *testing.T) {
	cases := []struct {
		input string
		valid bool
	}{
		{"", true},
		{"{}", true},
		{`{"query":"a"}`, true},
		{"{broken", false},
		{"[]", false},
		{"null", false},
		{"true", false},
		{strings.Repeat("a", 256*1024+1), false},
	}
	for _, tc := range cases {
		_, issue := validateAgentToolArguments(tc.input)
		if (issue == "") != tc.valid {
			t.Fatalf("args %q: validity = %v, expected %v", tc.input[:min(len(tc.input), 20)], issue == "", tc.valid)
		}
	}
}

func TestAgentModelCanRepairInvalidToolCalls(t *testing.T) {
	runtime := &agentRegressionRuntime{}
	svc := &service{toolRuntime: runtime}
	rounds := 0
	svc.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, definitions []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		rounds++
		if rounds >= 2 {
			last := messages[len(messages)-1]
			if last["role"] != "tool" || !strings.Contains(last["content"].(string), "工具执行失败") {
				if rounds < 4 {
					t.Fatalf("model did not receive structured tool failure in round %d: %v", rounds, last)
				}
			}
		}
		name := "read_file"
		args := "{}"
		if rounds == 1 {
			name = "not_advertised"
		} else if rounds == 2 {
			args = "{broken"
		} else if rounds == 4 {
			return "任务已经完成且文件已检查", "", nil, 1, nil
		}
		return "", "", []map[string]interface{}{{
			"id": "call-" + string(rune('0'+rounds)), "type": "function",
			"function": map[string]interface{}{"name": name, "arguments": args},
		}}, 1, nil
	}
	reply, _, _, _, _, err := svc.invokeLLMWithTools(
		context.Background(), &ModelConfig{}, nil, applog.TraceFields{}, nil,
		"", "conv-check", "char", "web", "req-check", "", "", "request_approval",
		nil, []tool.Tool{{Function: tool.Function{Name: "read_file"}}}, map[string]bool{}, context.Background(), testAgentLoopRecorder(t, "conv-check", "char", "req-check"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if rounds != 4 || reply == "" || runtime.lastName != "read_file" {
		t.Fatalf("invalid calls did not recover: rounds=%d reply=%q tool=%q", rounds, reply, runtime.lastName)
	}
}

func TestFailedToolResultsCannotBePresentedAsSuccess(t *testing.T) {
	outcome := toolResultToOutcome(ToolResult{
		Status: "FAILED", VisibleText: "Operation completed successfully",
	}, true)
	text := toolResultContent("execute_host_command", outcome)
	if !strings.HasPrefix(text, "工具执行失败：") {
		t.Fatalf("misleading tool output: %q", text)
	}
	unknown := toolResultToOutcome(ToolResult{Status: "UNKNOWN"}, true)
	if !unknown.HasError {
		t.Fatal("unknown execution outcome must never be reported as success")
	}
}
