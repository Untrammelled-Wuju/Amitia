package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type testChatLogsProvider struct {
	method    host_api.Method
	operation string
}

func (p *testChatLogsProvider) ExecuteChatLogs(
	_ context.Context,
	_ host_api.CallRequest,
	method host_api.Method,
	operation string,
	_ json.RawMessage,
) (any, error) {
	p.method = method
	p.operation = operation
	return map[string]any{"ok": true}, nil
}

func TestHostConversationLogsRoutesDispatchByCapability(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	provider := &testChatLogsProvider{}
	bridge := NewChatLogsBridge()
	bridge.SetProvider(provider)
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ChatLogs: bridge}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		method    host_api.Method
		operation string
	}{
		{host_api.MethodConversationLogsRead, "list"},
		{host_api.MethodConversationLogsManage, "restore"},
		{host_api.MethodConversationLogsExport, "export"},
	} {
		result := gateway.Call(context.Background(), host_api.CallRequest{
			CallID: "logs-" + string(item.method),
			RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
				InstanceID:  "runtime-1",
				ExtensionID: domain.ExtensionID("com.amitia/chat-logs"),
				ModuleID:    "chat-logs-runtime",
			},
			Method:  item.method,
			Version: 1,
			Input:   json.RawMessage(`{"operation":"` + item.operation + `","input":{}}`),
		})
		if result.Status != host_api.StatusSuccess {
			t.Fatalf("%s failed: %s %+v", item.method, result.Status, result.Error)
		}
		if provider.method != item.method || provider.operation != item.operation {
			t.Fatalf("unexpected chat logs dispatch: %s %s", provider.method, provider.operation)
		}
	}
}
