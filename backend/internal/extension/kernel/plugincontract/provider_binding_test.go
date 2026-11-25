package plugincontract

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/provideridentity"
)

func manualProviderKeySites(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	sites := make([]string, 0, 4)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for index, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, `Key() + "/"`) ||
				strings.Contains(line, `key + "/" + serviceID`) ||
				strings.Contains(line, `+ "/" + offer.ServiceID`) {
				sites = append(sites, name+":"+strconv.Itoa(index+1))
			}
		}
	}
	return sites
}

func TestProviderBindingsMatchEveryDerivationForRealManifest(t *testing.T) {
	manifest := loadEmoteManifest(t)
	bindings := manifest.ProviderBindings()
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	binding := bindings[0]
	if binding.ProviderID != "com.amitia/emote/emote-runtime/com.amitia/emote/ai" {
		t.Fatalf("provider id = %s", binding.ProviderID)
	}
	if binding.PluginID != "com.amitia/emote" || binding.ModuleID != "emote-runtime" {
		t.Fatalf("module = %s/%s", binding.PluginID, binding.ModuleID)
	}
	built, err := BuildProviderID(ModuleRef{PluginID: binding.PluginID, ModuleID: binding.ModuleID}, binding.ServiceID)
	if err == nil {
		t.Fatalf("BuildProviderID accepted catalog key %s", built)
	}
	if !strings.Contains(err.Error(), "more than 5 segments") {
		t.Fatalf("unexpected BuildProviderID error: %v", err)
	}
	if composeProviderID(ModuleRef{PluginID: binding.PluginID, ModuleID: binding.ModuleID}, binding.ServiceID) != binding.ProviderID {
		t.Fatalf("composer diverged from binding")
	}
	if provideridentity.CatalogProviderKey(binding.PluginID, binding.ModuleID, binding.ServiceID) != binding.ProviderID {
		t.Fatalf("provideridentity key diverged from binding")
	}
	graph := manifest.ResolveServiceGraph()
	if !graph.Valid() {
		t.Fatalf("service graph invalid: %+v", graph.Issues)
	}
	if leftovers := manualProviderKeySites(t); len(leftovers) != 0 {
		t.Fatalf("manual provider key composition remains: %v", leftovers)
	}
	catalog, err := ServiceCatalogFromModules(manifest.CatalogModules(true))
	if err != nil {
		t.Fatalf("catalog from modules: %v", err)
	}
	catalogBindings := catalog.ProviderBindings()
	if len(catalogBindings) != len(bindings) {
		t.Fatalf("catalog bindings = %d, want %d", len(catalogBindings), len(bindings))
	}
	if catalogBindings[0].ProviderID != binding.ProviderID || catalogBindings[0].Version != binding.Version || catalogBindings[0].ContractHash != binding.ContractHash {
		t.Fatalf("catalog binding = %+v, manifest binding = %+v", catalogBindings[0], binding)
	}
	if catalogBindings[0].Enabled != true {
		t.Fatalf("catalog binding disabled")
	}
	offline, err := ServiceCatalogFromModules(manifest.CatalogModules(false))
	if err != nil {
		t.Fatalf("offline catalog: %v", err)
	}
	if offline.ProviderBindings()[0].Enabled {
		t.Fatalf("disabled catalog reported enabled bindings")
	}
}

func TestCatalogModulesKeepManifestIdentityAndEnabledFlag(t *testing.T) {
	manifest := loadEmoteManifest(t)
	modules := manifest.CatalogModules(false)
	if len(modules) != len(manifest.Modules) {
		t.Fatalf("modules = %d, want %d", len(modules), len(manifest.Modules))
	}
	for index, module := range modules {
		if module.Module.PluginID != manifest.Plugin.ID {
			t.Fatalf("module %d plugin = %s", index, module.Module.PluginID)
		}
		if module.Module.ModuleID != manifest.Modules[index].ID {
			t.Fatalf("module %d id = %s", index, module.Module.ModuleID)
		}
		if module.Enabled {
			t.Fatalf("module %d enabled when requested disabled", index)
		}
	}
}

func TestProviderBindingsAreSortedAndIsolated(t *testing.T) {
	manifest := parseSyntheticManifest(t, `{
		"schemaVersion": 1,
		"plugin": {"id": "com.example/two", "name": {"default": "Two"}, "version": "1.0.0", "publisher": {"id": "com.example", "displayName": "Example"}},
		"compatibility": {"minHostVersion": "1.0.0"},
		"modules": [
			{"id": "zulu", "services": {"provides": [{"serviceId": "svc.zulu", "version": "1.2.0", "placement": "core", "scopes": ["global"]}]}},
			{"id": "alpha", "services": {"provides": [{"serviceId": "svc.alpha", "version": "1.0.0", "placement": "cloud", "scopes": ["character"]}]}}
		]
	}`)
	bindings := manifest.ProviderBindings()
	if len(bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(bindings))
	}
	if bindings[0].ModuleID != "alpha" || bindings[1].ModuleID != "zulu" {
		t.Fatalf("bindings not sorted: %s, %s", bindings[0].ProviderID, bindings[1].ProviderID)
	}
	bindings[0].Scopes[0] = ServiceScopeDevice
	if manifest.ProviderBindings()[0].Scopes[0] != ServiceScopeCharacter {
		t.Fatalf("scopes shared with manifest")
	}
}

func TestDiffProviderBindingsClassifiesAddedRemovedChanged(t *testing.T) {
	base := []ProviderBinding{
		{ProviderID: "a", ServiceID: "a", Version: "1.0.0", Scopes: []ServiceScope{ServiceScopeGlobal}, Placement: ServicePlacementCore, Enabled: true},
		{ProviderID: "b", ServiceID: "b", Version: "1.0.0", Scopes: []ServiceScope{ServiceScopeGlobal}, Placement: ServicePlacementCore, Enabled: true},
	}
	next := []ProviderBinding{
		{ProviderID: "a", ServiceID: "a", Version: "1.1.0", Scopes: []ServiceScope{ServiceScopeDevice}, Placement: ServicePlacementDevice, Enabled: false},
		{ProviderID: "c", ServiceID: "c", Version: "1.0.0", Scopes: []ServiceScope{ServiceScopeGlobal}, Placement: ServicePlacementCore, Enabled: true},
	}
	diff := DiffProviderBindings(base, next)
	if diff.Empty() {
		t.Fatalf("diff reported empty")
	}
	if diff.Touched() != 3 {
		t.Fatalf("touched = %d, want 3", diff.Touched())
	}
	if len(diff.Added) != 1 || diff.Added[0].ProviderID != "c" {
		t.Fatalf("added = %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].ProviderID != "b" {
		t.Fatalf("removed = %+v", diff.Removed)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Binding.ProviderID != "a" {
		t.Fatalf("changed = %+v", diff.Changed)
	}
	fields := strings.Join(diff.Changed[0].Fields, ",")
	if fields != "version,placement,scopes,enabled" {
		t.Fatalf("changed fields = %s", fields)
	}
	if !DiffProviderBindings(base, base).Empty() {
		t.Fatalf("replay produced a diff")
	}
	empty := DiffProviderBindings(nil, nil)
	if !empty.Empty() || empty.Touched() != 0 {
		t.Fatalf("nil diff = %+v", empty)
	}
}

func TestServiceCatalogFromModulesRejectsDuplicateProvider(t *testing.T) {
	module := CatalogModule{
		Module:   ModuleRef{PluginID: "com.example/dup", ModuleID: "core"},
		Services: ServiceDeclaration{Provides: []ServiceOffer{{ServiceID: "svc.one", Version: "1.0.0", Placement: ServicePlacementCore}, {ServiceID: "svc.one", Version: "1.0.0", Placement: ServicePlacementCore}}},
		Enabled:  true,
	}
	if _, err := ServiceCatalogFromModules([]CatalogModule{module}); err == nil {
		t.Fatalf("duplicate provider accepted")
	}
}

func TestManifestProjectionDeduplicatesDeclaredCapabilities(t *testing.T) {
	manifest := loadEmoteManifest(t)
	definition, err := manifest.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("project manifest: %v", err)
	}
	count := 0
	for _, module := range definition.Modules {
		if string(module.ID) != "emote-runtime" {
			continue
		}
		for _, capability := range module.ProvidedCapabilities {
			if capability.ID == "com.amitia/emote/ai" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("capability declared %d times, want 1", count)
	}
	synthetic := parseSyntheticManifest(t, `{
		"schemaVersion": 1,
		"plugin": {"id": "com.example/dedupe", "name": {"default": "Dedupe"}, "version": "1.0.0", "publisher": {"id": "com.example", "displayName": "Example"}},
		"compatibility": {"minHostVersion": "1.0.0"},
		"modules": [{
			"id": "core",
			"name": {"default": "Core"},
			"providedCapabilities": [{"id": "svc.one", "version": "2.0.0", "metadata": {"tier": "explicit"}}],
			"services": {"provides": [{"serviceId": "svc.one", "version": "1.0.0", "placement": "core", "scopes": ["global"]}]}
		}],
		"integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
	}`)
	projected, err := synthetic.ToExtensionDefinition()
	if err != nil {
		t.Fatalf("project synthetic manifest: %v", err)
	}
	if len(projected.Modules) != 1 || len(projected.Modules[0].ProvidedCapabilities) != 1 {
		t.Fatalf("projected capabilities = %+v", projected.Modules)
	}
	capability := projected.Modules[0].ProvidedCapabilities[0]
	if capability.Version != "2.0.0" || capability.Metadata["tier"] != "explicit" {
		t.Fatalf("explicit declaration lost: %+v", capability)
	}
}
