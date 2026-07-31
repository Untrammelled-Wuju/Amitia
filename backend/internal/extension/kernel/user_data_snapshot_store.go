package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const userDataRestoreBatchSize = 100

const userDataRestoreJournalDDL = `CREATE TABLE IF NOT EXISTS extension_package_user_data_restore_journal (
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
)`

type UserDataRestoreState string

const (
	UserDataRestorePending   UserDataRestoreState = "pending"
	UserDataRestoreImporting UserDataRestoreState = "importing"
	UserDataRestoreCompleted UserDataRestoreState = "completed"
	UserDataRestoreFailed    UserDataRestoreState = "failed"
)

type UserDataRestoreJournal struct {
	JournalID    string
	OperationID  string
	ExtensionID  string
	TableName    string
	TotalRows    int64
	ImportedRows int64
	State        UserDataRestoreState
	StartedAt    string
	UpdatedAt    string
	ErrorDetail  string
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
	_, err := s.db.ExecContext(ctx, userDataRestoreJournalDDL)
	return err
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
		if !exists || strings.TrimSpace(jsonlData) == "" {
			continue
		}
		if err := s.restoreTable(ctx, extensionID, operationID, table, jsonlData); err != nil {
			return fmt.Errorf("kernel: restore user data table %s: %w", table, err)
		}
	}
	return nil
}

func (s *UserDataSnapshotStore) restoreTable(ctx context.Context, extensionID, operationID, table, jsonlData string) error {
	records, err := parseJSONL(jsonlData)
	if err != nil {
		return fmt.Errorf("parse jsonl: %w", err)
	}
	journal, err := s.getOrCreateRestoreJournal(ctx, operationID, extensionID, table, int64(len(records)))
	if err != nil {
		return err
	}
	if journal.State == UserDataRestoreCompleted {
		return nil
	}
	if journal.State == UserDataRestoreFailed {
		journal.ImportedRows = 0
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		return err
	}
	cursor := int(journal.ImportedRows)
	if cursor > 0 && cursor < len(records) {
		records = records[cursor:]
	}
	imported, err := s.importRowsFromJSONL(ctx, table, records)
	if err != nil {
		_ = s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error())
		return err
	}
	totalImported := int64(cursor) + int64(imported)
	if err := s.deleteDifferingRows(ctx, table, records); err != nil {
		_ = s.updateRestoreJournalProgress(ctx, journal, totalImported)
		_ = s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error())
		return err
	}
	if err := s.updateRestoreJournalProgress(ctx, journal, totalImported); err != nil {
		return err
	}
	return s.updateRestoreJournalState(ctx, journal, UserDataRestoreCompleted, "")
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
	var state, startedAt, updatedAt, errorDetail string
	var importedRows int64
	err := s.db.QueryRowContext(ctx,
		`SELECT state, imported_rows, started_at, updated_at, error_detail
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=? AND table_name=?`, operationID, table,
	).Scan(&state, &importedRows, &startedAt, &updatedAt, &errorDetail)
	if err == nil {
		journal.State = UserDataRestoreState(state)
		journal.ImportedRows = importedRows
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
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		journal.JournalID, operationID, extensionID, table, totalRows, 0,
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

func (s *UserDataSnapshotStore) updateRestoreJournalProgress(ctx context.Context, journal *UserDataRestoreJournal, importedRows int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.ImportedRows = importedRows
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET imported_rows=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		importedRows, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) importRowsFromJSONL(ctx context.Context, table string, records []map[string]interface{}) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	imported := 0
	for i := 0; i < len(records); i += userDataRestoreBatchSize {
		end := i + userDataRestoreBatchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		batchImported, err := s.importBatch(ctx, table, batch)
		if err != nil {
			return imported + batchImported, err
		}
		imported += batchImported
	}
	return imported, nil
}

func (s *UserDataSnapshotStore) importBatch(ctx context.Context, table string, batch []map[string]interface{}) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}
	columns := extractColumnsFromRecords(batch[0])
	if len(columns) == 0 {
		return 0, fmt.Errorf("no columns detected for table %s", table)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import batch transaction: %w", err)
	}
	defer tx.Rollback()
	placeholders := strings.Repeat("?,", len(columns))
	placeholders = strings.TrimSuffix(placeholders, ",")
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
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
		args := make([]interface{}, len(columns))
		for i, col := range columns {
			args[i] = normalizeSQLValue(record[col])
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

func (s *UserDataSnapshotStore) deleteDifferingRows(ctx context.Context, table string, snapshotRecords []map[string]interface{}) error {
	idColumn := detectIDColumn(snapshotRecords)
	if idColumn == "" {
		return s.fullTableReplace(ctx, table, snapshotRecords)
	}
	snapshotIDs := make(map[string]struct{}, len(snapshotRecords))
	for _, record := range snapshotRecords {
		id := fmt.Sprint(record[idColumn])
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
	placeholders := strings.Repeat("?,", len(columns))
	placeholders = strings.TrimSuffix(placeholders, ",")
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
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
		args := make([]interface{}, len(columns))
		for i, col := range columns {
			args[i] = normalizeSQLValue(record[col])
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
		`SELECT table_name, state, imported_rows, total_rows, error_detail
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=?`, operationID)
	if err != nil {
		return fmt.Errorf("query restore journals: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, state, errorDetail string
		var importedRows, totalRows int64
		if err := rows.Scan(&table, &state, &importedRows, &totalRows, &errorDetail); err != nil {
			return fmt.Errorf("scan restore journal: %w", err)
		}
		if state != string(UserDataRestoreCompleted) {
			return fmt.Errorf("kernel: user data restore incomplete for table %s (state=%s)", table, state)
		}
		if errorDetail != "" {
			return fmt.Errorf("kernel: user data restore error for table %s: %s", table, errorDetail)
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
