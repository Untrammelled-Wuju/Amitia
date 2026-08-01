package kernel

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func newResourceSnapshotStoreForTest(t *testing.T) (*ResourceSnapshotStore, string) {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := NewResourceSnapshotStore(db, tmpDir)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return store, tmpDir
}

func TestNewResourceSnapshotStoreEnsureSchemaCreatesNamespaceHashColumn(t *testing.T) {
	store, _ := newResourceSnapshotStoreForTest(t)
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx,
		`INSERT INTO extension_package_resource_quarantine
		 (quarantine_id, operation_id, extension_id, resource_id, logical_path, content_hash,
		  storage_reference, namespace_hash, quarantine_path, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-q1", "op-test", "ext1", "r1", "/path/r1", "sha256:hash",
		"storage/ref1", "ns-hash-value", "/quarantine/r1", "quarantined",
		time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("failed to insert with namespace_hash: %v", err)
	}

	entries, err := store.listQuarantineEntries(ctx, "op-test", ResourceQuarantined)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].NamespaceHash != "ns-hash-value" {
		t.Fatalf("expected namespace_hash 'ns-hash-value', got %q", entries[0].NamespaceHash)
	}
}

func TestResourceQuarantineEntryPersistsNamespaceHash(t *testing.T) {
	store, tmpDir := newResourceSnapshotStoreForTest(t)
	ctx := context.Background()

	resourceDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(resourceDir, "ns-test.txt")
	if err := os.WriteFile(sourceFile, []byte("ns test data"), 0644); err != nil {
		t.Fatal(err)
	}

	resources := []domain.ResourceOwnership{
		{ResourceID: "r-ns", Reference: sourceFile, Metadata: map[string]any{"logicalPath": "ns-test.txt"}},
	}

	entries, err := store.QuarantineNewResources(ctx, "ext1", "op-ns", nil, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	dbEntries, err := store.listQuarantineEntries(ctx, "op-ns", ResourceQuarantined)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbEntries) != 1 {
		t.Fatalf("expected 1 db entry, got %d", len(dbEntries))
	}
	if dbEntries[0].NamespaceHash != "" {
		t.Fatalf("expected empty namespace_hash by default, got %q", dbEntries[0].NamespaceHash)
	}
	if dbEntries[0].StorageRef == "" {
		t.Fatal("expected non-empty storage reference")
	}
}

func TestListQuarantineEntriesByNamespaceHash(t *testing.T) {
	store, tmpDir := newResourceSnapshotStoreForTest(t)
	ctx := context.Background()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	entries := []ResourceQuarantineEntry{
		{
			QuarantineID:   "q-ns-1",
			OperationID:    "op-ns-list",
			ExtensionID:    "ext1",
			ResourceID:     "r-ns-1",
			LogicalPath:    "path1",
			ContentHash:    "sha256:hash1",
			StorageRef:     "ref1",
			NamespaceHash:  "ns-hash-shared",
			QuarantinePath: filepath.Join(tmpDir, "q1"),
			State:          ResourceQuarantined,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			QuarantineID:   "q-ns-2",
			OperationID:    "op-ns-list",
			ExtensionID:    "ext1",
			ResourceID:     "r-ns-2",
			LogicalPath:    "path2",
			ContentHash:    "sha256:hash2",
			StorageRef:     "ref2",
			NamespaceHash:  "ns-hash-other",
			QuarantinePath: filepath.Join(tmpDir, "q2"),
			State:          ResourceQuarantined,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	for _, entry := range entries {
		if err := store.persistQuarantineEntry(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}

	filtered, err := store.ListQuarantineEntriesByNamespaceHash(ctx, "ext1", "ns-hash-shared", ResourceQuarantined)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry with matching namespace hash, got %d", len(filtered))
	}
	if filtered[0].ResourceID != "r-ns-1" {
		t.Fatalf("expected r-ns-1, got %s", filtered[0].ResourceID)
	}

	empty, err := store.ListQuarantineEntriesByNamespaceHash(ctx, "ext1", "nonexistent-hash", ResourceQuarantined)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 entries for nonexistent hash, got %d", len(empty))
	}
}

func TestResourceSnapshotEntriesVerifyStorageReference(t *testing.T) {
	store, tmpDir := newResourceSnapshotStoreForTest(t)
	ctx := context.Background()

	resourceDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(resourceDir, "verify-ref.txt")
	content := []byte("verify ref content")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	resources := []domain.ResourceOwnership{
		{ResourceID: "r-verify-ref", Reference: sourceFile, Metadata: map[string]any{"logicalPath": "verify-ref.txt"}},
	}

	verified, err := store.ComputeVerifiedResourceTreeHash(ctx, resources)
	if err != nil {
		t.Fatalf("unexpected error computing verified tree hash before quarantine: %v", err)
	}
	if verified == "" {
		t.Fatal("expected non-empty verified tree hash before quarantine")
	}

	entries, err := store.QuarantineNewResources(ctx, "ext1", "op-verify-ref", nil, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.StorageRef == "" {
		t.Fatal("expected non-empty storage reference")
	}
	if entry.ContentHash == "" {
		t.Fatal("expected non-empty content hash")
	}
	if entry.ContentStorageReference == "" {
		t.Fatal("expected non-empty content storage reference")
	}
	if entry.OriginalPath == "" {
		t.Fatal("expected non-empty original path")
	}
	if entry.State != ResourceQuarantined {
		t.Fatalf("expected quarantined state after restore, got %s", entry.State)
	}

	resourcesAfter := []domain.ResourceOwnership{
		{ResourceID: "r-verify-ref", Reference: sourceFile, Metadata: map[string]any{"contentHash": entry.ContentHash, "originalPath": entry.OriginalPath}},
	}
	verifiedAfter, err := store.ComputeResourceTreeHash(ctx, "ext1", resourcesAfter)
	if err != nil {
		t.Fatalf("unexpected error computing reference tree hash: %v", err)
	}
	if verifiedAfter == "" {
		t.Fatal("expected non-empty tree hash with content hash metadata")
	}
}

func TestQuarantineSizeColumnMigration(t *testing.T) {
	store, tmpDir := newResourceSnapshotStoreForTest(t)
	ctx := context.Background()

	resourceDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(resourceDir, "size-check.txt")
	content := []byte("size check content")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	resources := []domain.ResourceOwnership{
		{ResourceID: "r-size", Reference: sourceFile},
	}

	entries, err := store.QuarantineNewResources(ctx, "ext1", "op-size", nil, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Size != int64(len(content)) {
		t.Fatalf("expected size %d, got %d", len(content), entries[0].Size)
	}
}
