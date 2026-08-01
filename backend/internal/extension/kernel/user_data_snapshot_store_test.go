package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func newUserDataTableTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func makeTestUserDataJSONL(extID, table string, entityType string, payloads ...interface{}) string {
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	var lines []string
	for _, payload := range payloads {
		payloadBytes, _ := json.Marshal(payload)
		hash := sha256.Sum256(payloadBytes)
		hashStr := "sha256:" + hex.EncodeToString(hash[:])
		record := map[string]interface{}{
			"schemaVersion": "1.0.0",
			"extensionID":   extID,
			"namespace":     nsPrefix + strings.TrimPrefix(table, nsPrefix),
			"entityType":    entityType,
			"entityID":      "entity-1",
			"operation":     "import",
			"payload":       payload,
			"payloadHash":   hashStr,
		}
		line, _ := json.Marshal(record)
		lines = append(lines, string(line))
	}
	return strings.Join(lines, "\n")
}

func buildUserStateJSON(extID, table, jsonlData string) string {
	state := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		DataExports:    map[string]string{table: jsonlData},
	}
	b, _ := json.Marshal(state)
	return string(b)
}

func TestRestoreUserData_ValidPayloadHash(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	// create table with columns matching record field names
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS ext_com_example_test_users (entityID TEXT, entityType TEXT, extensionID TEXT, namespace TEXT, operation TEXT, payload TEXT, payloadHash TEXT, schemaVersion TEXT)`); err != nil {
		t.Fatal(err)
	}
	payload := "hello-world"
	jsonlData := makeTestUserDataJSONL(extID, table, "user", payload)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-1", userStateJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestoreUserData_TamperedPayloadHash(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	payload := "hello-world"
	jsonlData := makeTestUserDataJSONL(extID, table, "user", payload)
	parts := strings.Split(jsonlData, "\n")
	var tampered []string
	for _, line := range parts {
		var record map[string]interface{}
		json.Unmarshal([]byte(line), &record)
		record["payloadHash"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		b, _ := json.Marshal(record)
		tampered = append(tampered, string(b))
	}
	tamperedJSONL := strings.Join(tampered, "\n")
	userStateJSON := buildUserStateJSON(extID, table, tamperedJSONL)

	err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-2", userStateJSON)
	if err == nil {
		t.Fatal("expected error for tampered payloadHash, got nil")
	}
	if !strings.Contains(err.Error(), "payloadHash mismatch") {
		t.Fatalf("expected payloadHash mismatch error, got: %v", err)
	}
}

func TestRestoreUserData_WrongExtensionID(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	wrongExtID := "com.example.other"
	table := "ext_com_example_other_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	payload := "hello-world"
	jsonlData := makeTestUserDataJSONL(extID, table, "user", payload)
	userStateJSON := buildUserStateJSON(wrongExtID, table, jsonlData)

	err := store.RestoreUserDataFromSnapshot(ctx, wrongExtID, "op-3", userStateJSON)
	if err == nil {
		t.Fatal("expected error for wrong extensionID, got nil")
	}
	if !strings.Contains(err.Error(), "extensionID mismatch") {
		t.Fatalf("expected extensionID mismatch error, got: %v", err)
	}
}

func TestRestoreUserData_WrongNamespace(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	payload := "hello-world"
	record := map[string]interface{}{
		"schemaVersion": "1.0.0",
		"extensionID":   extID,
		"namespace":     "wrong_namespace_user",
		"entityType":    "user",
		"entityID":      "entity-1",
		"operation":     "import",
		"payload":       payload,
		"payloadHash":   computeUserDataPayloadHash(payload),
	}
	line, _ := json.Marshal(record)
	jsonlData := string(line)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-4", userStateJSON)
	if err == nil {
		t.Fatal("expected error for wrong namespace, got nil")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("expected namespace error, got: %v", err)
	}
}

func TestRestoreUserData_EmptyEntityType(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	payload := "hello-world"
	record := map[string]interface{}{
		"schemaVersion": "1.0.0",
		"extensionID":   extID,
		"namespace":     migration.ExtensionNamespacePrefix(extID) + "users",
		"entityType":    "",
		"entityID":      "entity-1",
		"operation":     "import",
		"payload":       payload,
		"payloadHash":   computeUserDataPayloadHash(payload),
	}
	line, _ := json.Marshal(record)
	jsonlData := string(line)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-5", userStateJSON)
	if err == nil {
		t.Fatal("expected error for empty entityType, got nil")
	}
	if !strings.Contains(err.Error(), "entityType") {
		t.Fatalf("expected entityType error, got: %v", err)
	}
}

func TestRestoreUserData_EmptyPayloadHash(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	payload := "hello-world"
	record := map[string]interface{}{
		"schemaVersion": "1.0.0",
		"extensionID":   extID,
		"namespace":     migration.ExtensionNamespacePrefix(extID) + "users",
		"entityType":    "user",
		"entityID":      "entity-1",
		"operation":     "import",
		"payload":       payload,
		"payloadHash":   "",
	}
	line, _ := json.Marshal(record)
	jsonlData := string(line)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-6", userStateJSON)
	if err == nil {
		t.Fatal("expected error for empty payloadHash, got nil")
	}
	if !strings.Contains(err.Error(), "payloadHash") {
		t.Fatalf("expected payloadHash error, got: %v", err)
	}
}

func TestParseAndValidateUserDataJSONL_ValidRecord(t *testing.T) {
	data := `{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e1","operation":"upsert","payload":{"title":"hello"},"payloadHash":"sha256:abc"}
{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e2","operation":"upsert","payload":{"title":"world"},"payloadHash":"sha256:def"}`
	records, err := parseAndValidateUserDataJSONL(data)
	if err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0]["entityID"] != "e1" || records[1]["entityID"] != "e2" {
		t.Fatalf("record content mismatch: %+v", records)
	}
}

func TestParseAndValidateUserDataJSONL_SkipsEmptyLines(t *testing.T) {
	data := `
{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e1","operation":"upsert","payload":{},"payloadHash":"sha256:abc"}

`
	records, err := parseAndValidateUserDataJSONL(data)
	if err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

func TestParseAndValidateUserDataJSONL_InvalidJSON(t *testing.T) {
	data := `{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e1","operation":"upsert","payload":{},"payloadHash":"sha256:abc"}
not-json-at-all
{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e2","operation":"upsert","payload":{},"payloadHash":"sha256:def"}`
	_, err := parseAndValidateUserDataJSONL(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON line, got nil")
	}
	if !isPackageUserDataSnapshotError(err) {
		t.Fatalf("expected PACKAGE_USER_DATA_SNAPSHOT_INVALID, got: %v", err)
	}
}

func TestParseAndValidateUserDataJSONL_MissingField(t *testing.T) {
	data := `{"extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e1","operation":"upsert","payload":{},"payloadHash":"sha256:abc"}`
	_, err := parseAndValidateUserDataJSONL(data)
	if err == nil {
		t.Fatal("expected error for missing schemaVersion, got nil")
	}
	if !isPackageUserDataSnapshotError(err) {
		t.Fatalf("expected PACKAGE_USER_DATA_SNAPSHOT_INVALID for missing schemaVersion, got: %v", err)
	}
}

func TestRestoreUserDataFromSnapshot_MissingJSONL(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	userStateJSON := `{"mode":"repository","affectedTables":["ns_ext_1_config"],"dataExports":{},"recordCounts":{"ns_ext_1_config":2}}`
	err := store.RestoreUserDataFromSnapshot(ctx, "ext-1", "op-1", userStateJSON)
	if err == nil {
		t.Fatal("expected error for missing JSONL, got nil")
	}
	if !isPackageUserDataSnapshotError(err) {
		t.Fatalf("expected PACKAGE_USER_DATA_SNAPSHOT_INVALID for missing JSONL, got: %v", err)
	}
}

func TestRestoreUserDataFromSnapshot_EmptyJSONL(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	userStateJSON := `{"mode":"repository","affectedTables":["ns_ext_1_config"],"dataExports":{"ns_ext_1_config":""},"recordCounts":{"ns_ext_1_config":2}}`
	err := store.RestoreUserDataFromSnapshot(ctx, "ext-1", "op-1", userStateJSON)
	if err == nil {
		t.Fatal("expected error for empty JSONL, got nil")
	}
	if !isPackageUserDataSnapshotError(err) {
		t.Fatalf("expected PACKAGE_USER_DATA_SNAPSHOT_INVALID for empty JSONL, got: %v", err)
	}
}

func TestRestoreUserDataFromSnapshot_RecordCountMismatch(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	rec := map[string]interface{}{"schemaVersion":"1","extensionID":"ext-1","namespace":"ns_ext_1","entityType":"note","entityID":"e1","operation":"upsert","payload":map[string]interface{}{},"payloadHash":"sha256:abc"}
	jb, _ := json.Marshal(rec)
	jsonl := string(jb)
	dataExports := map[string]string{"ns_ext_1_config": jsonl}
	recordCounts := map[string]int64{"ns_ext_1_config": 3}
	userStateMap := map[string]interface{}{"mode": "repository", "affectedTables": []string{"ns_ext_1_config"}, "dataExports": dataExports, "recordCounts": recordCounts}
	userStateJSONBytes, _ := json.Marshal(userStateMap)
	userStateJSON := string(userStateJSONBytes)
	err := store.RestoreUserDataFromSnapshot(ctx, "ext-1", "op-1", userStateJSON)
	if err == nil {
		t.Fatal("expected error for record count mismatch, got nil")
	}
	if !isPackageUserDataSnapshotError(err) {
		t.Fatalf("expected PACKAGE_USER_DATA_SNAPSHOT_INVALID for record count mismatch, got: %v", err)
	}
}

func isPackageUserDataSnapshotError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *PackageOperationError
	if errors.As(err, &opErr) && opErr.Code == PackageErrCodeUserDataSnapshotInvalid {
		return true
	}
	var pkgErr *PackageError
	if errors.As(err, &pkgErr) && pkgErr.Code == PackageErrCodeUserDataSnapshotInvalid {
		return true
	}
	return false
}

func TestRestoreUserData_CursorCheckpointProgress(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	var payloads []interface{}
	for i := 0; i < 5; i++ {
		payloads = append(payloads, map[string]interface{}{"name": "user", "index": i})
	}
	jsonlData := makeTestUserDataJSONL(extID, table, "user", payloads...)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-checkpoint", userStateJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	appliedCount, cursor, batchHash, _, err := store.GetAppliedCount(ctx, "op-checkpoint", table)
	if err != nil {
		t.Fatalf("failed to get applied count: %v", err)
	}
	if appliedCount <= 0 {
		t.Fatalf("expected applied_count > 0, got %d", appliedCount)
	}
	if cursor == "" {
		t.Fatal("expected non-empty cursor after import")
	}
	if batchHash == "" {
		t.Fatal("expected non-empty batch_hash after import")
	}
	if !strings.HasPrefix(batchHash, "sha256:") {
		t.Fatalf("expected batch_hash to start with sha256:, got: %s", batchHash)
	}
}

func TestRestoreUserData_ResumeFromLastCheckpoint(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	var payloads []interface{}
	for i := 0; i < 3; i++ {
		payloads = append(payloads, map[string]interface{}{"name": "user", "index": i})
	}
	jsonlData := makeTestUserDataJSONL(extID, table, "user", payloads...)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-resume", userStateJSON); err != nil {
		t.Fatalf("first restore error: %v", err)
	}

	appliedCount1, cursor1, batchHash1, _, err := store.GetAppliedCount(ctx, "op-resume", table)
	if err != nil {
		t.Fatalf("failed to get first checkpoint: %v", err)
	}

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-resume", userStateJSON); err != nil {
		t.Fatalf("second restore error: %v", err)
	}

	appliedCount2, cursor2, batchHash2, _, err := store.GetAppliedCount(ctx, "op-resume", table)
	if err != nil {
		t.Fatalf("failed to get second checkpoint: %v", err)
	}

	if cursor1 != cursor2 {
		t.Fatalf("cursor mismatch after re-import: %s vs %s", cursor1, cursor2)
	}
	if batchHash1 != batchHash2 {
		t.Fatalf("batch_hash mismatch after re-import: %s vs %s", batchHash1, batchHash2)
	}
	if appliedCount1 != appliedCount2 {
		t.Fatalf("applied_count mismatch after re-import: %d vs %d", appliedCount1, appliedCount2)
	}
}

func TestRestoreUserData_BatchHashChangesPerBatch(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_users"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	var payloads []interface{}
	for i := 0; i < userDataRestoreBatchSize+5; i++ {
		payloads = append(payloads, map[string]interface{}{"name": "item", "id": i})
	}
	jsonlData := makeTestUserDataJSONL(extID, table, "item", payloads...)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-batch-hash", userStateJSON); err != nil {
		t.Fatalf("restore error: %v", err)
	}

	_, _, batchHash, _, err := store.GetAppliedCount(ctx, "op-batch-hash", table)
	if err != nil {
		t.Fatalf("failed to get batch hash: %v", err)
	}
	if !strings.HasPrefix(batchHash, "sha256:") {
		t.Fatalf("expected valid sha256 batch_hash, got: %s", batchHash)
	}
	if len(batchHash) != len("sha256:")+64 {
		t.Fatalf("expected 64 hex chars in batch_hash, got length %d", len(batchHash))
	}
}

func TestRestoreUserData_JournalStateWriteFailureStopsRestore(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_test_data (id TEXT PRIMARY KEY, entityID TEXT, entityType TEXT, namespace TEXT, schemaVersion TEXT, extensionID TEXT, payload TEXT, payloadHash TEXT, operation TEXT)`); err != nil {
		t.Fatal(err)
	}
	extID := "com.example.test"
	table := "ext_com_example_test_data"
	opID := "op-state-write-fail"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j-1", opID, extID, table, 1, 0, 0, "", "", "", "pending", now, now, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TRIGGER block_journal_state_update
		 BEFORE UPDATE ON extension_package_user_data_restore_journal
		 WHEN OLD.state = 'importing'
		 BEGIN
			 SELECT RAISE(ABORT, 'simulated journal state update failure');
		 END;`); err != nil {
		t.Fatal(err)
	}
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	payload := "test-value"
	payloadBytes, _ := json.Marshal(payload)
	hash := sha256.Sum256(payloadBytes)
	hashStr := "sha256:" + hex.EncodeToString(hash[:])
	recordJSON := fmt.Sprintf(`{"schemaVersion":"1.0.0","extensionID":"%s","namespace":"%sdata","entityType":"record","entityID":"entity-1","operation":"import","payload":"%s","payloadHash":"%s"}`, extID, nsPrefix, payload, hashStr)
	state := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		DataExports:    map[string]string{table: recordJSON},
	}
	userStateJSON, _ := json.Marshal(state)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, opID, string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when journal state write fails, got nil")
	}
	if !strings.Contains(err.Error(), "update journal state failed") {
		t.Fatalf("expected error to mention 'update journal state failed', got: %v", err)
	}
	_ = db.Close()
}

func TestRestoreUserData_JournalProgressWriteFailureStopsRestore(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_test_data2 (id TEXT PRIMARY KEY, entityID TEXT, entityType TEXT, namespace TEXT, schemaVersion TEXT, extensionID TEXT, payload TEXT, payloadHash TEXT, operation TEXT)`); err != nil {
		t.Fatal(err)
	}
	extID := "com.example.test"
	table := "ext_com_example_test_data2"
	opID := "op-progress-write-fail"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO ext_com_example_test_data2 (id, entityID, entityType, namespace, schemaVersion, extensionID, payload, payloadHash, operation) VALUES ('old-1', 'old-1', 'record', 'ext_com_example_test_data2', 'com.example.test', 'old-1', 'to-be-deleted', 'hash-of-old-1', 'import')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j-2", opID, extID, table, 1, 0, 0, "", "", "", "pending", now, now, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TRIGGER block_delete_rows
		 BEFORE DELETE ON ext_com_example_test_data2
		 BEGIN
			 SELECT RAISE(ABORT, 'simulated delete failure');
		 END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TRIGGER block_journal_on_delete_fail
		 BEFORE UPDATE ON extension_package_user_data_restore_journal
		 WHEN NEW.state = 'failed'
		 BEGIN
			 SELECT RAISE(ABORT, 'simulated journal update failure on delete fail');
		 END;`); err != nil {
		t.Fatal(err)
	}
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	payload := "new-value"
	payloadBytes, _ := json.Marshal(payload)
	hash := sha256.Sum256(payloadBytes)
	hashStr := "sha256:" + hex.EncodeToString(hash[:])
	recordJSON := fmt.Sprintf(`{"schemaVersion":"1.0.0","extensionID":"%s","namespace":"%sdata2","entityType":"record","entityID":"entity-new","operation":"import","payload":"%s","payloadHash":"%s"}`, extID, nsPrefix, payload, hashStr)
	state := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		DataExports:    map[string]string{table: recordJSON},
	}
	userStateJSON, _ := json.Marshal(state)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, opID, string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when journal progress/state write fails, got nil")
	}
	if !strings.Contains(err.Error(), "update journal") {
		t.Fatalf("expected error to mention 'update journal', got: %v", err)
	}
	_ = db.Close()
}

func TestRestoreUserData_JournalWriteFailureAfterImportStopsRestore(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_test_data3 (id TEXT PRIMARY KEY, entityID TEXT, entityType TEXT, namespace TEXT, schemaVersion TEXT, extensionID TEXT, payload TEXT, payloadHash TEXT, operation TEXT)`); err != nil {
		t.Fatal(err)
	}
	extID := "com.example.test"
	table := "ext_com_example_test_data3"
	opID := "op-import-success-journal-fails"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"j-3", opID, extID, table, 1, 0, 0, "", "", "", "pending", now, now, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TRIGGER block_journal_after_import
		 BEFORE UPDATE ON extension_package_user_data_restore_journal
		 WHEN OLD.state = 'importing' AND NEW.imported_rows > 0
		 BEGIN
			 SELECT RAISE(ABORT, 'simulated journal write failure after import');
		 END;`); err != nil {
		t.Fatal(err)
	}
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	payload := "ok-value"
	payloadBytes, _ := json.Marshal(payload)
	hash := sha256.Sum256(payloadBytes)
	hashStr := "sha256:" + hex.EncodeToString(hash[:])
	recordJSON := fmt.Sprintf(`{"schemaVersion":"1.0.0","extensionID":"%s","namespace":"%sdata3","entityType":"record","entityID":"entity-ok","operation":"import","payload":"%s","payloadHash":"%s"}`, extID, nsPrefix, payload, hashStr)
	state := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		DataExports:    map[string]string{table: recordJSON},
	}
	userStateJSON, _ := json.Marshal(state)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, opID, string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when journal write fails after import, got nil")
	}
	if !strings.Contains(err.Error(), "simulated journal write failure after import") {
		t.Fatalf("expected error to mention simulated failure, got: %v", err)
	}
	_ = db.Close()
}

func newUserDataSnapshotStoreForTest(t *testing.T) *UserDataSnapshotStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestNewUserDataSnapshotStoreEnsureSchemaCreatesNewColumns(t *testing.T) {
	store := newUserDataSnapshotStoreForTest(t)
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
	 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
	 VALUES ('j1', 'op1', 'ext1', 't1', 0, 0, 0, '', '', '', 'pending', '', '', '')`)
	if err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	var appliedCount int64
	var cursor, batchHash, namespaceHash string
	err = store.db.QueryRowContext(ctx,
		`SELECT applied_count, cursor, batch_hash, namespace_hash FROM extension_package_user_data_restore_journal WHERE journal_id = 'j1'`).Scan(&appliedCount, &cursor, &batchHash, &namespaceHash)
	if err != nil {
		t.Fatalf("failed to read back new columns: %v", err)
	}
	if appliedCount != 0 || cursor != "" || batchHash != "" || namespaceHash != "" {
		t.Fatalf("expected new columns to default to 0/empty, got: %d / %q / %q / %q", appliedCount, cursor, batchHash, namespaceHash)
	}
}

func TestUpdateAppliedCountPersistsAllFields(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_test_records"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_test_records (id TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}

	operationID := "op-update-test"
	payload := map[string]interface{}{"name": "test"}
	payloadBytes, _ := json.Marshal(payload)
	phash := sha256.Sum256(payloadBytes)
	hashStr := "sha256:" + hex.EncodeToString(phash[:])
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	jsonlData := fmt.Sprintf(`{"schemaVersion":"1.0.0","extensionID":"%s","namespace":"%srecords","entityType":"record","entityID":"e1","operation":"import","payload":{"name":"test"},"payloadHash":"%s"}`, extID, nsPrefix, hashStr)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, operationID, userStateJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := store.UpdateAppliedCount(ctx, operationID, table, 42, "c-42", "bh-42", "nsh-42"); err != nil {
		t.Fatalf("update applied count failed: %v", err)
	}
	applied, cursor, batchHash, nsHash, err := store.GetAppliedCount(ctx, operationID, table)
	if err != nil {
		t.Fatalf("get applied count failed: %v", err)
	}
	if applied != 42 {
		t.Fatalf("expected applied 42, got %d", applied)
	}
	if cursor != "c-42" {
		t.Fatalf("expected cursor c-42, got %q", cursor)
	}
	if batchHash != "bh-42" {
		t.Fatalf("expected batchHash bh-42, got %q", batchHash)
	}
	if nsHash != "nsh-42" {
		t.Fatalf("expected nsHash nsh-42, got %q", nsHash)
	}
}

func TestNewColumnsDefaultValues(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_default_test"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_default_test (id TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}

	operationID := "op-default-test"
	payload := map[string]interface{}{"name": "test"}
	payloadBytes, _ := json.Marshal(payload)
	phash := sha256.Sum256(payloadBytes)
	hashStr := "sha256:" + hex.EncodeToString(phash[:])
	nsPrefix := migration.ExtensionNamespacePrefix(extID)
	jsonlData := fmt.Sprintf(`{"schemaVersion":"1.0.0","extensionID":"%s","namespace":"%sdefault_test","entityType":"record","entityID":"e1","operation":"import","payload":{"name":"test"},"payloadHash":"%s"}`, extID, nsPrefix, hashStr)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.RestoreUserDataFromSnapshot(ctx, extID, operationID, userStateJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	applied, cursor, batchHash, nsHash, err := store.GetAppliedCount(ctx, operationID, table)
	if err != nil {
		t.Fatalf("get applied count failed: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected applied 1 after single-row import, got %d", applied)
	}
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}
	if batchHash == "" {
		t.Fatal("expected non-empty batchHash")
	}
	if nsHash != "" {
		t.Fatalf("expected empty nsHash, got %q", nsHash)
	}
}

func TestRestoreResumeFromCrashContinuesImport(t *testing.T) {
	ctx := context.Background()
	db := newUserDataTableTestDB(t)
	extID := "com.example.test"
	table := "ext_com_example_resume_test"

	store := NewUserDataSnapshotStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ext_com_example_resume_test (id TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}

	operationID := "op-resume-test"
	var payloads []interface{}
	for i := 0; i < userDataRestoreBatchSize+10; i++ {
		payloads = append(payloads, map[string]interface{}{"id": fmt.Sprintf("item-%d", i), "value": i})
	}
	jsonlData := makeTestUserDataJSONL(extID, table, "item", payloads...)
	userStateJSON := buildUserStateJSON(extID, table, jsonlData)

	if err := store.UpdateAppliedCount(ctx, operationID, table, 10, "cursor-10", "batch-hash-10", "ns-hash-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, operationID, userStateJSON); err != nil {
		t.Fatalf("unexpected error on resume: %v", err)
	}

	applied, _, _, _, err := store.GetAppliedCount(ctx, operationID, table)
	if err != nil {
		t.Fatal(err)
	}
	if applied != int64(len(payloads)) {
		t.Fatalf("expected %d applied after resume, got %d", len(payloads), applied)
	}
}

func TestJournalIndexCreation(t *testing.T) {
	store := newUserDataSnapshotStoreForTest(t)
	ctx := context.Background()
	var idxName string
	err := store.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_ext_pkg_udr_journal_ns_hash'`).Scan(&idxName)
	if err != nil {
		t.Fatalf("expected index idx_ext_pkg_udr_journal_ns_hash to exist: %v", err)
	}
	if idxName != "idx_ext_pkg_udr_journal_ns_hash" {
		t.Fatalf("expected idx_ext_pkg_udr_journal_ns_hash, got %q", idxName)
	}
}
