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

func TestCorrupt_Checkpoint(t *testing.T) {
	dm, dataRoot := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	ctx := context.Background()

	runtimeID := domain.RuntimeInstanceID("rt-corrupt")
	md := RuntimeMetadata{
		RuntimeID:   runtimeID,
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   time.Now().UTC(),
	}
	if err := store.SaveMetadata(ctx, md); err != nil {
		t.Fatal(err)
	}

	paths, err := dm.ResolveRuntimePaths(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	corruptFile := filepath.Join(paths.Root, checkpointFileName)
	if err := os.WriteFile(corruptFile, []byte("{invalid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.LoadCheckpoint(ctx, runtimeID)
	if err == nil {
		t.Fatal("expected error for corrupt checkpoint")
	}
	if !isErrKind(err, ErrCorrupt) {
		t.Fatalf("expected corrupt_checkpoint, got: %v", err)
	}
}

func TestCorrupt_MetadataRejected(t *testing.T) {
	dm, dataRoot := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	ctx := context.Background()

	runtimeID := domain.RuntimeInstanceID("rt-corrupt-md")

	paths, err := dm.ResolveRuntimePaths(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		t.Fatal(err)
	}

	corruptFile := filepath.Join(paths.Root, metadataFileName)
	if err := os.WriteFile(corruptFile, []byte("not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.LoadMetadata(ctx, runtimeID)
	if err == nil {
		t.Fatal("expected error for corrupt metadata")
	}
	if !isErrKind(err, ErrCorruptMetadata) {
		t.Fatalf("expected corrupt_metadata, got: %v", err)
	}
}

func TestCorrupt_UnsupportedSchemaVersion(t *testing.T) {
	dm, dataRoot := newTestDirectoryManager(t)
	store, _ := NewFileStore(dm)
	ctx := context.Background()

	runtimeID := domain.RuntimeInstanceID("rt-schema-version")

	paths, err := dm.ResolveRuntimePaths(runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		t.Fatal(err)
	}

	content := `{
		"schemaVersion": 999,
		"runtimeId": "rt-schema-version",
		"pluginId": "com.test",
		"runtimeState": "created",
		"createdAt": "2025-01-01T00:00:00Z",
		"updatedAt": "2025-01-01T00:00:00Z",
		"cleanShutdown": false
	}`
	if err := os.WriteFile(filepath.Join(paths.Root, checkpointFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.LoadCheckpoint(ctx, runtimeID)
	if err == nil {
		t.Fatal("expected error for unsupported schema")
	}
	if !isErrKind(err, ErrUnsupportedSchema) {
		t.Fatalf("expected unsupported_schema, got: %v", err)
	}
}

func TestValidation_InvalidRuntimeState(t *testing.T) {
	now := time.Now().UTC()
	md := RuntimeMetadata{
		RuntimeID:   "rt-invalid-state",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	cp := RuntimeCheckpoint{
		RuntimeID:    "rt-invalid-state",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeState("banana"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := ValidateCheckpoint(md, cp, nil)
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
	if !isErrKind(err, ErrInvalidState) {
		t.Fatalf("expected invalid_state, got: %v", err)
	}
}

func TestValidation_InvalidServiceState(t *testing.T) {
	now := time.Now().UTC()
	md := RuntimeMetadata{
		RuntimeID:   "rt-invalid-svc",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	cp := RuntimeCheckpoint{
		RuntimeID:    "rt-invalid-svc",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateCreated,
		Services: []ServiceCheckpoint{
			{ServiceID: "svc-1", State: runtime.ServiceRuntimeState("invalid")},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := ValidateCheckpoint(md, cp, nil)
	if err == nil {
		t.Fatal("expected error for invalid service state")
	}
	if !isErrKind(err, ErrInvalidState) {
		t.Fatalf("expected invalid_state, got: %v", err)
	}
}

func TestValidation_TimestampViolation(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-time.Hour)

	md := RuntimeMetadata{
		RuntimeID:   "rt-timestamp",
		PluginID:    "com.test",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	cp := RuntimeCheckpoint{
		RuntimeID:    "rt-timestamp",
		PluginID:     "com.test",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    earlier,
	}

	err := ValidateCheckpoint(md, cp, nil)
	if err == nil {
		t.Fatal("expected error for timestamp violation")
	}
}

func TestValidation_DeletedCheckpointIsMissing(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.LoadCheckpoint(ctx, "rt-never-existed")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !isErrKind(err, ErrNotFound) {
		t.Fatalf("expected not_found, got: %v", err)
	}
}

func TestValidation_PluginIDMismatch(t *testing.T) {
	now := time.Now().UTC()
	md := RuntimeMetadata{
		RuntimeID:   "rt-plugin-mismatch",
		PluginID:    "com.original",
		ExtensionID: "ext-1",
		CreatedAt:   now,
	}
	cp := RuntimeCheckpoint{
		RuntimeID:    "rt-plugin-mismatch",
		PluginID:     "com.changed",
		RuntimeState: domain.RuntimeStateCreated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := CheckpointConsistency(md, cp)
	if err == nil {
		t.Fatal("expected plugin id mismatch error")
	}
	if !isErrKind(err, ErrPluginIDMismatch) {
		t.Fatalf("expected plugin_id_mismatch, got: %v", err)
	}
}

var _ = dataRoot
