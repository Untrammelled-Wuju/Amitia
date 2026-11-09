package plugincontract

import (
	"encoding/json"
	"testing"
)

const validManifest = `{
  "schemaVersion": 1,
  "plugin": {
    "id": "com.example.plugin",
    "name": {"default": "Plugin"},
    "description": {"default": "Plugin"},
    "version": "1.0.0",
    "publisher": {"id": "com.example", "displayName": "Example", "trustLevel": "verified"}
  },
  "compatibility": {"minHostVersion": "1.0.0", "platforms": ["linux"]},
  "modules": [{
    "id": "main",
    "name": {"default": "Main"},
    "runtime": {"adapter": "amitia.javascript", "entry": "dist/index.js"},
    "services": {
      "provides": [{
        "serviceId": "example.service",
        "version": "1.0.0",
        "contractHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "scopes": ["global"],
        "placement": "device"
      }]
    },
    "dynamic": {"allowedAdapters": ["tool"], "maxTools": 10},
    "contributions": [{
      "id": "lookup",
      "kind": "tool",
      "adapter": "tool",
      "contractVersion": 1,
      "name": {"default": "Lookup"},
      "spec": {"toolId": "lookup"}
    }]
  }],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "tree"}
}`

func TestParseValidManifest(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	report := manifest.Validate()
	if report.HasErrors() {
		t.Fatalf("unexpected validation errors: %+v", report.Issues)
	}
	if manifest.Modules[0].Contributions[0].Adapter != "tool" {
		t.Fatalf("adapter not retained")
	}
}

func TestParseRejectsLegacyManifestFields(t *testing.T) {
	legacy := `{
	  "manifestVersion": 1,
	  "extension": {"id": "com.example/plugin", "name": {"default": "Plugin"}, "version": "1.0.0"},
	  "publisher": {"id": "com.example", "displayName": "Example"},
	  "modules": [],
	  "integrity": {"algorithm": "sha256", "contentTreeHash": "tree"}
	}`
	if _, err := Parse([]byte(legacy)); err == nil {
		t.Fatal("legacy manifest must be rejected")
	}
}

func TestContributionRequiresExplicitAdapter(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	manifest.Modules[0].Contributions[0].Adapter = ""
	report := manifest.Validate()
	if !hasIssue(report, "adapter_required") {
		t.Fatalf("expected adapter_required, got %+v", report.Issues)
	}
}

func TestManifestRejectsUnsafeEntry(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	manifest.Modules[0].Runtime.Entry = "../outside.js"
	report := manifest.Validate()
	if !hasIssue(report, "unsafe_path") {
		t.Fatalf("expected unsafe_path, got %+v", report.Issues)
	}
}

func TestManifestRejectsReservedSystemMetadata(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	manifest.Plugin.Metadata = map[string]any{"system.managed": true}
	report := manifest.Validate()
	if !hasIssue(report, "reserved_metadata") {
		t.Fatalf("expected reserved_metadata, got %+v", report.Issues)
	}
}

func TestManifestAllowsNonPrivilegedSystemDefaults(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	manifest.Plugin.Metadata = map[string]any{
		"system.defaultInstall": true,
		"system.defaultEnabled": true,
	}
	if report := manifest.Validate(); report.HasErrors() {
		t.Fatalf("non-privileged system defaults were rejected: %+v", report.Issues)
	}
}

func TestAdapterRegistryEnforcesTrustAndSnapshots(t *testing.T) {
	registry := NewAdapterRegistry(AdapterRegistrationPolicy{MinTrust: TrustVerified, AllowExternal: true})
	descriptor := adapterDescriptor(TrustVerified, true)
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := registry.Register(descriptor); err == nil {
		t.Fatal("duplicate adapter registration must fail")
	}
	snapshot := registry.Snapshot()
	if snapshot.Generation != 1 || len(snapshot.Adapters) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if err := registry.Unregister(descriptor.ID, descriptor.ContractVersion); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if _, ok := registry.Get(descriptor.ID, descriptor.ContractVersion); ok {
		t.Fatal("adapter remained registered")
	}
}

func TestAdapterRegistryRejectsLowTrust(t *testing.T) {
	registry := NewAdapterRegistry(AdapterRegistrationPolicy{MinTrust: TrustOfficial})
	err := registry.Register(adapterDescriptor(TrustVerified, false))
	if err == nil {
		t.Fatal("low trust adapter must fail")
	}
}

func TestServiceGraphOrdersProvidersBeforeConsumers(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{serviceProvider("provider-a", "com.example/a", "store", "1.2.0", 10)},
		Consumers: []ServiceConsumer{{
			Module: ModuleRef{PluginID: "com.example/b", ModuleID: "main"},
			Requirements: []ServiceRequirement{{
				ServiceID: "store", VersionRange: "^1.0.0",
			}},
		}},
	})
	if !resolution.Valid() {
		t.Fatalf("unexpected issues: %+v", resolution.Issues)
	}
	assertBefore(t, resolution.Graph.ActivationOrder, "com.example/a/main", "com.example/b/main")
	assertBefore(t, resolution.Graph.ReverseOrder, "com.example/b/main", "com.example/a/main")
}

func TestServiceGraphDetectsCycle(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{
			serviceProvider("provider-a", "com.example/a", "a.service", "1.0.0", 0),
			serviceProvider("provider-b", "com.example/b", "b.service", "1.0.0", 0),
		},
		Consumers: []ServiceConsumer{
			{Module: ModuleRef{PluginID: "com.example/a", ModuleID: "main"}, Requirements: []ServiceRequirement{{ServiceID: "b.service"}}},
			{Module: ModuleRef{PluginID: "com.example/b", ModuleID: "main"}, Requirements: []ServiceRequirement{{ServiceID: "a.service"}}},
		},
	})
	if resolution.Valid() {
		t.Fatal("cycle must fail")
	}
	if !hasGraphIssue(resolution, "service_dependency_cycle") {
		t.Fatalf("expected service_dependency_cycle, got %+v", resolution.Issues)
	}
}

func TestServiceGraphRejectsVersionMismatch(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{serviceProvider("provider-a", "com.example/a", "store", "2.0.0", 0)},
		Consumers: []ServiceConsumer{{
			Module:       ModuleRef{PluginID: "com.example/b", ModuleID: "main"},
			Requirements: []ServiceRequirement{{ServiceID: "store", VersionRange: "^1.0.0"}},
		}},
	})
	if resolution.Valid() || !hasGraphIssue(resolution, "required_service_unavailable") {
		t.Fatalf("expected service unavailable, got %+v", resolution.Issues)
	}
}

func TestServiceGraphRejectsExclusiveMultipleProviders(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{
			serviceProvider("provider-a", "com.example/a", "store", "1.0.0", 0),
			serviceProvider("provider-b", "com.example/b", "store", "1.0.0", 0),
		},
		Consumers: []ServiceConsumer{{
			Module:       ModuleRef{PluginID: "com.example/c", ModuleID: "main"},
			Requirements: []ServiceRequirement{{ServiceID: "store", Selection: ServiceSelectionExclusive}},
		}},
	})
	if resolution.Valid() || !hasGraphIssue(resolution, "exclusive_service_conflict") {
		t.Fatalf("expected exclusive conflict, got %+v", resolution.Issues)
	}
}

func TestServiceGraphSelectsAllProviders(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Providers: []ServiceDescriptor{
			serviceProvider("provider-a", "com.example/a", "store", "1.0.0", 0),
			serviceProvider("provider-b", "com.example/b", "store", "1.0.0", 0),
		},
		Consumers: []ServiceConsumer{{
			Module:       ModuleRef{PluginID: "com.example/c", ModuleID: "main"},
			Requirements: []ServiceRequirement{{ServiceID: "store", Selection: ServiceSelectionAll}},
		}},
	})
	if !resolution.Valid() {
		t.Fatalf("unexpected issues: %+v", resolution.Issues)
	}
	bindings := resolution.Graph.Bindings["com.example/c/main"]
	if len(bindings) != 1 || len(bindings[0].Providers) != 2 {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
}

func TestServiceGraphAllowsMissingOptionalService(t *testing.T) {
	resolution := (ServiceGraphResolver{}).Resolve(ServiceGraphRequest{
		Consumers: []ServiceConsumer{{
			Module:       ModuleRef{PluginID: "com.example/c", ModuleID: "main"},
			Requirements: []ServiceRequirement{{ServiceID: "optional.service", Optional: true}},
		}},
	})
	if !resolution.Valid() {
		t.Fatalf("optional service must not fail: %+v", resolution.Issues)
	}
}

func TestManifestResolvesItsServiceGraph(t *testing.T) {
	manifest, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	resolution := manifest.ResolveServiceGraph()
	if !resolution.Valid() {
		t.Fatalf("unexpected issues: %+v", resolution.Issues)
	}
	if len(resolution.Graph.ActivationOrder) != 1 || resolution.Graph.ActivationOrder[0] != "com.example.plugin/main" {
		t.Fatalf("unexpected activation order: %+v", resolution.Graph.ActivationOrder)
	}
}

func adapterDescriptor(trust TrustLevel, external bool) AdapterDescriptor {
	return AdapterDescriptor{
		ID:                 "amitia.test",
		ContractVersion:    1,
		SupportedKinds:     []string{"tool"},
		ContributionSchema: json.RawMessage(`{"type":"object"}`),
		Trust:              trust,
		External:           external,
	}
}

func serviceProvider(providerID, pluginID, serviceID, version string, priority int) ServiceDescriptor {
	return ServiceDescriptor{
		ProviderID: providerID,
		Module:     ModuleRef{PluginID: pluginID, ModuleID: "main"},
		ServiceID:  serviceID,
		Version:    version,
		Scopes:     []ServiceScope{ServiceScopeGlobal},
		Placement:  ServicePlacementDevice,
		Priority:   priority,
	}
}

func hasIssue(report ValidationReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func hasGraphIssue(resolution ServiceGraphResolution, code string) bool {
	for _, issue := range resolution.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func assertBefore(t *testing.T, values []string, first, second string) {
	t.Helper()
	firstIndex := -1
	secondIndex := -1
	for index, value := range values {
		if value == first {
			firstIndex = index
		}
		if value == second {
			secondIndex = index
		}
	}
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("expected %s before %s in %v", first, second, values)
	}
}
