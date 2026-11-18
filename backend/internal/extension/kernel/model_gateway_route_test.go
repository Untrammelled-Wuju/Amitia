package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type recordingModelGateway struct {
	request ModelInvokeRequest
	result  *ModelInvokeResult
	err     error
	called  bool
}

func (g *recordingModelGateway) InvokeModelGateway(_ context.Context, request ModelInvokeRequest) (*ModelInvokeResult, error) {
	g.called = true
	g.request = request
	return g.result, g.err
}

func newModelInvokeTestGateway(t *testing.T, port ModelGatewayPort) *host_api.DefaultGateway {
	t.Helper()
	gateway := newConversationMessageTestGateway()
	bridge := NewModelGatewayBridge(port)
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ModelGateway: bridge}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	return gateway
}

func TestModelInvokeRouteUsesProviderPermission(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodModelInvoke)
	if len(permissions) != 1 {
		t.Fatalf("expected one permission, got %d", len(permissions))
	}
	if permissions[0].Name != "provider.invoke" {
		t.Fatalf("expected provider.invoke permission, got %s", permissions[0].Name)
	}
}

func TestModelInvokeRouteReturnsGatewayResult(t *testing.T) {
	port := &recordingModelGateway{result: &ModelInvokeResult{
		Provider:     "amitia",
		Model:        "dsh-model",
		Protocol:     "openai_chat",
		Text:         "你好",
		FinishReason: "stop",
		ToolCalls:    []ModelInvokeToolCall{},
	}}
	gateway := newModelInvokeTestGateway(t, port)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-model",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-runtime"},
		Method:          host_api.MethodModelInvoke,
		Version:         1,
		Input:           json.RawMessage(`{"provider":"amitia","model":"dsh-model","system":"系统","messages":[{"role":"user","content":"你好"}],"tools":[{"name":"read_file","description":"读取","parameters":{"type":"object"}}],"maxOutputTokens":64}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !port.called {
		t.Fatal("expected model gateway to be called")
	}
	if port.request.Provider != "amitia" || port.request.Model != "dsh-model" || port.request.System != "系统" {
		t.Fatalf("unexpected forwarded request %+v", port.request)
	}
	if len(port.request.Messages) != 1 || port.request.Messages[0].Content != "你好" {
		t.Fatalf("unexpected forwarded messages %+v", port.request.Messages)
	}
	if len(port.request.Tools) != 1 || port.request.Tools[0].Name != "read_file" {
		t.Fatalf("unexpected forwarded tools %+v", port.request.Tools)
	}
	if port.request.MaxOutputTokens != 64 {
		t.Fatalf("unexpected maxOutputTokens %d", port.request.MaxOutputTokens)
	}
	var output map[string]any
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if output["text"] != "你好" || output["finishReason"] != "stop" || output["model"] != "dsh-model" {
		t.Fatalf("unexpected output %v", output)
	}
}

func TestModelInvokeRoutePreservesGatewayError(t *testing.T) {
	port := &recordingModelGateway{err: errors.New("API 返回 401: {\"error\":{\"message\":\"invalid api key\"}}")}
	gateway := newModelInvokeTestGateway(t, port)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-model-error",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-runtime"},
		Method:          host_api.MethodModelInvoke,
		Version:         1,
		Input:           json.RawMessage(`{"messages":[{"role":"user","content":"你好"}]}`),
	})
	if result.Status != host_api.StatusFailed {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Message != "API 返回 401: {\"error\":{\"message\":\"invalid api key\"}}" {
		t.Fatalf("expected original error message, got %+v", result.Error)
	}
}

func TestModelInvokeRouteRejectsInvalidInput(t *testing.T) {
	port := &recordingModelGateway{}
	gateway := newModelInvokeTestGateway(t, port)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-model-invalid",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-runtime"},
		Method:          host_api.MethodModelInvoke,
		Version:         1,
		Input:           json.RawMessage(`{"messages":"broken"}`),
	})
	if result.Status != host_api.StatusFailed {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if port.called {
		t.Fatal("expected model gateway not to be called")
	}
}
