package plugincontract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const idStabilityGoldenPath = "testdata/id-stability.json"

type idStabilityEntry struct {
	Source       string `json:"source"`
	ProviderID   string `json:"providerId"`
	ServiceID    string `json:"serviceId"`
	CapabilityID string `json:"capabilityId"`
	Version      string `json:"version"`
}

type idStabilitySnapshot struct {
	SchemaVersion int                `json:"schemaVersion"`
	Scope         []string           `json:"scope"`
	Count         int                `json:"count"`
	Entries       []idStabilityEntry `json:"entries"`
}

func idStabilityScope() []string {
	return []string{"builtin.core", "plugins-code/*/amitia-extension.json"}
}

func collectLockedIDEntries(t *testing.T) []idStabilityEntry {
	t.Helper()
	core, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build core catalog: %v", err)
	}
	catalogs := []DescriptorCatalog{
		core.ToCatalog(),
	}
	emote, err := ManifestDescriptorCatalog(*loadEmoteManifest(t))
	if err != nil {
		t.Fatalf("build emote catalog: %v", err)
	}
	catalogs = append(catalogs, emote)
	entries := make([]idStabilityEntry, 0, 8)
	for _, catalog := range catalogs {
		for _, entry := range catalog.Entries {
			entries = append(entries, idStabilityEntry{
				Source:       string(entry.Source),
				ProviderID:   entry.ProviderID,
				ServiceID:    entry.ServiceID,
				CapabilityID: entry.CapabilityID,
				Version:      entry.Version,
			})
		}
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Source != entries[right].Source {
			return entries[left].Source < entries[right].Source
		}
		if entries[left].CapabilityID != entries[right].CapabilityID {
			return entries[left].CapabilityID < entries[right].CapabilityID
		}
		return entries[left].ProviderID < entries[right].ProviderID
	})
	return entries
}

func snapshotOf(entries []idStabilityEntry) idStabilitySnapshot {
	return idStabilitySnapshot{
		SchemaVersion: 1,
		Scope:         idStabilityScope(),
		Count:         len(entries),
		Entries:       entries,
	}
}

func readIDStabilityGolden(t *testing.T) (idStabilitySnapshot, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "id-stability.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return idStabilitySnapshot{}, false
		}
		t.Fatalf("read id stability golden: %v", err)
	}
	var golden idStabilitySnapshot
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse id stability golden: %v", err)
	}
	return golden, true
}

func writeIDStabilityGolden(t *testing.T, snapshot idStabilitySnapshot) {
	t.Helper()
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("marshal id stability golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "id-stability.json"), append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write id stability golden: %v", err)
	}
}

func idStabilityKey(entry idStabilityEntry) string {
	return entry.Source + "|" + entry.CapabilityID + "|" + entry.ProviderID
}

func compareIDStability(golden, live []idStabilityEntry) []string {
	goldenByKey := make(map[string]idStabilityEntry, len(golden))
	for _, entry := range golden {
		goldenByKey[idStabilityKey(entry)] = entry
	}
	liveByKey := make(map[string]idStabilityEntry, len(live))
	for _, entry := range live {
		liveByKey[idStabilityKey(entry)] = entry
	}
	diffs := make([]string, 0, 8)
	for key, want := range goldenByKey {
		got, found := liveByKey[key]
		if !found {
			diffs = append(diffs, fmt.Sprintf("removed %s (source=%s provider=%s service=%s)", want.CapabilityID, want.Source, want.ProviderID, want.ServiceID))
			continue
		}
		switch compareDottedVersion(got.Version, want.Version) {
		case -1:
			diffs = append(diffs, fmt.Sprintf("version downgrade %s: %s -> %s", want.CapabilityID, want.Version, got.Version))
		case 1:
			diffs = append(diffs, fmt.Sprintf("version changed %s: %s -> %s (update golden explicitly)", want.CapabilityID, want.Version, got.Version))
		}
		if got.ServiceID != want.ServiceID {
			diffs = append(diffs, fmt.Sprintf("service changed %s: %s -> %s", want.CapabilityID, want.ServiceID, got.ServiceID))
		}
	}
	for key, got := range liveByKey {
		if _, found := goldenByKey[key]; !found {
			diffs = append(diffs, fmt.Sprintf("added %s (source=%s provider=%s service=%s)", got.CapabilityID, got.Source, got.ProviderID, got.ServiceID))
		}
	}
	sort.Strings(diffs)
	return diffs
}

func compareDottedVersion(left, right string) int {
	leftParts := strings.Split(strings.TrimSpace(left), ".")
	rightParts := strings.Split(strings.TrimSpace(right), ".")
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue := dottedVersionPart(leftParts, index)
		rightValue := dottedVersionPart(rightParts, index)
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

func dottedVersionPart(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(parts[index]))
	if err != nil {
		return 0
	}
	return value
}

func TestIDStabilityGoldenMatchesRepository(t *testing.T) {
	live := collectLockedIDEntries(t)
	if len(live) == 0 {
		t.Fatal("locked capability set is empty")
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeIDStabilityGolden(t, snapshotOf(live))
		t.Logf("updated %s with %d entries", idStabilityGoldenPath, len(live))
		return
	}
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing; run UPDATE_GOLDEN=1 go test -run TestIDStabilityGoldenMatchesRepository", idStabilityGoldenPath)
	}
	diffs := compareIDStability(golden.Entries, live)
	if len(diffs) != 0 {
		for _, diff := range diffs {
			t.Logf("DIFF %s", diff)
		}
		t.Fatalf("capability id stability broken: %d diff(s); %s", len(diffs), strings.Join(diffs, "; "))
	}
	if golden.Count != len(golden.Entries) {
		t.Fatalf("golden count = %d, entries = %d", golden.Count, len(golden.Entries))
	}
	if len(golden.Scope) == 0 {
		t.Fatal("golden scope empty")
	}
}

func TestIDStabilityGoldenCoversBuiltinCoreAndOfficialPluginOnly(t *testing.T) {
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing", idStabilityGoldenPath)
	}
	seen := make(map[string]bool, len(golden.Entries))
	sources := make(map[string]int, 4)
	for _, entry := range golden.Entries {
		sources[entry.Source]++
		if entry.Source != string(DescriptorSourceCore) && !strings.HasPrefix(entry.Source, "manifest.") {
			t.Fatalf("unexpected locked source %q for %s", entry.Source, entry.CapabilityID)
		}
		if err := ValidateCapabilityIdentity(entry.CapabilityID); err != nil {
			t.Fatalf("locked capability id %q invalid: %v", entry.CapabilityID, err)
		}
		if err := ValidateProviderID(entry.ProviderID); err != nil {
			t.Fatalf("locked provider id %q invalid: %v", entry.ProviderID, err)
		}
		if err := ValidateServiceID(entry.ServiceID); err != nil {
			t.Fatalf("locked service id %q invalid: %v", entry.ServiceID, err)
		}
		key := idStabilityKey(entry)
		if seen[key] {
			t.Fatalf("duplicate locked entry %s", key)
		}
		seen[key] = true
	}
	if sources[string(DescriptorSourceCore)] == 0 {
		t.Fatal("golden contains no builtin core entry")
	}
	if sources[string(DescriptorSourceManifestCapability)] == 0 && sources[string(DescriptorSourceManifestUIProvider)] == 0 {
		t.Fatal("golden contains no official plugin entry")
	}
	if !sort.SliceIsSorted(golden.Entries, func(left, right int) bool {
		if golden.Entries[left].Source != golden.Entries[right].Source {
			return golden.Entries[left].Source < golden.Entries[right].Source
		}
		if golden.Entries[left].CapabilityID != golden.Entries[right].CapabilityID {
			return golden.Entries[left].CapabilityID < golden.Entries[right].CapabilityID
		}
		return golden.Entries[left].ProviderID < golden.Entries[right].ProviderID
	}) {
		t.Fatal("golden entries are not sorted")
	}
}

func TestIDStabilityDetectsRenamedSegment(t *testing.T) {
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing", idStabilityGoldenPath)
	}
	renamed := make([]idStabilityEntry, len(golden.Entries))
	copy(renamed, golden.Entries)
	target := -1
	for index, entry := range renamed {
		if strings.HasSuffix(entry.CapabilityID, "/ai") {
			target = index
			break
		}
	}
	if target < 0 {
		t.Fatal("no capability ending in /ai found in golden")
	}
	original := renamed[target].CapabilityID
	segments := strings.Split(original, "/")
	segments[len(segments)-1] = "ai-renamed"
	renamed[target].CapabilityID = strings.Join(segments, "/")
	diffs := compareIDStability(golden.Entries, renamed)
	if len(diffs) == 0 {
		t.Fatal("renaming a capability segment produced no diff")
	}
	joined := strings.Join(diffs, "; ")
	if !strings.Contains(joined, "removed "+original) {
		t.Fatalf("diffs do not report the original id: %s", joined)
	}
	if !strings.Contains(joined, "added "+renamed[target].CapabilityID) {
		t.Fatalf("diffs do not report the renamed id: %s", joined)
	}
}

func TestIDStabilityDetectsVersionDowngrade(t *testing.T) {
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing", idStabilityGoldenPath)
	}
	downgraded := make([]idStabilityEntry, len(golden.Entries))
	copy(downgraded, golden.Entries)
	downgraded[0].Version = "0.0.1"
	if compareDottedVersion(golden.Entries[0].Version, "0.0.1") <= 0 {
		t.Skipf("golden version %q is not above 0.0.1", golden.Entries[0].Version)
	}
	diffs := compareIDStability(golden.Entries, downgraded)
	joined := strings.Join(diffs, "; ")
	if !strings.Contains(joined, "version downgrade "+golden.Entries[0].CapabilityID) {
		t.Fatalf("downgrade not reported: %s", joined)
	}
}

func TestIDStabilityForwardVersionRequiresExplicitGoldenUpdate(t *testing.T) {
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing", idStabilityGoldenPath)
	}
	bumped := make([]idStabilityEntry, len(golden.Entries))
	copy(bumped, golden.Entries)
	bumped[0].Version = "99.0.0"
	diffs := compareIDStability(golden.Entries, bumped)
	joined := strings.Join(diffs, "; ")
	if !strings.Contains(joined, "version changed "+golden.Entries[0].CapabilityID) {
		t.Fatalf("forward version change not reported: %s", joined)
	}
	if strings.Contains(joined, "downgrade") {
		t.Fatalf("forward version bump misreported as downgrade: %s", joined)
	}
}

func TestIDStabilitySnapshotIsDeterministic(t *testing.T) {
	first, err := json.Marshal(snapshotOf(collectLockedIDEntries(t)))
	if err != nil {
		t.Fatalf("marshal first snapshot: %v", err)
	}
	second, err := json.Marshal(snapshotOf(collectLockedIDEntries(t)))
	if err != nil {
		t.Fatalf("marshal second snapshot: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("id stability snapshot is not deterministic:\n%s\n%s", first, second)
	}
}

func TestIDStabilityIgnoresThirdPartyCapabilities(t *testing.T) {
	golden, found := readIDStabilityGolden(t)
	if !found {
		t.Fatalf("%s missing", idStabilityGoldenPath)
	}
	thirdParty := idStabilityEntry{
		Source:       string(DescriptorSourceManifestCapability),
		ProviderID:   "third.party/com.example/third-party/cap",
		ServiceID:    "com.example/third-party",
		CapabilityID: "com.example/third-party/cap",
		Version:      "1.0.0",
	}
	live := append(append([]idStabilityEntry(nil), golden.Entries...), thirdParty)
	diffs := compareIDStability(golden.Entries, live)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "added "+thirdParty.CapabilityID) {
		t.Fatalf("third-party entry must surface as a single added diff, got %v", diffs)
	}
	for _, entry := range golden.Entries {
		if entry.CapabilityID == thirdParty.CapabilityID {
			t.Fatal("third-party capability leaked into golden")
		}
	}
}
