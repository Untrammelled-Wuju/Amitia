package adapterprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

func TestSupervisorHandlerAttachesAndRevokesAdapter(t *testing.T) {
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust:      plugincontract.TrustVerified,
		AllowExternal: true,
	})
	router := plugincontract.NewContributionRouter(registry)
	handler := NewSupervisorHandler(registry, router)
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "external.remote",
		ContractVersion:    1,
		SupportedKinds:     []string{"remote_tool"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustVerified,
	}
	if err := handler.RegisterProvider(ProviderRegistration{ServiceID: "remote-service", Descriptor: descriptor}); err != nil {
		t.Fatal(err)
	}
	pluginStdin, hostStdin := io.Pipe()
	hostStdout, pluginStdout := io.Pipe()
	defer pluginStdin.Close()
	defer pluginStdout.Close()
	serveAdapter(t, pluginStdin, pluginStdout)
	session, err := handler.Attach(context.Background(), trusted_service.StdioProtocolSessionMeta{
		Protocol:  "adapter-provider",
		ServiceID: "remote-service",
	}, hostStdin, hostStdout)
	if err != nil {
		t.Fatal(err)
	}
	contribution := plugincontract.ContributionDescriptor{
		ID:              "remote",
		Kind:            "remote_tool",
		Adapter:         descriptor.ID,
		ContractVersion: 1,
		Name:            plugincontract.LocalizedText{Default: "Remote"},
	}
	registration, err := router.BuildAndActivate(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := router.DeactivateAndDiscard(
		context.Background(),
		plugincontract.ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
		registration,
	); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get(descriptor.ID, 1); ok {
		t.Fatal("external adapter remained registered after session close")
	}
	if router.Has(descriptor.ID, 1) {
		t.Fatal("external adapter route remained registered after session close")
	}
}

func serveAdapter(t *testing.T, input io.Reader, output io.Writer) {
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
				result = map[string]any{"id": "remote", "handleId": "handle-1"}
			}
			response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
			if _, err := output.Write(append(response, '\n')); err != nil {
				return
			}
		}
	}()
}
