package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

const userDataRestoreBatchSize = 100

const userDataRestoreJournalDDL = `CREATE TABLE IF NOT EXISTS extension_package_user_data_restore_journal (
	journal_id TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	extension_id TEXT NOT NULL,
	table_name TEXT NOT NULL,
	total_rows INTEGER NOT NULL DEFAULT 0,
	imported_rows INTEGER NOT NULL DEFAULT 0,
	applied_count INTEGER NOT NULL DEFAULT 0,
	cursor TEXT NOT NULL DEFAULT '',
	batch_hash TEXT NOT NULL DEFAULT '',
	namespace_hash TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'pending',
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	error_detail TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (operation_id, table_name)
)`

type UserDataRestoreState string

const (
	UserDataRestorePending   UserDataRestoreState = "pending"
	UserDataRestoreImporting UserDataRestoreState = "importing"
	UserDataRestoreCompleted UserDataRestoreState = "completed"
	UserDataRestoreFailed    UserDataRestoreState = "failed"
)

type UserDataRestoreJournal struct {
	JournalID     string
	OperationID   string
	ExtensionID   string
	TableName     string
	TotalRows     int64
	ImportedRows  int64
	AppliedCount  int64
	Cursor        string
	BatchHash     string
	NamespaceHash string
	State         UserDataRestoreState
	StartedAt     string
	UpdatedAt     string
	ErrorDetail   string
}

type UserDataSnapshotStore struct {
	db *sql.DB
}

func NewUserDataSnapshotStore(db *sql.DB) *UserDataSnapshotStore {
	return &UserDataSnapshotStore{db: db}
}

func (s *UserDataSnapshotStore) EnsureSchema(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("kernel: user data snapshot store database unavailable")
	}
	if _, err := s.db.ExecContext(ctx, userDataRestoreJournalDDL); err != nil {
		return err
	}
	if err := s.ensureJournalColumns(ctx); err != nil {
		return err
	}
	return nil
}

func (s *UserDataSnapshotStore) ensureJournalColumns(ctx context.Context) error {
	closures := []func(context.Context) error{
		s.ensureAppliedCountColumn,
		s.ensureCursorColumn,
		s.ensureBatchHashColumn,
		s.ensureNamespaceHashColumn,
	}
	for _, fn := range closures {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserDataSnapshotStore) ensureAppliedCountColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='applied_count'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN applied_count INTEGER NOT NULL DEFAULT 0`)
	return err
}

func (s *UserDataSnapshotStore) ensureCursorColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='cursor'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN cursor TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureBatchHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='batch_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN batch_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureNamespaceHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='namespace_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN namespace_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

type userDataRecord struct {
	SchemaVersion string                 `json:"schemaVersion"`
	ExtensionID   string                 `json:"extensionID"`
	Namespace     string                 `json:"namespace"`
	EntityType    string                 `json:"entityType"`
	EntityID      string                 `json:"entityID"`
	Operation     string                 `json:"operation"`
	Payload       map[string]interface{} `json:"payload"`
	PayloadHash   string                 `json:"payloadHash"`
}

func validateUserDataRecord(record userDataRecord, extensionID string) error {
	var missing []string
	if record.SchemaVersion == "" {
		missing = append(missing, "schemaVersion")
	}
	if record.ExtensionID == "" {
		missing = append(missing, "extensionID")
	}
	if record.Namespace == "" {
		missing = append(missing, "namespace")
	}
	if record.EntityType == "" {
		missing = append(missing, "entityType")
	}
	if record.EntityID == "" {
		missing = append(missing, "entityID")
	}
	if record.Operation == "" {
		missing = append(missing, "operation")
	}
	if record.Payload == nil {
		missing = append(missing, "payload")
	}
	if record.PayloadHash == "" {
		missing = append(missing, "payloadHash")
	}
	if len(missing) > 0 {
		return fmt.Errorf("kernel: user data record missing required fields: %s", strings.Join(missing, ", "))
	}
	if record.ExtensionID != extensionID {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data record extensionID mismatch: got %s expected %s", record.ExtensionID, extensionID))
	}
	expectedPrefix := strings.ToLower(migration.ExtensionNamespacePrefix(extensionID))
	if !strings.HasPrefix(strings.ToLower(record.Namespace), expectedPrefix) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data record namespace %q does not belong to extension namespace prefix %q", record.Namespace, expectedPrefix))
	}
	if !isValidEntityType(record.EntityType) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data record entityType %q not in allowed set", record.EntityType))
	}
	if !isValidOperation(record.Operation) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data record operation %q not in allowed set", record.Operation))
	}
	expectedHash := computeUserDataPayloadHash(record.Payload)
	if expectedHash != record.PayloadHash {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data record payloadHash mismatch for entity %s: expected %s got %s", record.EntityID, expectedHash, record.PayloadHash))
	}
	return nil
}

func isValidEntityType(entityType string) bool {
	switch entityType {
	case "entity", "record", "item", "document", "setting", "state", "snapshot":
		return true
	default:
		return false
	}
}

func isValidOperation(operation string) bool {
	switch operation {
	case "upsert", "delete", "insert", "update":
		return true
	default:
		return false
	}
}

func (s *UserDataSnapshotStore) RestoreUserDataFromSnapshot(ctx context.Context, extensionID, operationID, userStateJSON string) error {
	if s.db == nil {
		return fmt.Errorf("kernel: user data snapshot store database unavailable")
	}
	if err := s.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("kernel: ensure user data restore journal schema: %w", err)
	}
	var userState packageUserDataMigrationState
	if err := json.Unmarshal([]byte(userStateJSON), &userState); err != nil {
		return fmt.Errorf("kernel: user data snapshot corrupt: %w", err)
	}
	if userState.Mode == "none" || len(userState.AffectedTables) == 0 {
		return nil
	}
	for _, table := range userState.AffectedTables {
		jsonlData, exists := userState.DataExports[table]
		if !exists {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data snapshot missing for affected table %s", table))
		}
		if strings.TrimSpace(jsonlData) == "" {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data snapshot export empty for affected table %s", table))
		}
		expectedCount, hasCount := userState.RecordCounts[table]
		actualCount := int64(strings.Count(jsonlData, "\n"))
		if hasCount && expectedCount != actualCount {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data snapshot record count mismatch for table %s: expected %d, got %d", table, expectedCount, actualCount))
		}
		if err := s.restoreTable(ctx, extensionID, operationID, table, jsonlData); err != nil {
			return fmt.Errorf("kernel: restore user data table %s: %w", table, err)
		}
	}
	return nil
}

func (s *UserDataSnapshotStore) restoreTable(ctx context.Context, extensionID, operationID, table, jsonlData string) error {
	if !migration.IsExtensionNamespaceTable(table, extensionID) {
		return fmt.Errorf("kernel: user data table %q does not belong to extension namespace", table)
	}
	records, err := parseAndValidateJSONL(jsonlData, extensionID)
	if err != nil {
		return err
	}
	journal, err := s.getOrCreateRestoreJournal(ctx, operationID, extensionID, table, int64(len(records)))
	if err != nil {
		return err
	}
	if journal.State == UserDataRestoreCompleted {
		if err := s.verifyJournalAppliedCount(journal, int64(len(records))); err != nil {
			return err
		}
		return nil
	}
	if journal.State == UserDataRestoreFailed {
		journal.ImportedRows = 0
		journal.AppliedCount = 0
		journal.Cursor = ""
		journal.BatchHash = ""
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		return fmt.Errorf("kernel: update journal to importing: %w", err)
	}
	startCursor := int(journal.ImportedRows)
	batchRecords := records
	if startCursor > 0 && startCursor < len(records) {
		batchRecords = records[startCursor:]
	}
	imported, batchHash, importErr := s.importRowsFromJSONL(ctx, table, batchRecords, journal.AppliedCount)
	if importErr != nil {
		if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, importErr.Error()); journalErr != nil {
			return errors.Join(importErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
		}
		return importErr
	}
	totalImported := int64(startCursor) + int64(imported)
	cursorPos := totalImported
	if err := s.updateRestoreJournalProgress(ctx, journal, totalImported, strconv.FormatInt(cursorPos, 10), batchHash); err != nil {
		return fmt.Errorf("kernel: update journal progress: %w", err)
	}
	if err := s.deleteDifferingRows(ctx, table, records, extensionID); err != nil {
		if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error()); journalErr != nil {
			return errors.Join(err, fmt.Errorf("kernel: update journal to failed after diff delete: %w", journalErr))
		}
		return fmt.Errorf("kernel: delete differing rows for %s: %w", table, err)
	}
	appliedCount := journal.AppliedCount + int64(imported)
	if err := s.updateAppliedCount(ctx, journal, appliedCount); err != nil {
		return fmt.Errorf("kernel: update applied count: %w", err)
	}
	if totalImported != int64(len(records)) {
		return fmt.Errorf("kernel: import incomplete for table %s", table)
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreCompleted, ""); err != nil {
		return fmt.Errorf("kernel: update journal to completed: %w", err)
	}
	return nil
}

func (s *UserDataSnapshotStore) verifyJournalAppliedCount(journal *UserDataRestoreJournal, expected int64) error {
	if journal.State == UserDataRestoreCompleted && journal.ImportedRows != expected {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed journal imported rows %d does not match snapshot %d", journal.ImportedRows, expected))
	}
	return nil
}

func (s *UserDataSnapshotStore) getOrCreateRestoreJournal(ctx context.Context, operationID, extensionID, table string, totalRows int64) (*UserDataRestoreJournal, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal := &UserDataRestoreJournal{
		JournalID:   "ud-journal-" + operationID + "-" + table,
		OperationID: operationID,
		ExtensionID: extensionID,
		TableName:   table,
		TotalRows:   totalRows,
		State:       UserDataRestorePending,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	var state, startedAt, updatedAt, errorDetail, cursor, batchHash, namespaceHash string
	var importedRows, appliedCount int64
	err := s.db.QueryRowContext(ctx,
		`SELECT state, imported_rows, applied_count, cursor, batch_hash, namespace_hash, started_at, updated_at, error_detail
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=? AND table_name=?`, operationID, table,
	).Scan(&state, &importedRows, &appliedCount, &cursor, &batchHash, &namespaceHash, &startedAt, &updatedAt, &errorDetail)
	if err == nil {
		journal.State = UserDataRestoreState(state)
		journal.ImportedRows = importedRows
		journal.AppliedCount = appliedCount
		journal.Cursor = cursor
		journal.BatchHash = batchHash
		journal.NamespaceHash = namespaceHash
		journal.StartedAt = startedAt
		journal.UpdatedAt = updatedAt
		journal.ErrorDetail = errorDetail
		return journal, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query restore journal: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, namespace_hash, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		journal.JournalID, operationID, extensionID, table, totalRows, 0, 0, "", "", "",
		string(UserDataRestorePending), now, now, "")
	if err != nil {
		return nil, fmt.Errorf("create restore journal: %w", err)
	}
	return journal, nil
}

func (s *UserDataSnapshotStore) updateRestoreJournalState(ctx context.Context, journal *UserDataRestoreJournal, state UserDataRestoreState, errorDetail string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.State = state
	journal.ErrorDetail = errorDetail
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET state=?, error_detail=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		string(state), errorDetail, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) updateRestoreJournalProgress(ctx context.Context, journal *UserDataRestoreJournal, importedRows int64, cursor, batchHash string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.ImportedRows = importedRows
	journal.Cursor = cursor
	journal.BatchHash = batchHash
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET imported_rows=?, cursor=?, batch_hash=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		importedRows, cursor, batchHash, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) updateAppliedCount(ctx context.Context, journal *UserDataRestoreJournal, appliedCount int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.AppliedCount = appliedCount
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET applied_count=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		appliedCount, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) importRowsFromJSONL(ctx context.Context, table string, records []map[string]interface{}, startAppliedCount int64) (int, string, error) {
	if len(records) == 0 {
		return 0, "", nil
	}
	imported := 0
	var batchHasher = sha256.New()
	for i := 0; i < len(records); i += userDataRestoreBatchSize {
		end := i + userDataRestoreBatchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		batchImported, err := s.importBatch(ctx, table, batch)
		if err != nil {
			batchHash := hex.EncodeToString(batchHasher.Sum(nil))
			return imported + batchImported, batchHash, err
		}
		imported += batchImported
		batchHasher.Write([]byte(fmt.Sprintf("batch:%d:count:%d;", i/userDataRestoreBatchSize+1, batchImported)))
	}
	batchHash := hex.EncodeToString(batchHasher.Sum(nil))
	return imported, batchHash, nil
}

func (s *UserDataSnapshotStore) importBatch(ctx context.Context, table string, batch []map[string]interface{}) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}
	columns := extractColumnsFromRecords(batch[0])
	var filteredColumns []string
	for _, col := range columns {
		if strings.HasPrefix(col, "_") {
			continue
		}
		filteredColumns = append(filteredColumns, col)
	}
	columns = filteredColumns
	if len(columns) == 0 {
		return 0, fmt.Errorf("no columns detected for table %s", table)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import batch transaction: %w", err)
	}
	defer tx.Rollback()
	tableColumns, err := s.getTableColumns(ctx, table)
	if err != nil {
		return 0, fmt.Errorf("get columns for table %s: %w", table, err)
	}
	idColumn := detectIDColumnName(tableColumns)
	insertColumns := buildInsertColumns(columns, idColumn, tableColumns)
	placeholders := strings.Repeat("?,", len(insertColumns))
	placeholders = strings.TrimSuffix(placeholders, ",")
	quotedColumns := make([]string, len(insertColumns))
	for i, col := range insertColumns {
		quotedColumns[i] = quoteIdentifier(col)
	}
	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		quoteIdentifier(table), strings.Join(quotedColumns, ", "), placeholders)
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare import statement for %s: %w", table, err)
	}
	defer stmt.Close()
	imported := 0
	for _, record := range batch {
		args := make([]interface{}, len(insertColumns))
		for i, col := range insertColumns {
			if val, ok := record[col]; ok {
				args[i] = normalizeSQLValue(val)
			} else if col == idColumn {
				if entityID, ok := record["entity_id"]; ok {
					args[i] = normalizeSQLValue(entityID)
				} else {
					args[i] = nil
				}
			} else {
				args[i] = nil
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return imported, fmt.Errorf("import row into %s: %w", table, err)
		}
		imported++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import batch for %s: %w", table, err)
	}
	return imported, nil
}

func (s *UserDataSnapshotStore) getTableColumns(ctx context.Context, table string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdentifier(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func detectIDColumnName(columns []string) string {
	for _, col := range columns {
		if col == "id" || col == "entity_id" {
			return col
		}
	}
	return ""
}

func buildInsertColumns(recordColumns []string, idColumn string, tableColumns []string) []string {
	tableColSet := make(map[string]bool, len(tableColumns))
	for _, col := range tableColumns {
		tableColSet[col] = true
	}
	var result []string
	hasID := false
	for _, col := range recordColumns {
		if !tableColSet[col] {
			continue
		}
		result = append(result, col)
		if col == idColumn {
			hasID = true
		}
	}
	if !hasID && idColumn != "" && tableColSet[idColumn] {
		result = append(result, idColumn)
	}
	return result
}

func (s *UserDataSnapshotStore) deleteDifferingRows(ctx context.Context, table string, snapshotRecords []map[string]interface{}, extensionID string) error {
	tableColumns, err := s.getTableColumns(ctx, table)
	if err != nil {
		return fmt.Errorf("get columns for diff delete %s: %w", table, err)
	}
	idColumn := detectIDColumnName(tableColumns)
	if idColumn == "" {
		return s.fullTableReplace(ctx, table, snapshotRecords)
	}
	snapshotIDs := make(map[string]struct{}, len(snapshotRecords))
	for _, record := range snapshotRecords {
		var idVal interface{}
		if v, ok := record[idColumn]; ok {
			idVal = v
		} else if idColumn != "entity_id" {
			idVal = record["entity_id"]
		}
		id := fmt.Sprint(idVal)
		if id != "" && id != "<nil>" {
			snapshotIDs[id] = struct{}{}
		}
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", quoteIdentifier(idColumn), quoteIdentifier(table)))
	if err != nil {
		return fmt.Errorf("query current ids for %s: %w", table, err)
	}
	defer rows.Close()
	var toDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan id for %s: %w", table, err)
		}
		if _, exists := snapshotIDs[id]; !exists {
			toDelete = append(toDelete, id)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ids for %s: %w", table, err)
	}
	if len(toDelete) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete transaction for %s: %w", table, err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s=?", quoteIdentifier(table), quoteIdentifier(idColumn)))
	if err != nil {
		return fmt.Errorf("prepare delete statement for %s: %w", table, err)
	}
	defer stmt.Close()
	for _, id := range toDelete {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("delete differing row from %s: %w", table, err)
		}
	}
	return tx.Commit()
}

func (s *UserDataSnapshotStore) fullTableReplace(ctx context.Context, table string, snapshotRecords []map[string]interface{}) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin full replace transaction for %s: %w", table, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", quoteIdentifier(table))); err != nil {
		return fmt.Errorf("clear table %s: %w", table, err)
	}
	if len(snapshotRecords) == 0 {
		return tx.Commit()
	}
	columns := extractColumnsFromRecords(snapshotRecords[0])
	var filteredColumns []string
	for _, col := range columns {
		if strings.HasPrefix(col, "_") {
			continue
		}
		filteredColumns = append(filteredColumns, col)
	}
	columns = filteredColumns
	tableColumns, err := s.getTableColumns(ctx, table)
	if err != nil {
		return fmt.Errorf("get columns for table %s: %w", table, err)
	}
	idColumn := detectIDColumnName(tableColumns)
	insertColumns := buildInsertColumns(columns, idColumn, tableColumns)
	placeholders := strings.Repeat("?,", len(insertColumns))
	placeholders = strings.TrimSuffix(placeholders, ",")
	quotedColumns := make([]string, len(insertColumns))
	for i, col := range insertColumns {
		quotedColumns[i] = quoteIdentifier(col)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdentifier(table), strings.Join(quotedColumns, ", "), placeholders)
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare insert for full replace %s: %w", table, err)
	}
	defer stmt.Close()
	for _, record := range snapshotRecords {
		args := make([]interface{}, len(insertColumns))
		for i, col := range insertColumns {
			if val, ok := record[col]; ok {
				args[i] = normalizeSQLValue(val)
			} else if col == idColumn {
				if entityID, ok := record["entity_id"]; ok {
					args[i] = normalizeSQLValue(entityID)
				} else {
					args[i] = nil
				}
			} else {
				args[i] = nil
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("insert row during full replace %s: %w", table, err)
		}
	}
	return tx.Commit()
}

func (s *UserDataSnapshotStore) VerifyUserDataRestore(ctx context.Context, operationID string) error {
	if s.db == nil {
		return fmt.Errorf("kernel: user data snapshot store database unavailable")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT table_name, state, imported_rows, total_rows, applied_count, cursor, error_detail
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=?`, operationID)
	if err != nil {
		return fmt.Errorf("query restore journals: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, state, errorDetail, cursor string
		var importedRows, totalRows, appliedCount int64
		if err := rows.Scan(&table, &state, &importedRows, &totalRows, &appliedCount, &cursor, &errorDetail); err != nil {
			return fmt.Errorf("scan restore journal: %w", err)
		}
		if state != string(UserDataRestoreCompleted) {
			return fmt.Errorf("kernel: user data restore incomplete for table %s (state=%s)", table, state)
		}
		if errorDetail != "" {
			return fmt.Errorf("kernel: user data restore error for table %s: %s", table, errorDetail)
		}
		if importedRows != totalRows {
			return fmt.Errorf("kernel: user data restore record count mismatch for table %s: imported %d total %d", table, importedRows, totalRows)
		}
		if cursor != strconv.FormatInt(totalRows, 10) {
			return fmt.Errorf("kernel: user data restore cursor not at EOF for table %s: cursor=%s expected=%d", table, cursor, totalRows)
		}
	}
	return rows.Err()
}

func (s *UserDataSnapshotStore) ClearRestoreJournals(ctx context.Context, operationID string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM extension_package_user_data_restore_journal WHERE operation_id=?`, operationID)
	return err
}

func computeUserDataPayloadHash(payload interface{}) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func detectIDColumn(records []map[string]interface{}) string {
	if len(records) == 0 {
		return ""
	}
	first := records[0]
	for _, col := range []string{"id", "ID", "Id", "entity_id", "entityID", "resource_id", "resourceID", "row_id", "rowID", "uid", "UUID", "uuid"} {
		if _, exists := first[col]; exists {
			return col
		}
	}
	return ""
}

func normalizeSQLValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func parseAndValidateJSONL(jsonlData, extensionID string) ([]map[string]interface{}, error) {
	lines := strings.Split(jsonlData, "\n")
	var records []map[string]interface{}
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record userDataRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data record parse error at line %d: %w", lineNum+1, err))
		}
		if err := validateUserDataRecord(record, extensionID); err != nil {
			return nil, err
		}
		recordMap := map[string]interface{}{}
		if record.EntityID != "" {
			recordMap["entity_id"] = record.EntityID
		}
		for k, v := range record.Payload {
			recordMap[k] = v
		}
		recordMap["_line"] = lineNum + 1
		recordMap["_raw"] = line
		records = append(records, recordMap)
	}
	if len(records) == 0 {
		return nil, NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data snapshot has no valid records"))
	}
	return records, nil
}
