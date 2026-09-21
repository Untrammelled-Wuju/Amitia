package chat

import "testing"

func TestMessagesToModelRequestPreservesOpenAIToolLoopState(t *testing.T) {
	messages := []map[string]interface{}{
		{
			"role":    "assistant",
			"content": "",
			"tool_calls": []map[string]interface{}{
				{
					"id":   "call_calculate",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "calculate",
						"arguments": `{"expression":"2+2"}`,
					},
				},
			},
		},
		{
			"role":         "tool",
			"tool_call_id": "call_calculate",
			"content":      `{"result":4}`,
		},
	}

	req := messagesToModelRequest(&ModelConfig{APIType: "openai-compatible"}, messages, nil, false)
	if len(req.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(req.Messages))
	}
	if len(req.Messages[0].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls were lost: %#v", req.Messages[0])
	}
	call := req.Messages[0].ToolCalls[0]
	if call.ID != "call_calculate" || call.Name != "calculate" || call.ArgumentsJSON != `{"expression":"2+2"}` {
		t.Fatalf("assistant tool call mismatch: %#v", call)
	}
	if req.Messages[1].ToolCallID != "call_calculate" {
		t.Fatalf("tool_call_id was lost: %#v", req.Messages[1])
	}
}
