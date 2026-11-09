package plugincontract

import "testing"

func TestThirdPartyServiceProviderReplacesOfficialProviderByPriority(t *testing.T) {
	official := serviceProvider("official-workspace-service", "com.amitia/workspace", "workspace.files", "1.0.0", 100)
	thirdParty := serviceProvider("third-party-workspace-service", "com.example/workspace-pro", "workspace.files", "1.1.0", 300)
	consumer := ServiceConsumer{
		Module: ModuleRef{PluginID: "com.example/consumer", ModuleID: "main"},
		Requirements: []ServiceRequirement{{
			ServiceID:    "workspace.files",
			VersionRange: "^1.0.0",
		}},
	}

	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{official, thirdParty},
		Consumers: []ServiceConsumer{consumer},
	})
	if !resolution.Valid() {
		t.Fatalf("unexpected service graph issues: %+v", resolution.Issues)
	}
	bindings := resolution.Graph.Bindings[consumer.Module.Key()]
	if len(bindings) != 1 || len(bindings[0].Providers) != 1 {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
	if selected := bindings[0].Providers[0]; selected.ProviderID != thirdParty.ProviderID || selected.Module.PluginID != thirdParty.Module.PluginID {
		t.Fatalf("official provider was not replaced: %+v", selected)
	}
	if _, ok := resolution.Graph.SelectedProviders[official.ProviderID]; ok {
		t.Fatalf("official provider remained selected: %+v", resolution.Graph.SelectedProviders)
	}
	if _, ok := resolution.Graph.SelectedProviders[thirdParty.ProviderID]; !ok {
		t.Fatalf("third-party provider was not selected: %+v", resolution.Graph.SelectedProviders)
	}
}
