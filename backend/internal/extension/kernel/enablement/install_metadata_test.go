//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type mockRuntime struct {
	id      string
	workers int
}

func (m mockRuntime) ID() string   { return m.id }
func (m mockRuntime) Workers() int { return m.workers }

type mockSpawner struct {
	mu      sync.Mutex
	calls   int
	fail    error
	workers int
}

func (m *mockSpawner) Spawn(ctx context.Context, metadata Metadata) (RuntimeHandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.fail != nil {
		return nil, m.fail
	}
	workers := m.workers
	if workers == 0 {
		workers = 1
	}
	return mockRuntime{id: metadata.ID, workers: workers}, nil
}

func (m *mockSpawner) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func metadataLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s02 "+format, args...)
}

func metadataManifests(count int) []Manifest {
	manifests := make([]Manifest, 0, count)
	for index := 0; index < count; index++ {
		manifests = append(manifests, Manifest{
			ID:          fmt.Sprintf("plugin-%04d", index),
			Version:     "1.0.0",
			Permissions: []string{"memory.read", "network.limited"},
			Entry:       fmt.Sprintf("plugins/%04d/main.js", index),
		})
	}
	return manifests
}

func TestPluginMetadataThousandStaysCold(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	report, err := installer.InstallBatch(ctx, metadataManifests(1000))
	if err != nil {
		t.Fatalf("InstallBatch: %v", err)
	}
	if report.Installed != 1000 || report.Skipped != 0 {
		t.Fatalf("report=%+v", report)
	}
	for index := 0; index < 1000; index++ {
		if err := installer.Enable(ctx, fmt.Sprintf("plugin-%04d", index), 1); err != nil {
			t.Fatalf("Enable %d: %v", index, err)
		}
	}
	snapshot := installer.Snapshot()
	if snapshot.Plugins != 1000 || snapshot.Processes != 0 || snapshot.Workers != 0 || snapshot.SpawnCalls != 0 {
		t.Fatalf("snapshot=%+v want 1000/0/0/0", snapshot)
	}
	if spawner.Calls() != 0 {
		t.Fatalf("spawner calls=%d want 0", spawner.Calls())
	}
	metadataLog(t, "cold plugins=%d processes=%d workers=%d spawn_calls=%d", snapshot.Plugins, snapshot.Processes, snapshot.Workers, snapshot.SpawnCalls)
}

func TestPluginMetadataInstallDoesNotSpawn(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, metadataManifests(1)); err != nil {
		t.Fatalf("install: %v", err)
	}
	if spawner.Calls() != 0 || installer.Snapshot().Processes != 0 {
		t.Fatalf("install created a runtime: calls=%d", spawner.Calls())
	}
	if err := installer.Enable(ctx, "plugin-0000", 1); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if spawner.Calls() != 0 {
		t.Fatalf("enable created a runtime: calls=%d", spawner.Calls())
	}
	if err := installer.Activate(ctx, "plugin-0000", 2); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if spawner.Calls() != 1 || installer.Snapshot().Processes != 1 {
		t.Fatalf("activation did not spawn: calls=%d", spawner.Calls())
	}
	metadataLog(t, "install spawn_calls=0 enable_calls=0 activate_calls=1")
}

func TestPluginMetadataIndexFieldsOnly(t *testing.T) {
	ctx := context.Background()
	installer := NewInstaller(&mockSpawner{})
	if _, err := installer.InstallBatch(ctx, metadataManifests(2)); err != nil {
		t.Fatalf("install: %v", err)
	}
	entry, ok := installer.index.Get("plugin-0000")
	if !ok {
		t.Fatal("metadata entry missing")
	}
	if entry.ID != "plugin-0000" || entry.Version != "1.0.0" || len(entry.Permissions) != 2 || entry.Entry == "" {
		t.Fatalf("entry=%+v", entry)
	}
	state, ok := installer.State("plugin-0000")
	if !ok || state != LifecycleInstalled {
		t.Fatalf("state=%s ok=%v want installed", state, ok)
	}
	installer.mu.Lock()
	handles := len(installer.handles)
	installer.mu.Unlock()
	if handles != 0 {
		t.Fatalf("install created %d runtime handles", handles)
	}
	metadataLog(t, "metadata fields=4 runtime_handles=0 state=installed")
}

func TestPluginMetadataDuplicateIdempotent(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	manifests := metadataManifests(3)
	if _, err := installer.InstallBatch(ctx, manifests); err != nil {
		t.Fatalf("first install: %v", err)
	}
	report, err := installer.InstallBatch(ctx, manifests)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if report.Installed != 0 || report.Skipped != 3 {
		t.Fatalf("report=%+v", report)
	}
	if installer.Snapshot().Plugins != 3 || spawner.Calls() != 0 {
		t.Fatalf("snapshot=%+v calls=%d", installer.Snapshot(), spawner.Calls())
	}
	metadataLog(t, "duplicate installed=0 skipped=3 entries=3")
}

func TestPluginMetadataInvalidRejected(t *testing.T) {
	ctx := context.Background()
	installer := NewInstaller(&mockSpawner{})
	cases := [][]Manifest{
		{{ID: "", Version: "1.0.0"}},
		{{ID: "plugin-a", Version: " "}},
		{{ID: "plugin-a", Version: "1"}, {ID: "plugin-a", Version: "2"}},
	}
	for index, manifests := range cases {
		if _, err := installer.InstallBatch(ctx, manifests); !errors.Is(err, ErrManifestInvalid) && !errors.Is(err, ErrMetadataDuplicate) {
			t.Fatalf("case %d err=%v", index, err)
		}
	}
	if installer.Snapshot().Plugins != 0 {
		t.Fatalf("invalid batch left entries: %d", installer.Snapshot().Plugins)
	}
	metadataLog(t, "invalid rejected=3 entries=0")
}

func TestPluginMetadataCancelAtomic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, metadataManifests(10)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if installer.Snapshot().Plugins != 0 || spawner.Calls() != 0 {
		t.Fatalf("cancelled batch mutated state: %+v", installer.Snapshot())
	}
	metadataLog(t, "cancel entries=0 err=context.Canceled")
}

func TestPluginMetadataEnableNoSpawn(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, metadataManifests(5)); err != nil {
		t.Fatalf("install: %v", err)
	}
	for index := 0; index < 5; index++ {
		if err := installer.Enable(ctx, fmt.Sprintf("plugin-%04d", index), 1); err != nil {
			t.Fatalf("enable: %v", err)
		}
	}
	snapshot := installer.Snapshot()
	if snapshot.Processes != 0 || snapshot.Workers != 0 || spawner.Calls() != 0 {
		t.Fatalf("enable spawned runtime: %+v calls=%d", snapshot, spawner.Calls())
	}
	metadataLog(t, "enable plugins=5 processes=0 workers=0 spawn_calls=0")
}

func TestPluginMetadataUninstallRemoves(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, metadataManifests(2)); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := installer.Enable(ctx, "plugin-0000", 1); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := installer.Uninstall(ctx, "plugin-0000", 2); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if installer.Snapshot().Plugins != 1 {
		t.Fatalf("plugins=%d want 1", installer.Snapshot().Plugins)
	}
	if _, ok := installer.State("plugin-0000"); ok {
		t.Fatal("uninstalled plugin still has a machine")
	}
	if _, ok := installer.index.Get("plugin-0000"); ok {
		t.Fatal("uninstalled plugin still has metadata")
	}
	if spawner.Calls() != 0 {
		t.Fatalf("uninstall spawned runtime: %d", spawner.Calls())
	}
	metadataLog(t, "uninstall entries=1 state_missing=true spawn_calls=0")
}

func TestPluginMetadataActivationBoundary(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, metadataManifests(1)); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := installer.Activate(ctx, "plugin-0000", 1); !errors.Is(err, ErrInstallerState) {
		t.Fatalf("activate from installed err=%v want ErrInstallerState", err)
	}
	if spawner.Calls() != 0 {
		t.Fatalf("failed activation spawned: %d", spawner.Calls())
	}
	if err := installer.Enable(ctx, "plugin-0000", 1); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := installer.Activate(ctx, "plugin-0000", 2); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := installer.Activate(ctx, "plugin-0000", 3); err != nil {
		t.Fatalf("activate replay: %v", err)
	}
	if spawner.Calls() != 1 {
		t.Fatalf("replay spawned again: %d", spawner.Calls())
	}
	if err := installer.Enable(ctx, "missing", 1); !errors.Is(err, ErrInstallerState) {
		t.Fatalf("enable unknown err=%v", err)
	}
	metadataLog(t, "activation requires_enabled=true replay_spawns=1 unknown=ErrInstallerState")
}

func TestPluginMetadataConcurrentBatches(t *testing.T) {
	ctx := context.Background()
	spawner := &mockSpawner{}
	installer := NewInstaller(spawner)
	var wg sync.WaitGroup
	for batch := 0; batch < 2; batch++ {
		wg.Add(1)
		go func(batch int) {
			defer wg.Done()
			manifests := make([]Manifest, 0, 500)
			for index := 0; index < 500; index++ {
				manifests = append(manifests, Manifest{ID: fmt.Sprintf("plugin-%d-%04d", batch, index), Version: "1.0.0"})
			}
			if _, err := installer.InstallBatch(ctx, manifests); err != nil {
				t.Errorf("batch %d: %v", batch, err)
			}
		}(batch)
	}
	wg.Wait()
	snapshot := installer.Snapshot()
	if snapshot.Plugins != 1000 || snapshot.Processes != 0 || spawner.Calls() != 0 {
		t.Fatalf("snapshot=%+v calls=%d", snapshot, spawner.Calls())
	}
	metadataLog(t, "concurrent batches=2 entries=1000 spawn_calls=0")
}
