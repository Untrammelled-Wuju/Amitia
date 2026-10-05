package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedToolTestRuntime struct {
	execute func(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error)
	calls   int
}

func (r *ownedToolTestRuntime) Tools(context.Context, business.Inference) ([]tool.Tool, error) {
	return []tool.Tool{{Type: "function", Function: tool.Function{Name: "device_action", Parameters: tool.Parameters{Type: "object"}}}}, nil
}

func (r *ownedToolTestRuntime) Execute(ctx context.Context, inference business.Inference, id, name string, input json.RawMessage) (ToolResult, error) {
	r.calls++
	if r.execute != nil {
		return r.execute(ctx, inference, id, name, input)
	}
	return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"done":true}`)}, nil
}

func ownedToolCall(name string) []map[string]interface{} {
	return []map[string]interface{}{{"id": "call-one", "type": "function", "function": map[string]interface{}{"name": name, "arguments": "{}"}}}
}

func TestOwnedToolLoopUsesCoreModelAndKeepsAuthoritativeTargetScope(t *testing.T) {
	inference := business.Inference{Scope: coordination.ExecutionScope{CoreID: "core-c", InitiatorDeviceID: "device-a", TargetDeviceID: "device-b", RoleID: "b-role", ResourceOwnerID: "device-b", ExecutionID: "execution"}}
	runtime := &ownedToolTestRuntime{execute: func(_ context.Context, current business.Inference, id, name string, input json.RawMessage) (ToolResult, error) {
		if current.Scope != inference.Scope || id != "call-one" || name != "device_action" || string(input) != "{}" {
			t.Fatal("tool invocation lost its authoritative scope")
		}
		return ToolResult{Status: "SUCCESS", Output: json.RawMessage(`{"done":true}`)}, nil
	}}
	s := &service{ownedToolRuntime: runtime}
	var rounds int
	s.llmWithTools = func(_ context.Context, cfg *ModelConfig, messages []map[string]interface{}, definitions []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		if cfg.ID != 42 || len(definitions) != 1 {
			t.Fatal("device model or unapproved catalog selected")
		}
		rounds++
		if rounds == 1 {
			return "", "", ownedToolCall("device_action"), 2, nil
		}
		if len(messages) != 3 || messages[2]["role"] != "tool" || messages[2]["content"] != `{"done":true}` {
			t.Fatal("tool result was not returned to Core model")
		}
		return "动作完成", "", nil, 3, nil
	}
	result, err := s.generateOwnedWithTools(t.Context(), inference, &ModelConfig{ID: 42}, []map[string]interface{}{{"role": "user", "content": "执行"}}, &ownedReplySink{inference: inference})
	if err != nil || result.Text != "动作完成" || result.Tokens != 5 || runtime.calls != 1 {
		t.Fatalf("owned tool execution failed: %+v %v", result, err)
	}
}

func TestOwnedToolLoopRejectsDuplicateUnknownAndCancelledActions(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "duplicate", "unknown", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			runtime := &ownedToolTestRuntime{}
			if scenario == "unknown" {
				runtime.execute = func(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error) {
					return ToolResult{Status: "UNKNOWN"}, nil
				}
			}
			if scenario == "cancelled" {
				runtime.execute = func(context.Context, business.Inference, string, string, json.RawMessage) (ToolResult, error) {
					cancel()
					return ToolResult{Status: "SUCCESS"}, nil
				}
			}
			s := &service{ownedToolRuntime: runtime}
			var rounds int
			s.llmWithTools = func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
				rounds++
				name := "device_action"
				if scenario == "unauthorized" {
					name = "unlisted_action"
				}
				return "", "", ownedToolCall(name), 1, nil
			}
			result, err := s.generateOwnedWithTools(ctx, business.Inference{}, &ModelConfig{}, nil, &ownedReplySink{})
			if err == nil || !result.Partial || runtime.calls > 1 {
				t.Fatalf("unsafe action accepted or repeated: %+v %v calls=%d", result, err, runtime.calls)
			}
			if scenario == "unauthorized" && runtime.calls != 0 {
				t.Fatal("unlisted action executed")
			}
			if scenario == "unknown" && !errors.Is(err, business.ErrUncertainExecution) {
				t.Fatal("unknown action result retried")
			}
			if scenario == "cancelled" && (rounds != 1 || !errors.Is(err, context.Canceled)) {
				t.Fatal("cancelled provider continued model loop")
			}
		})
	}
}
