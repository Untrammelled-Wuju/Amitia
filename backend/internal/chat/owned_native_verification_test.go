package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

type ownedNativeVerificationRuntime struct {
	status string
	calls  int
}

func (r *ownedNativeVerificationRuntime) Tools(context.Context, business.Inference) ([]tool.Tool, error) {
	var definitions []tool.Tool
	for _, name := range []string{"harness_multi_agent_delegate", "harness_multi_agent_status", "harness_multi_agent_wait", "device_action"} {
		definitions = append(definitions, tool.Tool{Type: "function", Function: tool.Function{Name: name, Parameters: tool.Parameters{Type: "object"}}})
	}
	return definitions, nil
}

func (r *ownedNativeVerificationRuntime) Execute(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error) {
	r.calls++
	output, _ := json.Marshal(map[string]any{"coordinationId": "coord-real-shape", "status": r.status, "verification": "requires_parent_review", "assignments": []any{map[string]any{"workerRole": "coding_worker", "status": r.status}}, "verificationSummary": map[string]any{"requiresParentReview": true}})
	return ToolResult{Status: "SUCCESS", Output: output}, nil
}

func TestOwnedNativeTaskResultsCannotBecomeFalseCompletion(t *testing.T) {
	for _, name := range []string{"harness_multi_agent_delegate", "harness_multi_agent_status", "harness_multi_agent_wait"} {
		for _, status := range []string{"running", "succeeded", "failed", "cancelled"} {
			t.Run(name+"/"+status, func(t *testing.T) {
				runtime := &ownedNativeVerificationRuntime{status: status}
				var streamed strings.Builder
				inference := business.Inference{Emit: func(event business.Event) error { streamed.WriteString(event.Text); return nil }}
				s := &service{ownedToolRuntime: runtime}
				rounds := 0
				s.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
					rounds++
					if rounds == 1 {
						return "", "", ownedToolCall(name), 1, nil
					}
					return "全部代码已经完成并已验证合并", "所有工作已完成", nil, 2, nil
				}
				result, err := s.generateOwnedWithTools(t.Context(), inference, &ModelConfig{}, nil, &ownedReplySink{inference: inference})
				if err != nil || result.Partial || runtime.calls != 1 || rounds != 2 {
					t.Fatalf("native async result failed: %+v %v calls=%d rounds=%d", result, err, runtime.calls, rounds)
				}
				if strings.Contains(result.Text, "已经完成并已验证合并") || strings.Contains(streamed.String(), "已经完成并已验证合并") || strings.Contains(streamed.String(), "所有工作已完成") {
					t.Fatalf("unverified claim escaped: %+v stream=%s", result, streamed.String())
				}
				if status == "running" && !strings.Contains(result.Text, "后台执行") {
					t.Fatalf("pending task not identified: %s", result.Text)
				}
				if status == "succeeded" && !strings.Contains(result.Text, "审核") {
					t.Fatalf("worker success bypassed parent review: %s", result.Text)
				}
				if !strings.Contains(result.Text, "coord-real-shape") {
					t.Fatalf("task identity omitted: %s", result.Text)
				}
			})
		}
	}
}

func TestOwnedInternalWorkerCannotRecursivelyDelegate(t *testing.T) {
	runtime := &ownedNativeVerificationRuntime{status: "running"}
	s := &service{ownedToolRuntime: runtime}
	s.llmWithTools = func(_ context.Context, _ *ModelConfig, _ []map[string]interface{}, definitions []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		for _, definition := range definitions {
			if ownedNativeTaskTool(definition.Function.Name) {
				t.Fatalf("native delegation leaked to internal worker: %s", definition.Function.Name)
			}
		}
		return "", "", ownedToolCall("harness_multi_agent_delegate"), 1, nil
	}
	result, err := s.generateOwnedWithTools(t.Context(), business.Inference{InternalWorker: true}, &ModelConfig{}, nil, &ownedReplySink{})
	if err == nil || !result.Partial || runtime.calls != 0 {
		t.Fatalf("recursive delegation executed: %+v %v calls=%d", result, err, runtime.calls)
	}
}

func TestOwnedOrdinaryChatKeepsStreamingWithNativeCatalog(t *testing.T) {
	runtime := &ownedNativeVerificationRuntime{}
	streamed := false
	inference := business.Inference{Emit: func(event business.Event) error {
		if event.Text == "普通流式回复" {
			streamed = true
		}
		return nil
	}}
	s := &service{ownedToolRuntime: runtime}
	s.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return "普通流式回复", "", nil, 1, nil
	}
	result, err := s.generateOwnedWithTools(t.Context(), inference, &ModelConfig{}, nil, &ownedReplySink{inference: inference})
	if err != nil || result.Text != "普通流式回复" || !streamed || runtime.calls != 0 {
		t.Fatalf("ordinary streaming changed: %+v %v stream=%v", result, err, streamed)
	}
}
