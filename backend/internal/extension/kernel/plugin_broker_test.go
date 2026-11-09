package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestPluginBrokerResolvesDynamicServices(t *testing.T) {
	container := &Container{PluginServices: plugincontract.NewServiceCatalog()}
	broker := &PluginBroker{
		container:     container,
		contributions: make(map[string]map[string]dynamicContributionRecord),
		offers:        make(map[string][]plugincontract.ServiceOffer),
		requirements:  make(map[string][]plugincontract.ServiceRequirement),
	}
	provider := plugincontract.ModuleRef{PluginID: "com.example/provider", ModuleID: "main"}
	consumer := plugincontract.ModuleRef{PluginID: "com.example/consumer", ModuleID: "main"}
	if err := broker.ProvideService(provider, plugincontract.ServiceOffer{
		ServiceID: "example.echo",
		Version:   "1.0.0",
		Scopes:    []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement: plugincontract.ServicePlacementDevice,
	}); err != nil {
		t.Fatalf("provide service: %v", err)
	}
	providerIDs, err := broker.RequireService(consumer, plugincontract.ServiceRequirement{
		ServiceID:    "example.echo",
		VersionRange: "^1.0.0",
	})
	if err != nil {
		t.Fatalf("require service: %v", err)
	}
	if len(providerIDs) != 1 || providerIDs[0] != "com.example/provider/main/example.echo" {
		t.Fatalf("provider ids = %#v", providerIDs)
	}
	if err := broker.ReleaseService(consumer, "example.echo"); err != nil {
		t.Fatalf("release service: %v", err)
	}
	if err := broker.UnprovideService(provider, "example.echo"); err != nil {
		t.Fatalf("unprovide service: %v", err)
	}
}

func TestPluginBrokerRejectsProviderRemovalWithActiveDependency(t *testing.T) {
	container := &Container{PluginServices: plugincontract.NewServiceCatalog()}
	broker := &PluginBroker{
		container:     container,
		contributions: make(map[string]map[string]dynamicContributionRecord),
		offers:        make(map[string][]plugincontract.ServiceOffer),
		requirements:  make(map[string][]plugincontract.ServiceRequirement),
	}
	provider := plugincontract.ModuleRef{PluginID: "com.example/provider", ModuleID: "main"}
	consumer := plugincontract.ModuleRef{PluginID: "com.example/consumer", ModuleID: "main"}
	if err := broker.ProvideService(provider, plugincontract.ServiceOffer{
		ServiceID: "example.echo",
		Version:   "1.0.0",
		Scopes:    []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement: plugincontract.ServicePlacementDevice,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.RequireService(consumer, plugincontract.ServiceRequirement{
		ServiceID:    "example.echo",
		VersionRange: "^1.0.0",
	}); err != nil {
		t.Fatal(err)
	}
	if err := broker.UnprovideService(provider, "example.echo"); err == nil {
		t.Fatal("expected provider removal to be rejected while a required dependency remains")
	}
	if resolution := container.PluginServices.Resolve(); !resolution.Valid() {
		t.Fatalf("provider removal corrupted the service graph: %+v", resolution.Issues)
	}
	if err := broker.ReleaseService(consumer, "example.echo"); err != nil {
		t.Fatal(err)
	}
	if err := broker.UnprovideService(provider, "example.echo"); err != nil {
		t.Fatalf("provider removal after release: %v", err)
	}
}

func TestPluginBrokerPreservesManifestServicesDuringDynamicProvide(t *testing.T) {
	catalog := plugincontract.NewServiceCatalog()
	module := plugincontract.ModuleRef{PluginID: "com.example/provider", ModuleID: "main"}
	staticOffer := plugincontract.ServiceOffer{
		ServiceID: "example.static",
		Version:   "1.0.0",
		Scopes:    []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement: plugincontract.ServicePlacementDevice,
	}
	if err := catalog.Replace(plugincontract.CatalogModule{
		Module: module,
		Services: plugincontract.ServiceDeclaration{
			Provides: []plugincontract.ServiceOffer{staticOffer},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	key := module.Key()
	broker := &PluginBroker{
		container:     &Container{PluginServices: catalog},
		contributions: make(map[string]map[string]dynamicContributionRecord),
		offers: map[string][]plugincontract.ServiceOffer{
			key: {staticOffer},
		},
		requirements:   make(map[string][]plugincontract.ServiceRequirement),
		servicesLoaded: map[string]bool{key: true},
	}
	if err := broker.ProvideService(module, plugincontract.ServiceOffer{
		ServiceID: "example.dynamic",
		Version:   "1.0.0",
		Scopes:    []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement: plugincontract.ServicePlacementDevice,
	}); err != nil {
		t.Fatal(err)
	}
	resolution := catalog.Resolve()
	if !resolution.Valid() {
		t.Fatalf("service graph is invalid: %+v", resolution.Issues)
	}
	providers := resolution.Graph.Bindings[key]
	if len(providers) != 0 {
		t.Fatalf("unexpected consumer bindings: %+v", providers)
	}
	if len(broker.offers[key]) != 2 {
		t.Fatalf("dynamic provide replaced manifest services: %+v", broker.offers[key])
	}
}

func TestPluginBrokerRetainsFailedDynamicContributionForRetry(t *testing.T) {
	registry := plugincontract.NewAdapterRegistry(plugincontract.AdapterRegistrationPolicy{
		MinTrust: plugincontract.TrustUserTrusted,
	})
	descriptor := plugincontract.AdapterDescriptor{
		ID:                 "broker_test",
		ContractVersion:    1,
		SupportedKinds:     []string{"broker_test_kind"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              plugincontract.TrustUserTrusted,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatal(err)
	}
	router := plugincontract.NewContributionRouter(registry)
	handler := &brokerTestContributionHandler{deactivateErr: errors.New("deactivate failed")}
	if err := router.Register(descriptor.ID, descriptor.ContractVersion, handler); err != nil {
		t.Fatal(err)
	}
	broker := &PluginBroker{
		container:     &Container{PluginRoutes: router},
		contributions: make(map[string]map[string]dynamicContributionRecord),
		offers:        make(map[string][]plugincontract.ServiceOffer),
		requirements:  make(map[string][]plugincontract.ServiceRequirement),
	}
	module := plugincontract.ModuleRef{PluginID: "com.example/broker", ModuleID: "main"}
	contribution := plugincontract.ContributionDescriptor{
		ID:              "dynamic",
		Kind:            "broker_test_kind",
		Adapter:         descriptor.ID,
		ContractVersion: descriptor.ContractVersion,
		Name:            plugincontract.LocalizedText{Default: "Dynamic"},
	}
	if err := broker.RegisterContribution(context.Background(), module, contribution); err != nil {
		t.Fatal(err)
	}
	if err := broker.UnregisterContribution(context.Background(), module, contribution.ID); err == nil {
		t.Fatal("expected dynamic contribution deactivation failure")
	}
	if _, exists := broker.contributions[module.Key()][contribution.ID]; !exists {
		t.Fatal("failed deactivation removed the retryable contribution record")
	}
	handler.deactivateErr = nil
	if err := broker.UnregisterContribution(context.Background(), module, contribution.ID); err != nil {
		t.Fatalf("retry dynamic contribution unregister: %v", err)
	}
	if _, exists := broker.contributions[module.Key()][contribution.ID]; exists {
		t.Fatal("successful retry retained the dynamic contribution record")
	}
}

func TestPluginBrokerRegistersHostRoutes(t *testing.T) {
	container := &Container{PluginServices: plugincontract.NewServiceCatalog()}
	broker := NewPluginBroker(container)
	gateway := host_api.NewDefaultGateway()
	if err := registerPluginBrokerRoutes(gateway, broker); err != nil {
		t.Fatalf("register plugin broker routes: %v", err)
	}
	for _, method := range []host_api.Method{
		host_api.MethodPluginContributionRegister,
		host_api.MethodPluginContributionUnregister,
		host_api.MethodPluginServiceProvide,
		host_api.MethodPluginServiceUnprovide,
		host_api.MethodPluginServiceRequire,
		host_api.MethodPluginServiceRelease,
		host_api.MethodPluginAdapterInvoke,
		host_api.MethodPluginAdapterCancel,
	} {
		if _, ok := gateway.QueryCapability(t.Context(), method); !ok {
			t.Fatalf("route %s was not registered", method)
		}
	}
}

func TestPluginBrokerRequiresDeclaredAdapterPermissions(t *testing.T) {
	broker := NewPluginBroker(&Container{})
	module := &plugincontract.Module{
		Permissions: []plugincontract.Permission{{Name: "service.network.request"}},
	}
	if err := broker.validateAdapterPermissions(module, plugincontract.AdapterDescriptor{
		ID:                     "external.remote",
		PermissionRequirements: []string{"service.network.request"},
	}); err != nil {
		t.Fatalf("declared adapter permission rejected: %v", err)
	}
	if err := broker.validateAdapterPermissions(module, plugincontract.AdapterDescriptor{
		ID:                     "external.remote",
		PermissionRequirements: []string{"secret.read"},
	}); err == nil {
		t.Fatal("undeclared adapter permission was accepted")
	}
}

type brokerTestContributionHandler struct {
	deactivateErr error
}

type brokerTestContributionRegistration struct {
	id string
}

func (r *brokerTestContributionRegistration) ID() string {
	return r.id
}

func (h *brokerTestContributionHandler) Build(context.Context, plugincontract.ModuleRef, plugincontract.ContributionDescriptor) (plugincontract.ContributionRegistration, error) {
	return &brokerTestContributionRegistration{id: "broker-test"}, nil
}

func (h *brokerTestContributionHandler) Activate(context.Context, plugincontract.ContributionRegistration) error {
	return nil
}

func (h *brokerTestContributionHandler) Deactivate(context.Context, plugincontract.ContributionRegistration) error {
	return h.deactivateErr
}

func (h *brokerTestContributionHandler) Discard(context.Context, plugincontract.ContributionRegistration) error {
	return nil
}
