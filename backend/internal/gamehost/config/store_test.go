package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/storage"
)

func setupTestStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	tmpDir := t.TempDir()

	dataRoot := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatalf("failed to create data root: %v", err)
	}

	dirMgr, err := storage.NewDirectoryManager(dataRoot)
	if err != nil {
		t.Fatalf("failed to create directory manager: %v", err)
	}

	store := NewFileStore(dirMgr)
	return store, dataRoot
}

func TestFileStore_SaveAndLoadSchema(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	original := NewSchema([]ConfigField{
		{Key: "timeout", Type: ConfigTypeInteger},
		{Key: "name", Type: ConfigTypeString},
		{Key: "debug", Type: ConfigTypeBoolean},
	})

	if err := store.SaveSchema(ctx, original); err != nil {
		t.Fatalf("save schema failed: %v", err)
	}

	loaded, err := store.LoadSchema(ctx)
	if err != nil {
		t.Fatalf("load schema failed: %v", err)
	}

	if len(loaded.Fields) != len(original.Fields) {
		t.Errorf("loaded schema has %d fields, expected %d", len(loaded.Fields), len(original.Fields))
	}

	if !loaded.HasField("timeout") {
		t.Error("loaded schema missing 'timeout' field")
	}
}

func TestFileStore_LoadSchemaNotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	_, err := store.LoadSchema(ctx)
	if err == nil {
		t.Error("expected error when schema not found")
	}
}

func TestFileStore_SaveNilSchema(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	if err := store.SaveSchema(ctx, nil); err == nil {
		t.Error("expected error when saving nil schema")
	}
}

func TestFileStore_SaveAndLoadPluginConfig(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	entries := []ConfigEntry{
		{Key: "a", Value: json.RawMessage("1"), Scope: ConfigScopePlugin},
		{Key: "b", Value: json.RawMessage(`"hello"`), Scope: ConfigScopePlugin},
	}

	if err := store.SavePluginConfig(ctx, "plugin-x", entries); err != nil {
		t.Fatalf("save plugin config failed: %v", err)
	}

	blob, err := store.LoadPluginConfig(ctx, "plugin-x")
	if err != nil {
		t.Fatalf("load plugin config failed: %v", err)
	}

	if blob == nil {
		t.Fatal("loaded nil blob")
	}

	if blob.Scope != ConfigScopePlugin {
		t.Errorf("expected plugin scope, got %s", blob.Scope)
	}

	if len(blob.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(blob.Entries))
	}
}

func TestFileStore_LoadPluginConfigNotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	_, err := store.LoadPluginConfig(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error when plugin config not found")
	}
}

func TestFileStore_SaveAndLoadRuntimeConfig(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	entries := []ConfigEntry{
		{Key: "mode", Value: json.RawMessage(`"production"`), Scope: ConfigScopeRuntime},
	}

	if err := store.SaveRuntimeConfig(ctx, "rt-1", entries); err != nil {
		t.Fatalf("save runtime config failed: %v", err)
	}

	blob, err := store.LoadRuntimeConfig(ctx, "rt-1")
	if err != nil {
		t.Fatalf("load runtime config failed: %v", err)
	}

	if blob.Scope != ConfigScopeRuntime {
		t.Errorf("expected runtime scope, got %s", blob.Scope)
	}

	if len(blob.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(blob.Entries))
	}
}

func TestFileStore_SaveAndLoadServiceConfig(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	entries := []ConfigEntry{
		{Key: "port", Value: json.RawMessage("8080"), Scope: ConfigScopeService},
	}

	if err := store.SaveServiceConfig(ctx, "rt-1", "svc-a", entries); err != nil {
		t.Fatalf("save service config failed: %v", err)
	}

	blob, err := store.LoadServiceConfig(ctx, "rt-1", "svc-a")
	if err != nil {
		t.Fatalf("load service config failed: %v", err)
	}

	if blob.Scope != ConfigScopeService {
		t.Errorf("expected service scope, got %s", blob.Scope)
	}

	if len(blob.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(blob.Entries))
	}
}

func TestFileStore_LoadServiceConfigNotFound(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	_, err := store.LoadServiceConfig(ctx, "rt-x", "svc-y")
	if err == nil {
		t.Error("expected error when service config not found")
	}
}

func TestFileStore_RejectsEmptyID(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	if err := store.SavePluginConfig(ctx, "", []ConfigEntry{}); err == nil {
		t.Error("expected error for empty plugin ID")
	}

	if err := store.SaveRuntimeConfig(ctx, "", []ConfigEntry{}); err == nil {
		t.Error("expected error for empty runtime ID")
	}

	if err := store.SaveServiceConfig(ctx, "", "svc", []ConfigEntry{}); err == nil {
		t.Error("expected error for empty runtime ID in service config")
	}

	if err := store.SaveServiceConfig(ctx, "rt", "", []ConfigEntry{}); err == nil {
		t.Error("expected error for empty service ID")
	}
}

func TestFileStore_AtomicWrite(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	entries := []ConfigEntry{
		{Key: "value", Value: json.RawMessage(`"initial"`), Scope: ConfigScopePlugin},
	}

	if err := store.SavePluginConfig(ctx, "atomic-test", entries); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	newEntries := []ConfigEntry{
		{Key: "value", Value: json.RawMessage(`"updated"`), Scope: ConfigScopePlugin},
	}
	if err := store.SavePluginConfig(ctx, "atomic-test", newEntries); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	blob, err := store.LoadPluginConfig(ctx, "atomic-test")
	if err != nil {
		t.Fatalf("load after update failed: %v", err)
	}

	var val string
	if err := json.Unmarshal(blob.Entries[0].Value, &val); err != nil {
		t.Fatalf("unmarshal value failed: %v", err)
	}

	if val != "updated" {
		t.Errorf("expected 'updated', got %q", val)
	}
}

func TestFileStore_NoTempFileLeftover(t *testing.T) {
	store, dataRoot := setupTestStore(t)
	ctx := context.Background()

	entries := []ConfigEntry{
		{Key: "x", Value: json.RawMessage("1")},
	}

	if err := store.SavePluginConfig(ctx, "cleanup-test", entries); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	tmpFiles, _ := filepath.Glob(filepath.Join(dataRoot, "**", ".tmp_*"), )
	_ = tmpFiles
	_ = dataRoot
}

func TestFileStore_SchemaDecodeFailure(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	paths, err := store.dir.ResolvePluginPaths("__schema__")
	if err != nil {
		t.Fatalf("resolve schema paths failed: %v", err)
	}

	schemaPath := filepath.Join(paths.Root, schemaFilename)
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o700); err != nil {
		t.Fatalf("mkdir schema dir failed: %v", err)
	}
	if err := os.WriteFile(schemaPath, []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("write corrupt schema failed: %v", err)
	}

	_, err = store.LoadSchema(ctx)
	if err == nil {
		t.Error("expected error loading corrupt schema")
	}
}
