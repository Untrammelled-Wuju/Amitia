package kernel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
)

type pluginSearchProviderForTest struct{}

func (pluginSearchProviderForTest) ID() string { return "plugin-impl" }
func (pluginSearchProviderForTest) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{GeneralWeb: true, SearchKinds: []search.SearchKind{search.SearchKindWeb}, MaxResults: 10}
}
func (pluginSearchProviderForTest) Search(_ context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	return search.ProviderSearchResponse{Results: []search.SearchResult{{Title: "plugin result", URL: "https://example.com/plugin", Snippet: request.Query}}}, nil
}
func (pluginSearchProviderForTest) Health(context.Context) search.ProviderHealth {
	return search.ProviderHealthReady
}

func TestBuildSearchServiceRegistersImplicitNativeProvider(t *testing.T) {
	service := buildSearchService(search.Config{
		Enabled:         true,
		DefaultProvider: search.ProviderNative,
	}, nil, nil, nil)
	health := service.Health(context.Background())
	require.Equal(t, search.ProviderHealthReady, health[search.ProviderNative])
}

func TestBuildSearchServiceRegistersPluginProviderInUnifiedRegistry(t *testing.T) {
	cfg := search.DefaultConfig()
	cfg.Enabled = true
	service := buildSearchService(cfg, nil, nil, []SearchProviderRegistration{{
		InstanceID: "plugin_primary",
		Provider:   pluginSearchProviderForTest{},
		Priority:   80,
		Manifest: search.ProviderManifest{
			NetworkScopes: []string{"https://example.com"},
			CostModel:     search.CostModel{Metered: true, Unit: "credits"},
		},
	}})
	response, err := service.SearchAdvancedWithProvider(context.Background(), search.SearchRequest{Query: "amitia", Kind: search.SearchKindWeb, Limit: 5}, "invoke", "plugin_primary")
	if err != nil {
		t.Fatalf("plugin search: %v", err)
	}
	if response.Provider != "plugin_primary" || len(response.Results) != 1 {
		t.Fatalf("unexpected plugin response: %#v", response)
	}
	manifests := service.ProviderManifests()
	if len(manifests) != 1 || manifests[0].ID != "plugin_primary" || manifests[0].CostModel.Unit != "credits" {
		t.Fatalf("unexpected manifests: %#v", manifests)
	}
}
