package capability

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBuiltinRuntimeAdapterPreservesStructuredOutput(t *testing.T) {
	adapter := NewBuiltinRuntimeAdapter(func(_ context.Context, _ string, _ json.RawMessage, _ ToolInvocationContext) (json.RawMessage, error) {
		return json.RawMessage(`{"categories":[{"slug":"developer"}],"count":1}`), nil
	})
	result := adapter.Execute(context.Background(), RuntimeBinding{RuntimeType: RuntimeTypeBuiltin}, ToolInvocationContext{InvocationID: "inv-1"}, json.RawMessage(`{}`))
	if result.Status != ToolResultStatusSuccess {
		t.Fatalf("status = %s", result.Status)
	}
	if string(result.Structured) != `{"categories":[{"slug":"developer"}],"count":1}` {
		t.Fatalf("structured output = %s", result.Structured)
	}
	if len(result.Content) != 1 || result.Content[0].Text != string(result.Structured) {
		t.Fatalf("content = %+v", result.Content)
	}
}
