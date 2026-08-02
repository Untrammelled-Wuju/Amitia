package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

const userDataRestoreBatchSize = 100

const userDataBatchHashAlgorithmVersion = "content-chain-v1"

func userBatchGenesisHash() string {
	h := sha256.Sum256([]byte("amitia-userdata-restore-batch-genesis-v1"))
	return hex.EncodeToString(h[:])
}

func userBatchEmptySetHash() string {
	h := sha256.Sum256([]byte("amitia-userdata-restore-emptyset-v1"))
	return hex.EncodeToString(h[:])
}

type UserDataBatchIdentity struct {
	ExtensionID    string
	TableName      string
	Namespace      string
	EntityType     string
	SchemaVersion  string
	RecordCount    int64
	BatchAlgorithm string
}

func computeUserDataBatchHashFromIdentity(identity UserDataBatchIdentity, operationID string) string {
	if identity.RecordCount == 0 || identity.ExtensionID == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("v:%s:op:%s:ext:%s:tbl:%s:schema:%s:records:%d:entity:%s:ns:%s",
		userDataBatchHashAlgorithmVersion, operationID, identity.ExtensionID,
		identity.TableName, identity.SchemaVersion, identity.RecordCount,
		identity.EntityType, identity.Namespace)))
	return hex.EncodeToString(h.Sum(nil))
}

const (
	PackageErrCodeUserDataNamespaceViolation  = "PACKAGE_USER_DATA_NAMESPACE_VIOLATION"
	PackageErrCodeUserDataJournalHashMismatch = "PACKAGE_USER_DATA_JOURNAL_HASH_MISMATCH"
	PackageErrCodeUserDataJournalLegacyEmpty  = "PACKAGE_USER_DATA_JOURNAL_LEGACY_EMPTY"
)

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
	batch_index INTEGER NOT NULL DEFAULT 0,
	prev_batch_hash TEXT NOT NULL DEFAULT '',
	batch_algorithm_version TEXT NOT NULL DEFAULT '',
	batch_size INTEGER NOT NULL DEFAULT 0,
	namespace_hash TEXT NOT NULL DEFAULT '',
	aggregate_hash TEXT NOT NULL DEFAULT '',
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
	JournalID            string
	OperationID          string
	ExtensionID          string
	Namespace            string
	TableName            string
	TotalRows            int64
	ImportedRows         int64
	AppliedCount         int64
	Cursor               string
	BatchHash            string
	BatchIndex           int64
	PrevBatchHash        string
	BatchAlgorithmVersion string
	BatchSize            int64
	NamespaceHash        string
	AggregateHash        string
	State                UserDataRestoreState
	StartedAt            string
	UpdatedAt            string
	ErrorDetail          string
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
		s.ensureBatchIndexColumn,
		s.ensurePrevBatchHashColumn,
		s.ensureBatchAlgorithmVersionColumn,
		s.ensureBatchSizeColumn,
		s.ensureNamespaceHashColumn,
		s.ensureAggregateHashColumn,
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

func (s *UserDataSnapshotStore) ensureAggregateHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='aggregate_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN aggregate_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureBatchIndexColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='batch_index'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN batch_index INTEGER NOT NULL DEFAULT 0`)
	return err
}

func (s *UserDataSnapshotStore) ensurePrevBatchHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='prev_batch_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN prev_batch_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureBatchAlgorithmVersionColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='batch_algorithm_version'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN batch_algorithm_version TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureBatchSizeColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='batch_size'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN batch_size INTEGER NOT NULL DEFAULT 0`)
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
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record missing required fields: %s", strings.Join(missing, ", ")))
	}
	if record.ExtensionID != extensionID {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record extensionID mismatch: got %s expected %s", record.ExtensionID, extensionID))
	}
	resolver, resolverErr := ResolveExtensionUserDataNamespace(extensionID, record.Namespace)
	if resolverErr != nil {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: user data record namespace validation failed: %w", resolverErr))
	}
	if record.Namespace != resolver.CanonicalTable {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: user data record namespace %q does not match canonical table %q", record.Namespace, resolver.CanonicalTable))
	}
	if record.EntityType == "" {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record entityType is empty"))
	}
	if !isValidEntityType(record.EntityType) {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record entityType %q is not a valid entity type", record.EntityType))
	}
	if record.EntityType != resolver.LogicalEntityType {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record entityType %q does not match namespace logical entity type %q", record.EntityType, resolver.LogicalEntityType))
	}
	if !isValidOperation(record.Operation) {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record operation %q not in allowed set", record.Operation))
	}
	expectedHash := computeUserDataPayloadHash(record.Payload)
	if expectedHash != record.PayloadHash {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
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
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422, fmt.Errorf("kernel: user data snapshot corrupt: %w", err))
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
		rawRecords, parsedRecords, parseErr := parseAndValidateJSONL(jsonlData, extensionID)
		if parseErr != nil {
			return parseErr
		}
		actualCount := int64(len(parsedRecords))
		expectedCount, hasCount := userState.RecordCounts[table]
		if hasCount && expectedCount != actualCount {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data snapshot record count mismatch for table %s: expected %d, got %d", table, expectedCount, actualCount))
		}
		if err := s.restoreTable(ctx, extensionID, operationID, table, parsedRecords, rawRecords); err != nil {
			return fmt.Errorf("kernel: restore user data table %s: %w", table, err)
		}
	}
	return nil
}

func (s *UserDataSnapshotStore) restoreTable(ctx context.Context, extensionID, operationID, table string, parsedRecords []userDataRecord, rawRecords []map[string]interface{}) error {
	if !migration.IsExtensionNamespaceTable(table, extensionID) {
		return fmt.Errorf("kernel: user data table %q does not belong to extension namespace", table)
	}
	records := rawRecords
	entityTypeSet := make(map[string]struct{})
	var schemaVersion, namespace string
	for i, pr := range parsedRecords {
		entityTypeSet[pr.EntityType] = struct{}{}
		if i == 0 {
			schemaVersion = pr.SchemaVersion
			namespace = pr.Namespace
			continue
		}
		if pr.Namespace != namespace {
			return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: all records must share the same namespace, found %s vs %s", pr.Namespace, namespace))
		}
		if pr.SchemaVersion != schemaVersion {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: all records must share the same schema version, found %s vs %s", pr.SchemaVersion, schemaVersion))
		}
	}
	entityTypes := make([]string, 0, len(entityTypeSet))
	for et := range entityTypeSet {
		entityTypes = append(entityTypes, et)
	}
	if namespace == "" {
		resolver, resolverErr := ResolveExtensionUserDataNamespace(extensionID, table)
		if resolverErr != nil {
			return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: resolve namespace for table %s: %w", table, resolverErr))
		}
		namespace = resolver.CanonicalTable
		schemaVersion = "1.0.0"
	}
	expectedNamespaceHash := computeNamespaceHash(extensionID, table, namespace, entityTypes, schemaVersion, "v1")
	journal, err := s.getOrCreateRestoreJournal(ctx, operationID, extensionID, table, namespace, int64(len(records)), expectedNamespaceHash)
	if err != nil {
		return err
	}
	if journal.State == UserDataRestoreCompleted {
		return s.verifyJournalConsistency(journal, int64(len(records)))
	}
	if journal.State == UserDataRestoreFailed {
		journal.ImportedRows = 0
		journal.AppliedCount = 0
		journal.Cursor = ""
		journal.BatchHash = ""
	}
	startCursor, cursorErr := parseJournalCursor(journal, int64(len(records)))
	if cursorErr != nil {
		return cursorErr
	}
	if journal.AppliedCount != startCursor {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: journal applied_count %d does not match cursor %d", journal.AppliedCount, startCursor))
	}
	if len(parsedRecords) == 0 {
		if err := s.fullTableReplace(ctx, table, records); err != nil {
			failErr := fmt.Errorf("kernel: empty restore failed to clear table %s: %w", table, err)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		aggHashFromDB, aggErr := s.computeAggregateHashFromDB(ctx, table)
		if aggErr != nil {
			failErr := fmt.Errorf("kernel: compute aggregate hash for empty table %s: %w", table, aggErr)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if err := s.updateRestoreJournalHashes(ctx, journal, expectedNamespaceHash, aggHashFromDB); err != nil {
			return fmt.Errorf("kernel: update journal hashes for empty table: %w", err)
		}
		if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreCompleted, ""); err != nil {
			return fmt.Errorf("kernel: update journal to completed for empty table: %w", err)
		}
		return nil
	}
	if startCursor == int64(len(records)) {
		if startCursor > 0 {
			if err := s.deleteDifferingRows(ctx, table, records, extensionID); err != nil {
				failErr := fmt.Errorf("kernel: delete differing rows for %s: %w", table, err)
				if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
					return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
				}
				return failErr
			}
			recomputedAggHash, recomputeErr := s.computeAggregateHashFromDB(ctx, table)
			if recomputeErr != nil {
				failErr := fmt.Errorf("kernel: recompute aggregate hash on recovery for table %s: %w", table, recomputeErr)
				if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
					return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
				}
				return failErr
			}
			if journal.AggregateHash != recomputedAggHash {
				failErr := fmt.Errorf("kernel: aggregate hash mismatch on recovery for table %s", table)
				if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
					return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
				}
				return failErr
			}
			if err := s.updateRestoreJournalHashes(ctx, journal, expectedNamespaceHash, recomputedAggHash); err != nil {
				return fmt.Errorf("kernel: update journal hashes on recovery: %w", err)
			}
		}
		if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreCompleted, ""); err != nil {
			return fmt.Errorf("kernel: update journal to completed: %w", err)
		}
		return nil
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		return fmt.Errorf("kernel: update journal to importing: %w", err)
	}
	appliedCount := journal.AppliedCount
	prevBatchHash := journal.BatchHash
	if prevBatchHash == "" {
		prevBatchHash = userBatchGenesisHash()
	}
	batchIdx := journal.BatchIndex
	remaining := records[startCursor:]
	for i := 0; i < len(remaining); i += userDataRestoreBatchSize {
		end := i + userDataRestoreBatchSize
		if end > len(remaining) {
			end = len(remaining)
		}
		batch := remaining[i:end]
		batchIdx++
		batchHash := computeContentBoundBatchHash(batch, extensionID, startCursor+int64(i), prevBatchHash, int(batchIdx), schemaVersion, table, operationID)
		if batchHash == "" {
			if failErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed,
				fmt.Sprintf("batch content hash computation failed at cursor %d", startCursor+int64(i))); failErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: batch content hash failed"))
			}
			return fmt.Errorf("kernel: batch content hash computation failed at cursor %d", startCursor+int64(i))
		}
		batchApplied, _, err := s.importBatchAtomic(ctx, journal, table, batch, startCursor+int64(i), appliedCount, int(batchIdx), batchHash, prevBatchHash, journal.PrevBatchHash)
		if err != nil {
			if failErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error()); failErr != nil {
				return errors.Join(err, fmt.Errorf("kernel: update journal to failed: %w", failErr))
			}
			return err
		}
		if batchApplied != int64(len(batch)) {
			err := fmt.Errorf("kernel: batch import incomplete: expected %d, got %d", len(batch), batchApplied)
			if failErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error()); failErr != nil {
				return errors.Join(err, fmt.Errorf("kernel: update journal to failed: %w", failErr))
			}
			return err
		}
		prevBatchHash = batchHash
		appliedCount += batchApplied
	}
	if err := s.deleteDifferingRows(ctx, table, records, extensionID); err != nil {
		if failErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, err.Error()); failErr != nil {
			return errors.Join(err, fmt.Errorf("kernel: update journal to failed after diff delete: %w", failErr))
		}
		return fmt.Errorf("kernel: delete differing rows for %s: %w", table, err)
	}
	aggHashFromDB, aggErr := s.computeAggregateHashFromDB(ctx, table)
	if aggErr != nil {
		if failErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, aggErr.Error()); failErr != nil {
			return errors.Join(aggErr, fmt.Errorf("kernel: update journal to failed after aggregate hash failure: %w", failErr))
		}
		return fmt.Errorf("kernel: compute aggregate hash from DB for table %s: %w", table, aggErr)
	}
	if err := s.updateRestoreJournalHashes(ctx, journal, expectedNamespaceHash, aggHashFromDB); err != nil {
		return fmt.Errorf("kernel: update journal hashes: %w", err)
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreCompleted, ""); err != nil {
		return fmt.Errorf("kernel: update journal to completed: %w", err)
	}
	return nil
}

func parseJournalCursor(journal *UserDataRestoreJournal, totalRecords int64) (int64, error) {
	if journal.Cursor == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(journal.Cursor, 10, 64)
	if err != nil || cursor < 0 || cursor > totalRecords {
		return 0, NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: journal cursor %q out of valid range [0, %d]", journal.Cursor, totalRecords))
	}
	return cursor, nil
}

func (s *UserDataSnapshotStore) verifyJournalConsistency(journal *UserDataRestoreJournal, totalRecords int64) error {
	if journal.State == UserDataRestoreCompleted && journal.ImportedRows != totalRecords {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed journal imported rows %d does not match snapshot %d", journal.ImportedRows, totalRecords))
	}
	if journal.State == UserDataRestoreCompleted && journal.AppliedCount != totalRecords {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed journal applied count %d does not match snapshot %d", journal.AppliedCount, totalRecords))
	}
	if journal.State == UserDataRestoreCompleted && journal.Cursor != strconv.FormatInt(totalRecords, 10) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed journal cursor %q does not match expected %d", journal.Cursor, totalRecords))
	}
	return nil
}

func (s *UserDataSnapshotStore) getOrCreateRestoreJournal(ctx context.Context, operationID, extensionID, table, namespace string, totalRows int64, expectedNamespaceHash string) (*UserDataRestoreJournal, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal := &UserDataRestoreJournal{
		JournalID:   "ud-journal-" + operationID + "-" + table,
		OperationID: operationID,
		ExtensionID: extensionID,
		Namespace:   namespace,
		TableName:   table,
		TotalRows:   totalRows,
		State:       UserDataRestorePending,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	var state, startedAt, updatedAt, errorDetail, cursor, batchHash, prevBatchHash, batchAlgorithmVer, namespaceHash, aggregateHash string
	var importedRows, appliedCount, batchIndex, batchSize int64
	err := s.db.QueryRowContext(ctx,
		`SELECT state, imported_rows, applied_count, cursor, batch_hash, batch_index, prev_batch_hash, batch_algorithm_version, batch_size, namespace_hash, aggregate_hash, started_at, updated_at, error_detail
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=? AND table_name=?`, operationID, table,
	).Scan(&state, &importedRows, &appliedCount, &cursor, &batchHash, &batchIndex, &prevBatchHash, &batchAlgorithmVer, &batchSize, &namespaceHash, &aggregateHash, &startedAt, &updatedAt, &errorDetail)
	if err == nil {
		journal.State = UserDataRestoreState(state)
		journal.ImportedRows = importedRows
		journal.AppliedCount = appliedCount
		journal.Cursor = cursor
		journal.BatchHash = batchHash
		journal.BatchIndex = batchIndex
		journal.PrevBatchHash = prevBatchHash
		journal.BatchAlgorithmVersion = batchAlgorithmVer
		journal.BatchSize = batchSize
		journal.NamespaceHash = namespaceHash
		journal.AggregateHash = aggregateHash
		journal.StartedAt = startedAt
		journal.UpdatedAt = updatedAt
		journal.ErrorDetail = errorDetail
		if journal.ExtensionID != extensionID {
			return nil, NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: restore journal %s belongs to extension %s, cannot be used for extension %s", journal.JournalID, journal.ExtensionID, extensionID))
		}
		if journal.NamespaceHash == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s has empty namespace hash from legacy version, cannot continue restore", journal.JournalID))
		}
		if journal.NamespaceHash != expectedNamespaceHash {
			return nil, NewPackageError(PackageErrCodeUserDataJournalHashMismatch, 422,
				fmt.Errorf("kernel: restore journal %s namespace hash mismatch: expected %s, actual %s", journal.JournalID, expectedNamespaceHash, journal.NamespaceHash))
		}
		return journal, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query restore journal: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO extension_package_user_data_restore_journal
		 (journal_id, operation_id, extension_id, table_name, total_rows, imported_rows, applied_count, cursor, batch_hash, batch_index, prev_batch_hash, batch_algorithm_version, batch_size, namespace_hash, aggregate_hash, state, started_at, updated_at, error_detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		journal.JournalID, operationID, extensionID, table, totalRows, 0, 0, "", "", 0, "", userDataBatchHashAlgorithmVersion, userDataRestoreBatchSize, expectedNamespaceHash, "",
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

func (s *UserDataSnapshotStore) updateJournalAggregateHash(ctx context.Context, journal *UserDataRestoreJournal, aggregateHash string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.AggregateHash = aggregateHash
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET aggregate_hash=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		aggregateHash, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) updateRestoreJournalProgress(ctx context.Context, journal *UserDataRestoreJournal, appliedCount int64, cursor string, batchHash string, batchIdx int, prevBatchHash string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.AppliedCount = appliedCount
	journal.Cursor = cursor
	journal.BatchHash = batchHash
	journal.BatchIndex = int64(batchIdx)
	journal.PrevBatchHash = prevBatchHash
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET applied_count=?, cursor=?, batch_hash=?, batch_index=?, prev_batch_hash=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		appliedCount, cursor, batchHash, int64(batchIdx), prevBatchHash, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) importBatchAtomic(ctx context.Context, journal *UserDataRestoreJournal, table string, batch []map[string]interface{}, cursorBefore int64, appliedCountBefore int64, batchIdx int, batchHash string, prevBatchHash string, expectedPrevBatchHash string) (int64, string, error) {
	if len(batch) == 0 {
		return 0, "", nil
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
		return 0, "", NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422, fmt.Errorf("no columns detected for table %s", table))
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("begin atomic batch transaction: %w", err)
	}
	defer tx.Rollback()
	tableColumns, err := s.getTableColumnsTx(ctx, tx, table)
	if err != nil {
		return 0, "", fmt.Errorf("get columns for table %s: %w", table, err)
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
		return 0, "", fmt.Errorf("prepare import statement for %s: %w", table, err)
	}
	defer stmt.Close()
	imported := int64(0)
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
			return imported, "", fmt.Errorf("import row into %s: %w", table, err)
		}
		imported++
	}
	newCursor := cursorBefore + imported
	newAppliedCount := appliedCountBefore + imported
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET imported_rows=?, cursor=?, applied_count=?, batch_hash=?, batch_index=?, prev_batch_hash=?, updated_at=?
		 WHERE operation_id=? AND table_name=? AND state=? AND prev_batch_hash=?`,
		newCursor, strconv.FormatInt(newCursor, 10), newAppliedCount, batchHash, int64(batchIdx), prevBatchHash, now,
		journal.OperationID, journal.TableName, string(UserDataRestoreImporting), expectedPrevBatchHash)
	if err != nil {
		return 0, "", fmt.Errorf("update journal in batch transaction: %w", err)
	}
	rowsAff, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return 0, "", fmt.Errorf("query rows affected for journal update: %w", rowsErr)
	}
	if rowsAff != 1 {
		return 0, "", NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409, fmt.Errorf("kernel: journal update affected %d rows, expected exactly 1 (op=%s table=%s batch=%d)", rowsAff, journal.OperationID, journal.TableName, batchIdx))
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("commit atomic batch for %s: %w", table, err)
	}
	journal.ImportedRows = newCursor
	journal.Cursor = strconv.FormatInt(newCursor, 10)
	journal.AppliedCount = newAppliedCount
	journal.BatchHash = batchHash
	journal.BatchIndex = int64(batchIdx)
	journal.PrevBatchHash = prevBatchHash
	journal.UpdatedAt = now
	return imported, batchHash, nil
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

func (s *UserDataSnapshotStore) getTableColumnsTx(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdentifier(table)))
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

func (s *UserDataSnapshotStore) updateRestoreJournalHashes(ctx context.Context, journal *UserDataRestoreJournal, nsHash, aggHash string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	journal.NamespaceHash = nsHash
	journal.AggregateHash = aggHash
	journal.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
		 SET namespace_hash=?, aggregate_hash=?, updated_at=?
		 WHERE operation_id=? AND table_name=?`,
		nsHash, aggHash, now, journal.OperationID, journal.TableName)
	return err
}

func (s *UserDataSnapshotStore) computeAggregateHashFromDB(ctx context.Context, table string) (string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", quoteIdentifier(table)))
	if err != nil {
		return "", fmt.Errorf("kernel: query table for aggregate hash %s: %w", table, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("kernel: get columns for aggregate hash %s: %w", table, err)
	}
	hashes := make([]string, 0, 64)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("kernel: scan row for aggregate hash %s: %w", table, err)
		}
		payload := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			payload[col] = normalizeSQLValue(values[i])
		}
		hashes = append(hashes, computeUserDataPayloadHash(payload))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("kernel: iterate rows for aggregate hash %s: %w", table, err)
	}
	sort.Strings(hashes)
	hasher := sha256.New()
	for _, h := range hashes {
		hasher.Write([]byte(h))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *UserDataSnapshotStore) VerifyUserDataRestore(ctx context.Context, operationID, snapshotJSON string) error {
	if s.db == nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data snapshot store database unavailable"))
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT table_name, state, imported_rows, total_rows, applied_count, cursor, error_detail,
		        namespace_hash, aggregate_hash, batch_hash, extension_id, batch_index, batch_algorithm_version, batch_size
		 FROM extension_package_user_data_restore_journal
		 WHERE operation_id=?`, operationID)
	if err != nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: query restore journals for operation %s: %w", operationID, err))
	}
	defer rows.Close()
	found := false
	var snapshotState packageUserDataMigrationState
	if snapshotJSON != "" {
		if err := json.Unmarshal([]byte(snapshotJSON), &snapshotState); err != nil {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422, fmt.Errorf("kernel: user data snapshot corrupt: %w", err))
		}
	}
	for rows.Next() {
		found = true
		var table, state, errorDetail, cursor, namespaceHash, aggregateHash, batchHash, extensionID, batchAlgoVer string
		var importedRows, totalRows, appliedCount, batchIndex, batchSize int64
		if err := rows.Scan(&table, &state, &importedRows, &totalRows, &appliedCount, &cursor, &errorDetail,
			&namespaceHash, &aggregateHash, &batchHash, &extensionID, &batchIndex, &batchAlgoVer, &batchSize); err != nil {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: scan restore journal for operation %s: %w", operationID, err))
		}
		if state != string(UserDataRestoreCompleted) {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data restore incomplete for table %s (state=%s)", table, state))
		}
		if errorDetail != "" {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data restore error for table %s: %s", table, errorDetail))
		}
		if importedRows != totalRows {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data restore imported rows mismatch for table %s: imported %d total %d", table, importedRows, totalRows))
		}
		if appliedCount != totalRows {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data restore applied count mismatch for table %s: applied %d total %d", table, appliedCount, totalRows))
		}
		expectedCursor := strconv.FormatInt(totalRows, 10)
		if cursor != expectedCursor {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data restore cursor not at EOF for table %s: cursor=%s expected=%s", table, cursor, expectedCursor))
		}
		if namespaceHash == "" {
			return NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty namespace hash", table))
		}
		if totalRows > 0 && batchHash == "" {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty batch hash but %d rows were imported", table, totalRows))
		}
		if batchAlgoVer != userDataBatchHashAlgorithmVersion {
			return NewPackageError(PackageErrCodeSnapshotSchemaUnsupported, 422,
				fmt.Errorf("kernel: restore journal for table %s uses unsupported batch algorithm version: got %s expected %s", table, batchAlgoVer, userDataBatchHashAlgorithmVersion))
		}
		if aggregateHash == "" {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty aggregate hash", table))
		}
		storedHashForCompare := strings.TrimPrefix(aggregateHash, "sha256:")
		if _, decErr := hex.DecodeString(storedHashForCompare); decErr != nil || len(storedHashForCompare) != 64 {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: restore journal for table %s aggregate hash has invalid format", table))
		}
		actualAggHash, aggErr := s.computeAggregateHashFromDB(ctx, table)
		if aggErr != nil {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: recompute aggregate hash for table %s: %w", table, aggErr))
		}
		if actualAggHash != aggregateHash {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: aggregate hash mismatch for table %s: stored=%s actual=%s", table, aggregateHash, actualAggHash))
		}
		var dbCount int64
		countErr := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))).Scan(&dbCount)
		if countErr != nil {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: count records in table %s: %w", table, countErr))
		}
		if dbCount != totalRows {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: actual DB record count mismatch for table %s: expected %d actual %d", table, totalRows, dbCount))
		}
		if totalRows > 0 {
			jsonlData, exists := snapshotState.DataExports[table]
			if !exists {
				return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
					fmt.Errorf("kernel: snapshot missing data export for table %s, cannot recalculate batch chain", table))
			}
			records, parsedRecords, parseErr := parseAndValidateJSONL(jsonlData, extensionID)
			if parseErr != nil {
				return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
					fmt.Errorf("kernel: cannot re-parse snapshot JSONL for table %s: %w", table, parseErr))
			}
			if int64(len(parsedRecords)) != totalRows {
				return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
					fmt.Errorf("kernel: re-parsed record count mismatch for table %s: expected %d got %d", table, totalRows, len(parsedRecords)))
			}
			if len(parsedRecords) == 0 {
				return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
					fmt.Errorf("kernel: snapshot for table %s has zero records but totalRows=%d", table, totalRows))
			}
			effectiveBatchSize := batchSize
			if effectiveBatchSize <= 0 {
				effectiveBatchSize = userDataRestoreBatchSize
			}
			expectedFinalHash, expectedBatchCount, chainErr := recalculateBatchHashChain(records, extensionID, parsedRecords[0].SchemaVersion, parsedRecords[0].Namespace, operationID, effectiveBatchSize)
			if chainErr != nil {
				return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
					fmt.Errorf("kernel: batch hash chain recalculation failed for table %s: %w", table, chainErr))
			}
			if expectedFinalHash != batchHash {
				return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
					fmt.Errorf("kernel: batch hash chain mismatch for table %s: expected=%s actual=%s", table, expectedFinalHash, batchHash))
			}
			if batchIndex != expectedBatchCount {
				return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
					fmt.Errorf("kernel: batch index mismatch for table %s: expected %d batches, found %d", table, expectedBatchCount, batchIndex))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: iterate restore journals for operation %s: %w", operationID, err))
	}
	if !found {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: no restore journals found for operation %s", operationID))
	}
	return nil
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

func computeContentBoundBatchHash(batch []map[string]interface{}, extensionID string, cursor int64, prevBatchHash string, batchIdx int, schemaVersion, canonicalTable, operationID string) string {
	if len(batch) == 0 || extensionID == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("v:%s:op:%s:ext:%s:tbl:%s:schema:%s:batch:%d:cursor_before:%d",
		userDataBatchHashAlgorithmVersion, operationID, extensionID, canonicalTable, schemaVersion, batchIdx, cursor)))
	if prevBatchHash != "" {
		h.Write([]byte(":prev:" + prevBatchHash))
	}
	for idx, record := range batch {
		raw, exists := record["_raw"]
		if !exists {
			return ""
		}
		rawStr, ok := raw.(string)
		if !ok || strings.TrimSpace(rawStr) == "" {
			return ""
		}
		var parsed userDataRecord
		if err := json.Unmarshal([]byte(rawStr), &parsed); err != nil {
			return ""
		}
		if err := validateUserDataRecord(parsed, extensionID); err != nil {
			return ""
		}
		h.Write([]byte(fmt.Sprintf(":[%d:%s:%s:%s:%s:%s:%s:%s]", idx,
			parsed.ExtensionID, parsed.Namespace, parsed.SchemaVersion, parsed.EntityType, parsed.EntityID, parsed.Operation, parsed.PayloadHash)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func recalculateBatchHashChain(records []map[string]interface{}, extensionID, schemaVersion, canonicalTable, operationID string, batchSize int64) (string, int64, error) {
	if len(records) == 0 {
		return userBatchGenesisHash(), 0, nil
	}
	if extensionID == "" || schemaVersion == "" || canonicalTable == "" || operationID == "" {
		return "", 0, fmt.Errorf("kernel: missing required fields for batch hash chain recalculation")
	}
	prevHash := userBatchGenesisHash()
	batchCount := int64(0)
	totalRecords := int64(len(records))
	for i := int64(0); i < totalRecords; i += batchSize {
		end := i + batchSize
		if end > totalRecords {
			end = totalRecords
		}
		batch := records[i:end]
		batchCount++
		batchHash := computeContentBoundBatchHash(batch, extensionID, i, prevHash, int(batchCount), schemaVersion, canonicalTable, operationID)
		if batchHash == "" {
			return "", 0, fmt.Errorf("kernel: batch hash computation failed at batch %d, cursor %d", batchCount, i)
		}
		prevHash = batchHash
	}
	return prevHash, batchCount, nil
}

func computeContentBoundChainHash(records []map[string]interface{}, extensionID string, cursor int) string {
	if len(records) == 0 || extensionID == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte("chain:ext:" + extensionID + ":cursor:" + strconv.Itoa(cursor) + ":"))
	for idx, record := range records {
		raw, exists := record["_raw"]
		if !exists {
			return ""
		}
		rawStr, ok := raw.(string)
		if !ok || strings.TrimSpace(rawStr) == "" {
			return ""
		}
		var parsed userDataRecord
		if err := json.Unmarshal([]byte(rawStr), &parsed); err != nil {
			return ""
		}
		if err := validateUserDataRecord(parsed, extensionID); err != nil {
			return ""
		}
		h.Write([]byte(fmt.Sprintf("[%d:%s:%s:%s:%s:%s:%s]", idx,
			parsed.ExtensionID, parsed.Namespace, parsed.SchemaVersion, parsed.EntityType, parsed.EntityID, parsed.Operation)))
		h.Write([]byte(parsed.PayloadHash))
		h.Write([]byte(";"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type ExtensionUserDataNamespace struct {
	ExtensionID       string
	NamespacePrefix   string
	CanonicalTable    string
	LogicalEntityType string
}

func ResolveExtensionUserDataNamespace(extensionID, table string) (ExtensionUserDataNamespace, error) {
	normalizedExtID := migration.NormalizeExtensionID(extensionID)
	prefix := migration.ExtensionNamespacePrefix(extensionID)

	lowerTable := strings.ToLower(strings.TrimSpace(table))
	lowerPrefix := strings.ToLower(prefix)

	if !strings.HasPrefix(lowerTable, lowerPrefix) {
		return ExtensionUserDataNamespace{}, NewPackageError(
			PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: table %q does not belong to extension namespace prefix %q", table, prefix))
	}

	suffix := strings.TrimPrefix(lowerTable, lowerPrefix)
	if suffix == "" {
		return ExtensionUserDataNamespace{}, NewPackageError(
			PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: table %q has empty suffix after namespace prefix", table))
	}

	if !isValidTableSuffix(suffix) {
		return ExtensionUserDataNamespace{}, NewPackageError(
			PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: table suffix %q contains invalid characters", suffix))
	}

	return ExtensionUserDataNamespace{
		ExtensionID:       normalizedExtID,
		NamespacePrefix:   prefix,
		CanonicalTable:    table,
		LogicalEntityType: suffix,
	}, nil
}

func isValidTableSuffix(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func computeNamespaceHash(extensionID, table, namespace string, entityTypes []string, schemaVersion, isolationPolicyVersion string) string {
	hasher := sha256.New()
	normalizedExtID := strings.ToLower(strings.TrimSpace(extensionID))
	normalizedTable := strings.ToLower(strings.TrimSpace(table))
	normalizedNS := strings.ToLower(strings.TrimSpace(namespace))
	sortedEntityTypes := make([]string, len(entityTypes))
	copy(sortedEntityTypes, entityTypes)
	sort.Strings(sortedEntityTypes)
	normalizedEntityTypes := make([]string, len(sortedEntityTypes))
	for i, et := range sortedEntityTypes {
		normalizedEntityTypes[i] = strings.ToLower(strings.TrimSpace(et))
	}
	normalizedSchemaVer := strings.ToLower(strings.TrimSpace(schemaVersion))
	normalizedPolicyVer := strings.ToLower(strings.TrimSpace(isolationPolicyVersion))

	hasher.Write([]byte(normalizedPolicyVer + ":" + normalizedExtID + "|" + normalizedTable + "|" + normalizedNS + "|" + strings.Join(normalizedEntityTypes, ",") + "|" + normalizedSchemaVer))
	return "ns:" + hex.EncodeToString(hasher.Sum(nil))
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

func parseAndValidateJSONL(jsonlData, extensionID string) ([]map[string]interface{}, []userDataRecord, error) {
	lines := strings.Split(jsonlData, "\n")
	var records []map[string]interface{}
	var parsedRecords []userDataRecord
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record userDataRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, nil, NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: user data record parse error at line %d: %w", lineNum+1, err))
		}
		if err := validateUserDataRecord(record, extensionID); err != nil {
			return nil, nil, err
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
		parsedRecords = append(parsedRecords, record)
	}
	return records, parsedRecords, nil
}
