package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestHostNetworkRequestClassifiesUpstreamServerError(t *testing.T) {
	body := `{"code":500,"msg":"读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused","data":null}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(body))
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
		CallID: "network-upstream", RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/network"), ModuleID: "main",
		},
		Method: host_api.MethodNetworkRequest, Version: 1,
		Input: json.RawMessage(`{"url":"` + server.URL + `","method":"GET","failOnServerError":true}`),
	})
	if result.Error == nil {
		t.Fatal("expected upstream failure")
	}
	if result.Error.Code != host_api.ErrorCodeUpstream || result.Status != host_api.StatusFailed {
		t.Fatalf("unexpected classification: %s %s", result.Status, result.Error.Code)
	}
	if !strings.Contains(result.Error.Message, "读取插件市场分类") || !strings.Contains(result.Error.Message, "connection refused") {
		t.Fatalf("original upstream message lost: %s", result.Error.Message)
	}
	if result.Error.Detail != body {
		t.Fatalf("complete upstream body not preserved: %q", result.Error.Detail)
	}
	if !errors.Is(result.Error, host_api.ErrUpstream) {
		t.Fatalf("expected ErrUpstream in chain: %v", result.Error)
	}
}

func TestHostNetworkRequestClassifiesClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"slow":true}`))
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
		CallID: "network-timeout", RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/network"), ModuleID: "main",
		},
		Method: host_api.MethodNetworkRequest, Version: 1,
		Input: json.RawMessage(`{"url":"` + server.URL + `","method":"GET","timeoutMs":20}`),
	})
	if result.Error == nil {
		t.Fatal("expected timeout failure")
	}
	if result.Error.Code != host_api.ErrorCodeTimeout || result.Status != host_api.StatusTimeout {
		t.Fatalf("unexpected timeout classification: %s %s", result.Status, result.Error.Code)
	}
	if result.Error.Message == "" {
		t.Fatal("original transport error text lost")
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
