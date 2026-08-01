package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func mustMarshalJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestUserDataRestoreJournalMigratesNewColumns(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE extension_package_user_data_restore_journal (
		journal_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		table_name TEXT NOT NULL,
		total_rows INTEGER NOT NULL DEFAULT 0,
		imported_rows INTEGER NOT NULL DEFAULT 0,
		state TEXT NOT NULL DEFAULT 'pending',
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		error_detail TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (operation_id, table_name)
	)`); err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO extension_package_user_data_restore_journal
		(journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j1", "op1", "ext1", "ext_test_data", 10, 5, 5, "5", "sha256:batch", "sha256:ns", "importing",
		"2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z", ""); err != nil {
		t.Fatalf("insert with new columns: %v", err)
	}
	rows, err := db.Query(`PRAGMA table_info(extension_package_user_data_restore_journal)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wanted := map[string]bool{"applied_count": false, "cursor": false, "batch_hash": false, "namespace_hash": false}
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if _, exists := wanted[name]; exists {
			wanted[name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Fatalf("migration did not add %s column", name)
		}
	}
}

func TestUserDataRestoreJournalProgressAndStateUpdate(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal-progress.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	journal, err := store.getOrCreateRestoreJournal(ctx, "op-progress", "ext1", "ext_progress_table", 100)
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}
	if journal.State != UserDataRestorePending {
		t.Fatalf("expected pending state, got %s", journal.State)
	}
	if err := store.updateRestoreJournalProgress(ctx, journal, 50, "50", "sha256:batch1"); err != nil {
		t.Fatalf("update progress: %v", err)
	}
	if err := store.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		t.Fatalf("update state: %v", err)
	}
	if journal.ImportedRows != 50 {
		t.Fatalf("expected 50 imported rows, got %d", journal.ImportedRows)
	}
	if journal.Cursor != "50" {
		t.Fatalf("expected cursor '50', got %s", journal.Cursor)
	}
	if journal.BatchHash != "sha256:batch1" {
		t.Fatalf("expected batch hash 'sha256:batch1', got %s", journal.BatchHash)
	}
	if journal.State != UserDataRestoreImporting {
		t.Fatalf("expected importing state, got %s", journal.State)
	}
}

func TestUserDataRestoreJournalErrorNotSwallowed(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal-error.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	journal, err := store.getOrCreateRestoreJournal(ctx, "op-error", "ext1", "ext_error_table", 50)
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}
	if err := store.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, "import failed: connection lost"); err != nil {
		t.Fatalf("update state to failed: %v", err)
	}
	if journal.State != UserDataRestoreFailed {
		t.Fatalf("expected failed state, got %s", journal.State)
	}
	if !strings.Contains(journal.ErrorDetail, "connection lost") {
		t.Fatalf("expected error detail containing 'connection lost', got %s", journal.ErrorDetail)
	}
}

func TestUserDataSnapshotMissingTableFailsClosed(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "snapshot-missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	userStateJSON := `{"mode":"repository","affectedTables":["ext_missing_table"],"recordCounts":{"ext_missing_table":10}}"`
	if err := store.RestoreUserDataFromSnapshot(ctx, "ext-missing", "op-missing", userStateJSON); err == nil {
		t.Fatal("expected error for missing snapshot data, got nil")
	}
}

func TestUserDataRecordValidationRejectsMissingFields(t *testing.T) {
	record := userDataRecord{
		SchemaVersion: "1.0.0",
		ExtensionID:   "ext1",
		Namespace:     "ext_ext1_data",
		EntityType:    "entity",
		EntityID:      "e1",
		Operation:     "upsert",
		Payload:       map[string]any{"key": "value"},
	}
	if err := validateUserDataRecord(record, "ext1"); err == nil {
		t.Fatal("expected validation error for missing payloadHash, got nil")
	}
}

func TestUserDataRecordValidationRejectsWrongExtension(t *testing.T) {
	record := userDataRecord{
		SchemaVersion: "1.0.0",
		ExtensionID:   "ext2",
		Namespace:     "ext_ext1_data",
		EntityType:    "entity",
		EntityID:      "e1",
		Operation:     "upsert",
		Payload:       map[string]any{"key": "value"},
		PayloadHash:   computeUserDataPayloadHash(map[string]any{"key": "value"}),
	}
	if err := validateUserDataRecord(record, "ext1"); err == nil {
		t.Fatal("expected validation error for wrong extensionID, got nil")
	}
}

func TestUserDataRecordValidationRejectsWrongNamespace(t *testing.T) {
	record := userDataRecord{
		SchemaVersion: "1.0.0",
		ExtensionID:   "ext1",
		Namespace:     "ext_other_data",
		EntityType:    "entity",
		EntityID:      "e1",
		Operation:     "upsert",
		Payload:       map[string]any{"key": "value"},
		PayloadHash:   computeUserDataPayloadHash(map[string]any{"key": "value"}),
	}
	if err := validateUserDataRecord(record, "ext1"); err == nil {
		t.Fatal("expected validation error for wrong namespace prefix, got nil")
	}
}

func TestUserDataRecordValidationRejectsInvalidPayloadHash(t *testing.T) {
	record := userDataRecord{
		SchemaVersion: "1.0.0",
		ExtensionID:   "ext1",
		Namespace:     "ext_ext1_data",
		EntityType:    "entity",
		EntityID:      "e1",
		Operation:     "upsert",
		Payload:       map[string]any{"key": "value"},
		PayloadHash:   "sha256:invalid",
	}
	if err := validateUserDataRecord(record, "ext1"); err == nil {
		t.Fatal("expected validation error for invalid payload hash, got nil")
	}
}

func TestUserDataRecordValidationPassesValidRecord(t *testing.T) {
	payload := map[string]any{"key": "value"}
	record := userDataRecord{
		SchemaVersion: "1.0.0",
		ExtensionID:   "ext1",
		Namespace:     "ext_ext1_data",
		EntityType:    "entity",
		EntityID:      "e1",
		Operation:     "upsert",
		Payload:       payload,
		PayloadHash:   computeUserDataPayloadHash(payload),
	}
	if err := validateUserDataRecord(record, "ext1"); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestUserDataSnapshotImportAndVerify(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "snapshot-import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_testext_restore (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	line1 := `{"schemaVersion":"1.0.0","extensionID":"testext","namespace":"ext_testext_restore","entityType":"entity","entityID":"e1","operation":"upsert","payload":{"entity_value":"v1"},"payloadHash":"` + computeUserDataPayloadHash(payload1) + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"testext","namespace":"ext_testext_restore","entityType":"entity","entityID":"e2","operation":"upsert","payload":{"entity_value":"v2"},"payloadHash":"` + computeUserDataPayloadHash(payload2) + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_testext_restore"},
		RecordCounts:   map[string]int64{"ext_testext_restore": 2},
		DataExports:    map[string]string{"ext_testext_restore": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatalf("marshal user state: %v", err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "testext", "op-verify", string(userStateJSON)); err != nil {
		t.Fatalf("restore user data: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-verify"); err != nil {
		t.Fatalf("verify restore: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ext_testext_restore`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}

func TestUserDataSnapshotWithFlatRecordImport(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "flat-import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_flat_test (
		id TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"value": "v1", "category": "c1"}
	payload2 := map[string]any{"value": "v2", "category": "c2"}
	payloadHash1 := computeUserDataPayloadHash(payload1)
	payloadHash2 := computeUserDataPayloadHash(payload2)
	line1 := `{"schemaVersion":"1.0.0","extensionID":"flat","namespace":"ext_flat_ns","entityType":"entity","entityID":"x1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"flat","namespace":"ext_flat_ns","entityType":"entity","entityID":"x2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_flat_test"},
		RecordCounts:   map[string]int64{"ext_flat_test": 2},
		DataExports:    map[string]string{"ext_flat_test": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatalf("marshal user state: %v", err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "flat", "op-flat", string(userStateJSON)); err != nil {
		t.Fatalf("restore user data: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-flat"); err != nil {
		t.Fatalf("verify restore: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ext_flat_test`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}
