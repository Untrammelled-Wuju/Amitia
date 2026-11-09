package plugincontract

import "testing"

func TestServiceCatalogResolvesEnabledModules(t *testing.T) {
	catalog := NewServiceCatalog()
	provider := CatalogModule{
		Module:  ModuleRef{PluginID: "com.example/provider", ModuleID: "main"},
		Enabled: true,
		Services: ServiceDeclaration{Provides: []ServiceOffer{{
			ServiceID: "example.echo",
			Version:   "1.0.0",
			Scopes:    []ServiceScope{ServiceScopeGlobal},
			Placement: ServicePlacementDevice,
		}}},
	}
	consumer := CatalogModule{
		Module:  ModuleRef{PluginID: "com.example/consumer", ModuleID: "main"},
		Enabled: true,
		Services: ServiceDeclaration{Requires: []ServiceRequirement{{
			ServiceID:    "example.echo",
			VersionRange: "^1.0.0",
		}}},
	}
	if err := catalog.Replace(provider); err != nil {
		t.Fatalf("replace provider: %v", err)
	}
	if err := catalog.Replace(consumer); err != nil {
		t.Fatalf("replace consumer: %v", err)
	}
	resolution := catalog.Resolve()
	if !resolution.Valid() {
		t.Fatalf("unexpected service issues: %+v", resolution.Issues)
	}
	expected := []string{"com.example/provider/main", "com.example/consumer/main"}
	if len(resolution.Graph.ActivationOrder) != len(expected) {
		t.Fatalf("activation order: %#v", resolution.Graph.ActivationOrder)
	}
	for index, module := range expected {
		if resolution.Graph.ActivationOrder[index] != module {
			t.Fatalf("activation order[%d] = %s, want %s", index, resolution.Graph.ActivationOrder[index], module)
		}
	}
	if bindings := catalog.Bindings(consumer.Module); len(bindings) != 1 || len(bindings[0].Providers) != 1 {
		t.Fatalf("consumer bindings: %#v", bindings)
	}
}

func TestServiceCatalogDisableRemovesProvider(t *testing.T) {
	catalog := NewServiceCatalog()
	provider := CatalogModule{
		Module:  ModuleRef{PluginID: "com.example/provider", ModuleID: "main"},
		Enabled: true,
		Services: ServiceDeclaration{Provides: []ServiceOffer{{
			ServiceID: "example.echo",
			Version:   "1.0.0",
			Scopes:    []ServiceScope{ServiceScopeGlobal},
			Placement: ServicePlacementDevice,
		}}},
	}
	consumer := CatalogModule{
		Module:  ModuleRef{PluginID: "com.example/consumer", ModuleID: "main"},
		Enabled: true,
		Services: ServiceDeclaration{Requires: []ServiceRequirement{{
			ServiceID:    "example.echo",
			VersionRange: "^1.0.0",
		}}},
	}
	if err := catalog.Replace(provider); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Replace(consumer); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SetEnabled(provider.Module, false); err != nil {
		t.Fatal(err)
	}
	resolution := catalog.Resolve()
	if resolution.Valid() {
		t.Fatal("disabled provider must produce a required service issue")
	}
	if resolution.Issues[0].Code != "required_service_unavailable" {
		t.Fatalf("unexpected issue: %+v", resolution.Issues)
	}
}

func TestServiceCatalogRejectsDuplicateProviderID(t *testing.T) {
	catalog := NewServiceCatalog()
	first := CatalogModule{
		Module:  ModuleRef{PluginID: "com.example/first", ModuleID: "main"},
		Enabled: true,
		Services: ServiceDeclaration{Provides: []ServiceOffer{{
			ServiceID: "example.echo",
			Version:   "1.0.0",
			Scopes:    []ServiceScope{ServiceScopeGlobal},
			Placement: ServicePlacementDevice,
		}}},
	}
	second := first
	second.Services.Provides = append(second.Services.Provides, first.Services.Provides[0])
	if err := catalog.Replace(first); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Replace(second); err == nil {
		t.Fatal("duplicate provider id was accepted")
	}
}
