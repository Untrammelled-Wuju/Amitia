package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

const (
	regressionEmoteSnapshotSHA = "98b60bf1d46d271d51a3c33063c6d9b3414fd793b3b90ac39ffee7257390a74e"
	regressionEmoteListingSHA  = "7016a5c2df81450409fed64e3bbe75c482a359fbc5c64571717e052d04d1af4b"
	regressionEmoteAIContract  = "9d304e7b440103b58582b7cf620b2e6ef55ef103016a5131f2b27f2d23e71815"
	regressionCoreSnapshotSHA  = "ad1c49d32d7309ec831acc9376b36d0132d05a1fd30b7b43192b1f49cffabecb"
	regressionCoreMemorySearch = "15a85f82de71eb57bafcef70bd45140becc9ca138b6f0f9e54717ba2799a9237"
)

func digestBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestContractRegressionRealManifestSnapshotIsPinned(t *testing.T) {
	manifest := loadEmoteManifest(t)
	catalog, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build emote catalog: %v", err)
	}
	if catalog.Len() != 5 {
		t.Fatalf("entries = %d, want 5", catalog.Len())
	}
	snapshot, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot) != 2440 {
		t.Fatalf("snapshot bytes = %d, want 2440", len(snapshot))
	}
	if digestBytes(snapshot) != regressionEmoteSnapshotSHA {
		t.Fatalf("snapshot sha = %s, want %s", digestBytes(snapshot), regressionEmoteSnapshotSHA)
	}
	revision, err := catalog.Revision()
	if err != nil {
		t.Fatalf("revision: %v", err)
	}
	if revision != regressionEmoteSnapshotSHA {
		t.Fatalf("revision = %s, want snapshot sha %s", revision, regressionEmoteSnapshotSHA)
	}
	listing, err := json.Marshal(catalog.Listing())
	if err != nil {
		t.Fatalf("marshal listing: %v", err)
	}
	if len(listing) != 880 {
		t.Fatalf("listing bytes = %d, want 880", len(listing))
	}
	if digestBytes(listing) != regressionEmoteListingSHA {
		t.Fatalf("listing sha = %s, want %s", digestBytes(listing), regressionEmoteListingSHA)
	}
	entry, found := catalog.FindByCapabilityID("com.amitia/emote/ai")
	if !found {
		t.Fatal("com.amitia/emote/ai missing from emote catalog")
	}
	if entry.ContractHash != regressionEmoteAIContract {
		t.Fatalf("com.amitia/emote/ai contract hash = %s, want %s", entry.ContractHash, regressionEmoteAIContract)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("validate emote catalog: %v", err)
	}
}

func TestContractRegressionCoreCatalogSnapshotIsPinned(t *testing.T) {
	core, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build core catalog: %v", err)
	}
	catalog := core.ToCatalog()
	if catalog.Len() != 1 {
		t.Fatalf("core entries = %d, want 1", catalog.Len())
	}
	snapshot, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("core snapshot: %v", err)
	}
	if len(snapshot) != 7934 {
		t.Fatalf("core snapshot bytes = %d, want 7934", len(snapshot))
	}
	if digestBytes(snapshot) != regressionCoreSnapshotSHA {
		t.Fatalf("core snapshot sha = %s, want %s", digestBytes(snapshot), regressionCoreSnapshotSHA)
	}
	revision, err := catalog.Revision()
	if err != nil {
		t.Fatalf("core revision: %v", err)
	}
	if revision != regressionCoreSnapshotSHA {
		t.Fatalf("core revision = %s, want %s", revision, regressionCoreSnapshotSHA)
	}
	entry, found := catalog.FindByCapabilityID("builtin/core/memory.search")
	if !found {
		t.Fatal("builtin/core/memory.search missing from core catalog")
	}
	if entry.ContractHash != regressionCoreMemorySearch {
		t.Fatalf("memory.search contract hash = %s, want %s", entry.ContractHash, regressionCoreMemorySearch)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("validate core catalog: %v", err)
	}
}

func TestContractRegressionSingleSourceMergeMatchesFullCatalog(t *testing.T) {
	manifest := loadEmoteManifest(t)
	full, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build full catalog: %v", err)
	}
	capabilities, err := ManifestCapabilityDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build capability catalog: %v", err)
	}
	skills, err := ManifestSkillDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build skill catalog: %v", err)
	}
	providers, err := ManifestUIProviderDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build ui provider catalog: %v", err)
	}
	if capabilities.Len()+skills.Len()+providers.Len() != full.Len() {
		t.Fatalf("single source entries %d+%d+%d != full %d", capabilities.Len(), skills.Len(), providers.Len(), full.Len())
	}
	combined := DescriptorCatalog{
		Entries: make([]DescriptorEntry, 0, full.Len()),
		Schemas: make(map[string]json.RawMessage),
	}
	combined.Entries = append(combined.Entries, capabilities.Entries...)
	combined.Entries = append(combined.Entries, skills.Entries...)
	combined.Entries = append(combined.Entries, providers.Entries...)
	for _, part := range []DescriptorCatalog{capabilities, skills, providers} {
		for hash, schema := range part.Schemas {
			combined.Schemas[hash] = schema
		}
	}
	if err := combined.Validate(); err != nil {
		t.Fatalf("validate merged catalog: %v", err)
	}
	mergedSnapshot, err := combined.Snapshot()
	if err != nil {
		t.Fatalf("merged snapshot: %v", err)
	}
	fullSnapshot, err := full.Snapshot()
	if err != nil {
		t.Fatalf("full snapshot: %v", err)
	}
	if digestBytes(mergedSnapshot) != digestBytes(fullSnapshot) {
		t.Fatalf("merged snapshot sha %s != full %s", digestBytes(mergedSnapshot), digestBytes(fullSnapshot))
	}
	mergedRevision, err := combined.Revision()
	if err != nil {
		t.Fatalf("merged revision: %v", err)
	}
	fullRevision, err := full.Revision()
	if err != nil {
		t.Fatalf("full revision: %v", err)
	}
	if mergedRevision != fullRevision {
		t.Fatalf("merged revision %s != full %s", mergedRevision, fullRevision)
	}
}

func TestContractRegressionRepeatedBuildIsByteIdentical(t *testing.T) {
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
	if digestBytes(firstSnapshot) != digestBytes(secondSnapshot) {
		t.Fatalf("repeated build snapshot sha %s != %s", digestBytes(firstSnapshot), digestBytes(secondSnapshot))
	}
	firstRevision, err := first.Revision()
	if err != nil {
		t.Fatalf("first revision: %v", err)
	}
	secondRevision, err := second.Revision()
	if err != nil {
		t.Fatalf("second revision: %v", err)
	}
	if firstRevision != secondRevision {
		t.Fatalf("repeated build revision %s != %s", firstRevision, secondRevision)
	}
	diff, err := DiffCatalogs(first, second)
	if err != nil {
		t.Fatalf("repeated build diff: %v", err)
	}
	if !diff.Empty() {
		t.Fatalf("repeated build diff = %d entries, want empty", diff.Len())
	}
	firstQuery, err := json.Marshal(first.SelectByPermission("memory.read"))
	if err != nil {
		t.Fatalf("marshal first query: %v", err)
	}
	secondQuery, err := json.Marshal(second.SelectByPermission("memory.read"))
	if err != nil {
		t.Fatalf("marshal second query: %v", err)
	}
	if digestBytes(firstQuery) != digestBytes(secondQuery) {
		t.Fatal("permission query differs between repeated builds")
	}
	if len(first.SelectByPermission("memory.read")) != 0 {
		t.Fatal("real emote manifest must not declare memory.read")
	}
}

func TestContractRegressionNegativeVersionBumpIsDetectable(t *testing.T) {
	manifest := loadEmoteManifest(t)
	previous, err := ManifestDescriptorCatalog(*manifest)
	if err != nil {
		t.Fatalf("build previous catalog: %v", err)
	}
	mutated := *manifest
	mutated.Modules = make([]Module, len(manifest.Modules))
	copy(mutated.Modules, manifest.Modules)
	bumped := false
	for moduleIndex := range mutated.Modules {
		mutated.Modules[moduleIndex].ProvidedCapabilities = append([]ProvidedCapability(nil), manifest.Modules[moduleIndex].ProvidedCapabilities...)
		for capabilityIndex := range mutated.Modules[moduleIndex].ProvidedCapabilities {
			if mutated.Modules[moduleIndex].ProvidedCapabilities[capabilityIndex].ID == "com.amitia/emote/ai" {
				mutated.Modules[moduleIndex].ProvidedCapabilities[capabilityIndex].Version = "9.9.9"
				bumped = true
			}
		}
	}
	if !bumped {
		t.Fatal("com.amitia/emote/ai not found in manifest modules")
	}
	next, err := ManifestDescriptorCatalog(mutated)
	if err != nil {
		t.Fatalf("build mutated catalog: %v", err)
	}
	previousRevision, err := previous.Revision()
	if err != nil {
		t.Fatalf("previous revision: %v", err)
	}
	nextRevision, err := next.Revision()
	if err != nil {
		t.Fatalf("next revision: %v", err)
	}
	if previousRevision == nextRevision {
		t.Fatal("version bump must change revision")
	}
	diff, err := DiffCatalogs(previous, next)
	if err != nil {
		t.Fatalf("version bump diff: %v", err)
	}
	if len(diff.Changed) == 0 {
		t.Fatalf("version bump diff has no changed entries: %+v", diff)
	}
	change := diff.Changed[0]
	if change.CapabilityID != "com.amitia/emote/ai" {
		t.Fatalf("changed capability = %s, want com.amitia/emote/ai", change.CapabilityID)
	}
	if len(change.Fields) != 2 || change.Fields[0] != "version" || change.Fields[1] != "contractHash" {
		t.Fatalf("changed fields = %v, want [version contractHash]", change.Fields)
	}
	if change.InputSchemaChanged || change.OutputSchemaChanged {
		t.Fatal("version bump must not be reported as schema change")
	}
}

func TestContractRegressionAllSourceCatalogsValidate(t *testing.T) {
	manifest := loadEmoteManifest(t)
	catalogs := map[string]DescriptorCatalog{}
	for name, build := range map[string]func(Manifest) (DescriptorCatalog, error){
		"full":        ManifestDescriptorCatalog,
		"capability":  ManifestCapabilityDescriptorCatalog,
		"skill":       ManifestSkillDescriptorCatalog,
		"ui_provider": ManifestUIProviderDescriptorCatalog,
	} {
		catalog, err := build(*manifest)
		if err != nil {
			t.Fatalf("build %s catalog: %v", name, err)
		}
		catalogs[name] = catalog
	}
	core, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build core catalog: %v", err)
	}
	catalogs["core"] = core.ToCatalog()
	for name, catalog := range catalogs {
		if err := catalog.Validate(); err != nil {
			t.Fatalf("validate %s catalog: %v", name, err)
		}
		if conflicts := catalog.ConflictingEntries(); len(conflicts) != 0 {
			t.Fatalf("%s catalog conflicts = %d, want 0", name, len(conflicts))
		}
		revision, err := catalog.Revision()
		if err != nil {
			t.Fatalf("%s revision: %v", name, err)
		}
		if len(revision) != 64 {
			t.Fatalf("%s revision length = %d, want 64", name, len(revision))
		}
	}
	if len(catalogs["skill"].Entries) != 0 {
		t.Fatalf("real emote manifest must not produce skill entries, got %d", len(catalogs["skill"].Entries))
	}
}
