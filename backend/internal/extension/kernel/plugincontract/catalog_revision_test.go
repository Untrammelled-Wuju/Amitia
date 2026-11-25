package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func diffManifest(version string, extraCapability bool, bumpAlpha bool, schema string) Manifest {
	alphaCapabilities := []ProvidedCapability{
		{ID: "com.example/diff/alpha/one", Version: "1.0.0", Metadata: map[string]any{"inputSchema": json.RawMessage(schema)}},
		{ID: "com.example/diff/alpha/two", Version: "1.0.0"},
	}
	if bumpAlpha {
		alphaCapabilities[0].Version = "1.1.0"
	}
	modules := []Module{
		{ID: "alpha", Name: LocalizedText{Default: "Alpha"}, ProvidedCapabilities: alphaCapabilities},
		{ID: "beta", Name: LocalizedText{Default: "Beta"}, ProvidedCapabilities: []ProvidedCapability{{ID: "com.example/diff/beta/one", Version: "1.0.0"}}},
	}
	if extraCapability {
		modules = append(modules, Module{ID: "gamma", Name: LocalizedText{Default: "Gamma"}, ProvidedCapabilities: []ProvidedCapability{{ID: "com.example/diff/gamma/one", Version: "1.0.0"}}})
	}
	manifest := Manifest{
		Plugin:  PluginMeta{ID: "com.example/diff", Version: version},
		Modules: modules,
	}
	return manifest
}

func buildDiffCatalog(t *testing.T, manifest Manifest) DescriptorCatalog {
	t.Helper()
	catalog, err := ManifestCapabilityDescriptorCatalog(manifest)
	if err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	return catalog
}

func handEntry(source DescriptorSource, capabilityID, providerID, serviceID string) DescriptorEntry {
	return DescriptorEntry{
		Source:       source,
		ProviderID:   providerID,
		CapabilityID: capabilityID,
		ServiceID:    serviceID,
		Version:      "1.0.0",
		Descriptor: Descriptor{
			ID:        capabilityID,
			Summary:   "hand built",
			Tags:      []string{"capability", "plugin"},
			Scopes:    []ServiceScope{ServiceScopeDevice, ServiceScopePlugin},
			Placement: ServicePlacementDevice,
		},
	}
}

func TestCatalogRevisionIsContentAddressed(t *testing.T) {
	manifest := diffManifest("1.0.0", false, false, `{"type":"object"}`)
	catalog := buildDiffCatalog(t, manifest)
	first, err := CatalogRevision(catalog)
	if err != nil {
		t.Fatalf("revision: %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("revision = %q", first)
	}
	second, err := CatalogRevision(catalog)
	if err != nil {
		t.Fatalf("revision again: %v", err)
	}
	if first != second {
		t.Fatalf("revision is not stable: %s != %s", first, second)
	}
	snapshot, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	sum := sha256.Sum256(snapshot)
	if first != hex.EncodeToString(sum[:]) {
		t.Fatalf("revision %s does not match snapshot digest %s", first, hex.EncodeToString(sum[:]))
	}
	rebuilt := buildDiffCatalog(t, manifest)
	rebuiltRevision, err := rebuilt.Revision()
	if err != nil {
		t.Fatalf("rebuilt revision: %v", err)
	}
	if rebuiltRevision != first {
		t.Fatalf("rebuilt revision = %s, want %s", rebuiltRevision, first)
	}
}

func TestCatalogRevisionTracksAddedRemovedAndVersion(t *testing.T) {
	base := buildDiffCatalog(t, diffManifest("1.0.0", false, false, `{"type":"object"}`))
	baseRevision, err := CatalogRevision(base)
	if err != nil {
		t.Fatalf("base revision: %v", err)
	}
	bumped := buildDiffCatalog(t, diffManifest("1.0.0", false, true, `{"type":"object"}`))
	bumpedRevision, err := CatalogRevision(bumped)
	if err != nil {
		t.Fatalf("bumped revision: %v", err)
	}
	if bumpedRevision == baseRevision {
		t.Fatal("version bump did not change the revision")
	}
	added := buildDiffCatalog(t, diffManifest("1.0.0", true, false, `{"type":"object"}`))
	addedRevision, err := CatalogRevision(added)
	if err != nil {
		t.Fatalf("added revision: %v", err)
	}
	if addedRevision == baseRevision {
		t.Fatal("added capability did not change the revision")
	}
	empty := DescriptorCatalog{Entries: []DescriptorEntry{}, Schemas: map[string]json.RawMessage{}}
	emptyRevision, err := CatalogRevision(empty)
	if err != nil {
		t.Fatalf("empty revision: %v", err)
	}
	if emptyRevision == baseRevision {
		t.Fatal("empty catalog shares the base revision")
	}
}

func TestDiffCatalogsReportsOnlyTouchedEntries(t *testing.T) {
	previous := buildDiffCatalog(t, diffManifest("1.0.0", false, false, `{"type":"object"}`))
	next := buildDiffCatalog(t, diffManifest("2.0.0", true, true, `{"type":"object","required":["query"]}`))
	diff, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff.PreviousRevision == diff.NextRevision {
		t.Fatal("revisions must differ")
	}
	if diff.Empty() {
		t.Fatal("diff reported no changes")
	}
	if len(diff.Added) != 1 || diff.Added[0].CapabilityID != "com.example/diff/gamma/one" {
		t.Fatalf("added = %v", diff.Added)
	}
	// beta/one 的 serviceId 与 gamma/one 不同、alpha 两条未变
	if got := strings.Join(diff.TouchedCapabilityIDs(), ","); got != "com.example/diff/alpha/one,com.example/diff/gamma/one" {
		t.Fatalf("touched = %q", got)
	}
	if len(diff.Changed) != 1 {
		t.Fatalf("changed = %d, want 1 (%v)", len(diff.Changed), diff.Changed)
	}
	change := diff.Changed[0]
	if change.CapabilityID != "com.example/diff/alpha/one" {
		t.Fatalf("changed capability = %q", change.CapabilityID)
	}
	if got := strings.Join(change.Fields, ","); got != "version,contractHash" {
		t.Fatalf("changed fields = %q", got)
	}
	if !change.InputSchemaChanged {
		t.Fatal("input schema change not reported")
	}
	if change.OutputSchemaChanged {
		t.Fatal("output schema change reported without schema")
	}
	if diff.Len() != 2 {
		t.Fatalf("diff len = %d, want 2", diff.Len())
	}
}

func TestDiffCatalogsReportsRemovedEntries(t *testing.T) {
	previous := buildDiffCatalog(t, diffManifest("1.0.0", true, false, `{"type":"object"}`))
	next := buildDiffCatalog(t, diffManifest("1.0.0", false, false, `{"type":"object"}`))
	diff, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].CapabilityID != "com.example/diff/gamma/one" {
		t.Fatalf("removed = %v", diff.Removed)
	}
	if len(diff.Added) != 0 || len(diff.Changed) != 0 {
		t.Fatalf("added = %v changed = %v", diff.Added, diff.Changed)
	}
}

func TestDiffCatalogsEmptyForIdenticalCatalogs(t *testing.T) {
	previous := buildDiffCatalog(t, diffManifest("1.0.0", true, false, `{"type":"object"}`))
	next := buildDiffCatalog(t, diffManifest("1.0.0", true, false, `{"type":"object"}`))
	diff, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !diff.Empty() || diff.Len() != 0 {
		t.Fatalf("identical catalogs produced %d changes", diff.Len())
	}
	if diff.PreviousRevision != diff.NextRevision {
		t.Fatal("identical catalogs must share a revision")
	}
	if len(diff.TouchedCapabilityIDs()) != 0 {
		t.Fatalf("touched = %v", diff.TouchedCapabilityIDs())
	}
}

func TestDiffCatalogsReportsPlacementScopesAndPermissions(t *testing.T) {
	base := Manifest{
		Plugin: PluginMeta{ID: "com.example/perms", Version: "1.0.0"},
		Modules: []Module{{
			ID:          "runtime",
			Name:        LocalizedText{Default: "Runtime"},
			Placement:   ServicePlacementDevice,
			Permissions: []Permission{{Name: "memory.read"}},
			ProvidedCapabilities: []ProvidedCapability{{
				ID: "com.example/perms/runtime/one", Version: "1.0.0",
			}},
		}},
	}
	next := Manifest{
		Plugin: PluginMeta{ID: "com.example/perms", Version: "1.0.0"},
		Modules: []Module{{
			ID:          "runtime",
			Name:        LocalizedText{Default: "Runtime"},
			Placement:   ServicePlacementCloud,
			Permissions: []Permission{{Name: "memory.write"}},
			ProvidedCapabilities: []ProvidedCapability{{
				ID: "com.example/perms/runtime/one", Version: "1.0.0",
			}},
		}},
	}
	diff, err := DiffCatalogs(buildDiffCatalog(t, base), buildDiffCatalog(t, next))
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Changed) != 1 {
		t.Fatalf("changed = %d", len(diff.Changed))
	}
	fields := diff.Changed[0].Fields
	for _, expected := range []string{"placement", "scopes", "permissions", "contractHash"} {
		found := false
		for _, field := range fields {
			if field == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("field %q missing from %v", expected, fields)
		}
	}
}

func TestDiffCatalogsReportsSourceChange(t *testing.T) {
	previous := DescriptorCatalog{Entries: []DescriptorEntry{handEntry(DescriptorSourceCore, "com.example/src/runtime/one", "com.example/src/runtime/one.provider", "one")}, Schemas: map[string]json.RawMessage{}}
	next := DescriptorCatalog{Entries: []DescriptorEntry{handEntry(DescriptorSourceManifestCapability, "com.example/src/runtime/one", "com.example/src/runtime/one.provider", "one")}, Schemas: map[string]json.RawMessage{}}
	diff, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diff.Changed) != 1 {
		t.Fatalf("changed = %d", len(diff.Changed))
	}
	if got := strings.Join(diff.Changed[0].Fields, ","); got != "source" {
		t.Fatalf("changed fields = %q", got)
	}
}

func TestDetectCatalogConflictsDuplicateCapability(t *testing.T) {
	catalog := DescriptorCatalog{
		Entries: []DescriptorEntry{
			handEntry(DescriptorSourceCore, "com.example/dup/runtime/one", "com.example/dup/runtime/one.provider", "one"),
			handEntry(DescriptorSourceManifestCapability, "com.example/dup/runtime/one", "com.example/dup/runtime/one.provider", "one"),
		},
		Schemas: map[string]json.RawMessage{},
	}
	conflicts := DetectCatalogConflicts(catalog)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].Kind != CatalogConflictDuplicateCapability {
		t.Fatalf("kind = %q", conflicts[0].Kind)
	}
	if conflicts[0].Existing != DescriptorSourceCore || conflicts[0].Incoming != DescriptorSourceManifestCapability {
		t.Fatalf("sources = %s/%s", conflicts[0].Existing, conflicts[0].Incoming)
	}
	if !errors.Is(conflicts[0], ErrCatalogConflict) {
		t.Fatalf("conflict is not ErrCatalogConflict: %v", conflicts[0])
	}
}

func TestDetectCatalogConflictsDuplicateProvider(t *testing.T) {
	catalog := DescriptorCatalog{
		Entries: []DescriptorEntry{
			handEntry(DescriptorSourceManifestCapability, "com.example/dup/runtime/one", "com.example/shared/provider", "one"),
			handEntry(DescriptorSourceManifestUIProvider, "com.example/dup/ui/two", "com.example/shared/provider", "two"),
		},
		Schemas: map[string]json.RawMessage{},
	}
	conflicts := DetectCatalogConflicts(catalog)
	if len(conflicts) != 1 || conflicts[0].Kind != CatalogConflictDuplicateProvider {
		t.Fatalf("conflicts = %v", conflicts)
	}
	if conflicts[0].ProviderID != "com.example/shared/provider" {
		t.Fatalf("provider id = %q", conflicts[0].ProviderID)
	}
}

func TestDetectCatalogConflictsServiceMismatch(t *testing.T) {
	catalog := DescriptorCatalog{
		Entries: []DescriptorEntry{
			handEntry(DescriptorSourceManifestCapability, "com.example/dup/runtime/one", "com.example/dup/runtime/one.provider", "search"),
			handEntry(DescriptorSourceManifestSkill, "com.example/dup/skill/two", "com.example/dup/skill/two.provider", "search"),
		},
		Schemas: map[string]json.RawMessage{},
	}
	conflicts := DetectCatalogConflicts(catalog)
	if len(conflicts) != 1 || conflicts[0].Kind != CatalogConflictServiceMismatch {
		t.Fatalf("conflicts = %v", conflicts)
	}
	if conflicts[0].ServiceID != "search" || conflicts[0].ProviderID != "com.example/dup/runtime/one" {
		t.Fatalf("conflict detail = %+v", conflicts[0])
	}
}

func TestDetectCatalogConflictsCleanRealCatalog(t *testing.T) {
	catalog, err := ManifestDescriptorCatalog(*loadEmoteManifest(t))
	if err != nil {
		t.Fatalf("build emote catalog: %v", err)
	}
	if conflicts := DetectCatalogConflicts(catalog); len(conflicts) != 0 {
		t.Fatalf("real catalog reported %d conflicts: %v", len(conflicts), conflicts)
	}
	core, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build core catalog: %v", err)
	}
	if conflicts := DetectCatalogConflicts(core.ToCatalog()); len(conflicts) != 0 {
		t.Fatalf("core catalog reported conflicts: %v", conflicts)
	}
}

func TestDiffCatalogsIsDeterministic(t *testing.T) {
	previous := buildDiffCatalog(t, diffManifest("1.0.0", true, false, `{"type":"object"}`))
	next := buildDiffCatalog(t, diffManifest("2.0.0", true, true, `{"type":"object","required":["query"]}`))
	first, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("first diff: %v", err)
	}
	second, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("second diff: %v", err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("diff output is not deterministic")
	}
}
