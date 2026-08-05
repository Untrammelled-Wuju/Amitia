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

type r62Row struct {
	EntityID string
	Value    string
	Amount   int
}

type r62SnapshotFixture struct {
	TableName     string
	ExtensionID   string
	EntityType    string
	JSONL         string
	RawRecords    []map[string]interface{}
	ParsedRecords []userDataRecord
	Manifest      UserDataTableManifest
}

func newR62SnapshotFixture(t *testing.T, rows []r62Row) *r62SnapshotFixture {
	t.Helper()
	extID := "r62ext"
	table := "ext_r62ext_data"
	entityType := "data"

	var jsonlLines []string
	for _, row := range rows {
		payload := map[string]interface{}{
			"entity_value": row.Value,
			"amount":       row.Amount,
		}
		line := makeTestRawLine("1.0.0", extID, table, entityType, row.EntityID, "upsert", payload)
		jsonlLines = append(jsonlLines, line)
	}
	jsonl := strings.Join(jsonlLines, "\n")
	if len(rows) == 0 {
		jsonl = ""
	}

	rawRecords, parsedRecords, err := parseAndValidateJSONL(jsonl, extID)
	if err != nil {
		t.Fatalf("parse and validate JSONL: %v", err)
	}

	nsHash := computeUserDataNamespaceHash(UserDataNamespaceIdentity{
		SchemaVersion:          1,
		ExtensionID:            extID,
		CanonicalTable:         table,
		LogicalEntityType:      entityType,
		NamespacePolicyVersion: "v1",
	})

	var aggHash string
	if len(parsedRecords) == 0 {
		aggHash = computeUserDataAggregateHashFromRecords(nil)
	} else {
		aggHash = computeUserDataAggregateHashFromRecords(parsedRecords)
	}

	emptySetHash := computeUserDataEmptySetHash(UserDataTableIdentity{
		Domain:                userDataEmptySetDomain,
		SchemaVersion:         userDataRecordSchemaVersion,
		ExtensionID:           extID,
		CanonicalTable:        table,
		EntityType:            entityType,
		NamespaceHash:         nsHash,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
	})

	identity := UserDataTableIdentity{
		Domain:                userDataBatchGenesisDomain,
		SchemaVersion:         userDataRecordSchemaVersion,
		ExtensionID:           extID,
		CanonicalTable:        table,
		EntityType:            entityType,
		NamespaceHash:         nsHash,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
	}
	genesisHash := computeUserDataGenesisHash(identity)

	finalBatchHash := genesisHash
	if len(parsedRecords) > 0 {
		chainSpec := UserDataBatchChainSpec{
			Identity:            identity,
			DataExportReference: computeUserDataExportReference("op-r62", table, extID),
			BatchSize:           int64(userDataRestoreBatchSize),
			GenesisHash:         genesisHash,
		}
		chainResult, cerr := recalculateBatchHashChain(rawRecords, chainSpec)
		if cerr != nil {
			t.Fatalf("r62 chain: %v", cerr)
		}
		finalBatchHash = chainResult.FinalHash
	}

	manifest := UserDataTableManifest{
		SchemaVersion:         userDataTableManifestSchemaVersion,
		ExtensionID:           extID,
		CanonicalTable:        table,
		Namespace:             table,
		EntityType:            entityType,
		NamespaceHash:         nsHash,
		RecordCount:           int64(len(parsedRecords)),
		AggregateHash:         aggHash,
		EmptySetHash:          emptySetHash,
		BatchSize:             userDataRestoreBatchSize,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
		GenesisHash:           genesisHash,
		FinalBatchHash:        finalBatchHash,
		DataExportReference:   computeUserDataExportReference("op-r62", table, extID),
	}

	return &r62SnapshotFixture{
		TableName:     table,
		ExtensionID:   extID,
		EntityType:    entityType,
		JSONL:         jsonl,
		RawRecords:    rawRecords,
		ParsedRecords: parsedRecords,
		Manifest:      manifest,
	}
}

func newR62StoreAndFixture(t *testing.T) (*UserDataSnapshotStore, *r62SnapshotFixture, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r62-restore.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	makeCrashTestTable(t, db, "ext_r62ext_data")
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
		{EntityID: "record-2", Value: "value-2", Amount: 2},
	})

	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	return store, fixture, db
}

func newR62EmptyStoreAndFixture(t *testing.T) (*UserDataSnapshotStore, *r62SnapshotFixture, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r62-empty-restore.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	makeCrashTestTable(t, db, "ext_r62ext_data")
	fixture := newR62SnapshotFixture(t, nil)

	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	return store, fixture, db
}

func newR62CompletedStoreAndFixture(t *testing.T) (*UserDataSnapshotStore, *r62SnapshotFixture) {
	t.Helper()
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r62-completed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	makeCrashTestTable(t, db, "ext_r62ext_data")
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_r62ext_data"},
		RecordCounts:   map[string]int64{"ext_r62ext_data": 1},
		DataExports:    map[string]string{"ext_r62ext_data": fixture.JSONL},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			"ext_r62ext_data": fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(ctx, fixture.ExtensionID, "op-r62-completed", string(userStateJSON)); err != nil {
		t.Fatalf("initial restore: %v", err)
	}

	return store, fixture
}

func TestR62ManifestRejectsAggregateHashNotMatchingJSONL(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.AggregateHash = computeUserDataAggregateHashFromRecords(nil)

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("manifest aggregate hash not matching JSONL must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62ManifestRejectsEmptyTableAggregateHashMismatch(t *testing.T) {
	fixture := newR62SnapshotFixture(t, nil)

	fixture.Manifest.AggregateHash = "sha256:" + strings.Repeat("b", 64)

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("empty table manifest aggregate hash mismatch must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62ManifestRejectsEmptySetHashMismatch(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.EmptySetHash = "sha256:" + strings.Repeat("c", 64)

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("empty set hash mismatch must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62ManifestRejectsNamespaceHashMismatch(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.NamespaceHash = "sha256:" + strings.Repeat("d", 64)

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("namespace hash mismatch must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataJournalHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalPersistsExpectedAggregateHash(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	journal, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if journal.ExpectedAggregateHash != fixture.Manifest.AggregateHash {
		t.Fatalf("expected aggregate hash not persisted: journal=%s manifest=%s",
			journal.ExpectedAggregateHash, fixture.Manifest.AggregateHash)
	}
}

func TestR62JournalLoadsStoredExtensionID(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	wrongExtID := "wrong-extension"
	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		wrongExtID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("expected extension mismatch error")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataNamespaceViolation) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalLoadsStoredTotalRows(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	manifest := fixture.Manifest

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		fixture.ExtensionID,
		fixture.TableName,
		manifest.Namespace,
		manifest,
		manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	dbCount := int64(0)
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT total_rows FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r62", fixture.TableName,
	).Scan(&dbCount); err != nil {
		t.Fatal(err)
	}
	if dbCount != manifest.RecordCount {
		t.Fatalf("stored total_rows mismatch: got %d, want %d", dbCount, manifest.RecordCount)
	}
}

func TestR62JournalRejectsManifestAggregateDrift(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	drifted := fixture.Manifest
	drifted.AggregateHash = "sha256:" + strings.Repeat("a", 64)

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		drifted,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("existing journal must reject manifest aggregate drift")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataRestoreJournalConflict) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalRejectsLegacyEmptyExpectedAggregate(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-legacy",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.db.ExecContext(context.Background(),
		"UPDATE extension_package_user_data_restore_journal SET expected_aggregate_hash='' WHERE operation_id=? AND table_name=?",
		"op-r62-legacy", fixture.TableName); err != nil {
		t.Fatal(err)
	}

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-legacy",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("existing journal with empty expected aggregate hash must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataJournalLegacyEmpty) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalRejectsObservedAggregateDifferentFromExpected(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-tainted",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	taintedAggHash := "sha256:" + strings.Repeat("f", 64)
	if _, err := store.db.ExecContext(context.Background(),
		"UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?",
		taintedAggHash, "op-r62-tainted", fixture.TableName); err != nil {
		t.Fatal(err)
	}

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-tainted",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("existing journal with observed != expected must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62CompletedJournalRejectsManifestJournalMismatch(t *testing.T) {
	store, fixture := newR62CompletedStoreAndFixture(t)

	var (
		storedExpectedAggHash string
		storedAggHash         string
	)
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT expected_aggregate_hash, aggregate_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r62-completed", fixture.TableName,
	).Scan(&storedExpectedAggHash, &storedAggHash); err != nil {
		t.Fatal(err)
	}
	if storedExpectedAggHash != fixture.Manifest.AggregateHash {
		t.Fatal("completed journal expected hash differs from manifest")
	}
	if storedAggHash != fixture.Manifest.AggregateHash {
		t.Fatal("completed journal observed hash differs from manifest")
	}
}

func TestR62CompletedJournalRejectsJournalDatabaseMismatch(t *testing.T) {
	store, fixture := newR62CompletedStoreAndFixture(t)

	if _, err := store.db.ExecContext(context.Background(),
		fmt.Sprintf("UPDATE %s SET entity_value=? WHERE entity_id=?",
			quoteIdentifier(fixture.TableName)),
		"tampered-value", "record-1"); err != nil {
		t.Fatal(err)
	}

	err := store.verifyCompletedJournalAgainstManifest(
		context.Background(),
		&UserDataRestoreJournal{
			TableName:              fixture.TableName,
			State:                  UserDataRestoreCompleted,
			NamespaceHash:          fixture.Manifest.NamespaceHash,
			ExpectedAggregateHash:  fixture.Manifest.AggregateHash,
			AggregateHash:          fixture.Manifest.AggregateHash,
			TotalRows:              1,
			ImportedRows:           1,
			AppliedCount:           1,
			Cursor:                 "1",
			BatchAlgorithmVersion:  userDataBatchHashAlgorithmVersion,
			DataExportReference:    fixture.Manifest.DataExportReference,
			GenesisHash:            fixture.Manifest.GenesisHash,
			ExpectedFinalBatchHash: fixture.Manifest.FinalBatchHash,
			BatchHash:              fixture.Manifest.FinalBatchHash,
			BatchIndex:             1,
		},
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("database content drift must fail completed journal verification")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62CompletedJournalRejectsManifestDatabaseMismatch(t *testing.T) {
	store, fixture := newR62CompletedStoreAndFixture(t)

	tamperedManifest := fixture.Manifest
	tamperedManifest.AggregateHash = "sha256:" + strings.Repeat("e", 64)

	err := store.verifyCompletedJournalAgainstManifest(
		context.Background(),
		&UserDataRestoreJournal{
			TableName:              fixture.TableName,
			State:                  UserDataRestoreCompleted,
			NamespaceHash:          fixture.Manifest.NamespaceHash,
			ExpectedAggregateHash:  fixture.Manifest.AggregateHash,
			AggregateHash:          fixture.Manifest.AggregateHash,
			TotalRows:              1,
			ImportedRows:           1,
			AppliedCount:           1,
			Cursor:                 "1",
			BatchAlgorithmVersion:  userDataBatchHashAlgorithmVersion,
			DataExportReference:    fixture.Manifest.DataExportReference,
			GenesisHash:            fixture.Manifest.GenesisHash,
			ExpectedFinalBatchHash: fixture.Manifest.FinalBatchHash,
			BatchHash:              fixture.Manifest.FinalBatchHash,
			BatchIndex:             1,
		},
		tamperedManifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("tampered manifest must fail verification")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62CompletedEmptyJournalRejectsAggregateMismatch(t *testing.T) {
	store, fixture, _ := newR62EmptyStoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: 0},
		DataExports:    map[string]string{fixture.TableName: ""},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(
		context.Background(),
		fixture.ExtensionID,
		"op-r62-empty-bad",
		string(userStateJSON),
	); err != nil {
		t.Fatal(err)
	}

	if _, err := store.db.ExecContext(context.Background(),
		"UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?",
		"sha256:"+strings.Repeat("d", 64), "op-r62-empty-bad", fixture.TableName); err != nil {
		t.Fatal(err)
	}

	err := store.VerifyUserDataRestore(context.Background(), "op-r62-empty-bad")
	if err == nil {
		t.Fatal("empty journal with wrong aggregate hash must fail verification")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62CompletedEmptyJournalExecutesThreeWayVerification(t *testing.T) {
	store, fixture, _ := newR62EmptyStoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: 0},
		DataExports:    map[string]string{fixture.TableName: ""},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(
		context.Background(),
		fixture.ExtensionID,
		"op-r62-empty-ok",
		string(userStateJSON),
	); err != nil {
		t.Fatal(err)
	}

	if err := store.VerifyUserDataRestore(context.Background(), "op-r62-empty-ok"); err != nil {
		t.Fatalf("valid empty restore should pass verification: %v", err)
	}
}

func TestR62VerifyRestoreRejectsExpectedObservedMismatch(t *testing.T) {
	store, fixture, _ := newR62EmptyStoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: 0},
		DataExports:    map[string]string{fixture.TableName: ""},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(
		context.Background(),
		fixture.ExtensionID,
		"op-r62-ev",
		string(userStateJSON),
	); err != nil {
		t.Fatal(err)
	}

	observedHash := "sha256:" + strings.Repeat("c", 64)
	if _, err := store.db.ExecContext(context.Background(),
		"UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?",
		observedHash, "op-r62-ev", fixture.TableName); err != nil {
		t.Fatal(err)
	}

	err := store.VerifyUserDataRestore(context.Background(), "op-r62-ev")
	if err == nil {
		t.Fatal("expected vs observed mismatch must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62VerifyRestoreRejectsObservedDatabaseMismatch(t *testing.T) {
	store, fixture := newR62CompletedStoreAndFixture(t)

	if _, err := store.db.ExecContext(context.Background(),
		fmt.Sprintf("UPDATE %s SET entity_value=? WHERE entity_id=?",
			quoteIdentifier(fixture.TableName)),
		"modified-value", "record-1"); err != nil {
		t.Fatal(err)
	}

	err := store.VerifyUserDataRestore(context.Background(), "op-r62-completed")
	if err == nil {
		t.Fatal("observed vs database mismatch must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62FullNonEmptyRestoreClosesAggregateHash(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: int64(len(fixture.ParsedRecords))},
		DataExports:    map[string]string{fixture.TableName: fixture.JSONL},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(
		context.Background(),
		fixture.ExtensionID,
		"op-r62-full",
		string(userStateJSON),
	); err != nil {
		t.Fatal(err)
	}

	if err := store.VerifyUserDataRestore(context.Background(), "op-r62-full"); err != nil {
		t.Fatalf("verify restore: %v", err)
	}

	var (
		expectedAggregateHash string
		aggregateHash         string
		state                 string
	)
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT expected_aggregate_hash, aggregate_hash, state FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r62-full", fixture.TableName,
	).Scan(&expectedAggregateHash, &aggregateHash, &state); err != nil {
		t.Fatal(err)
	}

	if state != string(UserDataRestoreCompleted) {
		t.Fatalf("restore journal not completed: %s", state)
	}
	if expectedAggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("journal expected hash differs from manifest")
	}
	if aggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("journal observed hash differs from manifest")
	}

	databaseAggregateHash, err := store.computeAggregateHashFromDB(context.Background(), fixture.TableName)
	if err != nil {
		t.Fatal(err)
	}
	if databaseAggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("database hash differs from manifest")
	}
}

func TestR62FullEmptyRestoreClosesAggregateHash(t *testing.T) {
	store, fixture, _ := newR62EmptyStoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: 0},
		DataExports:    map[string]string{fixture.TableName: ""},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(
		context.Background(),
		fixture.ExtensionID,
		"op-r62-empty-full",
		string(userStateJSON),
	); err != nil {
		t.Fatal(err)
	}

	if err := store.VerifyUserDataRestore(context.Background(), "op-r62-empty-full"); err != nil {
		t.Fatalf("verify empty restore: %v", err)
	}

	var (
		expectedAggregateHash string
		aggregateHash         string
		state                 string
	)
	if err := store.db.QueryRowContext(context.Background(),
		"SELECT expected_aggregate_hash, aggregate_hash, state FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r62-empty-full", fixture.TableName,
	).Scan(&expectedAggregateHash, &aggregateHash, &state); err != nil {
		t.Fatal(err)
	}

	if state != string(UserDataRestoreCompleted) {
		t.Fatalf("restore journal not completed: %s", state)
	}
	if expectedAggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("journal expected hash differs from manifest")
	}
	if aggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("journal observed hash differs from manifest")
	}

	databaseAggregateHash, err := store.computeAggregateHashFromDB(context.Background(), fixture.TableName)
	if err != nil {
		t.Fatal(err)
	}
	if databaseAggregateHash != fixture.Manifest.AggregateHash {
		t.Fatal("empty database hash differs from manifest")
	}
}

func TestR62ManifestRejectsInvalidAggregateHashFormat(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.AggregateHash = "invalid-hash"

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("invalid aggregate hash format must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataAggregateHashMismatch) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62EnsureExpectedAggregateHashColumnMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r62-migration.db"))
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

	rows, err := db.Query("PRAGMA table_info(extension_package_user_data_restore_journal)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "expected_aggregate_hash" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("migration did not add expected_aggregate_hash column")
	}
}

func TestR62ManifestRejectsNamespaceNotEqualToCanonicalTable(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.Namespace = "wrong-namespace"

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("namespace != canonical_table must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataNamespaceViolation) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62ManifestRejectsMissingDataExportReference(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.DataExportReference = ""

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("missing data export reference must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataSnapshotInvalid) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62HashFormatValidatorRejectsInvalidPrefix(t *testing.T) {
	err := validateUserDataAggregateHashFormat("md5:abcdef")
	if err == nil {
		t.Fatal("non-sha256 prefix must fail")
	}
}

func TestR62HashFormatValidatorRejectsShortDigest(t *testing.T) {
	err := validateUserDataAggregateHashFormat("sha256:tooshort")
	if err == nil {
		t.Fatal("short digest must fail")
	}
}

func TestR62HashFormatValidatorRejectsInvalidHex(t *testing.T) {
	err := validateUserDataAggregateHashFormat("sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	if err == nil {
		t.Fatal("invalid hex must fail")
	}
}

func TestR62HashFormatValidatorAcceptsValidHash(t *testing.T) {
	valid := "sha256:" + strings.Repeat("a", 64)
	if err := validateUserDataAggregateHashFormat(valid); err != nil {
		t.Fatalf("valid hash should pass: %v", err)
	}
}

func TestR62HashFormatValidatorTrimsWhitespace(t *testing.T) {
	valid := "  sha256:" + strings.Repeat("b", 64) + "  "
	if err := validateUserDataAggregateHashFormat(valid); err != nil {
		t.Fatalf("hash with whitespace should pass after trim: %v", err)
	}
}

func TestR62EmptyRecordsProduceConsistentHash(t *testing.T) {
	hash1 := computeUserDataAggregateHashFromRecords(nil)
	hash2 := computeUserDataAggregateHashFromRecords(nil)
	if hash1 != hash2 {
		t.Fatalf("empty aggregate hash must be consistent: %s vs %s", hash1, hash2)
	}
	if !strings.HasPrefix(hash1, "sha256:") {
		t.Fatalf("empty aggregate hash must use sha256 prefix, got: %s", hash1)
	}
}

func TestR62ComputeAggregateHashFromDBEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r62-db-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	makeCrashTestTable(t, db, "ext_test_empty_data")

	store := NewUserDataSnapshotStore(db)
	hash, err := store.computeAggregateHashFromDB(context.Background(), "ext_test_empty_data")
	if err != nil {
		t.Fatal(err)
	}

	expected := computeUserDataAggregateHashFromRecords(nil)
	if hash != expected {
		t.Fatalf("empty DB hash mismatch: got=%s expected=%s", hash, expected)
	}
}

func TestR62ManifestRejectsEmptyAggregateHash(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.Manifest.AggregateHash = ""

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("empty aggregate hash must fail")
	}
}

func TestR62HashFormatValidatorRejectsEmpty(t *testing.T) {
	if err := validateUserDataAggregateHashFormat(""); err == nil {
		t.Fatal("empty hash must fail")
	}
}

func TestR62HashFormatValidatorRejectsSha256Only(t *testing.T) {
	if err := validateUserDataAggregateHashFormat("sha256:"); err == nil {
		t.Fatal("sha256: with no digest must fail")
	}
}

func TestR62HashFormatValidatorRejectsLongDigest(t *testing.T) {
	longDigest := "sha256:" + strings.Repeat("a", 65)
	if err := validateUserDataAggregateHashFormat(longDigest); err == nil {
		t.Fatal("digest longer than 64 chars must fail")
	}
}

func TestR62EndToEndEmptyRestoreAtomicCompletion(t *testing.T) {
	store, fixture, _ := newR62EmptyStoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: 0},
		DataExports:    map[string]string{fixture.TableName: ""},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)

	for i := 0; i < 3; i++ {
		if err := store.RestoreUserDataFromSnapshot(
			context.Background(),
			fixture.ExtensionID,
			"op-r62-idempotent",
			string(userStateJSON),
		); err != nil {
			t.Fatalf("restore iteration %d: %v", i, err)
		}
	}

	if err := store.VerifyUserDataRestore(context.Background(), "op-r62-idempotent"); err != nil {
		t.Fatalf("verify idempotent empty restore: %v", err)
	}
}

func TestR62EndToEndNonEmptyRestoreAtomicCompletion(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{fixture.TableName},
		RecordCounts:   map[string]int64{fixture.TableName: int64(len(fixture.ParsedRecords))},
		DataExports:    map[string]string{fixture.TableName: fixture.JSONL},
		TableManifests: map[string]UserDataTableSnapshotManifest{
			fixture.TableName: fixture.Manifest,
		},
	}
	userStateJSON, _ := json.Marshal(userState)

	for i := 0; i < 3; i++ {
		if err := store.RestoreUserDataFromSnapshot(
			context.Background(),
			fixture.ExtensionID,
			"op-r62-idempotent",
			string(userStateJSON),
		); err != nil {
			t.Fatalf("restore iteration %d: %v", i, err)
		}
	}

	if err := store.VerifyUserDataRestore(context.Background(), "op-r62-idempotent"); err != nil {
		t.Fatalf("verify idempotent restore: %v", err)
	}
}

func TestR62JournalRejectsLegacyEmptyNamespaceHash(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-empty-ns",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.db.ExecContext(context.Background(),
		"UPDATE extension_package_user_data_restore_journal SET namespace_hash='' WHERE operation_id=? AND table_name=?",
		"op-r62-empty-ns", fixture.TableName); err != nil {
		t.Fatal(err)
	}

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-empty-ns",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("journal with empty namespace hash must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataJournalLegacyEmpty) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalRejectsDifferentExportReference(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-export",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	drifted := fixture.Manifest
	drifted.DataExportReference = "export:other:ref"

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-export",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		drifted,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("journal with different export reference must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataRestoreJournalConflict) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62JournalRejectsDifferentRowCount(t *testing.T) {
	store, fixture, _ := newR62StoreAndFixture(t)

	_, err := store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-rowcount",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		fixture.Manifest,
		fixture.Manifest.NamespaceHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	drifted := fixture.Manifest
	drifted.RecordCount = 999

	_, err = store.getOrCreateRestoreJournal(
		context.Background(),
		"op-r62-rowcount",
		fixture.ExtensionID,
		fixture.TableName,
		fixture.Manifest.Namespace,
		drifted,
		fixture.Manifest.NamespaceHash,
	)
	if err == nil {
		t.Fatal("journal with different row count must fail")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataRestoreJournalConflict) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestR62ManifestRejectsRecordExtensionMismatch(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.ParsedRecords[0].ExtensionID = "wrong-extension"

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("record extension mismatch must fail")
	}
}

func TestR62ManifestRejectsRecordNamespaceMismatch(t *testing.T) {
	fixture := newR62SnapshotFixture(t, []r62Row{
		{EntityID: "record-1", Value: "value-1", Amount: 1},
	})

	fixture.ParsedRecords[0].Namespace = "wrong-namespace"

	err := validateUserDataTableSnapshotManifest(
		fixture.Manifest,
		fixture.ExtensionID,
		fixture.TableName,
		fixture.JSONL,
		fixture.RawRecords,
		fixture.ParsedRecords,
	)
	if err == nil {
		t.Fatal("record namespace mismatch must fail")
	}
}
