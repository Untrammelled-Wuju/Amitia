package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

func newTestStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataRoot := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatalf("failed to create data root: %v", err)
	}
	dm, err := storage.NewDirectoryManager(dataRoot)
	if err != nil {
		t.Fatalf("failed to create directory manager: %v", err)
	}
	store, err := NewFileStore(dm)
	if err != nil {
		t.Fatalf("failed to create file store: %v", err)
	}
	return store, dataRoot
}

func newTestRuntimeID(t *testing.T, name string) domain.RuntimeInstanceID {
	t.Helper()
	return domain.RuntimeInstanceID(name)
}

func TestFileStore_SaveLoadMetadata(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	md := RuntimeMetadata{
		SchemaVersion:      MetadataSchemaVersion,
		RuntimeID:          "test-runtime-1",
		PluginID:           "com.example.plugin",
		ExtensionID:        "ext-123",
		PluginVersion:      "1.0.0",
		CreatedAt:          now,
		DescriptorRevision: "rev-abc123",
	}

	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	loaded, err := store.LoadMetadata(ctx, md.RuntimeID)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	if loaded.RuntimeID != md.RuntimeID {
		t.Fatalf("runtime id mismatch: %s vs %s", loaded.RuntimeID, md.RuntimeID)
	}
	if loaded.PluginID != md.PluginID {
		t.Fatalf("plugin id mismatch: %s vs %s", loaded.PluginID, md.PluginID)
	}
	if loaded.ExtensionID != md.ExtensionID {
		t.Fatalf("extension id mismatch")
	}
	if loaded.PluginVersion != md.PluginVersion {
		t.Fatalf("plugin version mismatch")
	}
	if !loaded.CreatedAt.Equal(md.CreatedAt) {
		t.Fatalf("created at mismatch: %v vs %v", loaded.CreatedAt, md.CreatedAt)
	}
	if loaded.DescriptorRevision != md.DescriptorRevision {
		t.Fatalf("descriptor revision mismatch")
	}
}

func TestFileStore_SaveLoadCheckpoint(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	cp := RuntimeCheckpoint{
		SchemaVersion:      MetadataSchemaVersion,
		RuntimeID:          "test-runtime-2",
		PluginID:           "com.example.plugin",
		RuntimeState:       domain.RuntimeStateRunning,
		Services: []ServiceCheckpoint{
			{
				ServiceID: "bridge",
				State:     runtime.ServiceStateRunning,
				Required:  true,
				UpdatedAt: now,
			},
			{
				ServiceID: "agent",
				State:     runtime.ServiceStateStarting,
				Required:  true,
				UpdatedAt: now,
			},
		},
		DescriptorRevision: "rev-abc",
		CreatedAt:          now,
		UpdatedAt:          now,
		CleanShutdown:      false,
	}

	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	loaded, err := store.LoadCheckpoint(ctx, cp.RuntimeID)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if loaded.RuntimeID != cp.RuntimeID {
		t.Fatalf("runtime id mismatch")
	}
	if loaded.PluginID != cp.PluginID {
		t.Fatalf("plugin id mismatch")
	}
	if loaded.RuntimeState != cp.RuntimeState {
		t.Fatalf("runtime state mismatch")
	}
	if len(loaded.Services) != len(cp.Services) {
		t.Fatalf("services count mismatch: %d vs %d", len(loaded.Services), len(cp.Services))
	}
	if loaded.Services[0].ServiceID != "agent" || loaded.Services[1].ServiceID != "bridge" {
		t.Fatalf("services not sorted: %s, %s", loaded.Services[0].ServiceID, loaded.Services[1].ServiceID)
	}
	if loaded.CleanShutdown != cp.CleanShutdown {
		t.Fatalf("clean shutdown mismatch")
	}
}

func TestFileStore_ServicesSortedOrder(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	cp := RuntimeCheckpoint{
		RuntimeID:    "runtime-sort",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateRunning,
		Services: []ServiceCheckpoint{
			{ServiceID: "vision", State: runtime.ServiceStateRunning, UpdatedAt: now},
			{ServiceID: "bridge", State: runtime.ServiceStateRunning, UpdatedAt: now},
			{ServiceID: "agent", State: runtime.ServiceStateRunning, UpdatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadCheckpoint(ctx, cp.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}

	expected := []domain.ServiceID{"agent", "bridge", "vision"}
	for i, svc := range loaded.Services {
		if svc.ServiceID != expected[i] {
			t.Fatalf("service[%d] = %s, want %s", i, svc.ServiceID, expected[i])
		}
	}
}

func TestFileStore_AtomicWritePreservesOld(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	v1 := RuntimeCheckpoint{
		RuntimeID:    "runtime-atomic",
		PluginID:     "com.atomic",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
		CleanShutdown: false,
	}

	if err := store.SaveCheckpoint(ctx, v1); err != nil {
		t.Fatalf("failed to save v1: %v", err)
	}

	loadedV1, err := store.LoadCheckpoint(ctx, v1.RuntimeID)
	if err != nil {
		t.Fatalf("failed to load v1: %v", err)
	}

	v2 := v1
	v2.RuntimeState = domain.RuntimeStateRunning
	v2.UpdatedAt = now.Add(time.Second)
	v2.CleanShutdown = false

	if err := store.SaveCheckpoint(ctx, v2); err != nil {
		t.Fatalf("failed to save v2: %v", err)
	}

	loadedV2, err := store.LoadCheckpoint(ctx, v1.RuntimeID)
	if err != nil {
		t.Fatalf("failed to load after v2: %v", err)
	}

	if loadedV2.RuntimeState != domain.RuntimeStateRunning {
		t.Fatalf("expected running state, got %s", loadedV2.RuntimeState)
	}
	if loadedV1.RuntimeState != domain.RuntimeStateCreated {
		t.Fatalf("v1 state changed unexpectedly: %s", loadedV1.RuntimeState)
	}
}

func TestFileStore_InvalidRuntimeIDMismatch(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	cp := RuntimeCheckpoint{
		RuntimeID:    "runtime-mismatch",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	_, err := store.LoadCheckpoint(ctx, "other-runtime")
	if err == nil {
		t.Fatal("expected error for non-existent runtime")
	}
	if !isErrKind(err, ErrNotFound) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestFileStore_InvalidStateRejected(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	cp := RuntimeCheckpoint{
		RuntimeID:    "runtime-invalid-state",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeState("invalid_state"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.SaveCheckpoint(ctx, cp); err == nil {
		t.Fatal("expected error for invalid runtime state")
	}
}

func TestFileStore_MetadataSourceOfTruth(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	md := RuntimeMetadata{
		RuntimeID:   "runtime-identity",
		PluginID:    "com.original",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}

	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	cp := RuntimeCheckpoint{
		RuntimeID:    "runtime-identity",
		PluginID:     "com.original",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	loadedMeta, err := store.LoadMetadata(ctx, "runtime-identity")
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	if loadedMeta.PluginID != md.PluginID {
		t.Fatalf("metadata plugin id mismatch")
	}

	loadedCP, err := store.LoadCheckpoint(ctx, "runtime-identity")
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if err := ValidateCheckpoint(loadedMeta, loadedCP, nil); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestFileStore_RuntimeIDMismatch(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	md := RuntimeMetadata{
		RuntimeID:   "runtime-id-1",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}

	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	cp := RuntimeCheckpoint{
		RuntimeID:    "runtime-id-2",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	loadedMeta, _ := store.LoadMetadata(ctx, "runtime-id-1")
	loadedCP, _ := store.LoadCheckpoint(ctx, "runtime-id-1")

	err := CheckpointConsistency(loadedMeta, loadedCP)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !isErrKind(err, ErrRuntimeIDMismatch) {
		t.Fatalf("expected runtime_id_mismatch, got: %v", err)
	}
}

func TestFileStore_DeleteMetadata(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	md := RuntimeMetadata{
		RuntimeID:   "runtime-delete",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}

	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	has, err := store.HasMetadata(ctx, md.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("metadata should exist")
	}

	if err := store.DeleteMetadata(ctx, md.RuntimeID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	has, err = store.HasMetadata(ctx, md.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("metadata should not exist after delete")
	}
}

func TestFileStore_PathEscapesStorageRoot(t *testing.T) {
	store, dataRoot := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	md := RuntimeMetadata{
		RuntimeID:   "runtime-path-1",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}

	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	paths, err := store.dir.ResolveRuntimePaths("runtime-path-1")
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	fullPath := filepath.Join(paths.Root, metadataFileName)
	dataRootAbs, _ := filepath.Abs(dataRoot)
	fullPathAbs, _ := filepath.Abs(fullPath)

	if !strings.HasPrefix(fullPathAbs, dataRootAbs) {
		t.Fatalf("file path %s escapes data root %s", fullPathAbs, dataRootAbs)
	}
}

func TestFileStore_InvalidMetadataRejected(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	invalidMD := RuntimeMetadata{
		RuntimeID: "",
		PluginID:  "com.test",
	}

	err := store.SaveMetadata(ctx, invalidMD)
	if err == nil {
		t.Fatal("expected error for empty runtime id")
	}
}
