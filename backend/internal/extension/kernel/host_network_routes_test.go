package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func TestHostNetworkRequestUsesGenericClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("X-Test") != "1" {
			t.Errorf("unexpected request: %s %s", request.Method, request.Header.Get("X-Test"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		NetworkClient: &http.Client{}, NetworkAllowPrivate: true,
	}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "network-1", RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/network"), ModuleID: "main",
		},
		Method: host_api.MethodNetworkRequest, Version: 1,
		Input: json.RawMessage(`{"url":"` + server.URL + `","method":"POST","headers":{"X-Test":"1"},"body":{"value":1}}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("network request failed: %s %+v", result.Status, result.Error)
	}
	var output struct {
		Status int            `json:"status"`
		Body   map[string]any `json:"body"`
		OK     bool           `json:"ok"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != http.StatusOK || output.Body["received"] != true || !output.OK {
		t.Fatalf("unexpected network response: %+v", output)
	}
}

func TestHostNetworkRequestRejectsPrivateTargetWhenDisabled(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{NetworkClient: &http.Client{}}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "network-private", RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/network"), ModuleID: "main",
		},
		Method: host_api.MethodNetworkRequest, Version: 1,
		Input: json.RawMessage(`{"url":"http://127.0.0.1:9/private"}`),
	})
	if result.Status == host_api.StatusSuccess {
		t.Fatal("private network target must be rejected")
	}
}

func TestPrivateNetworkGuardRejectsLoopbackAtDial(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))
	defer server.Close()
	client, err := privateNetworkGuardedHTTPClient(&http.Client{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL)
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("private network dial must fail")
	}
	if reached {
		t.Fatal("private network server was reached")
	}
}
