package capability

import (
	"context"
	"testing"
)

type stubAndroidProvider struct {
	response AndroidBridgeResponse
}

func (s stubAndroidProvider) Execute(context.Context, AndroidBridgeRequest) AndroidBridgeResponse {
	return s.response
}

func (s stubAndroidProvider) Health(context.Context) HealthStatus {
	return HealthReady
}

func TestAndroidRuntimeAdapterErrorIncludesVisibleMessage(t *testing.T) {
	adapter := NewAndroidRuntimeAdapter(stubAndroidProvider{
		response: AndroidBridgeResponse{
			Status: "error",
			Error: &AndroidError{
				Code:    "VIRTUAL_DISPLAY_LAUNCH_FAILED",
				Message: "no launch activity for com.example",
			},
		},
	})
	result := adapter.Execute(
		context.Background(),
		RuntimeBinding{RuntimeType: RuntimeTypeAndroid_Native, HandlerName: "virtual_display.launch"},
		ToolInvocationContext{InvocationID: "test-native-error"},
		[]byte(`{}`),
	)
	if result.Status != ToolResultStatusFailed {
		t.Fatalf("expected failed status, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Message != "no launch activity for com.example" {
		t.Fatalf("unexpected mapped error: %+v", result.Error)
	}
	if len(result.Content) != 1 || result.Content[0].Text == "" {
		t.Fatalf("expected visible error content, got %+v", result.Content)
	}
}
