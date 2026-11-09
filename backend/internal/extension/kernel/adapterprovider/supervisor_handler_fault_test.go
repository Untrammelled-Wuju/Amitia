package adapterprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestSupervisorHandlerReplacesActiveAdapter(t *testing.T) {
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust:      plugincontract.TrustUserTrusted,
		AllowExternal: true,
	})
	router := plugincontract.NewContributionRouter(registry)
	handler := NewSupervisorHandler(registry, router)
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "external.remote",
		ContractVersion:    1,
		SupportedKinds:     []string{"remote_tool"},
		Operations:         []string{"test"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustUserTrusted,
	}
	if err := handler.RegisterProvider(ProviderRegistration{ServiceID: "remote-service", Descriptor: descriptor}); err != nil {
		t.Fatal(err)
	}
	first := attachTestProvider(t, handler, "first")
	defer first.Close()
	if result := callRemoteBuild(t, router, descriptor); result != "first" {
		t.Fatalf("first provider result = %s", result)
	}
	if result := callRemoteInvoke(t, router, descriptor); result != "first" {
		t.Fatalf("first provider invocation = %s", result)
	}
	second := attachTestProvider(t, handler, "second")
	defer second.Close()
	_ = first.Close()
	if result := callRemoteBuild(t, router, descriptor); result != "second" {
		t.Fatalf("replacement provider result = %s", result)
	}
	if result := callRemoteInvoke(t, router, descriptor); result != "second" {
		t.Fatalf("replacement provider invocation = %s", result)
	}
}

func TestSupervisorHandlerDrainsRegistrationFromPreviousProvider(t *testing.T) {
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust:      plugincontract.TrustUserTrusted,
		AllowExternal: true,
	})
	router := plugincontract.NewContributionRouter(registry)
	handler := NewSupervisorHandler(registry, router)
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "external.remote",
		ContractVersion:    1,
		SupportedKinds:     []string{"remote_tool"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustUserTrusted,
	}
	if err := handler.RegisterProvider(ProviderRegistration{ServiceID: "remote-service", Descriptor: descriptor}); err != nil {
		t.Fatal(err)
	}
	firstDiscard := make(chan string, 1)
	first := attachRecordedProvider(t, handler, "first", firstDiscard)
	defer first.Close()
	contribution := remoteContribution(descriptor)
	oldRegistration, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
	)
	if err != nil {
		t.Fatal(err)
	}
	secondDiscard := make(chan string, 1)
	second := attachRecordedProvider(t, handler, "second", secondDiscard)
	defer second.Close()
	if result := callRemoteBuild(t, router, descriptor); result != "second" {
		t.Fatalf("replacement provider result = %s", result)
	}
	if err := router.DeactivateAndDiscard(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
		oldRegistration,
	); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-firstDiscard:
		if id != "first" {
			t.Fatalf("old registration discarded by %s", id)
		}
	default:
		t.Fatal("old registration was not drained by its owning provider")
	}
	select {
	case id := <-secondDiscard:
		t.Fatalf("replacement provider discarded old registration %s", id)
	default:
	}
}

func TestSupervisorHandlerFailsCallsAfterProviderDisconnects(t *testing.T) {
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust:      plugincontract.TrustUserTrusted,
		AllowExternal: true,
	})
	router := plugincontract.NewContributionRouter(registry)
	handler := NewSupervisorHandler(registry, router)
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "external.remote",
		ContractVersion:    1,
		SupportedKinds:     []string{"remote_tool"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustUserTrusted,
	}
	if err := handler.RegisterProvider(ProviderRegistration{ServiceID: "remote-service", Descriptor: descriptor}); err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	transport := plugincontract.NewStdioAdapterProviderTransport(client, client)
	provider := &plugincontract.ExternalAdapterProvider{Descriptor: descriptor, Transport: transport}
	if err := provider.Register(registry); err != nil {
		t.Fatal(err)
	}
	if err := router.Register(descriptor.ID, descriptor.ContractVersion, provider); err != nil {
		t.Fatal(err)
	}
	_ = server.Close()
	_, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		plugincontract.ContributionDescriptor{
			ID:              "remote",
			Kind:            "remote_tool",
			Adapter:         descriptor.ID,
			ContractVersion: 1,
			Name:            plugincontract.LocalizedText{Default: "Remote"},
		},
	)
	if err == nil {
		t.Fatal("expected transport failure")
	}
}

func attachTestProvider(t *testing.T, handler *SupervisorHandler, id string) io.Closer {
	t.Helper()
	return attachRecordedProvider(t, handler, id, nil)
}

func attachRecordedProvider(t *testing.T, handler *SupervisorHandler, id string, discarded chan<- string) io.Closer {
	t.Helper()
	pluginStdin, hostStdin := io.Pipe()
	hostStdout, pluginStdout := io.Pipe()
	serveAdapterWithID(t, pluginStdin, pluginStdout, id, discarded)
	session, err := handler.Attach(context.Background(), trusted_service.StdioProtocolSessionMeta{
		Protocol:  "adapter-provider",
		ServiceID: "remote-service",
	}, hostStdin, hostStdout)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func serveAdapterWithID(t *testing.T, input io.Reader, output io.Writer, id string, discarded chan<- string) {
	t.Helper()
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			var request struct {
				ID     uint64          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				continue
			}
			result := map[string]any{}
			switch request.Method {
			case plugincontract.ExternalAdapterMethodHealth:
				result = map[string]any{"healthy": true}
			case plugincontract.ExternalAdapterMethodBuild:
				result = map[string]any{"id": id, "handleId": id}
			case plugincontract.ExternalAdapterMethodInvoke:
				result = map[string]any{"invocationId": id, "output": map[string]any{"provider": id}}
			case plugincontract.ExternalAdapterMethodDiscard:
				var params struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(request.Params, &params) == nil && discarded != nil {
					discarded <- params.ID
				}
			}
			response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
			if _, err := output.Write(append(response, '\n')); err != nil {
				return
			}
		}
	}()
}

func callRemoteBuild(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor) string {
	t.Helper()
	registration, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		plugincontract.ContributionDescriptor{
			ID:              "remote",
			Kind:            "remote_tool",
			Adapter:         descriptor.ID,
			ContractVersion: 1,
			Name:            plugincontract.LocalizedText{Default: "Remote"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return registration.ID()
}

func callRemoteInvoke(t *testing.T, router *plugincontract.ContributionRouter, descriptor plugincontract.AdapterDescriptor) string {
	t.Helper()
	result, err := router.InvokeAdapter(context.Background(), descriptor.ID, descriptor.ContractVersion, plugincontract.AdapterInvocationRequest{
		InvocationID:   "invoke",
		Module:         plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		ContributionID: "remote",
		Operation:      "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	return output.Provider
}

func remoteContribution(descriptor plugincontract.AdapterDescriptor) plugincontract.ContributionDescriptor {
	kind := "remote_tool"
	if len(descriptor.SupportedKinds) > 0 {
		kind = descriptor.SupportedKinds[0]
	}
	return plugincontract.ContributionDescriptor{
		ID:              "remote",
		Kind:            kind,
		Adapter:         descriptor.ID,
		ContractVersion: descriptor.ContractVersion,
		Name:            plugincontract.LocalizedText{Default: "Remote"},
	}
}
