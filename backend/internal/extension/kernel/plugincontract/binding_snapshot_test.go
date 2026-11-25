package plugincontract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func deviceBinding(capabilityID, spaceID, deviceID, providerID string) Binding {
	return Binding{
		Key: BindingKey{
			CapabilityID: capabilityID,
			SpaceID:      spaceID,
			DeviceID:     deviceID,
			Scope:        BindingScopeDevice,
		},
		ProviderID:         providerID,
		ProviderInstanceID: providerID + ":instance",
	}
}

func spaceBinding(capabilityID, spaceID, providerID string) Binding {
	return Binding{
		Key: BindingKey{
			CapabilityID: capabilityID,
			SpaceID:      spaceID,
			Scope:        BindingScopeSpace,
		},
		ProviderID:         providerID,
		ProviderInstanceID: providerID + ":instance",
	}
}

func globalBinding(capabilityID, providerID string) Binding {
	return Binding{
		Key: BindingKey{
			CapabilityID: capabilityID,
			Scope:        BindingScopeGlobal,
		},
		ProviderID:         providerID,
		ProviderInstanceID: providerID + ":instance",
	}
}

func TestBindingSnapshotIsImmutable(t *testing.T) {
	bindings := []Binding{
		globalBinding("memory.search", "builtin.core"),
		deviceBinding("memory.search", "space-1", "device-1", "device.host"),
	}
	snapshot, err := NewBindingSnapshot("boot-1", 1, bindings)
	if err != nil {
		t.Fatalf("NewBindingSnapshot: %v", err)
	}
	bindings[0] = globalBinding("memory.delete", "mutated.core")
	if snapshot.Len() != 2 {
		t.Fatalf("snapshot length changed with caller slice: %d", snapshot.Len())
	}
	if _, ok := snapshot.Lookup(BindingKey{CapabilityID: "memory.search", Scope: BindingScopeGlobal}); !ok {
		t.Fatalf("snapshot lost global binding after caller mutation")
	}
	exposed := snapshot.Bindings()
	exposed[0] = globalBinding("memory.delete", "mutated.core")
	if _, ok := snapshot.Lookup(BindingKey{CapabilityID: "memory.search", Scope: BindingScopeGlobal}); !ok {
		t.Fatalf("snapshot mutated through Bindings copy")
	}
	if _, ok := snapshot.Lookup(BindingKey{CapabilityID: "memory.delete", Scope: BindingScopeGlobal}); ok {
		t.Fatalf("snapshot accepted mutation from Bindings copy")
	}
}

func TestBindingSnapshotScopeOrder(t *testing.T) {
	snapshot, err := NewBindingSnapshot("boot-1", 7, []Binding{
		globalBinding("memory.search", "global.host"),
		spaceBinding("memory.search", "space-1", "space.host"),
		deviceBinding("memory.search", "space-1", "device-1", "device.host"),
	})
	if err != nil {
		t.Fatalf("NewBindingSnapshot: %v", err)
	}
	request := BindingRequest{CapabilityID: "memory.search", SpaceID: "space-1", DeviceID: "device-1"}
	resolution, err := snapshot.Resolve(request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolution.Scope != BindingScopeDevice || resolution.Binding.ProviderID != "device.host" {
		t.Fatalf("expected device binding first, got scope=%s provider=%s", resolution.Scope, resolution.Binding.ProviderID)
	}
	if resolution.Revision != 7 || resolution.Epoch != "boot-1" {
		t.Fatalf("resolution carries revision=%d epoch=%s", resolution.Revision, resolution.Epoch)
	}
	if strings.Join(resolution.Trail, ",") != "device=hit" {
		t.Fatalf("unexpected trail %v", resolution.Trail)
	}

	spaceOnly, err := snapshot.Resolve(BindingRequest{CapabilityID: "memory.search", SpaceID: "space-1"})
	if err != nil {
		t.Fatalf("Resolve space: %v", err)
	}
	if spaceOnly.Scope != BindingScopeSpace || spaceOnly.Binding.ProviderID != "space.host" {
		t.Fatalf("expected space binding, got scope=%s provider=%s", spaceOnly.Scope, spaceOnly.Binding.ProviderID)
	}
	if strings.Join(spaceOnly.Trail, ",") != "device=skip,space=hit" {
		t.Fatalf("unexpected space trail %v", spaceOnly.Trail)
	}

	globalOnly, err := snapshot.Resolve(BindingRequest{CapabilityID: "memory.search"})
	if err != nil {
		t.Fatalf("Resolve global: %v", err)
	}
	if globalOnly.Scope != BindingScopeGlobal || globalOnly.Binding.ProviderID != "global.host" {
		t.Fatalf("expected global binding, got scope=%s provider=%s", globalOnly.Scope, globalOnly.Binding.ProviderID)
	}
	if strings.Join(globalOnly.Trail, ",") != "device=skip,space=skip,global=hit" {
		t.Fatalf("unexpected global trail %v", globalOnly.Trail)
	}
}

func TestBindingSnapshotStrictMiss(t *testing.T) {
	snapshot, err := NewBindingSnapshot("boot-1", 3, []Binding{deviceBinding("memory.search", "space-1", "device-1", "device.host")})
	if err != nil {
		t.Fatalf("NewBindingSnapshot: %v", err)
	}
	_, err = snapshot.Resolve(BindingRequest{CapabilityID: "memory.search", SpaceID: "space-1", DeviceID: "device-2"})
	if !errors.Is(err, ErrBindingNotFound) {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
	var notFound *BindingNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected typed BindingNotFoundError, got %T", err)
	}
	if notFound.Revision != 3 || notFound.Epoch != "boot-1" {
		t.Fatalf("not-found error lost snapshot identity: revision=%d epoch=%s", notFound.Revision, notFound.Epoch)
	}
	message := err.Error()
	for _, want := range []string{"device-2", "device", "space", "global"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error message %q does not mention %q", message, want)
		}
	}
	if _, err := snapshot.Resolve(BindingRequest{}); !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("expected ErrBindingInvalid for empty capability, got %v", err)
	}
	_, err = snapshot.Resolve(BindingRequest{CapabilityID: "memory.search"})
	if !errors.Is(err, ErrBindingNotFound) {
		t.Fatalf("expected ErrBindingNotFound without identifiers, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing spaceId, deviceId") {
		t.Fatalf("error should name missing identifiers, got %q", err.Error())
	}
}

func TestBindingSnapshotRevisionAndEpoch(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	first, err := store.Install([]Binding{globalBinding("memory.search", "global.host")})
	if err != nil {
		t.Fatalf("Install first: %v", err)
	}
	if first.Revision() != 1 || first.Epoch() != "boot-1" {
		t.Fatalf("first install revision=%d epoch=%s", first.Revision(), first.Epoch())
	}
	second, err := store.Install([]Binding{globalBinding("memory.search", "global.host")})
	if err != nil {
		t.Fatalf("Install second: %v", err)
	}
	if second.Revision() != 2 {
		t.Fatalf("second install revision=%d", second.Revision())
	}
	if second.Digest() != first.Digest() {
		t.Fatalf("identical bindings should keep digest: %s vs %s", second.Digest(), first.Digest())
	}
	rebooted, err := NewBindingSnapshotStore("boot-2")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore reboot: %v", err)
	}
	third, err := rebooted.Install([]Binding{globalBinding("memory.search", "global.host")})
	if err != nil {
		t.Fatalf("Install after reboot: %v", err)
	}
	if third.Revision() != 1 || third.Epoch() != "boot-2" {
		t.Fatalf("revision/epoch should restart per boot, got revision=%d epoch=%s", third.Revision(), third.Epoch())
	}
}

func TestBindingSnapshotRetainsInflightCalls(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Install([]Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Install rev1: %v", err)
	}
	handle, err := store.Acquire()
	if err != nil {
		t.Fatalf("Acquire rev1: %v", err)
	}
	second, err := store.Acquire()
	if err != nil {
		t.Fatalf("Acquire rev1 again: %v", err)
	}
	if _, err := store.Install([]Binding{globalBinding("memory.search", "next.host")}); err != nil {
		t.Fatalf("Install rev2: %v", err)
	}
	if _, err := store.Install([]Binding{globalBinding("memory.search", "third.host")}); err != nil {
		t.Fatalf("Install rev3: %v", err)
	}
	resolution, err := handle.Resolve(BindingRequest{CapabilityID: "memory.search"})
	if err != nil {
		t.Fatalf("inflight resolve: %v", err)
	}
	if resolution.Revision != 1 || resolution.Binding.ProviderID != "global.host" {
		t.Fatalf("inflight call moved to revision=%d provider=%s", resolution.Revision, resolution.Binding.ProviderID)
	}
	current, err := store.Current().Resolve(BindingRequest{CapabilityID: "memory.search"})
	if err != nil {
		t.Fatalf("current resolve: %v", err)
	}
	if current.Revision != 3 || current.Binding.ProviderID != "third.host" {
		t.Fatalf("current snapshot should be revision 3, got revision=%d provider=%s", current.Revision, current.Binding.ProviderID)
	}
	if revisions := store.RetainedRevisions(); len(revisions) != 1 || revisions[0] != 1 {
		t.Fatalf("retained revisions = %v, want [1]", revisions)
	}
	handle.Release()
	handle.Release()
	if revisions := store.RetainedRevisions(); len(revisions) != 1 || revisions[0] != 1 {
		t.Fatalf("retained snapshots = %v after first release, want [1]", revisions)
	}
	if _, err := second.Resolve(BindingRequest{CapabilityID: "memory.search"}); err != nil {
		t.Fatalf("second inflight call should still resolve: %v", err)
	}
	second.Release()
	second.Release()
	if store.RetainedCount() != 0 {
		t.Fatalf("retained snapshots = %v after release, want empty", store.RetainedRevisions())
	}
	if _, err := handle.Resolve(BindingRequest{CapabilityID: "memory.search"}); !errors.Is(err, ErrSnapshotRelease) {
		t.Fatalf("released handle should refuse resolution, got %v", err)
	}
	if _, err := store.Acquire(); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
}

func TestBindingSnapshotRejectsInvalidBindings(t *testing.T) {
	cases := []struct {
		name     string
		bindings []Binding
	}{
		{name: "empty capability", bindings: []Binding{{Key: BindingKey{Scope: BindingScopeGlobal}, ProviderID: "p", ProviderInstanceID: "i"}}},
		{name: "empty provider", bindings: []Binding{{Key: BindingKey{CapabilityID: "c", Scope: BindingScopeGlobal}, ProviderInstanceID: "i"}}},
		{name: "unknown scope", bindings: []Binding{{Key: BindingKey{CapabilityID: "c", Scope: BindingScope("cluster")}, ProviderID: "p", ProviderInstanceID: "i"}}},
		{name: "device without device id", bindings: []Binding{{Key: BindingKey{CapabilityID: "c", SpaceID: "s", Scope: BindingScopeDevice}, ProviderID: "p", ProviderInstanceID: "i"}}},
		{name: "space with device id", bindings: []Binding{{Key: BindingKey{CapabilityID: "c", SpaceID: "s", DeviceID: "d", Scope: BindingScopeSpace}, ProviderID: "p", ProviderInstanceID: "i"}}},
		{name: "global with space", bindings: []Binding{{Key: BindingKey{CapabilityID: "c", SpaceID: "s", Scope: BindingScopeGlobal}, ProviderID: "p", ProviderInstanceID: "i"}}},
		{name: "duplicate key", bindings: []Binding{globalBinding("c", "p1"), globalBinding("c", "p2")}},
	}
	for _, testCase := range cases {
		if _, err := NewBindingSnapshot("boot-1", 1, testCase.bindings); !errors.Is(err, ErrBindingInvalid) {
			t.Fatalf("%s: expected ErrBindingInvalid, got %v", testCase.name, err)
		}
	}
	if _, err := NewBindingSnapshot("", 1, nil); !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("empty epoch should be invalid, got %v", err)
	}
	if _, err := NewBindingSnapshot("boot-1", 0, nil); !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("revision 0 should be invalid, got %v", err)
	}
	if _, err := NewBindingSnapshotStore("  "); !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("blank epoch store should be invalid, got %v", err)
	}
}

func TestBindingSnapshotDigestIsOrderIndependent(t *testing.T) {
	first, err := NewBindingSnapshot("boot-1", 1, []Binding{
		globalBinding("memory.search", "global.host"),
		spaceBinding("memory.search", "space-1", "space.host"),
	})
	if err != nil {
		t.Fatalf("NewBindingSnapshot first: %v", err)
	}
	second, err := NewBindingSnapshot("boot-1", 1, []Binding{
		spaceBinding("memory.search", "space-1", "space.host"),
		globalBinding("memory.search", "global.host"),
	})
	if err != nil {
		t.Fatalf("NewBindingSnapshot second: %v", err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("digest depends on input order: %s vs %s", first.Digest(), second.Digest())
	}
	if len(first.Digest()) != 64 {
		t.Fatalf("digest should be sha256 hex, got %q", first.Digest())
	}
	changed, err := NewBindingSnapshot("boot-1", 1, []Binding{globalBinding("memory.search", "other.host")})
	if err != nil {
		t.Fatalf("NewBindingSnapshot changed: %v", err)
	}
	if changed.Digest() == first.Digest() {
		t.Fatalf("digest did not change with provider change")
	}
}

func TestBindingSnapshotStoreConcurrentUse(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Install([]Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	var writers sync.WaitGroup
	writers.Add(1)
	go func() {
		defer writers.Done()
		for index := 0; index < 50; index++ {
			if _, err := store.Install([]Binding{globalBinding("memory.search", "global.host")}); err != nil {
				t.Errorf("Install round %d: %v", index, err)
				return
			}
		}
	}()
	var readers sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for round := 0; round < 50; round++ {
				handle, err := store.Acquire()
				if err != nil {
					t.Errorf("Acquire: %v", err)
					return
				}
				resolution, err := handle.Resolve(BindingRequest{CapabilityID: "memory.search"})
				if err != nil {
					t.Errorf("Resolve: %v", err)
					handle.Release()
					return
				}
				if resolution.Binding.ProviderID != "global.host" {
					t.Errorf("unexpected provider %s", resolution.Binding.ProviderID)
				}
				handle.Release()
			}
		}()
	}
	readers.Wait()
	writers.Wait()
	if store.RetainedCount() != 0 {
		t.Fatalf("retained snapshots = %v after concurrent use, want empty", store.RetainedRevisions())
	}
}

func TestBindingSnapshotStoreEmpty(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Acquire(); !errors.Is(err, ErrSnapshotEmpty) {
		t.Fatalf("expected ErrSnapshotEmpty, got %v", err)
	}
	var handle *BindingHandle
	if handle.Snapshot() != nil || handle.Revision() != 0 {
		t.Fatalf("nil handle should report zero values")
	}
	if _, err := handle.Resolve(BindingRequest{CapabilityID: "memory.search"}); !errors.Is(err, ErrSnapshotEmpty) {
		t.Fatalf("nil handle resolve should report empty snapshot, got %v", err)
	}
	handle.Release()
}

type bindingGoldenResolution struct {
	Name       string         `json:"name"`
	Request    BindingRequest `json:"request"`
	Scope      BindingScope   `json:"scope"`
	ProviderID string         `json:"providerId"`
	Trail      []string       `json:"trail"`
}

type bindingGolden struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Epoch         string                    `json:"epoch"`
	Revision      uint64                    `json:"revision"`
	ScopeOrder    []BindingScope            `json:"scopeOrder"`
	Bindings      []Binding                 `json:"bindings"`
	Digest        string                    `json:"digest"`
	Resolutions   []bindingGoldenResolution `json:"resolutions"`
}

func bindingGoldenFixture() (bindingGolden, error) {
	golden := bindingGolden{
		SchemaVersion: 1,
		Epoch:         "boot-1",
		Revision:      3,
		ScopeOrder:    append([]BindingScope(nil), BindingScopeOrder...),
		Bindings: []Binding{
			deviceBinding("memory.search", "space-1", "device-1", "device.host"),
			spaceBinding("memory.search", "space-1", "space.host"),
			globalBinding("memory.search", "global.host"),
			deviceBinding("memory.delete", "space-1", "device-1", "device.host"),
			spaceBinding("memory.delete", "space-1", "space.host"),
		},
		Resolutions: []bindingGoldenResolution{
			{Name: "device-first", Request: BindingRequest{CapabilityID: "memory.search", SpaceID: "space-1", DeviceID: "device-1"}},
			{Name: "space-fallback", Request: BindingRequest{CapabilityID: "memory.search", SpaceID: "space-1"}},
			{Name: "global-fallback", Request: BindingRequest{CapabilityID: "memory.search"}},
			{Name: "second-capability", Request: BindingRequest{CapabilityID: "memory.delete", SpaceID: "space-1"}},
		},
	}
	snapshot, err := NewBindingSnapshot(golden.Epoch, golden.Revision, golden.Bindings)
	if err != nil {
		return bindingGolden{}, err
	}
	golden.Digest = snapshot.Digest()
	for index := range golden.Resolutions {
		resolution, err := snapshot.Resolve(golden.Resolutions[index].Request)
		if err != nil {
			return bindingGolden{}, err
		}
		golden.Resolutions[index].Scope = resolution.Scope
		golden.Resolutions[index].ProviderID = resolution.Binding.ProviderID
		golden.Resolutions[index].Trail = resolution.Trail
	}
	return golden, nil
}

func TestBindingSnapshotMatchesGolden(t *testing.T) {
	path := filepath.Join("testdata", "binding-snapshot.json")
	actual, err := bindingGoldenFixture()
	if err != nil {
		t.Fatalf("build golden fixture: %v", err)
	}
	raw, readErr := os.ReadFile(path)
	if readErr == nil {
		var expected bindingGolden
		if err := json.Unmarshal(raw, &expected); err != nil {
			t.Fatalf("decode golden %s: %v", path, err)
		}
		if len(actual.ScopeOrder) != len(expected.ScopeOrder) {
			t.Fatalf("scope order length drifted: %v vs %v", expected.ScopeOrder, actual.ScopeOrder)
		}
		for index := range expected.ScopeOrder {
			if expected.ScopeOrder[index] != actual.ScopeOrder[index] {
				t.Fatalf("scope order drifted: %v vs %v", expected.ScopeOrder, actual.ScopeOrder)
			}
		}
		if expected.Digest != actual.Digest || expected.Epoch != actual.Epoch || expected.Revision != actual.Revision {
			t.Fatalf("binding snapshot drifted from golden %s\n--- expected ---\n%s\n--- actual ---\n%s", path, mustJSON(t, expected), mustJSON(t, actual))
		}
		if len(expected.Resolutions) != len(actual.Resolutions) {
			t.Fatalf("resolution count drifted: %d vs %d", len(expected.Resolutions), len(actual.Resolutions))
		}
		for index := range expected.Resolutions {
			if mustJSON(t, expected.Resolutions[index]) != mustJSON(t, actual.Resolutions[index]) {
				t.Fatalf("resolution %q drifted\n--- expected ---\n%s\n--- actual ---\n%s", actual.Resolutions[index].Name, mustJSON(t, expected.Resolutions[index]), mustJSON(t, actual.Resolutions[index]))
			}
		}
		return
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read golden %s: %v", path, readErr)
	}
	writeGolden(t, path, actual)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func writeGolden(t *testing.T, path string, golden any) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		t.Fatalf("golden %s is missing; rerun with UPDATE_GOLDEN=1 to create it", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create testdata: %v", err)
	}
	if err := os.WriteFile(path, []byte(mustJSON(t, golden)+"\n"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
}
