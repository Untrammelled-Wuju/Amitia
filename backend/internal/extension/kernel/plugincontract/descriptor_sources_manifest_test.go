package plugincontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func loadEmoteManifest(t *testing.T) *Manifest {
	t.Helper()
	workspaceRoot := filepath.Join("..", "..", "..", "..", "..")
	path := filepath.Join(workspaceRoot, "plugins-code", "emote", "amitia-extension.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read emote manifest: %v", err)
	}
	manifest, err := Parse(data)
	if err != nil {
		t.Fatalf("parse emote manifest: %v", err)
	}
	return manifest
}

func parseSyntheticManifest(t *testing.T, raw string) *Manifest {
	t.Helper()
	manifest, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse synthetic manifest: %v", err)
	}
	return manifest
}

func mustFindEntry(t *testing.T, catalog DescriptorCatalog, capabilityID string) DescriptorEntry {
	t.Helper()
	entry, found := catalog.FindByCapabilityID(capabilityID)
	if !found {
		t.Fatalf("capability %q missing from catalog of %d entries", capabilityID, catalog.Len())
	}
	return entry
}

func scopeNames(scopes []ServiceScope) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, string(scope))
	}
	return out
}

func permissionNames(permissions []DescriptorPermission) []string {
	out := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, permission.Capability)
	}
	return out
}

func TestManifestDescriptorCatalogRealEmoteManifest(t *testing.T) {
	manifest := loadEmoteManifest(t)
	catalog, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build manifest descriptor catalog: %v", err)
	}
	if catalog.Len() != 5 {
		t.Fatalf("catalog entries = %d, want 5", catalog.Len())
	}
	counts := catalog.CountBySource()
	if counts[DescriptorSourceManifestCapability] != 1 {
		t.Fatalf("providedCapability entries = %d, want 1", counts[DescriptorSourceManifestCapability])
	}
	if counts[DescriptorSourceManifestUIProvider] != 4 {
		t.Fatalf("uiProvider entries = %d, want 4", counts[DescriptorSourceManifestUIProvider])
	}
	if _, exists := counts[DescriptorSourceManifestSkill]; exists {
		t.Fatalf("unexpected agentSkill entries: %d", counts[DescriptorSourceManifestSkill])
	}

	capability := mustFindEntry(t, catalog, "com.amitia/emote/ai")
	if capability.Source != DescriptorSourceManifestCapability {
		t.Fatalf("capability source = %q", capability.Source)
	}
	if capability.ProviderID != "com.amitia/emote/ai.provider" {
		t.Fatalf("capability provider id = %q", capability.ProviderID)
	}
	if capability.ServiceID != "ai" {
		t.Fatalf("capability service id = %q", capability.ServiceID)
	}
	if capability.Version != "1.0.0" {
		t.Fatalf("capability version = %q, want capability-declared 1.0.0", capability.Version)
	}
	if capability.Descriptor.Placement != ServicePlacementDevice {
		t.Fatalf("capability placement = %q", capability.Descriptor.Placement)
	}
	if got := scopeNames(capability.Descriptor.Scopes); len(got) != 2 || got[0] != "device" || got[1] != "plugin" {
		t.Fatalf("capability scopes = %v", got)
	}
	if len(capability.Descriptor.Permissions) != 0 {
		t.Fatalf("capability permissions = %v, want none (module declares none)", permissionNames(capability.Descriptor.Permissions))
	}
	if got := strings.Join(capability.Descriptor.Tags, ","); got != "capability,plugin" {
		t.Fatalf("capability tags = %q", got)
	}
	if len(capability.InputSchema) != 0 || len(capability.OutputSchema) != 0 {
		t.Fatalf("capability declares no schema, got input=%d output=%d bytes", len(capability.InputSchema), len(capability.OutputSchema))
	}
	if capability.Descriptor.InputRef != nil || capability.Descriptor.OutputRef != nil {
		t.Fatalf("capability refs must stay empty without declared schema")
	}
	if len(capability.ContractHash) != 64 {
		t.Fatalf("capability contract hash = %q", capability.ContractHash)
	}
	if capability.Descriptor.Summary != "表情包运行时 提供能力 ai" {
		t.Fatalf("capability summary = %q", capability.Descriptor.Summary)
	}

	uiProvider := mustFindEntry(t, catalog, "com.amitia/emote/emote-ui/emote-page-provider")
	if uiProvider.Source != DescriptorSourceManifestUIProvider {
		t.Fatalf("ui provider source = %q", uiProvider.Source)
	}
	if uiProvider.ProviderID != "com.amitia/emote/emote-ui/emote-page-provider.provider" {
		t.Fatalf("ui provider id = %q", uiProvider.ProviderID)
	}
	if uiProvider.ServiceID != "emote-page-provider" {
		t.Fatalf("ui provider service id = %q", uiProvider.ServiceID)
	}
	if uiProvider.Version != "1.1.11" {
		t.Fatalf("ui provider version = %q, want plugin fallback 1.1.11", uiProvider.Version)
	}
	if got := scopeNames(uiProvider.Descriptor.Scopes); len(got) != 2 || got[0] != "device" || got[1] != "plugin" {
		t.Fatalf("ui provider scopes = %v", got)
	}
	if got := strings.Join(uiProvider.Descriptor.Tags, ","); got != "provider,ui" {
		t.Fatalf("ui provider tags = %q", got)
	}
	if !strings.HasPrefix(uiProvider.Descriptor.Summary, "UI Provider ") {
		t.Fatalf("ui provider summary = %q", uiProvider.Descriptor.Summary)
	}

	for _, id := range []string{
		"com.amitia/emote/emote-ui/emote-workspace-sidebar-provider",
		"com.amitia/emote/emote-ui/emote-routes",
		"com.amitia/emote/emote-ui/emote-message-provider",
	} {
		mustFindEntry(t, catalog, id)
	}
	if _, found := catalog.FindByServiceID("ai"); !found {
		t.Fatal("service id ai not resolvable")
	}
	if _, found := catalog.FindByCapabilityID("com.amitia/emote/emote-command"); found {
		t.Fatal("tool contribution leaked into descriptor catalog")
	}
	if _, found := catalog.FindByCapabilityID("com.amitia/emote/emote-ui/emote-page"); found {
		t.Fatal("ui_page contribution leaked into descriptor catalog")
	}
}

func TestManifestDescriptorCatalogCoversOnlyDeclaredSources(t *testing.T) {
	manifest := loadEmoteManifest(t)
	skillCatalog, err := ManifestSkillDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build skill catalog: %v", err)
	}
	if skillCatalog.Len() != 0 {
		t.Fatalf("skill catalog entries = %d, want 0", skillCatalog.Len())
	}
	capabilityCatalog, err := ManifestCapabilityDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build capability catalog: %v", err)
	}
	if capabilityCatalog.Len() != 1 {
		t.Fatalf("capability catalog entries = %d, want 1", capabilityCatalog.Len())
	}
	uiCatalog, err := ManifestUIProviderDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build ui provider catalog: %v", err)
	}
	if uiCatalog.Len() != 4 {
		t.Fatalf("ui provider catalog entries = %d, want 4", uiCatalog.Len())
	}
	for _, entry := range uiCatalog.Entries {
		if entry.Source != DescriptorSourceManifestUIProvider {
			t.Fatalf("ui provider catalog mixed source %q", entry.Source)
		}
	}
}

func TestManifestDescriptorCatalogSyntheticSkill(t *testing.T) {
	manifest := parseSyntheticManifest(t, `{
  "schemaVersion": 1,
  "plugin": {"id": "com.example/skills", "name": {"default": "Example"}, "version": "9.9.9", "publisher": {"id": "com.example", "displayName": "Example"}},
  "compatibility": {"minHostVersion": "0.1.0"},
  "modules": [
    {
      "id": "skill-runtime",
      "name": {"default": "技能运行时"},
      "version": "2.0.0",
      "placement": "cloud",
      "contributions": [
        {
          "id": "summarize",
          "kind": "agent_skill",
          "adapter": "host-agent-skill",
          "contractVersion": 1,
          "name": {"default": "总结"},
          "version": "3.1.4",
          "schema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]},
          "requiredPermissions": ["model.invoke", "model.invoke", ""],
          "requiredScope": ["conversation", "bogus"]
        },
        {
          "id": "ignored-tool",
          "kind": "tool",
          "adapter": "host-tool",
          "contractVersion": 1,
          "name": {"default": "工具"},
          "requiredPermissions": ["tool.invoke"]
        }
      ]
    }
  ],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
}`)
	catalog, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build manifest descriptor catalog: %v", err)
	}
	if catalog.Len() != 1 {
		t.Fatalf("catalog entries = %d, want 1", catalog.Len())
	}
	entry := mustFindEntry(t, catalog, "com.example/skills/skill-runtime/summarize")
	if entry.Source != DescriptorSourceManifestSkill {
		t.Fatalf("skill source = %q", entry.Source)
	}
	if entry.ProviderID != "com.example/skills/skill-runtime/summarize.provider" {
		t.Fatalf("skill provider id = %q", entry.ProviderID)
	}
	if entry.Version != "3.1.4" {
		t.Fatalf("skill version = %q, want contribution-declared 3.1.4", entry.Version)
	}
	if entry.Descriptor.Placement != ServicePlacementCloud {
		t.Fatalf("skill placement = %q", entry.Descriptor.Placement)
	}
	if got := strings.Join(scopeNames(entry.Descriptor.Scopes), ","); got != "conversation,plugin" {
		t.Fatalf("skill scopes = %q", got)
	}
	if got := strings.Join(permissionNames(entry.Descriptor.Permissions), ","); got != "model.invoke" {
		t.Fatalf("skill permissions = %q", got)
	}
	if entry.Descriptor.Permissions[0].Risk != "" {
		t.Fatalf("skill permission risk = %q, want empty", entry.Descriptor.Permissions[0].Risk)
	}
	if got := strings.Join(entry.Descriptor.Tags, ","); got != "skill" {
		t.Fatalf("skill tags = %q", got)
	}
	if entry.Descriptor.Summary != "技能 总结" {
		t.Fatalf("skill summary = %q", entry.Descriptor.Summary)
	}
	if entry.Descriptor.InputRef == nil {
		t.Fatal("skill input ref missing")
	}
	if entry.Descriptor.OutputRef != nil {
		t.Fatal("skill output ref must stay empty")
	}
	resolved, err := catalog.Resolve(*entry.Descriptor.InputRef)
	if err != nil {
		t.Fatalf("resolve skill input schema: %v", err)
	}
	if !bytes.Contains(resolved, []byte(`"text"`)) {
		t.Fatalf("resolved skill schema = %s", resolved)
	}
	if !json.Valid(resolved) {
		t.Fatalf("resolved skill schema is not valid JSON: %s", resolved)
	}
	if len(catalog.Schemas) != 1 {
		t.Fatalf("catalog schema count = %d, want 1", len(catalog.Schemas))
	}
	if _, found := catalog.FindByCapabilityID("com.example/skills/skill-runtime/ignored-tool"); found {
		t.Fatal("tool contribution leaked into descriptor catalog")
	}
}

func TestManifestDescriptorCatalogVersionFallback(t *testing.T) {
	manifest := parseSyntheticManifest(t, `{
  "schemaVersion": 1,
  "plugin": {"id": "com.example/fallback", "name": {"default": "Fallback"}, "version": "5.0.0", "publisher": {"id": "com.example", "displayName": "Example"}},
  "compatibility": {"minHostVersion": "0.1.0"},
  "modules": [
    {"id": "runtime", "name": {"default": "Runtime"}, "providedCapabilities": [{"id": "com.example/fallback/one"}]},
    {"id": "second", "name": {"default": "Second"}, "version": "6.2.1", "providedCapabilities": [{"id": "com.example/fallback/two"}]}
  ],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
}`)
	catalog, err := ManifestCapabilityDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build manifest descriptor catalog: %v", err)
	}
	one := mustFindEntry(t, catalog, "com.example/fallback/one")
	if one.Version != "5.0.0" {
		t.Fatalf("plugin-version fallback = %q", one.Version)
	}
	two := mustFindEntry(t, catalog, "com.example/fallback/two")
	if two.Version != "6.2.1" {
		t.Fatalf("module-version fallback = %q", two.Version)
	}
	for _, entry := range catalog.Entries {
		if entry.Descriptor.Placement != ServicePlacementDevice {
			t.Fatalf("%s placement = %q, want device default", entry.CapabilityID, entry.Descriptor.Placement)
		}
		if got := strings.Join(scopeNames(entry.Descriptor.Scopes), ","); got != "device,plugin" {
			t.Fatalf("%s scopes = %q", entry.CapabilityID, got)
		}
	}
}

func TestManifestDescriptorCatalogIsDeterministic(t *testing.T) {
	manifest := loadEmoteManifest(t)
	first, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	second, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	firstSnapshot, err := first.Snapshot()
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	secondSnapshot, err := second.Snapshot()
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !bytes.Equal(firstSnapshot, secondSnapshot) {
		t.Fatal("manifest descriptor catalog snapshot is not deterministic")
	}
	listing := first.Listing()
	if len(listing) != first.Len() {
		t.Fatalf("listing length = %d, catalog length = %d", len(listing), first.Len())
	}
	for index := 1; index < len(listing); index++ {
		if listing[index-1].ID >= listing[index].ID {
			t.Fatalf("listing not sorted: %q >= %q", listing[index-1].ID, listing[index].ID)
		}
	}
}

func TestCoreDescriptorCatalogConvertsToGenericCatalog(t *testing.T) {
	core, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build core catalog: %v", err)
	}
	catalog := core.ToCatalog()
	if catalog.Len() != len(core.Entries) {
		t.Fatalf("converted entries = %d, want %d", catalog.Len(), len(core.Entries))
	}
	if len(catalog.Schemas) != len(core.Schemas) {
		t.Fatalf("converted schemas = %d, want %d", len(catalog.Schemas), len(core.Schemas))
	}
	for _, entry := range catalog.Entries {
		if entry.Source != DescriptorSourceCore {
			t.Fatalf("%s source = %q", entry.CapabilityID, entry.Source)
		}
		if entry.ProviderID != CoreProviderPluginID+"/"+CoreProviderModuleID+"/"+entry.ServiceID {
			t.Fatalf("%s provider id = %q", entry.CapabilityID, entry.ProviderID)
		}
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("converted catalog invalid: %v", err)
	}
	coreEntry, found := core.Find(CoreMemorySearchServiceID)
	if !found {
		t.Fatalf("core catalog missing %q", CoreMemorySearchServiceID)
	}
	entry := mustFindEntry(t, catalog, coreEntry.CapabilityID)
	if entry.Descriptor.InputRef == nil || entry.Descriptor.OutputRef == nil {
		t.Fatal("core entry refs must survive conversion")
	}
	if _, err := catalog.Resolve(*entry.Descriptor.InputRef); err != nil {
		t.Fatalf("resolve converted core schema: %v", err)
	}
}

func TestManifestDescriptorCatalogRejectsDuplicateCapability(t *testing.T) {
	manifest := parseSyntheticManifest(t, `{
  "schemaVersion": 1,
  "plugin": {"id": "com.example/dup", "name": {"default": "Dup"}, "version": "1.0.0", "publisher": {"id": "com.example", "displayName": "Example"}},
  "compatibility": {"minHostVersion": "0.1.0"},
  "modules": [
    {"id": "alpha", "name": {"default": "Alpha"}, "providedCapabilities": [{"id": "com.example/dup/ai"}]},
    {"id": "beta", "name": {"default": "Beta"}, "providedCapabilities": [{"id": "com.example/dup/ai"}]}
  ],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
}`)
	_, err := ManifestDescriptorCatalog(*manifest)
	if err == nil {
		t.Fatal("duplicate capability accepted")
	}
	if !errors.Is(err, ErrInvalidManifestDescriptor) {
		t.Fatalf("duplicate error chain = %v", err)
	}
	var detail *ManifestDescriptorError
	if !errors.As(err, &detail) {
		t.Fatalf("duplicate error type = %T", err)
	}
	if detail.ModuleID != "beta" || detail.LocalID != "com.example/dup/ai" {
		t.Fatalf("duplicate error detail = module %q local %q", detail.ModuleID, detail.LocalID)
	}
}

func TestManifestDescriptorCatalogRejectsInvalidIdentity(t *testing.T) {
	manifest := parseSyntheticManifest(t, `{
  "schemaVersion": 1,
  "plugin": {"id": "com.example/invalid", "name": {"default": "Invalid"}, "version": "1.0.0", "publisher": {"id": "com.example", "displayName": "Example"}},
  "compatibility": {"minHostVersion": "0.1.0"},
  "modules": [
    {"id": "Bad Module", "name": {"default": "Bad"}, "providedCapabilities": [{"id": "ai"}]}
  ],
  "integrity": {"algorithm": "sha256", "contentTreeHash": "0000000000000000000000000000000000000000000000000000000000000000"}
}`)
	_, err := ManifestDescriptorCatalog(*manifest)
	if err == nil {
		t.Fatal("invalid module id accepted")
	}
	if !errors.Is(err, ErrInvalidManifestDescriptor) {
		t.Fatalf("invalid identity error chain = %v", err)
	}
	var detail *ManifestDescriptorError
	if !errors.As(err, &detail) {
		t.Fatalf("invalid identity error type = %T", err)
	}
	if detail.PluginID != "com.example/invalid" || detail.LocalID != "ai" {
		t.Fatalf("invalid identity detail = plugin %q local %q", detail.PluginID, detail.LocalID)
	}
}

func TestManifestDescriptorCatalogRejectsBrokenSchema(t *testing.T) {
	manifest := Manifest{
		Plugin: PluginMeta{ID: "com.example/broken", Version: "1.0.0"},
		Modules: []Module{{
			ID:   "runtime",
			Name: LocalizedText{Default: "Runtime"},
			ProvidedCapabilities: []ProvidedCapability{{
				ID:       "com.example/broken/ai",
				Metadata: map[string]any{"inputSchema": json.RawMessage(`{"type":`)},
			}},
		}},
	}
	_, err := ManifestDescriptorCatalog(manifest)
	if err == nil {
		t.Fatal("broken schema accepted")
	}
	if !errors.Is(err, ErrInvalidManifestDescriptor) {
		t.Fatalf("broken schema error chain = %v", err)
	}
	if !strings.Contains(err.Error(), "input schema") && !strings.Contains(err.Error(), "metadata inputSchema") {
		t.Fatalf("broken schema error = %v", err)
	}
}

func TestManifestDescriptorCatalogScalesTo500EntriesWithoutRuntime(t *testing.T) {
	modules := make([]Module, 0, 50)
	for moduleIndex := 0; moduleIndex < 50; moduleIndex++ {
		module := Module{
			ID:        "runtime-" + strconv.Itoa(moduleIndex),
			Name:      LocalizedText{Default: "运行时 " + strconv.Itoa(moduleIndex)},
			Version:   "1.0.0",
			Placement: ServicePlacementDevice,
		}
		for capabilityIndex := 0; capabilityIndex < 10; capabilityIndex++ {
			module.ProvidedCapabilities = append(module.ProvidedCapabilities, ProvidedCapability{
				ID:       "com.example/scale/runtime-" + strconv.Itoa(moduleIndex) + "/cap-" + strconv.Itoa(capabilityIndex),
				Metadata: map[string]any{"inputSchema": sharedScaleSchema()},
			})
		}
		modules = append(modules, module)
	}
	manifest := Manifest{
		Plugin:  PluginMeta{ID: "com.example/scale", Version: "2.0.0"},
		Modules: modules,
	}
	before := runtime.NumGoroutine()
	started := time.Now()
	catalog, err := ManifestCapabilityDescriptorCatalog(manifest)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("build 500 entry catalog: %v", err)
	}
	if catalog.Len() != 500 {
		t.Fatalf("entries = %d, want 500", catalog.Len())
	}
	if len(catalog.Schemas) != 1 {
		t.Fatalf("schema count = %d, want 1 (500 entries share one canonical schema)", len(catalog.Schemas))
	}
	if after := runtime.NumGoroutine(); after != before {
		t.Fatalf("goroutines changed %d -> %d while building descriptors", before, after)
	}
	snapshot, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("snapshot 500 entries: %v", err)
	}
	again, err := ManifestCapabilityDescriptorCatalog(manifest)
	if err != nil {
		t.Fatalf("rebuild 500 entry catalog: %v", err)
	}
	secondSnapshot, err := again.Snapshot()
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !bytes.Equal(snapshot, secondSnapshot) {
		t.Fatal("500 entry snapshot is not deterministic")
	}
	for _, entry := range catalog.Entries {
		if entry.Source != DescriptorSourceManifestCapability {
			t.Fatalf("%s source = %q", entry.CapabilityID, entry.Source)
		}
		if entry.ProviderID != entry.CapabilityID+providerIDSuffix {
			t.Fatalf("%s provider id = %q", entry.CapabilityID, entry.ProviderID)
		}
		if entry.Descriptor.InputRef == nil {
			t.Fatalf("%s lost shared input ref", entry.CapabilityID)
		}
	}
	t.Logf("entries=500 schemas=%d snapshotBytes=%d buildElapsed=%s goroutinesDelta=0", len(catalog.Schemas), len(snapshot), elapsed)
}

func sharedScaleSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`)
}

func TestManifestDescriptorAdaptersAreInMemoryOnly(t *testing.T) {
	for _, name := range []string{"descriptor_catalog.go", "descriptor_sources_manifest.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(data)
		for _, forbidden := range []string{"os/exec", "net.Dial", "go func(", "plugin-host", "task-host", "NewAdapterRegistry", "StartHost"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s references %q; descriptor adapters must stay in memory", name, forbidden)
			}
		}
	}
}
