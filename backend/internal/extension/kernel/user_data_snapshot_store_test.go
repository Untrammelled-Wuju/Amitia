package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	journal, err := store.getOrCreateRestoreJournal(ctx, "op-progress", "ext1", "ext_progress_table", "", 100, "")
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}
	if journal.State != UserDataRestorePending {
		t.Fatalf("expected pending state, got %s", journal.State)
	}
	if err := store.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		t.Fatalf("update state: %v", err)
	}
	if journal.State != UserDataRestoreImporting {
		t.Fatalf("expected importing state, got %s", journal.State)
	}
	if journal.ImportedRows != 0 {
		t.Fatalf("expected 0 imported rows on init, got %d", journal.ImportedRows)
	}
	if journal.Cursor != "" {
		t.Fatalf("expected empty cursor on init, got %s", journal.Cursor)
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
	journal, err := store.getOrCreateRestoreJournal(ctx, "op-error", "ext1", "ext_error_table", "", 50, "")
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
		EntityType:    "data",
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
	line1 := `{"schemaVersion":"1.0.0","extensionID":"testext","namespace":"ext_testext_restore","entityType":"restore","entityID":"e1","operation":"upsert","payload":{"entity_value":"v1"},"payloadHash":"` + computeUserDataPayloadHash(payload1) + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"testext","namespace":"ext_testext_restore","entityType":"restore","entityID":"e2","operation":"upsert","payload":{"entity_value":"v2"},"payloadHash":"` + computeUserDataPayloadHash(payload2) + `"}`
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
	if err := store.VerifyUserDataRestore(ctx, "op-verify", string(userStateJSON)); err != nil {
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
	line1 := `{"schemaVersion":"1.0.0","extensionID":"flat","namespace":"ext_flat_test","entityType":"test","entityID":"x1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"flat","namespace":"ext_flat_test","entityType":"test","entityID":"x2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
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
	if err := store.VerifyUserDataRestore(ctx, "op-flat", string(userStateJSON)); err != nil {
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

func makeTestRawLine(schemaVersion, extensionID, namespace, entityType, entityID, operation string, payload map[string]any) string {
	payloadHash := computeUserDataPayloadHash(payload)
	return `{"schemaVersion":"` + schemaVersion + `","extensionID":"` + extensionID + `","namespace":"` + namespace + `","entityType":"` + entityType + `","entityID":"` + entityID + `","operation":"` + operation + `","payload":` + mustMarshalJSON(payload) + `,"payloadHash":"` + payloadHash + `"}`
}

func wrapRawRecords(rawLines ...string) []map[string]any {
	records := make([]map[string]any, 0, len(rawLines))
	for _, line := range rawLines {
		records = append(records, map[string]any{"_raw": line})
	}
	return records
}

func TestComputeContentBoundBatchHashSameCountDifferentContent(t *testing.T) {
	payloadA1 := map[string]any{"key": "value_a1"}
	payloadA2 := map[string]any{"key": "value_a2"}
	payloadB1 := map[string]any{"key": "value_b1"}
	payloadB2 := map[string]any{"key": "value_b2"}
	batchA := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", payloadA1),
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e2", "upsert", payloadA2),
	)
	batchB := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", payloadB1),
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e2", "upsert", payloadB2),
	)
	hashA := computeContentBoundBatchHash(batchA, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test")
	hashB := computeContentBoundBatchHash(batchB, "ext1", 0, "", 2, "1.0.0", "ext_ext1_data", "op-test")
	if hashA == "" || hashB == "" {
		t.Fatalf("expected non-empty hashes, got A=%s B=%s", hashA, hashB)
	}
	if hashA == hashB {
		t.Fatalf("same count but different content should produce different hashes")
	}
}

func TestComputeContentBoundBatchHashSameCountDifferentOrder(t *testing.T) {
	payload1 := map[string]any{"key": "value1"}
	payload2 := map[string]any{"key": "value2"}
	batchForward := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", payload1),
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e2", "upsert", payload2),
	)
	batchReversed := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e2", "upsert", payload2),
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", payload1),
	)
	hashForward := computeContentBoundBatchHash(batchForward, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test")
	hashReversed := computeContentBoundBatchHash(batchReversed, "ext1", 0, "", 2, "1.0.0", "ext_ext1_data", "op-test")
	if hashForward == "" || hashReversed == "" {
		t.Fatalf("expected non-empty hashes")
	}
	if hashForward == hashReversed {
		t.Fatalf("different record order should produce different hashes")
	}
}

func TestComputeContentBoundBatchHashSingleFieldChange(t *testing.T) {
	basePayload := map[string]any{"key": "value"}
	baseBatch := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", basePayload),
	)
	baseHash := computeContentBoundBatchHash(baseBatch, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test")

	changeNamespace := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_other", "other", "e1", "upsert", basePayload),
	)
	if computeContentBoundBatchHash(changeNamespace, "ext1", 0, "", 1, "1.0.0", "ext_ext1_other", "op-test") == baseHash {
		t.Fatalf("namespace change should alter hash")
	}

	changeOperation := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "delete", basePayload),
	)
	if computeContentBoundBatchHash(changeOperation, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test") == baseHash {
		t.Fatalf("operation change should alter hash")
	}

	changeSchemaVersion := wrapRawRecords(
		makeTestRawLine("2.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", basePayload),
	)
	if computeContentBoundBatchHash(changeSchemaVersion, "ext1", 0, "", 1, "2.0.0", "ext_ext1_data", "op-test") == baseHash {
		t.Fatalf("schemaVersion change should alter hash")
	}

	changeEntityType := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "setting", "e1", "upsert", basePayload),
	)
	if computeContentBoundBatchHash(changeEntityType, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test") == baseHash {
		t.Fatalf("entityType change should alter hash")
	}
}

func TestComputeContentBoundBatchHashStable(t *testing.T) {
	payload1 := map[string]any{"key": "value1"}
	payload2 := map[string]any{"key": "value2"}
	batch := wrapRawRecords(
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e1", "upsert", payload1),
		makeTestRawLine("1.0.0", "ext1", "ext_ext1_data", "data", "e2", "upsert", payload2),
	)
	hash1 := computeContentBoundBatchHash(batch, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test")
	hash2 := computeContentBoundBatchHash(batch, "ext1", 0, "", 1, "1.0.0", "ext_ext1_data", "op-test")
	if hash1 != hash2 {
		t.Fatalf("same content must produce same hash: %s vs %s", hash1, hash2)
	}
	hash3 := computeContentBoundBatchHash(batch, "ext1", 10, "", 2, "1.0.0", "ext_ext1_data", "op-test")
	if hash3 == hash1 || hash3 == "" {
		t.Fatalf("different startCursor should produce different non-empty hash: %s vs %s", hash1, hash3)
	}
}

func TestRestoreTableStoresContentBoundBatchHash(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "content-hash.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_shash_data (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	line1 := makeTestRawLine("1.0.0", "shash", "ext_shash_data", "data", "e1", "upsert", payload1)
	line2 := makeTestRawLine("1.0.0", "shash", "ext_shash_data", "data", "e2", "upsert", payload2)
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_shash_data"},
		RecordCounts:   map[string]int64{"ext_shash_data": 2},
		DataExports:    map[string]string{"ext_shash_data": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "shash", "op-store-hash", string(userStateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var batchHash string
	if err := db.QueryRowContext(ctx,
		`SELECT batch_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?`,
		"op-store-hash", "ext_shash_data").Scan(&batchHash); err != nil {
		t.Fatalf("query batch hash: %v", err)
	}
	if batchHash == "" {
		t.Fatalf("completed journal must not store empty batch_hash")
	}
	staleHash := "batch:1:count:2"
	if batchHash == staleHash {
		t.Fatalf("batch hash must not be the stale count-only format: %s", batchHash)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-store-hash", string(userStateJSON)); err != nil {
		t.Fatalf("verify restore: %v", err)
	}
}

func TestRestoreTableResumeAfterPartialCommit(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "resume-partial.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_rsm_data (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payload3 := map[string]any{"entity_value": "v3"}
	line1 := makeTestRawLine("1.0.0", "rsm", "ext_rsm_data", "data", "e1", "upsert", payload1)
	line2 := makeTestRawLine("1.0.0", "rsm", "ext_rsm_data", "data", "e2", "upsert", payload2)
	line3 := makeTestRawLine("1.0.0", "rsm", "ext_rsm_data", "data", "e3", "upsert", payload3)
	jsonl := line1 + "\n" + line2 + "\n" + line3 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_rsm_data"},
		RecordCounts:   map[string]int64{"ext_rsm_data": 3},
		DataExports:    map[string]string{"ext_rsm_data": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "rsm", "op-resume", string(userStateJSON)); err != nil {
		t.Fatalf("initial restore: %v", err)
	}

	var completeHash string
	if err := db.QueryRowContext(ctx,
		`SELECT batch_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?`,
		"op-resume", "ext_rsm_data").Scan(&completeHash); err != nil {
		t.Fatalf("query complete hash: %v", err)
	}
	if completeHash == "" {
		t.Fatalf("expected non-empty hash after complete restore")
	}
	if err := store.VerifyUserDataRestore(ctx, "op-resume", string(userStateJSON)); err != nil {
		t.Fatalf("verify complete restore: %v", err)
	}
}

func setupRestoreForVerifyTest(t *testing.T) (*sql.DB, *UserDataSnapshotStore, string, string, string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "verify-setup.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE ext_vsetup_verify_setup (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	line1 := makeTestRawLine("1.0.0", "vsetup", "ext_vsetup_verify_setup", "verify_setup", "e1", "upsert", payload1)
	line2 := makeTestRawLine("1.0.0", "vsetup", "ext_vsetup_verify_setup", "verify_setup", "e2", "upsert", payload2)
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_vsetup_verify_setup"},
		RecordCounts:   map[string]int64{"ext_vsetup_verify_setup": 2},
		DataExports:    map[string]string{"ext_vsetup_verify_setup": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "vsetup", "op-verify-setup", string(userStateJSON)); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-verify-setup", string(userStateJSON)); err != nil {
		t.Fatalf("baseline verify should pass: %v", err)
	}
	return db, store, "op-verify-setup", "ext_vsetup_verify_setup", string(userStateJSON)
}

func TestVerifyFailsOnAppliedCountMismatch(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET applied_count=? WHERE operation_id=? AND table_name=?`,
		1, opID, table); err != nil {
		t.Fatalf("tamper applied_count: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for applied_count mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "applied count mismatch") {
		t.Fatalf("expected applied count mismatch error, got: %v", err)
	}
}

func TestVerifyFailsOnCursorMismatch(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET cursor=? WHERE operation_id=? AND table_name=?`,
		"1", opID, table); err != nil {
		t.Fatalf("tamper cursor: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for cursor mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "cursor not at EOF") {
		t.Fatalf("expected cursor mismatch error, got: %v", err)
	}
}

func TestVerifyFailsOnBatchHashEmpty(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET batch_hash=? WHERE operation_id=? AND table_name=?`,
		"", opID, table); err != nil {
		t.Fatalf("clear batch_hash: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for empty batch_hash with rows>0, got nil")
	}
	if !strings.Contains(err.Error(), "empty batch hash") {
		t.Fatalf("expected empty batch hash error, got: %v", err)
	}
}

func TestVerifyFailsOnAggregateHashMismatch(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	validTamperedHash := "sha256:" + strings.Repeat("aa", 32)
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?`,
		validTamperedHash, opID, table); err != nil {
		t.Fatalf("tamper aggregate_hash: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for aggregate hash mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "aggregate hash mismatch") {
		t.Fatalf("expected aggregate hash mismatch error, got: %v", err)
	}
}

func TestVerifyFailsOnDBExtraRecord(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (entity_id, entity_value) VALUES (?, ?)`, table),
		"extra_e", "extra_val"); err != nil {
		t.Fatalf("insert extra row: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for DB extra record, got nil")
	}
	if !strings.Contains(err.Error(), "aggregate hash mismatch") {
		t.Fatalf("expected aggregate hash mismatch error for extra record, got: %v", err)
	}
}

func TestVerifyFailsOnDBMissingRecord(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE entity_id = ?`, table),
		"e2"); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for DB missing record, got nil")
	}
	if !strings.Contains(err.Error(), "aggregate hash mismatch") {
		t.Fatalf("expected aggregate hash mismatch error for missing record, got: %v", err)
	}
}

func TestVerifyFailsOnIncompleteJournal(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state=? WHERE operation_id=? AND table_name=?`,
		string(UserDataRestoreImporting), opID, table); err != nil {
		t.Fatalf("set importing state: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for incomplete journal, got nil")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete error, got: %v", err)
	}
}

func TestVerifyFailsOnNoJournals(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "no-journals.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	snapshotJSON := `{"mode":"repository","affectedTables":["users_v1"],"recordCounts":{"users_v1":3},"dataExports":{"users_v1":"[]"},"tableManifests":{"users_v1":{"tableName":"users_v1","recordCount":3,"schemaVersion":1,"extensionID":"com.example/test","canonicalTable":"users_v1","genesisHash":"sha256:aa","emptySetHash":"sha256:bb","batchHash":"sha256:cc","aggregateHash":"sha256:dd","batchAlgorithmVersion":"v1","batchSize":100,"namespaceHash":"sha256:ee"}}}`
	err = store.VerifyUserDataRestore(ctx, "op-nonexistent", snapshotJSON)
	if err == nil {
		t.Fatal("expected error for no journals, got nil")
	}
	if !strings.Contains(err.Error(), "no restore journals found") {
		t.Fatalf("expected no journals error, got: %v", err)
	}
}

func TestVerifyPassesAllConsistent(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "all-consistent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_conz_consistent (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payload3 := map[string]any{"entity_value": "v3"}
	payload4 := map[string]any{"entity_value": "v4"}
	line1 := makeTestRawLine("1.0.0", "conz", "ext_conz_consistent", "consistent", "a1", "upsert", payload1)
	line2 := makeTestRawLine("1.0.0", "conz", "ext_conz_consistent", "consistent", "a2", "upsert", payload2)
	line3 := makeTestRawLine("1.0.0", "conz", "ext_conz_consistent", "consistent", "a3", "upsert", payload3)
	line4 := makeTestRawLine("1.0.0", "conz", "ext_conz_consistent", "consistent", "a4", "upsert", payload4)
	jsonl := line1 + "\n" + line2 + "\n" + line3 + "\n" + line4 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_conz_consistent"},
		RecordCounts:   map[string]int64{"ext_conz_consistent": 4},
		DataExports:    map[string]string{"ext_conz_consistent": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "conz", "op-consistent", string(userStateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-consistent", string(userStateJSON)); err != nil {
		t.Fatalf("verify should pass when all consistent: %v", err)
	}
	var dbCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ext_conz_consistent`).Scan(&dbCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if dbCount != 4 {
		t.Fatalf("expected 4 rows, got %d", dbCount)
	}
}

func TestVerifyFailsOnImportedRowsMismatch(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET imported_rows=? WHERE operation_id=? AND table_name=?`,
		1, opID, table); err != nil {
		t.Fatalf("tamper imported_rows: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for imported_rows mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "imported rows mismatch") {
		t.Fatalf("expected imported rows mismatch error, got: %v", err)
	}
}

func TestVerifyFailsOnEmptyAggregateHash(t *testing.T) {
	db, store, opID, table, snapshotJSON := setupRestoreForVerifyTest(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?`,
		"", opID, table); err != nil {
		t.Fatalf("clear aggregate_hash: %v", err)
	}
	err := store.VerifyUserDataRestore(ctx, opID, snapshotJSON)
	if err == nil {
		t.Fatal("expected error for empty aggregate_hash, got nil")
	}
	if !strings.Contains(err.Error(), "empty aggregate hash") {
		t.Fatalf("expected empty aggregate hash error, got: %v", err)
	}
}

func TestRestoreTableRecoveryHashMatchSucceeds(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "recovery-match.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_rok_data (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	line1 := makeTestRawLine("1.0.0", "rok", "ext_rok_data", "data", "e1", "upsert", payload1)
	line2 := makeTestRawLine("1.0.0", "rok", "ext_rok_data", "data", "e2", "upsert", payload2)
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_rok_data"},
		RecordCounts:   map[string]int64{"ext_rok_data": 2},
		DataExports:    map[string]string{"ext_rok_data": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "rok", "op-recovery-match", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}

	var firstHash string
	if err := db.QueryRowContext(ctx,
		`SELECT batch_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?`,
		"op-recovery-match", "ext_rok_data").Scan(&firstHash); err != nil {
		t.Fatalf("query first hash: %v", err)
	}
	if firstHash == "" {
		t.Fatalf("expected non-empty hash after first restore")
	}

	_, err = db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state='importing', imported_rows=2, cursor='2' WHERE operation_id=? AND table_name=?`,
		"op-recovery-match", "ext_rok_data")
	if err != nil {
		t.Fatalf("set importing state: %v", err)
	}

	identicalStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "rok", "op-recovery-match", string(identicalStateJSON)); err != nil {
		t.Fatalf("recovery restore with matching hash should succeed: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-recovery-match", string(identicalStateJSON)); err != nil {
		t.Fatalf("verify restore: %v", err)
	}
}

// ============================================================
// 故障注入测试：验证每批原子提交的崩溃恢复行为
// ============================================================

func makeCrashTestTable(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`, table)); err != nil {
		t.Fatalf("create table %s: %v", table, err)
	}
}

func makeNLines(t *testing.T, extensionID, namespace string, count int, prefix string) string {
	t.Helper()
	var lines string
	for i := 0; i < count; i++ {
		payload := map[string]any{"entity_value": fmt.Sprintf("%s_v_%d", prefix, i)}
		line := makeTestRawLine("1.0.0", extensionID, namespace, "data", fmt.Sprintf("%s_e_%04d", prefix, i), "upsert", payload)
		lines += line + "\n"
	}
	return lines
}

// TestFIV_CrashBeforeFirstBatch: 第一批提交前崩溃
// 现象：Journal 尚未有业务批次提交，重启后从头开始完整执行
func TestFIV_CrashBeforeFirstBatch(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-before-first.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi1_data")
	extFi1 := "fi1"
	jsonl := makeNLines(t, extFi1, "ext_fi1_data", 5, "batch0")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi1_data"},
		RecordCounts:   map[string]int64{"ext_fi1_data": 5},
		DataExports:    map[string]string{"ext_fi1_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi1, "op-fi1", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state='pending', imported_rows=0, applied_count=0, cursor='', batch_hash='', aggregate_hash='', batch_index=0, prev_batch_hash='' WHERE operation_id=? AND table_name=?`,
		"op-fi1", "ext_fi1_data"); err != nil {
		t.Fatalf("reset journal: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM ext_fi1_data"); err != nil {
		t.Fatalf("clear data: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi1, "op-fi1", string(userStateJSON)); err != nil {
		t.Fatalf("restart restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-fi1", string(userStateJSON)); err != nil {
		t.Fatalf("verify after restart: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi1_data").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 rows after restart, got %d", count)
	}
}

// TestFIV_CrashAfterFirstBatchCommits: 第一批事务提交后崩溃
// 现象：Journal 记录了第一批的进度，重启后从第二批开始继续
func TestFIV_CrashAfterFirstBatchCommits(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-after-first.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi2_data")
	extFi2 := "fi2"
	jsonl := makeNLines(t, extFi2, "ext_fi2_data", 250, "b")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi2_data"},
		RecordCounts:   map[string]int64{"ext_fi2_data": 250},
		DataExports:    map[string]string{"ext_fi2_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi2, "op-fi2", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	var fullCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi2_data").Scan(&fullCount); err != nil {
		t.Fatalf("full count: %v", err)
	}
	if fullCount != 250 {
		t.Fatalf("expected 250 rows, got %d", fullCount)
	}

	recordsFi2, parsedFi2, _ := parseAndValidateJSONL(jsonl, extFi2)
	h0fi2 := computeContentBoundBatchHash(recordsFi2[0:100], extFi2, 0, userBatchGenesisHash(), 1, parsedFi2[0].SchemaVersion, "ext_fi2_data", "op-fi2")
	_, err = db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state='importing', imported_rows=100, applied_count=100, cursor='100', batch_index=1, prev_batch_hash=?, batch_hash=? WHERE operation_id=? AND table_name=?`,
		userBatchGenesisHash(), h0fi2, "op-fi2", "ext_fi2_data")
	if err != nil {
		t.Fatalf("set partial progress: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"DELETE FROM ext_fi2_data WHERE entity_id IN (SELECT entity_id FROM ext_fi2_data ORDER BY entity_id LIMIT 150 OFFSET 100)"); err != nil {
		t.Fatalf("simulate data loss: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi2, "op-fi2", string(userStateJSON)); err != nil {
		t.Fatalf("restart restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-fi2", string(userStateJSON)); err != nil {
		t.Fatalf("verify after resume: %v", err)
	}
	var finalCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi2_data").Scan(&finalCount); err != nil {
		t.Fatalf("final count: %v", err)
	}
	if finalCount != 250 {
		t.Fatalf("expected 250 rows after resume, got %d", finalCount)
	}
}

// TestFIV_JournalUpdateFailureRollsBackBatch: Journal 更新失败时业务数据回滚
// 现象：importBatchAtomic 事务内的 UPDATE Journal 失败导致整批回滚
func TestFIV_JournalUpdateFailureRollsBackBatch(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-journal-fail.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi3_data")
	extFi3 := "fi3"
	jsonl := makeNLines(t, extFi3, "ext_fi3_data", 10, "jf")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi3_data"},
		RecordCounts:   map[string]int64{"ext_fi3_data": 10},
		DataExports:    map[string]string{"ext_fi3_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_journal_update BEFORE UPDATE ON extension_package_user_data_restore_journal BEGIN SELECT RAISE(ABORT, 'simulated journal update failure'); END`); err != nil {
		t.Fatalf("create fail trigger: %v", err)
	}
	err = store.RestoreUserDataFromSnapshot(ctx, extFi3, "op-fi3", string(userStateJSON))
	if err == nil {
		t.Fatalf("expected error when journal update fails, got nil")
	}
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi3_data").Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 rows (batch rolled back), got %d", count)
	}
}

// TestFIV_MidBatchFailure: 第 N 批中间失败
// 现象：第 2 批导入时结构损坏，Journal 进入 failed，已提交批次保留
func TestFIV_MidBatchFailure(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-mid-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi4_data")
	extFi4 := "fi4"
	jsonl := makeNLines(t, extFi4, "ext_fi4_data", 250, "mb")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi4_data"},
		RecordCounts:   map[string]int64{"ext_fi4_data": 250},
		DataExports:    map[string]string{"ext_fi4_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi4, "op-fi4", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	_, err = db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state='importing', imported_rows=100, applied_count=100, cursor='100' WHERE operation_id=? AND table_name=?`,
		"op-fi4", "ext_fi4_data")
	if err != nil {
		t.Fatalf("set partial progress: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM ext_fi4_data WHERE entity_id > 'mb_e_0099'"); err != nil {
		t.Fatalf("delete rows beyond first batch: %v", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_insert AFTER INSERT ON ext_fi4_data BEGIN SELECT RAISE(ABORT, 'simulated insert failure on batch 2'); END`); err != nil {
		t.Fatalf("create fail trigger: %v", err)
	}

	err = store.RestoreUserDataFromSnapshot(ctx, extFi4, "op-fi4", string(userStateJSON))
	if err == nil {
		t.Fatalf("expected failure on mid-batch restore, got nil")
	}
	var state string
	if err := db.QueryRowContext(ctx,
		"SELECT state FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-fi4", "ext_fi4_data").Scan(&state); err != nil {
		t.Fatalf("query state: %v", err)
	}
	if state != "failed" {
		t.Fatalf("expected failed state, got %s", state)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi4_data").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 100 {
		t.Fatalf("expected 100 committed rows preserved after failure, got %d", count)
	}

}

// TestFIV_RestartExecutesOnlyRemainingBatches: 重启后只执行剩余批次
// 现象：前150条仍有，后150条待续跑，游标在 150
func TestFIV_RestartExecutesOnlyRemainingBatches(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-remaining.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi5_data")
	extFi5 := "fi5"
	jsonl := makeNLines(t, extFi5, "ext_fi5_data", 300, "rm")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi5_data"},
		RecordCounts:   map[string]int64{"ext_fi5_data": 300},
		DataExports:    map[string]string{"ext_fi5_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi5, "op-fi5", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	records5, parsed5, _ := parseAndValidateJSONL(jsonl, extFi5)
	h0fi5 := computeContentBoundBatchHash(records5[0:100], extFi5, 0, userBatchGenesisHash(), 1, parsed5[0].SchemaVersion, "ext_fi5_data", "op-fi5")
	_, err = db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET state='importing', imported_rows=100, applied_count=100, cursor='100', batch_index=1, prev_batch_hash=?, batch_hash=? WHERE operation_id=? AND table_name=?`,
		userBatchGenesisHash(), h0fi5, "op-fi5", "ext_fi5_data")
	if err != nil {
		t.Fatalf("set mid-batch progress: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"DELETE FROM ext_fi5_data WHERE entity_id IN (SELECT entity_id FROM ext_fi5_data ORDER BY entity_id LIMIT 200 OFFSET 100)"); err != nil {
		t.Fatalf("simulate data loss: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi5, "op-fi5", string(userStateJSON)); err != nil {
		t.Fatalf("restart restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-fi5", string(userStateJSON)); err != nil {
		t.Fatalf("verify after restart: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi5_data").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 300 {
		t.Fatalf("expected 300 rows after resume, got %d", count)
	}
	var cursor string
	if err := db.QueryRowContext(ctx,
		"SELECT cursor FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-fi5", "ext_fi5_data").Scan(&cursor); err != nil {
		t.Fatalf("query cursor: %v", err)
	}
	if cursor != "300" {
		t.Fatalf("expected cursor=300, got %s", cursor)
	}
}

// TestFIV_RepeatedRestoreIsIdempotent: 重复调用 Restore 保持幂等（INSERT OR REPLACE 语义）
func TestFIV_RepeatedRestoreIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fi-idempotent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	makeCrashTestTable(t, db, "ext_fi6_data")
	extFi6 := "fi6"
	jsonl := makeNLines(t, extFi6, "ext_fi6_data", 20, "idmp")

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fi6_data"},
		RecordCounts:   map[string]int64{"ext_fi6_data": 20},
		DataExports:    map[string]string{"ext_fi6_data": jsonl},
	}
	userStateJSON, _ := json.Marshal(userState)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extFi6, "op-fi6", string(userStateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, extFi6, "op-fi6", string(userStateJSON)); err != nil {
		t.Fatalf("second restore: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, extFi6, "op-fi6", string(userStateJSON)); err != nil {
		t.Fatalf("third restore: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ext_fi6_data").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 20 {
		t.Fatalf("expected 20 rows (no duplication), got %d", count)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-fi6", string(userStateJSON)); err != nil {
		t.Fatalf("verify after idempotent calls: %v", err)
	}
}
