package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestPluginServiceSyncBlocksProviderDisableWithRequiredConsumer(t *testing.T) {
	catalog := plugincontract.NewServiceCatalog()
	provider := synchronizedServiceProviderManifest()
	consumer := synchronizedServiceConsumerManifest()
	if err := setPluginContractServices(catalog, provider, true); err != nil {
		t.Fatal(err)
	}
	if err := setPluginContractServices(catalog, consumer, true); err != nil {
		t.Fatal(err)
	}
	if err := setPluginContractServices(catalog, provider, false); err == nil {
		t.Fatal("expected required service consumer to block provider disable")
	}
	if resolution := catalog.Resolve(); !resolution.Valid() {
		t.Fatalf("failed provider disable corrupted the service graph: %+v", resolution.Issues)
	}
	if err := setPluginContractServices(catalog, consumer, false); err != nil {
		t.Fatal(err)
	}
	if err := setPluginContractServices(catalog, provider, false); err != nil {
		t.Fatalf("provider disable after consumer: %v", err)
	}
}

func TestPluginServiceSyncRollsBackMissingDependencyActivation(t *testing.T) {
	catalog := plugincontract.NewServiceCatalog()
	consumer := synchronizedServiceConsumerManifest()
	if err := setPluginContractServices(catalog, consumer, true); err == nil {
		t.Fatal("expected missing required service dependency")
	}
	if _, exists := catalog.Get(plugincontract.ModuleRef{
		PluginID: consumer.Plugin.ID,
		ModuleID: consumer.Modules[0].ID,
	}); exists {
		t.Fatal("failed activation retained the consumer module")
	}
}

func TestPluginServiceSyncBlocksProviderRemovalWithRequiredConsumer(t *testing.T) {
	catalog := plugincontract.NewServiceCatalog()
	provider := synchronizedServiceProviderManifest()
	consumer := synchronizedServiceConsumerManifest()
	if err := setPluginContractServices(catalog, provider, true); err != nil {
		t.Fatal(err)
	}
	if err := setPluginContractServices(catalog, consumer, true); err != nil {
		t.Fatal(err)
	}
	if err := removePluginContractServices(catalog, provider); err == nil {
		t.Fatal("expected required service consumer to block provider removal")
	}
	if resolution := catalog.Resolve(); !resolution.Valid() {
		t.Fatalf("failed provider removal corrupted the service graph: %+v", resolution.Issues)
	}
}

func synchronizedServiceProviderManifest() *plugincontract.Manifest {
	return &plugincontract.Manifest{
		Plugin: plugincontract.PluginMeta{ID: "com.example/provider"},
		Modules: []plugincontract.Module{{
			ID: "main",
			Services: plugincontract.ServiceDeclaration{
				Provides: []plugincontract.ServiceOffer{{
					ServiceID: "example.echo",
					Version:   "1.0.0",
					Scopes:    []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
					Placement: plugincontract.ServicePlacementDevice,
				}},
			},
		}},
	}
}

func synchronizedServiceConsumerManifest() *plugincontract.Manifest {
	return &plugincontract.Manifest{
		Plugin: plugincontract.PluginMeta{ID: "com.example/consumer"},
		Modules: []plugincontract.Module{{
			ID: "main",
			Services: plugincontract.ServiceDeclaration{
				Requires: []plugincontract.ServiceRequirement{{
					ServiceID:    "example.echo",
					VersionRange: "^1.0.0",
				}},
			},
		}},
	}
}
