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

const (
	userDataTableManifestSchemaVersion = 1
	userDataRecordSchemaVersion        = "1.0.0"

	userDataBatchHashAlgorithmVersion = "content-chain-v3"

	userDataBatchGenesisDomain = "amitia-userdata-batch-genesis-v3"

	userDataBatchHashDomain = "amitia-userdata-batch-v3"

	userDataEmptySetDomain = "amitia-userdata-empty-set-v1"
)

type UserDataNamespaceIdentity struct {
	SchemaVersion          int    `json:"schemaVersion"`
	ExtensionID            string `json:"extensionId"`
	CanonicalTable         string `json:"canonicalTable"`
	LogicalEntityType      string `json:"logicalEntityType"`
	NamespacePolicyVersion string `json:"namespacePolicyVersion"`
}

type UserDataTableIdentity struct {
	Domain                string `json:"domain"`
	SchemaVersion         string `json:"schemaVersion"`
	ExtensionID           string `json:"extensionId"`
	CanonicalTable        string `json:"canonicalTable"`
	EntityType            string `json:"entityType"`
	NamespaceHash         string `json:"namespaceHash"`
	BatchAlgorithmVersion string `json:"batchAlgorithmVersion"`
}

type UserDataBatchChainSpec struct {
	Identity            UserDataTableIdentity
	DataExportReference string
	BatchSize           int64
	GenesisHash         string
}

type UserDataBatchChainResult struct {
	GenesisHash  string
	PreviousHash string
	FinalHash    string
	BatchCount   int64
	RecordCount  int64
}

type userDataBatchRecordDigest struct {
	Index       int    `json:"index"`
	RawJSONHash string `json:"rawJsonHash"`
	ExtensionID string `json:"extensionId"`
	Namespace   string `json:"namespace"`
	EntityType  string `json:"entityType"`
	EntityID    string `json:"entityId"`
	Operation   string `json:"operation"`
	PayloadHash string `json:"payloadHash"`
}

type userDataBatchHashPayload struct {
	Domain                string                      `json:"domain"`
	BatchAlgorithmVersion string                      `json:"batchAlgorithmVersion"`
	TableIdentity         UserDataTableIdentity       `json:"tableIdentity"`
	DataExportReference   string                      `json:"dataExportReference"`
	BatchIndex            int64                       `json:"batchIndex"`
	CursorBefore          int64                       `json:"cursorBefore"`
	CursorAfter           int64                       `json:"cursorAfter"`
	PreviousBatchHash     string                      `json:"previousBatchHash"`
	Records               []userDataBatchRecordDigest `json:"records"`
}

func computeUserDataNamespaceHash(identity UserDataNamespaceIdentity) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("ns-policy:%s|ext:%s|table:%s|logicalType:%s|schemaVer:%d",
		identity.NamespacePolicyVersion,
		strings.ToLower(strings.TrimSpace(identity.ExtensionID)),
		strings.ToLower(strings.TrimSpace(identity.CanonicalTable)),
		strings.ToLower(strings.TrimSpace(identity.LogicalEntityType)),
		identity.SchemaVersion)))
	return "ns:" + hex.EncodeToString(hasher.Sum(nil))
}

func computeUserDataEmptySetHash(identity UserDataTableIdentity) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("domain:%s|nsHash:%s|ext:%s|table:%s|entityType:%s|schemaVer:%s|algo:%s",
		identity.Domain,
		identity.NamespaceHash,
		strings.ToLower(strings.TrimSpace(identity.ExtensionID)),
		strings.ToLower(strings.TrimSpace(identity.CanonicalTable)),
		strings.ToLower(strings.TrimSpace(identity.EntityType)),
		strings.ToLower(strings.TrimSpace(identity.SchemaVersion)),
		identity.BatchAlgorithmVersion)))
	return "eset:" + hex.EncodeToString(hasher.Sum(nil))
}

func computeUserDataGenesisHash(identity UserDataTableIdentity) string {
	if identity.Domain != userDataBatchGenesisDomain {
		return ""
	}
	if strings.TrimSpace(identity.SchemaVersion) == "" ||
		strings.TrimSpace(identity.ExtensionID) == "" ||
		strings.TrimSpace(identity.CanonicalTable) == "" ||
		strings.TrimSpace(identity.EntityType) == "" ||
		strings.TrimSpace(identity.NamespaceHash) == "" ||
		identity.BatchAlgorithmVersion != userDataBatchHashAlgorithmVersion {
		return ""
	}
	raw, err := json.Marshal(struct {
		Domain                string `json:"domain"`
		SchemaVersion         string `json:"schemaVersion"`
		ExtensionID           string `json:"extensionId"`
		CanonicalTable        string `json:"canonicalTable"`
		EntityType            string `json:"entityType"`
		NamespaceHash         string `json:"namespaceHash"`
		BatchAlgorithmVersion string `json:"batchAlgorithmVersion"`
	}{
		Domain:                identity.Domain,
		SchemaVersion:         strings.ToLower(strings.TrimSpace(identity.SchemaVersion)),
		ExtensionID:           strings.ToLower(strings.TrimSpace(identity.ExtensionID)),
		CanonicalTable:        strings.ToLower(strings.TrimSpace(identity.CanonicalTable)),
		EntityType:            strings.ToLower(strings.TrimSpace(identity.EntityType)),
		NamespaceHash:         strings.TrimSpace(identity.NamespaceHash),
		BatchAlgorithmVersion: identity.BatchAlgorithmVersion,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "genesis:sha256:" + hex.EncodeToString(sum[:])
}

func validateUserDataBatchChainSpec(spec UserDataBatchChainSpec) error {
	if spec.Identity.Domain != userDataBatchGenesisDomain {
		return fmt.Errorf("kernel: batch chain genesis domain mismatch")
	}
	if spec.Identity.BatchAlgorithmVersion != userDataBatchHashAlgorithmVersion {
		return fmt.Errorf("kernel: unsupported batch algorithm version %q", spec.Identity.BatchAlgorithmVersion)
	}
	if strings.TrimSpace(spec.Identity.SchemaVersion) == "" {
		return fmt.Errorf("kernel: batch chain schema version missing")
	}
	if strings.TrimSpace(spec.Identity.ExtensionID) == "" {
		return fmt.Errorf("kernel: batch chain extension id missing")
	}
	if strings.TrimSpace(spec.Identity.CanonicalTable) == "" {
		return fmt.Errorf("kernel: batch chain canonical table missing")
	}
	if strings.TrimSpace(spec.Identity.EntityType) == "" {
		return fmt.Errorf("kernel: batch chain entity type missing")
	}
	if strings.TrimSpace(spec.Identity.NamespaceHash) == "" {
		return fmt.Errorf("kernel: batch chain namespace hash missing")
	}
	if strings.TrimSpace(spec.DataExportReference) == "" {
		return fmt.Errorf("kernel: batch chain data export reference missing")
	}
	if spec.BatchSize <= 0 {
		return fmt.Errorf("kernel: batch chain batch size must be positive")
	}
	if spec.BatchSize != int64(userDataRestoreBatchSize) {
		return fmt.Errorf("kernel: batch chain batch size mismatch: expected=%d actual=%d", userDataRestoreBatchSize, spec.BatchSize)
	}
	expectedGenesis := computeUserDataGenesisHash(spec.Identity)
	if expectedGenesis == "" {
		return fmt.Errorf("kernel: batch chain genesis computation failed")
	}
	if strings.TrimSpace(spec.GenesisHash) == "" {
		return fmt.Errorf("kernel: batch chain genesis hash missing")
	}
	if spec.GenesisHash != expectedGenesis {
		return fmt.Errorf("kernel: batch chain genesis hash mismatch: expected=%s actual=%s", expectedGenesis, spec.GenesisHash)
	}
	return nil
}

func buildUserDataBatchChainSpec(manifest UserDataTableManifest) (UserDataBatchChainSpec, error) {
	spec := UserDataBatchChainSpec{
		Identity: UserDataTableIdentity{
			Domain:                userDataBatchGenesisDomain,
			SchemaVersion:         userDataRecordSchemaVersion,
			ExtensionID:           manifest.ExtensionID,
			CanonicalTable:        manifest.CanonicalTable,
			EntityType:            manifest.EntityType,
			NamespaceHash:         manifest.NamespaceHash,
			BatchAlgorithmVersion: manifest.BatchAlgorithmVersion,
		},
		DataExportReference: manifest.DataExportReference,
		BatchSize:           int64(manifest.BatchSize),
		GenesisHash:         manifest.GenesisHash,
	}
	if err := validateUserDataBatchChainSpec(spec); err != nil {
		return UserDataBatchChainSpec{}, NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409, err)
	}
	return spec, nil
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

	genesis_hash TEXT NOT NULL DEFAULT '',
	expected_final_batch_hash TEXT NOT NULL DEFAULT '',

	expected_aggregate_hash TEXT NOT NULL DEFAULT '',
	aggregate_hash TEXT NOT NULL DEFAULT '',

	data_export_reference TEXT NOT NULL DEFAULT '',

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
	JournalID   string
	OperationID string
	ExtensionID string
	Namespace   string
	TableName   string

	TotalRows    int64
	ImportedRows int64
	AppliedCount int64
	Cursor       string

	BatchHash             string
	BatchIndex            int64
	PrevBatchHash         string
	BatchAlgorithmVersion string
	BatchSize             int64

	NamespaceHash string

	GenesisHash            string
	ExpectedFinalBatchHash string

	ExpectedAggregateHash string
	AggregateHash         string

	DataExportReference string

	State       UserDataRestoreState
	StartedAt   string
	UpdatedAt   string
	ErrorDetail string
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
		s.ensureExpectedAggregateHashColumn,
		s.ensureAggregateHashColumn,
		s.ensureDataExportReferenceColumn,
		s.ensureGenesisHashColumn,
		s.ensureExpectedFinalBatchHashColumn,
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

func (s *UserDataSnapshotStore) ensureExpectedAggregateHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='expected_aggregate_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN expected_aggregate_hash TEXT NOT NULL DEFAULT ''`)
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

func (s *UserDataSnapshotStore) ensureDataExportReferenceColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='data_export_reference'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN data_export_reference TEXT NOT NULL DEFAULT ''`)
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

func (s *UserDataSnapshotStore) ensureGenesisHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='genesis_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN genesis_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *UserDataSnapshotStore) ensureExpectedFinalBatchHashColumn(ctx context.Context) error {
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('extension_package_user_data_restore_journal') WHERE name='expected_final_batch_hash'`,
	).Scan(&name)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE extension_package_user_data_restore_journal ADD COLUMN expected_final_batch_hash TEXT NOT NULL DEFAULT ''`)
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
	if !isValidEntityType(record.EntityType) {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record entityType %q is not a valid identifier", record.EntityType))
	}
	if record.EntityType != resolver.LogicalEntityType {
		return NewPackageError(PackageErrCodeUserDataRecordInvalid, 422,
			fmt.Errorf("kernel: user data record entityType %q does not match namespace logical entity type %q", record.EntityType, resolver.LogicalEntityType))
	}
	if record.Operation != "" && !isValidOperation(record.Operation) {
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

const maxEntityTypeLength = 128

func isASCIIAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isReservedEntityType(value string) bool {
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "ext_") {
		return true
	}
	if strings.HasPrefix(lower, "system_") {
		return true
	}
	if strings.HasPrefix(lower, "internal_") {
		return true
	}
	return false
}

func isValidEntityType(entityType string) bool {
	if entityType == "" || len(entityType) > maxEntityTypeLength {
		return false
	}
	for i, r := range entityType {
		if i == 0 {
			if !isASCIIAlpha(r) && r != '_' {
				return false
			}
			continue
		}
		if !isASCIIAlpha(r) && !isASCIIDigit(r) && r != '_' {
			return false
		}
	}
	return !isReservedEntityType(entityType)
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
	if userState.Mode == "none" {
		if len(userState.AffectedTables) != 0 ||
			len(userState.TableManifests) != 0 ||
			len(userState.DataExports) != 0 {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: mode none contains user data evidence"))
		}
		return nil
	}
	if len(userState.AffectedTables) == 0 {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data snapshot has no affected tables"))
	}
	if len(userState.TableManifests) != len(userState.AffectedTables) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest count mismatch"))
	}
	seen := make(map[string]struct{}, len(userState.AffectedTables))
	for _, table := range userState.AffectedTables {
		if _, duplicate := seen[table]; duplicate {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: duplicate affected table %s", table))
		}
		seen[table] = struct{}{}
		manifest, exists := userState.TableManifests[table]
		if !exists {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: table %s manifest missing", table))
		}
		jsonlData, exists := userState.DataExports[table]
		if !exists {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: table %s data export missing", table))
		}
		rawRecords, parsedRecords, err := parseAndValidateJSONL(jsonlData, extensionID)
		if err != nil {
			return err
		}
		if err := validateUserDataTableSnapshotManifest(manifest, extensionID, table, jsonlData, rawRecords, parsedRecords); err != nil {
			return err
		}
		if err := s.restoreTable(ctx, extensionID, operationID, table, manifest, parsedRecords, rawRecords); err != nil {
			return fmt.Errorf("kernel: restore user data table %s: %w", table, err)
		}
	}
	return nil
}

func (s *UserDataSnapshotStore) restoreTable(ctx context.Context, extensionID, operationID, table string, manifest UserDataTableManifest, parsedRecords []userDataRecord, rawRecords []map[string]interface{}) error {
	if !migration.IsExtensionNamespaceTable(table, extensionID) {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: user data table %q does not belong to extension namespace", table))
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
	var resolver ExtensionUserDataNamespace
	if namespace == "" {
		namespace = manifest.Namespace
		schemaVersion = userDataRecordSchemaVersion
		var resolverErr error
		resolver, resolverErr = ResolveExtensionUserDataNamespace(extensionID, table)
		if resolverErr != nil {
			return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: resolve namespace for table %s: %w", table, resolverErr))
		}
	} else {
		var resolverErr error
		resolver, resolverErr = ResolveExtensionUserDataNamespace(extensionID, table)
		if resolverErr != nil {
			return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: resolve namespace for table %s: %w", table, resolverErr))
		}
	}
	expectedNamespaceHash := computeUserDataNamespaceHash(UserDataNamespaceIdentity{
		SchemaVersion:          1,
		ExtensionID:            extensionID,
		CanonicalTable:         namespace,
		LogicalEntityType:      resolver.LogicalEntityType,
		NamespacePolicyVersion: "v1",
	})
	journal, err := s.getOrCreateRestoreJournal(ctx, operationID, extensionID, table, namespace, manifest, expectedNamespaceHash)
	if err != nil {
		return err
	}
	if journal.State == UserDataRestoreCompleted {
		return s.verifyCompletedJournalAgainstManifest(ctx, journal, manifest, expectedNamespaceHash)
	}
	if journal.State == UserDataRestoreFailed {
		if err := s.resetRestoreJournalToGenesis(ctx, journal, manifest); err != nil {
			return err
		}
	}
	startCursor, cursorErr := parseJournalCursor(journal, manifest.RecordCount)
	if cursorErr != nil {
		return cursorErr
	}
	if journal.AppliedCount != startCursor {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: journal applied_count %d does not match cursor %d", journal.AppliedCount, startCursor))
	}
	if journal.BatchAlgorithmVersion != userDataBatchHashAlgorithmVersion {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: journal %s batch algorithm version %q does not match current %q", journal.JournalID, journal.BatchAlgorithmVersion, userDataBatchHashAlgorithmVersion))
	}
	chainSpec, chainErr := buildUserDataBatchChainSpec(manifest)
	if chainErr != nil {
		return chainErr
	}
	if verifyErr := verifyRestoreJournalBatchPrefix(journal, rawRecords, chainSpec); verifyErr != nil {
		return verifyErr
	}
	if len(parsedRecords) == 0 {
		genesisHash := computeUserDataGenesisHash(UserDataTableIdentity{
			Domain:                userDataBatchGenesisDomain,
			SchemaVersion:         schemaVersion,
			ExtensionID:           extensionID,
			CanonicalTable:        namespace,
			EntityType:            resolver.LogicalEntityType,
			NamespaceHash:         expectedNamespaceHash,
			BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
		})
		if genesisHash == "" {
			failErr := fmt.Errorf("kernel: compute genesis hash failed for table %s", table)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if err := s.fullTableReplace(ctx, table, records); err != nil {
			failErr := fmt.Errorf("kernel: empty restore failed to clear table %s: %w", table, err)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		var dbCount int64
		countErr := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))).Scan(&dbCount)
		if countErr != nil {
			failErr := fmt.Errorf("kernel: verify empty table record count for %s: %w", table, countErr)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if dbCount != 0 {
			failErr := fmt.Errorf("kernel: empty table %s still has %d records after clear", table, dbCount)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		emptyContentHash, aggErr := s.computeAggregateHashFromDB(ctx, table)
		if aggErr != nil {
			failErr := fmt.Errorf("kernel: compute empty content aggregate hash for %s: %w", table, aggErr)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		expectedEmptyAggregateHash := computeUserDataAggregateHashFromRecords(nil)
		if expectedEmptyAggregateHash != emptyContentHash {
			failErr := fmt.Errorf("kernel: empty table %s aggregate hash mismatch: expected %s, got %s", table, expectedEmptyAggregateHash, emptyContentHash)
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if emptyContentHash != manifest.AggregateHash {
			failErr := NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
				fmt.Errorf("kernel: empty database aggregate differs from manifest for table %s: database=%s manifest=%s", table, emptyContentHash, manifest.AggregateHash))
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if manifest.FinalBatchHash != manifest.GenesisHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: empty table %s final batch hash must equal genesis", table))
		}
		if err := s.completeRestoreJournalWithAggregate(ctx, journal, manifest, userDataRestoreCompletionEvidence{
			Cursor:        "0",
			ImportedRows:  0,
			AppliedCount:  0,
			BatchIndex:    0,
			BatchHash:     genesisHash,
			AggregateHash: emptyContentHash,
		}); err != nil {
			return err
		}
		return s.verifyCompletedJournalAgainstManifest(ctx, journal, manifest, expectedNamespaceHash)
	}
	if startCursor == manifest.RecordCount {
		if journal.BatchHash != manifest.FinalBatchHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: completed cursor has wrong final batch hash for table %s", table))
		}
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
		if recomputedAggHash != manifest.AggregateHash {
			failErr := NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
				fmt.Errorf("kernel: aggregate hash mismatch on recovery for table %s", table))
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
			}
			return failErr
		}
		if err := s.completeRestoreJournalWithAggregate(ctx, journal, manifest, userDataRestoreCompletionEvidence{
			Cursor:        strconv.FormatInt(manifest.RecordCount, 10),
			ImportedRows:  manifest.RecordCount,
			AppliedCount:  manifest.RecordCount,
			BatchIndex:    journal.BatchIndex,
			BatchHash:     journal.BatchHash,
			AggregateHash: recomputedAggHash,
		}); err != nil {
			return err
		}
		return s.verifyCompletedJournalAgainstManifest(ctx, journal, manifest, expectedNamespaceHash)
	}
	if err := s.updateRestoreJournalState(ctx, journal, UserDataRestoreImporting, ""); err != nil {
		return fmt.Errorf("kernel: update journal to importing: %w", err)
	}
	appliedCount := journal.AppliedCount
	prevBatchHash := journal.BatchHash
	batchIdx := journal.BatchIndex
	remaining := records[startCursor:]
	for i := 0; i < len(remaining); i += userDataRestoreBatchSize {
		end := i + userDataRestoreBatchSize
		if end > len(remaining) {
			end = len(remaining)
		}
		batch := remaining[i:end]
		batchIdx++
		batchHash, err := computeContentBoundBatchHash(batch, chainSpec, startCursor+int64(i), prevBatchHash, int64(batchIdx))
		if err != nil {
			failErr := NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: batch content hash computation failed at cursor %d: %w", startCursor+int64(i), err))
			if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
				return errors.Join(failErr, journalErr)
			}
			return failErr
		}
		batchApplied, _, err := s.importBatchAtomic(ctx, journal, table, batch, startCursor+int64(i), appliedCount, int(batchIdx), batchHash, prevBatchHash, journal.BatchHash, journal.BatchIndex)
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
	if prevBatchHash != manifest.FinalBatchHash {
		failErr := NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: restored batch chain final hash mismatch for table %s: journal=%s manifest=%s", table, prevBatchHash, manifest.FinalBatchHash))
		if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
			return errors.Join(failErr, journalErr)
		}
		return failErr
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
	if aggHashFromDB != manifest.AggregateHash {
		failErr := NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: aggregate hash mismatch for table %s: expected %s, got %s", table, manifest.AggregateHash, aggHashFromDB))
		if journalErr := s.updateRestoreJournalState(ctx, journal, UserDataRestoreFailed, failErr.Error()); journalErr != nil {
			return errors.Join(failErr, fmt.Errorf("kernel: update journal to failed: %w", journalErr))
		}
		return failErr
	}
	if err := s.completeRestoreJournalWithAggregate(ctx, journal, manifest, userDataRestoreCompletionEvidence{
		Cursor:        strconv.FormatInt(manifest.RecordCount, 10),
		ImportedRows:  manifest.RecordCount,
		AppliedCount:  appliedCount,
		BatchIndex:    int64(batchIdx),
		BatchHash:     prevBatchHash,
		AggregateHash: aggHashFromDB,
	}); err != nil {
		return err
	}
	return s.verifyCompletedJournalAgainstManifest(ctx, journal, manifest, expectedNamespaceHash)
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

func (s *UserDataSnapshotStore) getOrCreateRestoreJournal(
	ctx context.Context,
	operationID string,
	extensionID string,
	table string,
	namespace string,
	manifest UserDataTableManifest,
	expectedNamespaceHash string,
) (*UserDataRestoreJournal, error) {
	if strings.TrimSpace(manifest.AggregateHash) == "" {
		return nil, NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest aggregate hash missing for restore journal %s/%s", operationID, table))
	}
	if err := validateUserDataAggregateHashFormat(manifest.AggregateHash); err != nil {
		return nil, NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409, err)
	}
	if manifest.DataExportReference == "" {
		return nil, NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: manifest data export reference missing for table %s", table))
	}

	nsHash := manifest.NamespaceHash
	if nsHash == "" {
		nsHash = expectedNamespaceHash
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	journal := &UserDataRestoreJournal{
		JournalID:   "ud-journal-" + operationID + "-" + table,
		OperationID: operationID,
		ExtensionID: extensionID,
		Namespace:   namespace,
		TableName:   table,

		TotalRows:             manifest.RecordCount,
		NamespaceHash:         nsHash,
		ExpectedAggregateHash: manifest.AggregateHash,
		DataExportReference:   manifest.DataExportReference,
		State:                 UserDataRestorePending,
		StartedAt:             now,
		UpdatedAt:             now,
	}

	var (
		storedExtensionID  string
		storedTotalRows    int64
		state              string
		startedAt          string
		updatedAt          string
		errorDetail        string
		cursor             string
		batchHash          string
		prevBatchHash      string
		batchAlgorithmVer  string
		namespaceHash      string
		expectedAggHash    string
		aggregateHash      string
		dataExportRef      string
		importedRows       int64
		appliedCount       int64
		batchIndex         int64
		batchSize          int64
		genesisHash        string
		expectedFinalBatch string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT
extension_id,
total_rows,
state,
imported_rows,
applied_count,
cursor,
batch_hash,
batch_index,
prev_batch_hash,
batch_algorithm_version,
batch_size,
namespace_hash,
genesis_hash,
expected_final_batch_hash,
expected_aggregate_hash,
aggregate_hash,
data_export_reference,
started_at,
updated_at,
error_detail
 FROM extension_package_user_data_restore_journal
 WHERE operation_id=?
   AND table_name=?`,
		operationID, table,
	).Scan(
		&storedExtensionID,
		&storedTotalRows,
		&state,
		&importedRows,
		&appliedCount,
		&cursor,
		&batchHash,
		&batchIndex,
		&prevBatchHash,
		&batchAlgorithmVer,
		&batchSize,
		&namespaceHash,
		&genesisHash,
		&expectedFinalBatch,
		&expectedAggHash,
		&aggregateHash,
		&dataExportRef,
		&startedAt,
		&updatedAt,
		&errorDetail,
	)

	if err == nil {
		journal.ExtensionID = storedExtensionID
		journal.TotalRows = storedTotalRows
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
		journal.GenesisHash = genesisHash
		journal.ExpectedFinalBatchHash = expectedFinalBatch
		journal.ExpectedAggregateHash = expectedAggHash
		journal.AggregateHash = aggregateHash
		journal.DataExportReference = dataExportRef
		journal.StartedAt = startedAt
		journal.UpdatedAt = updatedAt
		journal.ErrorDetail = errorDetail

		if journal.ExtensionID != extensionID {
			return nil, NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: restore journal %s belongs to extension %s, cannot be used for extension %s", journal.JournalID, journal.ExtensionID, extensionID))
		}
		if journal.TotalRows != manifest.RecordCount {
			return nil, NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
				fmt.Errorf("kernel: restore journal %s row count differs from manifest: journal=%d manifest=%d", journal.JournalID, journal.TotalRows, manifest.RecordCount))
		}
		if journal.NamespaceHash == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s has empty namespace hash", journal.JournalID))
		}
		if journal.NamespaceHash != expectedNamespaceHash {
			return nil, NewPackageError(PackageErrCodeUserDataJournalHashMismatch, 422,
				fmt.Errorf("kernel: restore journal %s namespace hash mismatch: expected=%s actual=%s", journal.JournalID, expectedNamespaceHash, journal.NamespaceHash))
		}
		if journal.ExpectedAggregateHash == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s has empty expected aggregate hash", journal.JournalID))
		}
		if journal.ExpectedAggregateHash != manifest.AggregateHash {
			return nil, NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
				fmt.Errorf("kernel: restore journal %s expected aggregate differs from manifest: journal=%s manifest=%s", journal.JournalID, journal.ExpectedAggregateHash, manifest.AggregateHash))
		}
		if journal.DataExportReference == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s has empty data export reference", journal.JournalID))
		}
		if journal.DataExportReference != manifest.DataExportReference {
			return nil, NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
				fmt.Errorf("kernel: restore journal %s belongs to export %s, cannot use export %s", journal.JournalID, journal.DataExportReference, manifest.DataExportReference))
		}
		if journal.GenesisHash == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s has empty genesis hash", journal.JournalID))
		}
		if journal.GenesisHash != manifest.GenesisHash {
			return nil, NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: restore journal %s genesis differs from manifest", journal.JournalID))
		}
		if journal.ExpectedFinalBatchHash == "" {
			return nil, NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal %s expected final batch hash missing", journal.JournalID))
		}
		if journal.ExpectedFinalBatchHash != manifest.FinalBatchHash {
			return nil, NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
				fmt.Errorf("kernel: restore journal %s final batch hash differs from manifest", journal.JournalID))
		}
		if journal.BatchAlgorithmVersion != manifest.BatchAlgorithmVersion {
			return nil, NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: restore journal %s batch algorithm differs from manifest", journal.JournalID))
		}
		if journal.BatchSize != int64(manifest.BatchSize) {
			return nil, NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: restore journal %s batch size differs from manifest", journal.JournalID))
		}
		if journal.AggregateHash != "" && journal.AggregateHash != journal.ExpectedAggregateHash {
			return nil, NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
				fmt.Errorf("kernel: restore journal %s observed aggregate differs from expected aggregate: observed=%s expected=%s", journal.JournalID, journal.AggregateHash, journal.ExpectedAggregateHash))
		}
		return journal, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("query restore journal: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO
 extension_package_user_data_restore_journal (
journal_id,
operation_id,
extension_id,
table_name,
total_rows,
imported_rows,
applied_count,
cursor,
batch_hash,
batch_index,
prev_batch_hash,
batch_algorithm_version,
batch_size,
namespace_hash,
genesis_hash,
expected_final_batch_hash,
expected_aggregate_hash,
aggregate_hash,
data_export_reference,
state,
started_at,
updated_at,
error_detail
 ) VALUES (
?, ?, ?, ?, ?,
?, ?, ?, ?, ?,
?, ?, ?, ?, ?,
?, ?, ?, ?, ?,
?, ?, ?
 )`,
		journal.JournalID,
		operationID,
		extensionID,
		table,
		manifest.RecordCount,
		0,
		0,
		"0",
		manifest.GenesisHash,
		0,
		"",
		userDataBatchHashAlgorithmVersion,
		userDataRestoreBatchSize,
		expectedNamespaceHash,
		manifest.GenesisHash,
		manifest.FinalBatchHash,
		manifest.AggregateHash,
		"",
		manifest.DataExportReference,
		string(UserDataRestorePending),
		now,
		now,
		"",
	)

	if err != nil {
		return nil, fmt.Errorf("create restore journal: %w", err)
	}

	journal.BatchAlgorithmVersion = userDataBatchHashAlgorithmVersion
	journal.BatchSize = userDataRestoreBatchSize
	journal.GenesisHash = manifest.GenesisHash
	journal.ExpectedFinalBatchHash = manifest.FinalBatchHash
	journal.BatchHash = manifest.GenesisHash

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

func (s *UserDataSnapshotStore) resetRestoreJournalToGenesis(
	ctx context.Context,
	journal *UserDataRestoreJournal,
	manifest UserDataTableManifest,
) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal
 SET state=?,
imported_rows=0,
applied_count=0,
cursor='',
batch_hash=?,
batch_index=0,
prev_batch_hash='',
aggregate_hash='',
error_detail='',
updated_at=?
 WHERE operation_id=?
   AND table_name=?
   AND genesis_hash=?
   AND expected_final_batch_hash=?`,
		string(UserDataRestorePending),
		manifest.GenesisHash,
		now,
		journal.OperationID,
		journal.TableName,
		manifest.GenesisHash,
		manifest.FinalBatchHash,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
			fmt.Errorf("kernel: reset restore journal to genesis affected %d rows", rowsAffected))
	}
	journal.State = UserDataRestorePending
	journal.ImportedRows = 0
	journal.AppliedCount = 0
	journal.Cursor = ""
	journal.BatchHash = manifest.GenesisHash
	journal.BatchIndex = 0
	journal.PrevBatchHash = ""
	journal.AggregateHash = ""
	journal.ErrorDetail = ""
	journal.UpdatedAt = now
	return nil
}

func (s *UserDataSnapshotStore) importBatchAtomic(
	ctx context.Context,
	journal *UserDataRestoreJournal,
	table string,
	batch []map[string]interface{},
	cursorBefore int64,
	appliedCountBefore int64,
	batchIndex int,
	batchHash string,
	previousBatchHash string,
	expectedCurrentBatchHash string,
	expectedBatchIndex int64,
) (int64, string, error) {
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
		 WHERE operation_id=? AND table_name=? AND state=? AND batch_hash=? AND batch_index=? AND cursor=?`,
		newCursor, strconv.FormatInt(newCursor, 10), newAppliedCount, batchHash, int64(batchIndex), previousBatchHash, now,
		journal.OperationID, journal.TableName, string(UserDataRestoreImporting), expectedCurrentBatchHash, expectedBatchIndex, strconv.FormatInt(cursorBefore, 10))
	if err != nil {
		return 0, "", fmt.Errorf("update journal in batch transaction: %w", err)
	}
	rowsAff, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		return 0, "", fmt.Errorf("query rows affected for journal update: %w", rowsErr)
	}
	if rowsAff != 1 {
		return 0, "", NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409, fmt.Errorf("kernel: journal update affected %d rows, expected exactly 1 (op=%s table=%s batch=%d)", rowsAff, journal.OperationID, journal.TableName, batchIndex))
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("commit atomic batch for %s: %w", table, err)
	}
	journal.ImportedRows = newCursor
	journal.Cursor = strconv.FormatInt(newCursor, 10)
	journal.AppliedCount = newAppliedCount
	journal.BatchHash = batchHash
	journal.BatchIndex = int64(batchIndex)
	journal.PrevBatchHash = previousBatchHash
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
	if detectIDColumnName(columns) == "" {
		return "", NewPackageError(PackageErrCodeUserDataEntityIDMissing, 422,
			fmt.Errorf("kernel: table %s has no id or entity_id column during aggregate verification", table))
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

func computeUserDataAggregateHash(extensionID, namespace, entityType, schemaVersion string, rawRecords []map[string]interface{}) string {
	hashes := make([]string, 0, len(rawRecords))
	for _, record := range rawRecords {
		hashes = append(hashes, computeUserDataRecordHash(extensionID, namespace, entityType, schemaVersion, record))
	}
	sort.Strings(hashes)
	hasher := sha256.New()
	for _, h := range hashes {
		hasher.Write([]byte(h))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func computeUserDataAggregateHashFromRecords(records []userDataRecord) string {
	hashes := make([]string, 0, len(records))
	for _, record := range records {
		payloadHash := record.PayloadHash
		if payloadHash == "" {
			payloadHash = computeUserDataPayloadHash(record.Payload)
		}
		hashes = append(hashes, payloadHash)
	}
	sort.Strings(hashes)
	hasher := sha256.New()
	for _, payloadHash := range hashes {
		hasher.Write([]byte(payloadHash))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func computeUserDataExportReference(operationID, table, extensionID string) string {
	operationID = strings.TrimSpace(operationID)
	table = strings.TrimSpace(table)
	extensionID = strings.TrimSpace(extensionID)
	if operationID == "" || table == "" || extensionID == "" {
		return ""
	}
	return fmt.Sprintf("export:%s:%s:%s", operationID, table, extensionID)
}

type userDataSnapshotExportIdentity struct {
	SchemaVersion  int    `json:"schemaVersion"`
	ExtensionID    string `json:"extensionId"`
	CanonicalTable string `json:"canonicalTable"`
	EntityType     string `json:"entityType"`
	NamespaceHash  string `json:"namespaceHash"`
	RecordCount    int64  `json:"recordCount"`
	JSONLHash      string `json:"jsonlHash"`
}

func computeCapturedUserDataExportReference(
	extensionID string,
	canonicalTable string,
	entityType string,
	namespaceHash string,
	recordCount int64,
	jsonlData string,
) (string, error) {
	extensionID = strings.TrimSpace(extensionID)
	canonicalTable = strings.TrimSpace(canonicalTable)
	entityType = strings.TrimSpace(entityType)
	namespaceHash = strings.TrimSpace(namespaceHash)
	if extensionID == "" || canonicalTable == "" || entityType == "" || namespaceHash == "" || recordCount < 0 {
		return "", fmt.Errorf("kernel: captured user data export identity incomplete")
	}
	jsonlDigest := sha256.Sum256([]byte(jsonlData))
	identity := userDataSnapshotExportIdentity{
		SchemaVersion:  1,
		ExtensionID:    extensionID,
		CanonicalTable: canonicalTable,
		EntityType:     entityType,
		NamespaceHash:  namespaceHash,
		RecordCount:    recordCount,
		JSONLHash:      "sha256:" + hex.EncodeToString(jsonlDigest[:]),
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("kernel: marshal captured export identity: %w", err)
	}
	identityDigest := sha256.Sum256(raw)
	return "snapshot-export:sha256:" + hex.EncodeToString(identityDigest[:]), nil
}

func computeUserDataRecordHash(extensionID, namespace, entityType, schemaVersion string, rawPayload map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("ext:%s:ns:%s:entity:%s:schema:%s:payload:%v",
		strings.ToLower(strings.TrimSpace(extensionID)),
		strings.ToLower(strings.TrimSpace(namespace)),
		strings.ToLower(strings.TrimSpace(entityType)),
		strings.ToLower(strings.TrimSpace(schemaVersion)),
		rawPayload)))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func validateUserDataAggregateHashFormat(value string) error {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "sha256:") {
		return fmt.Errorf("aggregate hash must use sha256 prefix")
	}
	digest := strings.TrimPrefix(value, "sha256:")
	if len(digest) != 64 {
		return fmt.Errorf("aggregate hash digest length must be 64")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("aggregate hash digest is not valid hex: %w", err)
	}
	return nil
}

func validateUserDataTableSnapshotManifest(manifest UserDataTableManifest, extensionID string, table string, jsonlData string, rawRecords []map[string]interface{}, parsedRecords []userDataRecord) error {
	if manifest.SchemaVersion != userDataTableManifestSchemaVersion {
		return NewPackageError(PackageErrCodeSnapshotSchemaUnsupported, 422,
			fmt.Errorf("kernel: table manifest schema version mismatch for %s: expected %d, got %d", table, userDataTableManifestSchemaVersion, manifest.SchemaVersion))
	}
	if manifest.ExtensionID != extensionID {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest extension_id mismatch for %s: expected %s, got %s", table, extensionID, manifest.ExtensionID))
	}
	if manifest.CanonicalTable != table {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest canonical_table mismatch: expected %s, got %s", table, manifest.CanonicalTable))
	}
	if manifest.Namespace == "" {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: table manifest namespace missing for %s", table))
	}
	if manifest.RecordCount != int64(len(parsedRecords)) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest record count mismatch for %s: manifest=%d parsed=%d", table, manifest.RecordCount, len(parsedRecords)))
	}
	if len(rawRecords) != len(parsedRecords) {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: raw and parsed record count mismatch for %s: raw=%d parsed=%d", table, len(rawRecords), len(parsedRecords)))
	}
	if manifest.AggregateHash == "" {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest aggregate hash missing for %s", table))
	}
	if err := validateUserDataAggregateHashFormat(manifest.AggregateHash); err != nil {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest aggregate hash format invalid for table %s: %w", table, err))
	}
	expectedAggregateHash := computeUserDataAggregateHashFromRecords(parsedRecords)
	if manifest.AggregateHash != expectedAggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest aggregate hash mismatch for table %s: expected=%s actual=%s", table, expectedAggregateHash, manifest.AggregateHash))
	}
	if strings.TrimSpace(manifest.DataExportReference) == "" {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table manifest data export reference missing for %s", table))
	}
	resolver, err := ResolveExtensionUserDataNamespace(extensionID, table)
	if err != nil {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: resolve manifest namespace for table %s: %w", table, err))
	}
	if manifest.Namespace != resolver.CanonicalTable {
		return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
			fmt.Errorf("kernel: table manifest namespace mismatch for %s: namespace=%s canonical=%s", table, manifest.Namespace, resolver.CanonicalTable))
	}
	expectedNamespaceHash := computeUserDataNamespaceHash(UserDataNamespaceIdentity{
		SchemaVersion:          1,
		ExtensionID:            extensionID,
		CanonicalTable:         resolver.CanonicalTable,
		LogicalEntityType:      resolver.LogicalEntityType,
		NamespacePolicyVersion: "v1",
	})
	if manifest.NamespaceHash == "" {
		return NewPackageError(PackageErrCodeUserDataJournalHashMismatch, 422,
			fmt.Errorf("kernel: manifest namespace hash missing for table %s", table))
	}
	if manifest.NamespaceHash != expectedNamespaceHash {
		return NewPackageError(PackageErrCodeUserDataJournalHashMismatch, 422,
			fmt.Errorf("kernel: manifest namespace hash mismatch for table %s: expected=%s actual=%s", table, expectedNamespaceHash, manifest.NamespaceHash))
	}
	if manifest.EmptySetHash == "" {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest empty set hash missing for table %s", table))
	}
	expectedEmptySetHash := computeUserDataEmptySetHash(UserDataTableIdentity{
		Domain:                userDataEmptySetDomain,
		SchemaVersion:         userDataRecordSchemaVersion,
		ExtensionID:           extensionID,
		CanonicalTable:        resolver.CanonicalTable,
		EntityType:            resolver.LogicalEntityType,
		NamespaceHash:         expectedNamespaceHash,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
	})
	if expectedEmptySetHash == "" {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 500,
			fmt.Errorf("kernel: failed to calculate empty set hash for table %s", table))
	}
	if manifest.EmptySetHash != expectedEmptySetHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest empty set hash mismatch for table %s: expected=%s actual=%s", table, expectedEmptySetHash, manifest.EmptySetHash))
	}
	if manifest.BatchAlgorithmVersion != userDataBatchHashAlgorithmVersion {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: table %s batch algorithm mismatch: expected=%s actual=%s", table, userDataBatchHashAlgorithmVersion, manifest.BatchAlgorithmVersion))
	}
	if manifest.BatchSize != userDataRestoreBatchSize {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: table %s batch size mismatch: expected=%d actual=%d", table, userDataRestoreBatchSize, manifest.BatchSize))
	}
	chainSpec, err := buildUserDataBatchChainSpec(manifest)
	if err != nil {
		return err
	}
	chainResult, err := recalculateBatchHashChain(rawRecords, chainSpec)
	if err != nil {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: recalculate manifest batch chain for table %s: %w", table, err))
	}
	if manifest.GenesisHash != chainResult.GenesisHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: table %s genesis hash mismatch: manifest=%s calculated=%s", table, manifest.GenesisHash, chainResult.GenesisHash))
	}
	if manifest.FinalBatchHash == "" || manifest.FinalBatchHash != chainResult.FinalHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: table %s final batch hash mismatch: manifest=%s calculated=%s", table, manifest.FinalBatchHash, chainResult.FinalHash))
	}
	if manifest.RecordCount == 0 {
		if manifest.FinalBatchHash != manifest.GenesisHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: empty table %s final batch hash must equal genesis", table))
		}
	} else {
		if manifest.FinalBatchHash == manifest.GenesisHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
				fmt.Errorf("kernel: non-empty table %s final batch hash equals genesis", table))
		}
	}
	for _, record := range parsedRecords {
		if record.ExtensionID != extensionID {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: record extension mismatch for table %s", table))
		}
		if record.Namespace != manifest.Namespace {
			return NewPackageError(PackageErrCodeUserDataNamespaceViolation, 422,
				fmt.Errorf("kernel: record namespace mismatch for table %s: manifest=%s record=%s", table, manifest.Namespace, record.Namespace))
		}
	}
	_ = jsonlData
	return nil
}

func (s *UserDataSnapshotStore) verifyAggregateHashClosure(
	ctx context.Context,
	journal *UserDataRestoreJournal,
	manifest UserDataTableManifest,
) error {
	if journal == nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: restore journal missing"))
	}
	if err := validateUserDataAggregateHashFormat(manifest.AggregateHash); err != nil {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest aggregate hash invalid for table %s: %w", journal.TableName, err))
	}
	if journal.ExpectedAggregateHash == "" {
		return NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
			fmt.Errorf("kernel: journal expected aggregate hash missing for table %s", journal.TableName))
	}
	if journal.ExpectedAggregateHash != manifest.AggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: manifest and journal expected aggregate mismatch for table %s: manifest=%s journalExpected=%s", journal.TableName, manifest.AggregateHash, journal.ExpectedAggregateHash))
	}
	if journal.AggregateHash == "" {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: completed journal aggregate hash missing for table %s", journal.TableName))
	}
	if journal.AggregateHash != journal.ExpectedAggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: journal observed and expected aggregate mismatch for table %s: observed=%s expected=%s", journal.TableName, journal.AggregateHash, journal.ExpectedAggregateHash))
	}
	actualAggregateHash, err := s.computeAggregateHashFromDB(ctx, journal.TableName)
	if err != nil {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: recompute database aggregate hash for table %s: %w", journal.TableName, err))
	}
	if actualAggregateHash != manifest.AggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: database and manifest aggregate mismatch for table %s: database=%s manifest=%s", journal.TableName, actualAggregateHash, manifest.AggregateHash))
	}
	if actualAggregateHash != journal.AggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: database and journal aggregate mismatch for table %s: database=%s journal=%s", journal.TableName, actualAggregateHash, journal.AggregateHash))
	}
	return nil
}

type userDataRestoreCompletionEvidence struct {
	Cursor        string
	ImportedRows  int64
	AppliedCount  int64
	BatchIndex    int64
	BatchHash     string
	AggregateHash string
}

func (s *UserDataSnapshotStore) completeRestoreJournalWithAggregate(
	ctx context.Context,
	journal *UserDataRestoreJournal,
	manifest UserDataTableManifest,
	evidence userDataRestoreCompletionEvidence,
) error {
	if journal == nil {
		return fmt.Errorf("kernel: restore journal missing")
	}
	if journal.ExpectedAggregateHash == "" || journal.ExpectedAggregateHash != manifest.AggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: journal expected aggregate does not match manifest for table %s", journal.TableName))
	}
	if evidence.AggregateHash != manifest.AggregateHash {
		return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 409,
			fmt.Errorf("kernel: completion database aggregate does not match manifest for table %s: database=%s manifest=%s", journal.TableName, evidence.AggregateHash, manifest.AggregateHash))
	}
	if evidence.ImportedRows != manifest.RecordCount || evidence.AppliedCount != manifest.RecordCount {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completion row count mismatch for table %s: manifest=%d imported=%d applied=%d", journal.TableName, manifest.RecordCount, evidence.ImportedRows, evidence.AppliedCount))
	}
	expectedCursor := strconv.FormatInt(manifest.RecordCount, 10)
	if evidence.Cursor != expectedCursor {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completion cursor mismatch for table %s: expected=%s actual=%s", journal.TableName, expectedCursor, evidence.Cursor))
	}
	if evidence.BatchHash != manifest.FinalBatchHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completion final batch hash differs from manifest for table %s: completion=%s manifest=%s", journal.TableName, evidence.BatchHash, manifest.FinalBatchHash))
	}
	if journal.GenesisHash != manifest.GenesisHash || journal.ExpectedFinalBatchHash != manifest.FinalBatchHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completion journal chain identity differs from manifest for table %s", journal.TableName))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`UPDATE
 extension_package_user_data_restore_journal
 SET
state=?,
cursor=?,
imported_rows=?,
applied_count=?,
batch_index=?,
batch_hash=?,
aggregate_hash=?,
error_detail='',
updated_at=?
 WHERE operation_id=?
   AND table_name=?
   AND state=?
   AND total_rows=?
   AND namespace_hash=?
   AND expected_aggregate_hash=?
   AND data_export_reference=?`,
		string(UserDataRestoreCompleted),
		evidence.Cursor,
		evidence.ImportedRows,
		evidence.AppliedCount,
		evidence.BatchIndex,
		evidence.BatchHash,
		evidence.AggregateHash,
		now,
		journal.OperationID,
		journal.TableName,
		string(journal.State),
		manifest.RecordCount,
		journal.NamespaceHash,
		manifest.AggregateHash,
		manifest.DataExportReference,
	)
	if err != nil {
		return fmt.Errorf("kernel: atomically complete restore journal for table %s: %w", journal.TableName, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
			fmt.Errorf("kernel: completion journal update affected %d rows for operation=%s table=%s", rowsAffected, journal.OperationID, journal.TableName))
	}
	journal.State = UserDataRestoreCompleted
	journal.Cursor = evidence.Cursor
	journal.ImportedRows = evidence.ImportedRows
	journal.AppliedCount = evidence.AppliedCount
	journal.BatchIndex = evidence.BatchIndex
	journal.BatchHash = evidence.BatchHash
	journal.AggregateHash = evidence.AggregateHash
	journal.ErrorDetail = ""
	journal.UpdatedAt = now
	return nil
}

func (s *UserDataSnapshotStore) verifyCompletedJournalAgainstManifest(
	ctx context.Context,
	journal *UserDataRestoreJournal,
	manifest UserDataTableManifest,
	expectedNamespaceHash string,
) error {
	if journal == nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed restore journal missing"))
	}
	if journal.State != UserDataRestoreCompleted {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: journal for table %s not completed: state=%s", journal.TableName, journal.State))
	}
	if journal.ErrorDetail != "" {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: journal for table %s has error: %s", journal.TableName, journal.ErrorDetail))
	}
	if journal.NamespaceHash == "" || (manifest.NamespaceHash != "" && journal.NamespaceHash != manifest.NamespaceHash) {
		return NewPackageError(PackageErrCodeUserDataJournalHashMismatch, 422,
			fmt.Errorf("kernel: journal namespace hash mismatch for table %s: manifest=%s journal=%s", journal.TableName, manifest.NamespaceHash, journal.NamespaceHash))
	}
	if journal.DataExportReference == "" || journal.DataExportReference != manifest.DataExportReference {
		return NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
			fmt.Errorf("kernel: completed journal export identity mismatch for table %s", journal.TableName))
	}
	if journal.BatchAlgorithmVersion != userDataBatchHashAlgorithmVersion {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: journal batch algorithm version mismatch for table %s: expected=%s actual=%s", journal.TableName, userDataBatchHashAlgorithmVersion, journal.BatchAlgorithmVersion))
	}
	if journal.TotalRows != manifest.RecordCount {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table %s journal total_rows %d != manifest record_count %d", journal.TableName, journal.TotalRows, manifest.RecordCount))
	}
	if journal.ImportedRows != manifest.RecordCount || journal.AppliedCount != manifest.RecordCount {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table %s completed row counts differ: manifest=%d imported=%d applied=%d", journal.TableName, manifest.RecordCount, journal.ImportedRows, journal.AppliedCount))
	}
	expectedCursor := strconv.FormatInt(manifest.RecordCount, 10)
	if journal.Cursor != expectedCursor {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: table %s cursor %s != expected %s", journal.TableName, journal.Cursor, expectedCursor))
	}
	var databaseCount int64
	if err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(journal.TableName)),
	).Scan(&databaseCount); err != nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: count completed restore table %s: %w", journal.TableName, err))
	}
	if databaseCount != manifest.RecordCount {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: completed database row count mismatch for table %s: manifest=%d database=%d", journal.TableName, manifest.RecordCount, databaseCount))
	}
	if journal.GenesisHash == "" || journal.GenesisHash != manifest.GenesisHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completed journal genesis mismatch for table %s", journal.TableName))
	}
	if journal.ExpectedFinalBatchHash == "" || journal.ExpectedFinalBatchHash != manifest.FinalBatchHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completed journal expected final hash mismatch for table %s", journal.TableName))
	}
	if journal.BatchHash == "" || journal.BatchHash != manifest.FinalBatchHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completed journal final batch hash mismatch for table %s: journal=%s manifest=%s", journal.TableName, journal.BatchHash, manifest.FinalBatchHash))
	}
	expectedBatchCount := int64(0)
	if manifest.RecordCount > 0 {
		expectedBatchCount = (manifest.RecordCount + int64(manifest.BatchSize) - 1) / int64(manifest.BatchSize)
	}
	if journal.BatchIndex != expectedBatchCount {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: completed journal batch count mismatch for table %s: journal=%d expected=%d", journal.TableName, journal.BatchIndex, expectedBatchCount))
	}
	return s.verifyAggregateHashClosure(ctx, journal, manifest)
}

func (s *UserDataSnapshotStore) VerifyUserDataRestore(ctx context.Context, operationID string) error {
	if s.db == nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: user data snapshot store database unavailable"))
	}
	if err := s.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("kernel: ensure user data restore journal schema: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT table_name,
       state,
       imported_rows,
       total_rows,
       applied_count,
       cursor,
       error_detail,
       namespace_hash,
       expected_aggregate_hash,
       aggregate_hash,
       batch_hash,
       extension_id,
       batch_index,
       batch_algorithm_version,
       batch_size,
       data_export_reference,
       genesis_hash,
       expected_final_batch_hash
 FROM extension_package_user_data_restore_journal
 WHERE operation_id=?`, operationID)
	if err != nil {
		return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
			fmt.Errorf("kernel: query restore journals for operation %s: %w", operationID, err))
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var table, state, errorDetail, cursor, namespaceHash, expectedAggregateHash, aggregateHash, batchHash, extensionID, batchAlgoVer, dataExportRef, genesisHash, expectedFinalBatchHash string
		var importedRows, totalRows, appliedCount, batchIndex, batchSize int64
		if err := rows.Scan(&table, &state, &importedRows, &totalRows, &appliedCount, &cursor, &errorDetail,
			&namespaceHash, &expectedAggregateHash, &aggregateHash, &batchHash, &extensionID, &batchIndex, &batchAlgoVer, &batchSize, &dataExportRef, &genesisHash, &expectedFinalBatchHash); err != nil {
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
		if dataExportRef == "" {
			return NewPackageError(PackageErrCodeUserDataSnapshotInvalid, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty data_export_reference", table))
		}
		if genesisHash == "" {
			return NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty genesis hash", table))
		}
		if expectedFinalBatchHash == "" {
			return NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty expected final batch hash", table))
		}
		if totalRows > 0 && batchHash == "" {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty batch hash but %d rows were imported", table, totalRows))
		}
		if batchHash != expectedFinalBatchHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
				fmt.Errorf("kernel: restore journal final batch mismatch for table %s: expected=%s actual=%s", table, expectedFinalBatchHash, batchHash))
		}
		if totalRows == 0 && batchHash != genesisHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
				fmt.Errorf("kernel: empty table %s final batch hash differs from genesis", table))
		}
		if totalRows > 0 && batchHash == genesisHash {
			return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 422,
				fmt.Errorf("kernel: non-empty table %s final batch hash equals genesis", table))
		}
		if batchAlgoVer != userDataBatchHashAlgorithmVersion {
			return NewPackageError(PackageErrCodeSnapshotSchemaUnsupported, 422,
				fmt.Errorf("kernel: restore journal for table %s uses unsupported batch algorithm version: got %s expected %s", table, batchAlgoVer, userDataBatchHashAlgorithmVersion))
		}
		if expectedAggregateHash == "" {
			return NewPackageError(PackageErrCodeUserDataJournalLegacyEmpty, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty expected aggregate hash", table))
		}
		if err := validateUserDataAggregateHashFormat(expectedAggregateHash); err != nil {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: restore journal expected aggregate hash invalid for table %s: %w", table, err))
		}
		if aggregateHash == "" {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: restore journal for table %s has empty aggregate hash", table))
		}
		if expectedAggregateHash != aggregateHash {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: restore journal aggregate hash mismatch for table %s: expected=%s observed=%s", table, expectedAggregateHash, aggregateHash))
		}
		actualAggregateHash, err := s.computeAggregateHashFromDB(ctx, table)
		if err != nil {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: recompute aggregate hash for table %s: %w", table, err))
		}
		if actualAggregateHash != expectedAggregateHash || actualAggregateHash != aggregateHash {
			return NewPackageError(PackageErrCodeUserDataAggregateHashMismatch, 422,
				fmt.Errorf("kernel: restore aggregate hash mismatch for table %s: expected=%s observed=%s database=%s", table, expectedAggregateHash, aggregateHash, actualAggregateHash))
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

func computeContentBoundBatchHash(
	batch []map[string]interface{},
	spec UserDataBatchChainSpec,
	cursorBefore int64,
	previousBatchHash string,
	batchIndex int64,
) (string, error) {
	if err := validateUserDataBatchChainSpec(spec); err != nil {
		return "", err
	}
	if len(batch) == 0 {
		return "", fmt.Errorf("kernel: empty batch cannot produce a batch hash")
	}
	if cursorBefore < 0 {
		return "", fmt.Errorf("kernel: batch cursor before cannot be negative")
	}
	if batchIndex <= 0 {
		return "", fmt.Errorf("kernel: batch index must start at one")
	}
	if strings.TrimSpace(previousBatchHash) == "" {
		return "", fmt.Errorf("kernel: previous batch hash missing")
	}
	if batchIndex == 1 && previousBatchHash != spec.GenesisHash {
		return "", fmt.Errorf("kernel: first batch must start from manifest genesis hash")
	}
	recordDigests := make([]userDataBatchRecordDigest, 0, len(batch))
	for index, record := range batch {
		raw, exists := record["_raw"]
		if !exists {
			return "", fmt.Errorf("kernel: batch record %d raw JSON missing", index)
		}
		rawString, ok := raw.(string)
		if !ok || strings.TrimSpace(rawString) == "" {
			return "", fmt.Errorf("kernel: batch record %d raw JSON invalid", index)
		}
		var parsed userDataRecord
		if err := json.Unmarshal([]byte(rawString), &parsed); err != nil {
			return "", fmt.Errorf("kernel: parse batch record %d: %w", index, err)
		}
		if err := validateUserDataRecord(parsed, spec.Identity.ExtensionID); err != nil {
			return "", fmt.Errorf("kernel: validate batch record %d: %w", index, err)
		}
		if parsed.Namespace != spec.Identity.CanonicalTable {
			return "", fmt.Errorf("kernel: batch record %d namespace mismatch", index)
		}
		if parsed.EntityType != spec.Identity.EntityType {
			return "", fmt.Errorf("kernel: batch record %d entity type mismatch", index)
		}
		if parsed.SchemaVersion != spec.Identity.SchemaVersion {
			return "", fmt.Errorf("kernel: batch record %d schema version mismatch", index)
		}
		rawDigest := sha256.Sum256([]byte(rawString))
		recordDigests = append(recordDigests, userDataBatchRecordDigest{
			Index:       index,
			RawJSONHash: "sha256:" + hex.EncodeToString(rawDigest[:]),
			ExtensionID: parsed.ExtensionID,
			Namespace:   parsed.Namespace,
			EntityType:  parsed.EntityType,
			EntityID:    parsed.EntityID,
			Operation:   parsed.Operation,
			PayloadHash: parsed.PayloadHash,
		})
	}
	payload := userDataBatchHashPayload{
		Domain:                userDataBatchHashDomain,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
		TableIdentity:         spec.Identity,
		DataExportReference:   spec.DataExportReference,
		BatchIndex:            batchIndex,
		CursorBefore:          cursorBefore,
		CursorAfter:           cursorBefore + int64(len(batch)),
		PreviousBatchHash:     previousBatchHash,
		Records:               recordDigests,
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("kernel: marshal batch hash payload: %w", err)
	}
	sum := sha256.Sum256(rawPayload)
	return "batch:sha256:" + hex.EncodeToString(sum[:]), nil
}

func recalculateBatchHashChain(
	records []map[string]interface{},
	spec UserDataBatchChainSpec,
) (UserDataBatchChainResult, error) {
	var result UserDataBatchChainResult
	if err := validateUserDataBatchChainSpec(spec); err != nil {
		return result, err
	}
	result.GenesisHash = spec.GenesisHash
	result.FinalHash = spec.GenesisHash
	result.RecordCount = int64(len(records))
	if len(records) == 0 {
		return result, nil
	}
	previousHash := spec.GenesisHash
	totalRecords := int64(len(records))
	for cursor := int64(0); cursor < totalRecords; cursor += spec.BatchSize {
		end := cursor + spec.BatchSize
		if end > totalRecords {
			end = totalRecords
		}
		batch := records[int(cursor):int(end)]
		batchIndex := result.BatchCount + 1
		batchHash, err := computeContentBoundBatchHash(batch, spec, cursor, previousHash, batchIndex)
		if err != nil {
			return UserDataBatchChainResult{}, fmt.Errorf("kernel: compute batch %d at cursor %d: %w", batchIndex, cursor, err)
		}
		result.PreviousHash = previousHash
		result.FinalHash = batchHash
		result.BatchCount = batchIndex
		previousHash = batchHash
	}
	return result, nil
}

func verifyRestoreJournalBatchPrefix(
	journal *UserDataRestoreJournal,
	records []map[string]interface{},
	spec UserDataBatchChainSpec,
) error {
	if journal == nil {
		return fmt.Errorf("kernel: restore journal missing")
	}
	cursor, err := parseJournalCursor(journal, int64(len(records)))
	if err != nil {
		return err
	}
	if cursor < int64(len(records)) && cursor%spec.BatchSize != 0 {
		return NewPackageError(PackageErrCodeUserDataRestoreJournalConflict, 409,
			fmt.Errorf("kernel: journal cursor %d is not aligned to batch size %d", cursor, spec.BatchSize))
	}
	prefix := records[:int(cursor)]
	result, err := recalculateBatchHashChain(prefix, spec)
	if err != nil {
		return err
	}
	if journal.BatchIndex != result.BatchCount {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: journal batch index mismatch: journal=%d recalculated=%d", journal.BatchIndex, result.BatchCount))
	}
	if journal.BatchHash == "" || journal.BatchHash != result.FinalHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: journal batch hash mismatch at cursor %d: journal=%s recalculated=%s", cursor, journal.BatchHash, result.FinalHash))
	}
	if journal.PrevBatchHash != result.PreviousHash {
		return NewPackageError(PackageErrCodeUserDataBatchHashMismatch, 409,
			fmt.Errorf("kernel: journal previous batch hash mismatch at cursor %d: journal=%s recalculated=%s", cursor, journal.PrevBatchHash, result.PreviousHash))
	}
	return nil
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
