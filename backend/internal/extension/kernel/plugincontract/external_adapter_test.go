package plugincontract

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeAdapterTransport struct {
	calls []string
}

func (t *fakeAdapterTransport) Call(_ context.Context, method string, _ any, result any) error {
	t.calls = append(t.calls, method)
	switch method {
	case ExternalAdapterMethodBuild:
		return assignJSON(result, map[string]any{"id": "remote-tool", "handleId": "handle-1"})
	case ExternalAdapterMethodHealth:
		return assignJSON(result, AdapterProviderStatus{Healthy: true})
	case ExternalAdapterMethodInvoke:
		return assignJSON(result, AdapterInvocationResult{InvocationID: "invoke-1", Output: json.RawMessage(`{"ok":true}`)})
	default:
		return assignJSON(result, map[string]any{})
	}
}

func (*fakeAdapterTransport) Close() error {
	return nil
}

func assignJSON(target any, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func TestExternalAdapterProviderUsesContributionRouter(t *testing.T) {
	registry := NewAdapterRegistry(AdapterRegistrationPolicy{MinTrust: TrustVerified, AllowExternal: true})
	transport := &fakeAdapterTransport{}
	provider := &ExternalAdapterProvider{
		Descriptor: AdapterDescriptor{
			ID:                 "community.remote",
			ContractVersion:    1,
			SupportedKinds:     []string{"remote_tool"},
			Operations:         []string{"echo"},
			ContributionSchema: json.RawMessage(`{"type":"object"}`),
			Trust:              TrustVerified,
		},
		Transport: transport,
	}
	if err := provider.Register(registry); err != nil {
		t.Fatal(err)
	}
	router := NewContributionRouter(registry)
	if err := router.Register(provider.Descriptor.ID, provider.Descriptor.ContractVersion, provider); err != nil {
		t.Fatal(err)
	}
	contribution := ContributionDescriptor{
		ID:              "remote",
		Kind:            "remote_tool",
		Adapter:         provider.Descriptor.ID,
		ContractVersion: 1,
		Name:            LocalizedText{Default: "Remote"},
	}
	registration, err := router.BuildAndActivate(
		context.Background(),
		ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	invocation, err := router.InvokeAdapter(context.Background(), provider.Descriptor.ID, provider.Descriptor.ContractVersion, AdapterInvocationRequest{
		InvocationID:   "invoke-1",
		Module:         ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		ContributionID: contribution.ID,
		Operation:      "echo",
		Input:          json.RawMessage(`{"value":"hello"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.InvocationID != "invoke-1" || string(invocation.Output) != `{"ok":true}` {
		t.Fatalf("unexpected invocation: %+v", invocation)
	}
	if err := router.CancelAdapterInvocation(context.Background(), provider.Descriptor.ID, provider.Descriptor.ContractVersion, AdapterCancellationRequest{
		InvocationID: "invoke-1",
		Reason:       "test",
	}); err != nil {
		t.Fatal(err)
	}
	if err := router.DeactivateAndDiscard(
		context.Background(),
		ModuleRef{PluginID: "com.example/remote", ModuleID: "main"},
		contribution,
		registration,
	); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		ExternalAdapterMethodBuild,
		ExternalAdapterMethodActivate,
		ExternalAdapterMethodHealth,
		ExternalAdapterMethodInvoke,
		ExternalAdapterMethodCancel,
		ExternalAdapterMethodDeactivate,
		ExternalAdapterMethodDiscard,
	}
	if len(transport.calls) != len(expected) {
		t.Fatalf("calls = %#v", transport.calls)
	}
	for index := range expected {
		if transport.calls[index] != expected[index] {
			t.Fatalf("call[%d] = %s, want %s", index, transport.calls[index], expected[index])
		}
	}
}
