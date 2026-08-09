package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

func newTestClassifier(t *testing.T, descriptors map[domain.PluginID]domain.PluginDescriptor) (*RecoveryClassifier, string) {
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
	resolver := &fakeResolver{descriptors: descriptors}
	cl, err := NewRecoveryClassifier(dm, store, resolver)
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}
	return cl, dataRoot
}

func seedRuntime(t *testing.T, dm storage.DirectoryManager, store *FileStore, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, clean bool, state domain.RuntimeState, now time.Time) {
	t.Helper()
	ctx := context.Background()

	md := RuntimeMetadata{
		RuntimeID:   runtimeID,
		PluginID:    pluginID,
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	services := []ServiceCheckpoint{
		{ServiceID: "svc-a", State: runtime.ServiceStateRunning, Required: true, UpdatedAt: now},
	}

	cp := RuntimeCheckpoint{
		RuntimeID:     runtimeID,
		PluginID:      pluginID,
		RuntimeState:  state,
		Services:      services,
		CreatedAt:     now,
		UpdatedAt:     now,
		CleanShutdown: clean,
	}
	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}
}

func TestRecoveryClassifier_ClassifyClean(t *testing.T) {
	cl, _ := newTestClassifier(t, nil)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	seedRuntime(t, dm, store, "rt-clean", "com.test", true, domain.RuntimeStateStopped, now)

	info, err := cl.Classify(context.Background(), "rt-clean")
	if err != nil {
		t.Fatalf("classify failed: %v", err)
	}

	if info.RecoveryStatus != RecoveryStatusClean {
		t.Fatalf("expected clean, got: %s", info.RecoveryStatus)
	}
}

func TestRecoveryClassifier_ClassifyUnclean(t *testing.T) {
	cl, _ := newTestClassifier(t, nil)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	seedRuntime(t, dm, store, "rt-unclean", "com.test", false, domain.RuntimeStateRunning, now)

	info, err := cl.Classify(context.Background(), "rt-unclean")
	if err != nil {
		t.Fatalf("classify failed: %v", err)
	}

	if info.RecoveryStatus != RecoveryStatusUnclean {
		t.Fatalf("expected unclean, got: %s", info.RecoveryStatus)
	}
}

func TestRecoveryClassifier_ClassifyMissing(t *testing.T) {
	cl, _ := newTestClassifier(t, nil)

	info, err := cl.Classify(context.Background(), "rt-nonexistent")
	if err != nil {
		t.Fatalf("classify failed: %v", err)
	}

	if info.RecoveryStatus != RecoveryStatusMissing {
		t.Fatalf("expected missing, got: %s", info.RecoveryStatus)
	}
}

func TestRecoveryClassifier_ListStoredRuntimeIDs(t *testing.T) {
	cl, _ := newTestClassifier(t, nil)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	seedRuntime(t, dm, store, "rt-a", "com.test", true, domain.RuntimeStateStopped, now)
	seedRuntime(t, dm, store, "rt-b", "com.test", true, domain.RuntimeStateStopped, now)
	seedRuntime(t, dm, store, "rt-c", "com.test", false, domain.RuntimeStateRunning, now)

	ids, err := cl.ListStoredRuntimeIDs(context.Background())
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 runtimes, got: %d", len(ids))
	}

	expected := []domain.RuntimeInstanceID{"rt-a", "rt-b", "rt-c"}
	for i, id := range ids {
		if id != expected[i] {
			t.Fatalf("ids[%d] = %s, want %s", i, id, expected[i])
		}
	}
}

func TestRecoveryClassifier_RandomDirectoryIgnored(t *testing.T) {
	cl, dataRoot := newTestClassifier(t, nil)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	seedRuntime(t, dm, store, "rt-valid", "com.test", true, domain.RuntimeStateStopped, now)

	runtimesDir := filepath.Join(dataRoot, "gamehost", "runtimes")
	randomDir := filepath.Join(runtimesDir, "random-no-metadata")
	if err := os.MkdirAll(randomDir, 0o700); err != nil {
		t.Fatalf("failed to create random dir: %v", err)
	}

	ids, err := cl.ListStoredRuntimeIDs(context.Background())
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	for _, id := range ids {
		if id == "random-no-metadata" {
			t.Fatal("random directory should not be listed")
		}
	}
}

func TestRecoveryClassifier_OrphanedRuntime(t *testing.T) {
	descriptor := domain.PluginDescriptor{
		ID:          "com.exists",
		ExtensionID: "ext-1",
		Name:        "Exists",
		Version:     "1.0.0",
	}
	descriptors := map[domain.PluginID]domain.PluginDescriptor{
		"com.exists": descriptor,
	}

	cl, _ := newTestClassifier(t, descriptors)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	seedRuntime(t, dm, store, "rt-orphan", "com.unknown", false, domain.RuntimeStateRunning, now)

	info, err := cl.Classify(context.Background(), "rt-orphan")
	if err != nil {
		t.Fatalf("classify failed: %v", err)
	}

	if info.RecoveryStatus != RecoveryStatusOrphaned {
		t.Fatalf("expected orphaned, got: %s", info.RecoveryStatus)
	}
}

func TestRecoveryClassifier_InvalidMismatch(t *testing.T) {
	cl, _ := newTestClassifier(t, nil)
	dm, _ := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	now := time.Now().UTC()

	md := RuntimeMetadata{
		RuntimeID:   "rt-mismatch-id",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	if err := store.SaveMetadata(context.Background(), md); err != nil {
		t.Fatal(err)
	}

	cp := RuntimeCheckpoint{
		RuntimeID:    "rt-different-id",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.SaveCheckpoint(context.Background(), cp); err != nil {
		t.Fatal(err)
	}

	info, err := cl.Classify(context.Background(), "rt-mismatch-id")
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if info.RecoveryStatus != RecoveryStatusInvalid {
		t.Fatalf("expected invalid, got: %s", info.RecoveryStatus)
	}
}

func newTestDirectoryManager(t *testing.T) (storage.DirectoryManager, string) {
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
	return dm, dataRoot
}
