package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type testChatImportProvider struct {
	operation string
	input     json.RawMessage
}

func (p *testChatImportProvider) ExecuteChatImport(
	_ context.Context,
	_ host_api.CallRequest,
	operation string,
	input json.RawMessage,
) (any, error) {
	p.operation = operation
	p.input = append(json.RawMessage(nil), input...)
	return map[string]any{"operation": operation}, nil
}

func TestHostImportExecuteUsesBridge(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	provider := &testChatImportProvider{}
	bridge := NewChatImportBridge()
	bridge.SetProvider(provider)
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ChatImport: bridge}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "import-execute-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "runtime-1",
			ExtensionID: domain.ExtensionID("com.amitia/chat-import"),
			ModuleID:    "chat-import-runtime",
		},
		Method:  host_api.MethodImportExecute,
		Version: 1,
		Input:   json.RawMessage(`{"operation":"parse","input":{"rawText":"甲：你好\n乙：你好"}}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("import execute failed: %s %+v", result.Status, result.Error)
	}
	if provider.operation != "parse" || string(provider.input) != `{"rawText":"甲：你好\n乙：你好"}` {
		t.Fatalf("unexpected bridge dispatch: %q %s", provider.operation, provider.input)
	}
}
