package capability

import (
	"context"
	"encoding/json"
	"testing"
)

func TestJavaScriptRuntimeInjectsTrustedInvocationContext(t *testing.T) {
	var received map[string]any
	adapter := NewJavaScriptRuntimeAdapter(func(_ context.Context, _, _ string, _ string, input json.RawMessage, _ ToolInvocationContext) (json.RawMessage, error) {
		if err := json.Unmarshal(input, &received); err != nil {
			return nil, err
		}
		return json.RawMessage(`{"ok":true}`), nil
	}, nil)
	result := adapter.Execute(context.Background(), RuntimeBinding{
		RuntimeType: RuntimeTypePluginJS,
		Metadata: map[string]any{
			"extensionId": "com.example/plugin",
			"moduleId":    "plugin-runtime",
		},
		HandlerName: "send",
	}, ToolInvocationContext{
		InvocationID:   "inv-1",
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		Channel:        "qq_personal",
		SessionID:      "session-1",
		TraceID:        "trace-1",
		OperationID:    "operation-1",
		ExecutionTarget: InvocationExecutionTarget{
			Placement: "device",
			DeviceID:  "device-1",
		},
	}, json.RawMessage(`{"action":"send","emoteId":"emote-1"}`))
	if result.Status != ToolResultStatusSuccess {
		t.Fatalf("execution failed: %+v", result)
	}
	if string(result.Structured) != `{"ok":true}` {
		t.Fatalf("structured result missing: %s", result.Structured)
	}
	invocation, _ := received["__amitiaContext"].(map[string]any)
	if invocation["conversationId"] != "conversation-1" || invocation["channel"] != "qq_personal" || invocation["characterId"] != "character-1" {
		t.Fatalf("invocation context missing or incorrect: %+v", received)
	}
}

func TestJavaScriptRuntimeKeepsPlainTextUnstructured(t *testing.T) {
	adapter := NewJavaScriptRuntimeAdapter(func(context.Context, string, string, string, json.RawMessage, ToolInvocationContext) (json.RawMessage, error) {
		return json.RawMessage(`plain result`), nil
	}, nil)
	result := adapter.Execute(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypePluginJS}, ToolInvocationContext{InvocationID: "inv-2"}, nil)
	if result.Status != ToolResultStatusSuccess || len(result.Structured) != 0 {
		t.Fatalf("plain text was incorrectly structured: %+v", result)
	}
}
