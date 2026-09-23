package capability

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSearchWebResolvesAndExecutesWithReadyProvider(t *testing.T) {
	ctx := context.Background()
	registry := NewProviderRegistry()
	provider := CapabilityProviderDefinition{
		ID:           "com.amitia.builtin.search.provider",
		CapabilityID: "search.web",
		Kind:         ProviderKindBuiltin,
		Placement:    ProviderPlacementCore,
		ExtensionID:  "com.amitia.builtin.search",
		ModuleID:     "search-runtime",
		Runtime: RuntimeBinding{
			RuntimeType: RuntimeTypeSearch,
			RuntimeID:   "search-runtime",
			HandlerName: "web.run",
		},
		Priority: 100,
	}
	if err := registry.RegisterDefinition(provider); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterInstance(CapabilityProviderInstance{
		ID:           "search-instance",
		ProviderID:   provider.ID,
		CapabilityID: provider.CapabilityID,
		Placement:    provider.Placement,
		Health:       HealthReady,
		Availability: ProviderAvailabilityAvailable,
	}); err != nil {
		t.Fatal(err)
	}

	adapters := NewRuntimeAdapterRegistry()
	adapter := NewSearchRuntimeAdapter(
		func(context.Context, string, string, ToolInvocationContext, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"operation":"search","mode":"fast","stats":{}}`), nil
		},
		func(context.Context, string) HealthStatus {
			return HealthReady
		},
	)
	adapters.Register(RuntimeTypeSearch, adapter)

	resolver := NewResolver(NewProviderCatalogAdapter(registry))
	resolver.SetRuntimeCatalog(NewRuntimeAdapterCatalogAdapter(adapters))
	resolution, err := resolver.Resolve(CapabilityResolutionRequest{
		CapabilityID: "search.web",
		AllowCore:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.HasResult() {
		t.Fatal("search.web did not resolve")
	}
	result := adapter.Execute(ctx, provider.Runtime, ToolInvocationContext{InvocationID: "inv-1"}, json.RawMessage(`{"search_query":[{"q":"probe"}],"mode":"fast"}`))
	if result.Status != ToolResultStatusSuccess {
		t.Fatalf("result = %#v", result)
	}
}
