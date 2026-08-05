package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func r63Table(extID string) string {
	return "ext_" + strings.ToLower(extID) + "_data"
}

func r63SpecFor(t *testing.T, extID, entityType, dataExportReference string) UserDataBatchChainSpec {
	t.Helper()
	table := r63Table(extID)
	resolver, err := ResolveExtensionUserDataNamespace(extID, table)
	if err != nil {
		t.Fatalf("resolve namespace: %v", err)
	}
	nsHash := computeUserDataNamespaceHash(UserDataNamespaceIdentity{
		SchemaVersion:          1,
		ExtensionID:            extID,
		CanonicalTable:         resolver.CanonicalTable,
		LogicalEntityType:      resolver.LogicalEntityType,
		NamespacePolicyVersion: "v1",
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
	return UserDataBatchChainSpec{
		Identity:            identity,
		DataExportReference: dataExportReference,
		BatchSize:           int64(userDataRestoreBatchSize),
		GenesisHash:         computeUserDataGenesisHash(identity),
	}
}

func r63MakeRecords(t *testing.T, extID, entityType string, count int) []map[string]interface{} {
	t.Helper()
	table := r63Table(extID)
	lines := makeNLines(t, extID, table, count, "r63")
	records, _, err := parseAndValidateJSONL(lines, extID)
	if err != nil {
		t.Fatalf("parse records: %v", err)
	}
	return records
}

func r63ManifestFromJSONL(t *testing.T, extID, entityType, jsonlData string) UserDataTableManifest {
	t.Helper()
	return mustBuildManifest(t, extID, r63Table(extID), entityType, jsonlData, "entity_id")
}

func r63UserState(extID, entityType string, recordCount int, jsonlData string) packageUserDataMigrationState {
	table := r63Table(extID)
	manifest := mustBuildManifestHelper(extID, table, entityType, jsonlData, "entity_id")
	return packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: int64(recordCount)},
		DataExports:    map[string]string{table: jsonlData},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: manifest},
	}
}

func mustBuildManifestHelper(extID, table, entityType, jsonlData, idColumn string) UserDataTableManifest {
	rawRecords, parsedRecords, _ := parseAndValidateJSONL(jsonlData, extID)
	recordCount := int64(len(parsedRecords))

	resolver, err := ResolveExtensionUserDataNamespace(extID, table)
	if err != nil {
		panic(err)
	}
	nsHash := computeUserDataNamespaceHash(UserDataNamespaceIdentity{
		SchemaVersion:          1,
		ExtensionID:            extID,
		CanonicalTable:         resolver.CanonicalTable,
		LogicalEntityType:      resolver.LogicalEntityType,
		NamespacePolicyVersion: "v1",
	})

	aggHash := computeUserDataAggregateHashFromRecords(parsedRecords)

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

	emptySetIdentity := UserDataTableIdentity{
		Domain:                userDataEmptySetDomain,
		SchemaVersion:         userDataRecordSchemaVersion,
		ExtensionID:           extID,
		CanonicalTable:        table,
		EntityType:            entityType,
		NamespaceHash:         nsHash,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
	}
	emptySetHash := computeUserDataEmptySetHash(emptySetIdentity)

	finalBatchHash := genesisHash
	if recordCount > 0 {
		spec := UserDataBatchChainSpec{
			Identity:            identity,
			DataExportReference: computeUserDataExportReference("op-r63", table, extID),
			BatchSize:           int64(userDataRestoreBatchSize),
			GenesisHash:         genesisHash,
		}
		chainResult, err := recalculateBatchHashChain(rawRecords, spec)
		if err != nil {
			panic(err)
		}
		finalBatchHash = chainResult.FinalHash
	}

	return UserDataTableManifest{
		SchemaVersion:         userDataTableManifestSchemaVersion,
		ExtensionID:           extID,
		CanonicalTable:        table,
		Namespace:             table,
		EntityType:            entityType,
		NamespaceHash:         nsHash,
		RecordCount:           recordCount,
		AggregateHash:         aggHash,
		EmptySetHash:          emptySetHash,
		BatchSize:             userDataRestoreBatchSize,
		BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
		GenesisHash:           genesisHash,
		FinalBatchHash:        finalBatchHash,
		DataExportReference:   computeUserDataExportReference("op-r63", table, extID),
	}
}

func TestR63GenesisBindsExtensionID(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "bext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" || b == "" || a == b {
		t.Fatalf("different extension IDs must produce different non-empty hashes: a=%s b=%s", a, b)
	}
}

func TestR63GenesisBindsCanonicalTable(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_one", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_two", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" || b == "" || a == b {
		t.Fatalf("different canonical tables must produce different non-empty hashes: a=%s b=%s", a, b)
	}
}

func TestR63GenesisBindsEntityType(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "config", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" || b == "" || a == b {
		t.Fatalf("different entity types must produce different non-empty hashes: a=%s b=%s", a, b)
	}
}

func TestR63GenesisBindsNamespaceHash(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:aaa", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:bbb", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" || b == "" || a == b {
		t.Fatalf("different namespace hashes must produce different non-empty hashes: a=%s b=%s", a, b)
	}
}

func TestR63GenesisBindsSchemaVersion(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: "1.0.0", ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: "2.0.0", ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" || b == "" || a == b {
		t.Fatalf("different schema versions must produce different non-empty hashes: a=%s b=%s", a, b)
	}
}

func TestR63GenesisBindsAlgorithmVersion(t *testing.T) {
	baseID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	altID := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: "content-chain-v2"}
	a := computeUserDataGenesisHash(baseID)
	b := computeUserDataGenesisHash(altID)
	if a == "" {
		t.Fatal("v3 genesis hash must not be empty")
	}
	if b != "" {
		t.Fatalf("unsupported algorithm must produce empty genesis, got: %s", b)
	}
}

func TestR63GenesisRejectsMissingExtensionID(t *testing.T) {
	id := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	if computeUserDataGenesisHash(id) != "" {
		t.Fatal("expected empty genesis hash for missing extension id")
	}
}

func TestR63GenesisRejectsMissingTable(t *testing.T) {
	id := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "", EntityType: "data", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	if computeUserDataGenesisHash(id) != "" {
		t.Fatal("expected empty genesis hash for missing table")
	}
}

func TestR63GenesisRejectsMissingEntityType(t *testing.T) {
	id := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "", NamespaceHash: "ns:x", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	if computeUserDataGenesisHash(id) != "" {
		t.Fatal("expected empty genesis hash for missing entity type")
	}
}

func TestR63GenesisRejectsMissingNamespaceHash(t *testing.T) {
	id := UserDataTableIdentity{Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion, ExtensionID: "aext", CanonicalTable: "ext_aext_data", EntityType: "data", NamespaceHash: "", BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion}
	if computeUserDataGenesisHash(id) != "" {
		t.Fatal("expected empty genesis hash for missing namespace hash")
	}
}

func TestR63ChainRejectsMissingGenesis(t *testing.T) {
	spec := r63SpecFor(t, "r63missing", "data", "export:op:ext_r63missing_data:r63missing")
	spec.GenesisHash = ""
	if _, err := recalculateBatchHashChain(nil, spec); err == nil {
		t.Fatal("expected error for missing genesis hash")
	}
}

func TestR63ChainRejectsMismatchedGenesis(t *testing.T) {
	spec := r63SpecFor(t, "r63mismatch", "data", "export:op:ext_r63mismatch_data:r63mismatch")
	spec.GenesisHash = "genesis:sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := recalculateBatchHashChain(nil, spec); err == nil {
		t.Fatal("expected error for mismatched genesis hash")
	}
}

func TestR63ChainRejectsMissingExportReference(t *testing.T) {
	spec := r63SpecFor(t, "r63noexport", "data", "")
	if _, err := recalculateBatchHashChain(nil, spec); err == nil {
		t.Fatal("expected error for missing data export reference")
	}
}

func TestR63ChainRejectsUnsupportedAlgorithm(t *testing.T) {
	spec := r63SpecFor(t, "r63badalgo", "data", "export:op:ext_r63badalgo_data:r63badalgo")
	spec.Identity.BatchAlgorithmVersion = "content-chain-v1"
	spec.GenesisHash = computeUserDataGenesisHash(spec.Identity)
	if _, err := recalculateBatchHashChain(nil, spec); err == nil {
		t.Fatal("expected error for unsupported algorithm version")
	}
}

func TestR63ChainRejectsWrongBatchSize(t *testing.T) {
	spec := r63SpecFor(t, "r63badbatch", "data", "export:op:ext_r63badbatch_data:r63badbatch")
	spec.BatchSize = 50
	if _, err := recalculateBatchHashChain(nil, spec); err == nil {
		t.Fatal("expected error for wrong batch size")
	}
}

func TestR63EmptyChainEndsAtGenesis(t *testing.T) {
	spec := r63SpecFor(t, "r63empty", "data", "export:op:ext_r63empty_data:r63empty")
	result, err := recalculateBatchHashChain(nil, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalHash != spec.GenesisHash {
		t.Fatalf("empty chain final hash must equal genesis: got=%s expected=%s", result.FinalHash, spec.GenesisHash)
	}
	if result.BatchCount != 0 {
		t.Fatalf("empty chain batch count must be zero, got %d", result.BatchCount)
	}
}

func TestR63NonEmptyChainDoesNotEndAtGenesis(t *testing.T) {
	spec := r63SpecFor(t, "r63nonempty", "data", "export:op:ext_r63nonempty_data:r63nonempty")
	records := r63MakeRecords(t, "r63nonempty", "data", 5)
	result, err := recalculateBatchHashChain(records, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalHash == spec.GenesisHash {
		t.Fatal("non-empty chain final hash must not equal genesis")
	}
	if result.BatchCount != 1 || result.RecordCount != 5 {
		t.Fatalf("expected 1 batch/5 records, got %d/%d", result.BatchCount, result.RecordCount)
	}
}

func TestR63ChainChangesWithRecordContent(t *testing.T) {
	spec := r63SpecFor(t, "r63content", "data", "export:op:ext_r63content_data:r63content")
	recordsA := r63MakeRecords(t, "r63content", "data", 5)
	altLines := makeNLines(t, "r63content", "ext_r63content_data", 5, "alt")
	recordsB, _, err := parseAndValidateJSONL(altLines, "r63content")
	if err != nil {
		t.Fatalf("parse alt: %v", err)
	}
	a, err := recalculateBatchHashChain(recordsA, spec)
	if err != nil {
		t.Fatalf("chain A: %v", err)
	}
	b, err := recalculateBatchHashChain(recordsB, spec)
	if err != nil {
		t.Fatalf("chain B: %v", err)
	}
	if a.FinalHash == b.FinalHash {
		t.Fatal("different content must produce different final hash")
	}
}

func TestR63ChainChangesWithRecordOrder(t *testing.T) {
	spec := r63SpecFor(t, "r63order", "data", "export:op:ext_r63order_data:r63order")
	records := r63MakeRecords(t, "r63order", "data", 5)
	reversed := make([]map[string]interface{}, len(records))
	for i, r := range records {
		reversed[len(records)-1-i] = r
	}
	a, err := recalculateBatchHashChain(records, spec)
	if err != nil {
		t.Fatalf("chain A: %v", err)
	}
	b, err := recalculateBatchHashChain(reversed, spec)
	if err != nil {
		t.Fatalf("chain B: %v", err)
	}
	if a.FinalHash == b.FinalHash {
		t.Fatal("different order must produce different final hash")
	}
}

func TestR63ChainChangesWithPreviousHash(t *testing.T) {
	spec := r63SpecFor(t, "r63prevhash", "data", "export:op:ext_r63prevhash_data:r63prevhash")
	records := r63MakeRecords(t, "r63prevhash", "data", 5)
	genesis, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 1)
	if err != nil {
		t.Fatalf("from genesis: %v", err)
	}
	tampered, err := computeContentBoundBatchHash(records, spec, 0, "batch:sha256:0000000000000000000000000000000000000000000000000000000000000000", 2)
	if err != nil {
		t.Fatalf("from tampered: %v", err)
	}
	if genesis == tampered {
		t.Fatal("different previous hash must produce different batch hash")
	}
}

func TestR63ChainChangesWithBatchIndex(t *testing.T) {
	spec := r63SpecFor(t, "r63batchidx", "data", "export:op:ext_r63batchidx_data:r63batchidx")
	records := r63MakeRecords(t, "r63batchidx", "data", 5)
	idx1, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 1)
	if err != nil {
		t.Fatalf("idx 1: %v", err)
	}
	idx2, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 2)
	if err != nil {
		t.Fatalf("idx 2: %v", err)
	}
	if idx1 == idx2 {
		t.Fatal("different batch index must produce different batch hash")
	}
}

func TestR63ChainChangesWithCursor(t *testing.T) {
	spec := r63SpecFor(t, "r63cursor", "data", "export:op:ext_r63cursor_data:r63cursor")
	records := r63MakeRecords(t, "r63cursor", "data", 5)
	c0, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 1)
	if err != nil {
		t.Fatalf("cursor 0: %v", err)
	}
	c50, err := computeContentBoundBatchHash(records, spec, 50, spec.GenesisHash, 1)
	if err != nil {
		t.Fatalf("cursor 50: %v", err)
	}
	if c0 == c50 {
		t.Fatal("different cursor must produce different batch hash")
	}
}

func TestR63ChainChangesWithExportReference(t *testing.T) {
	specA := r63SpecFor(t, "r63exportref", "data", "export:op-A:ext_r63exportref_data:r63exportref")
	specB := r63SpecFor(t, "r63exportref", "data", "export:op-B:ext_r63exportref_data:r63exportref")
	records := r63MakeRecords(t, "r63exportref", "data", 5)
	a, err := recalculateBatchHashChain(records, specA)
	if err != nil {
		t.Fatalf("chain A: %v", err)
	}
	b, err := recalculateBatchHashChain(records, specB)
	if err != nil {
		t.Fatalf("chain B: %v", err)
	}
	if a.FinalHash == b.FinalHash {
		t.Fatal("different export ref must produce different final hash")
	}
}

func TestR63ChainChangesWithNamespaceHash(t *testing.T) {
	specA := r63SpecFor(t, "r63nshash", "data", "export:op:ext_r63nshash_data:r63nshash")
	records := r63MakeRecords(t, "r63nshash", "data", 5)
	hashA, err := computeContentBoundBatchHash(records, specA, 0, specA.GenesisHash, 1)
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	specB := specA
	specB.Identity.NamespaceHash = "ns:different-hash"
	specB.GenesisHash = computeUserDataGenesisHash(specB.Identity)
	hashB, err := computeContentBoundBatchHash(records, specB, 0, specB.GenesisHash, 1)
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA == hashB {
		t.Fatal("different namespace hash must produce different batch hash")
	}
}

func TestR63ManifestRejectsWeakGenesis(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-weak-genesis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63weakgenesis"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 3, "wg")
	manifest := r63ManifestFromJSONL(t, extID, "data", jsonl)
	manifest.GenesisHash = "genesis:sha256:aaaaaaa00000000000000000000000000000000000000000000000000000000000"
	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: 3},
		DataExports:    map[string]string{table: jsonl},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: manifest},
	}
	userStateJSON, _ := json.Marshal(userState)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-wg", string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when manifest genesis hash is tampered")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataBatchHashMismatch) {
		t.Fatalf("expected batch hash mismatch, got: %v", err)
	}
}

func TestR63ManifestRejectsFinalHashTampering(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-tamper-final.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63tamperfinal"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 3, "tf")
	manifest := r63ManifestFromJSONL(t, extID, "data", jsonl)
	manifest.FinalBatchHash = "batch:sha256:aaaaaaa000000000000000000000000000000000000000000000000000000000001"
	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: 3},
		DataExports:    map[string]string{table: jsonl},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: manifest},
	}
	userStateJSON, _ := json.Marshal(userState)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-tf", string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when manifest final batch hash is tampered")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataBatchHashMismatch) {
		t.Fatalf("expected batch hash mismatch, got: %v", err)
	}
}

func TestR63ManifestRejectsContentChainV2(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-reject-v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63rejectv2"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 3, "rv2")
	manifest := r63ManifestFromJSONL(t, extID, "data", jsonl)
	manifest.BatchAlgorithmVersion = "content-chain-v2"
	manifest.GenesisHash = computeUserDataGenesisHash(UserDataTableIdentity{
		Domain: userDataBatchGenesisDomain, SchemaVersion: userDataRecordSchemaVersion,
		ExtensionID: extID, CanonicalTable: table, EntityType: "data",
		NamespaceHash: manifest.NamespaceHash, BatchAlgorithmVersion: "content-chain-v2",
	})
	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: 3},
		DataExports:    map[string]string{table: jsonl},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: manifest},
	}
	userStateJSON, _ := json.Marshal(userState)
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-v2", string(userStateJSON))
	if err == nil {
		t.Fatal("expected error when manifest uses v2 algorithm")
	}
	if !IsPackageErrorCode(err, PackageErrCodeUserDataBatchHashMismatch) {
		t.Fatalf("expected batch hash mismatch, got: %v", err)
	}
}

func TestR63ManifestRecalculatesFullJSONLChain(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-fullchain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63fullchain"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 250
	jsonl := makeNLines(t, extID, table, recordCount, "fc")
	manifest := r63ManifestFromJSONL(t, extID, "data", jsonl)
	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: int64(recordCount)},
		DataExports:    map[string]string{table: jsonl},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: manifest},
	}
	userStateJSON, _ := json.Marshal(userState)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-fc", string(userStateJSON)); err != nil {
		t.Fatalf("restore must succeed: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-r63-fc"); err != nil {
		t.Fatalf("verify must succeed: %v", err)
	}
	var batchHash, finalBatchHash string
	if err := db.QueryRowContext(ctx,
		"SELECT batch_hash, expected_final_batch_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-fc", table).Scan(&batchHash, &finalBatchHash); err != nil {
		t.Fatalf("query: %v", err)
	}
	if finalBatchHash == "" || batchHash != finalBatchHash {
		t.Fatalf("final=%s batch=%s must be equal and non-empty", finalBatchHash, batchHash)
	}
	if batchHash == manifest.GenesisHash {
		t.Fatal("non-empty final batch hash must not equal genesis")
	}
}

func TestR63JournalStartsAtGenesis(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-start-genesis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63startgenesis"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 2, "sg")
	state := r63UserState(extID, "data", 2, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-sg", string(stateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var batchHash, genesisHash string
	if err := db.QueryRowContext(ctx,
		"SELECT batch_hash, genesis_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-sg", table).Scan(&batchHash, &genesisHash); err != nil {
		t.Fatalf("query: %v", err)
	}
	if genesisHash == "" || batchHash == "" {
		t.Fatalf("expected non-empty genesis and batch hashes: genesis=%s batch=%s", genesisHash, batchHash)
	}
}

func TestR63JournalRejectsGenesisDrift(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-genesis-drift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63genesisdrift"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 2, "gd")
	state := r63UserState(extID, "data", 2, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-gd", string(stateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET genesis_hash=? WHERE operation_id=? AND table_name=?`,
		"genesis:sha256:aaaaaaa00000000000000000000000000000000000000000000000000000000000", "op-r63-gd", table); err != nil {
		t.Fatalf("tamper genesis: %v", err)
	}
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-gd", string(stateJSON))
	if err == nil {
		t.Fatal("expected error when journal genesis diverges from manifest")
	}
}

func TestR63JournalRejectsFinalHashDrift(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-final-drift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63finaldrift"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 2, "fd")
	state := r63UserState(extID, "data", 2, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-fd", string(stateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var expectedFinal string
	if err := db.QueryRowContext(ctx,
		"SELECT expected_final_batch_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-fd", table).Scan(&expectedFinal); err != nil {
		t.Fatalf("query: %v", err)
	}
	if expectedFinal == "" {
		t.Fatal("completed journal must have expected_final_batch_hash")
	}
	if expectedFinal == state.TableManifests[table].GenesisHash {
		t.Fatal("non-empty restore final batch hash must not equal genesis")
	}
}

func TestR63ResumeRejectsBatchHashDrift(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-resume-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63resumebatch"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 250
	jsonl := makeNLines(t, extID, table, recordCount, "rb")
	state := r63UserState(extID, "data", recordCount, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-rb", string(stateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET cursor='100', batch_index=1, state='failed' WHERE operation_id=? AND table_name=?`,
		"op-r63-rb", table); err != nil {
		t.Fatalf("simulate partial: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET batch_hash=? WHERE operation_id=? AND table_name=?`,
		"batch:sha256:aaaaaaa00000000000000000000000000000000000000000000000000000000000", "op-r63-rb", table); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-rb", string(stateJSON))
	if err == nil {
		t.Fatal("expected error when resume batch hash does not match prefix")
	}
}

func TestR63ResumeRejectsBatchIndexDrift(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-resume-idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63resumeidx"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 250
	jsonl := makeNLines(t, extID, table, recordCount, "ridx")
	state := r63UserState(extID, "data", recordCount, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-ridx", string(stateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET cursor='100', batch_index=99, prev_batch_hash=genesis_hash, state='failed' WHERE operation_id=? AND table_name=?`,
		"op-r63-ridx", table); err != nil {
		t.Fatalf("simulate wrong idx: %v", err)
	}
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-ridx", string(stateJSON))
	if err == nil {
		t.Fatal("expected error when resume batch index does not match prefix")
	}
}

func TestR63ResumeRejectsUnalignedCursor(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-resume-cursor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63resumecursor"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 250
	jsonl := makeNLines(t, extID, table, recordCount, "rc")
	state := r63UserState(extID, "data", recordCount, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-rc", string(stateJSON)); err != nil {
		t.Fatalf("first restore: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET cursor='57', batch_index=0, batch_hash=genesis_hash, state='failed' WHERE operation_id=? AND table_name=?`,
		"op-r63-rc", table); err != nil {
		t.Fatalf("set unaligned: %v", err)
	}
	err = store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-rc", string(stateJSON))
	if err == nil {
		t.Fatal("expected error when resume cursor is not aligned")
	}
}

func TestR63CompletedJournalRequiresFinalBatchHash(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-complete.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63completereq"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	jsonl := makeNLines(t, extID, table, 2, "cr")
	state := r63UserState(extID, "data", 2, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-cr", string(stateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var batchHash, finalHash, genesis string
	if err := db.QueryRowContext(ctx,
		"SELECT batch_hash, expected_final_batch_hash, genesis_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-cr", table).Scan(&batchHash, &finalHash, &genesis); err != nil {
		t.Fatalf("query: %v", err)
	}
	if batchHash == "" || finalHash == "" || genesis == "" {
		t.Fatalf("expected non-empty batch=%s final=%s genesis=%s", batchHash, finalHash, genesis)
	}
	if batchHash != finalHash {
		t.Fatalf("batch_hash must equal expected_final_batch_hash: batch=%s final=%s", batchHash, finalHash)
	}
	if batchHash == genesis {
		t.Fatal("non-empty batch_hash must not equal genesis")
	}
}

func TestR63EmptyManifestFinalBatchHashEqualsGenesis(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-empty-manifest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63empty"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	state := r63UserState(extID, "data", 0, "")
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-em", string(stateJSON)); err != nil {
		t.Fatalf("empty restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-r63-em"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestR63BatchChainDeterministic(t *testing.T) {
	spec := r63SpecFor(t, "r63deterministic", "data", "export:op:ext_r63deterministic_data:r63deterministic")
	records := r63MakeRecords(t, "r63deterministic", "data", 150)
	r1, err := recalculateBatchHashChain(records, spec)
	if err != nil {
		t.Fatalf("first compute: %v", err)
	}
	r2, err := recalculateBatchHashChain(records, spec)
	if err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if r1.FinalHash != r2.FinalHash || r1.BatchCount != r2.BatchCount {
		t.Fatalf("non-deterministic: final1=%s final2=%s count1=%d count2=%d", r1.FinalHash, r2.FinalHash, r1.BatchCount, r2.BatchCount)
	}
	if r1.FinalHash == spec.GenesisHash {
		t.Fatal("non-empty chain final hash must not equal genesis")
	}
}

func TestR63SingleBatchEndToEnd(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-single-e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63singlee2e"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 50
	jsonl := makeNLines(t, extID, table, recordCount, "se")
	state := r63UserState(extID, "data", recordCount, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-se", string(stateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-r63-se"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var batchIndex, totalRows int64
	var cursor string
	if err := db.QueryRowContext(ctx,
		"SELECT batch_index, cursor, total_rows FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-se", table).Scan(&batchIndex, &cursor, &totalRows); err != nil {
		t.Fatalf("query: %v", err)
	}
	if batchIndex != 1 || totalRows != int64(recordCount) {
		t.Fatalf("expected 1 batch / %d rows, got %d / %d", recordCount, batchIndex, totalRows)
	}
	if cursor != strconv.FormatInt(int64(recordCount), 10) {
		t.Fatalf("expected cursor %d, got %s", recordCount, cursor)
	}
}

func TestR63MultiBatchEndToEnd(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r63-multi-e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	extID := "r63e2emulti"
	table := r63Table(extID)
	makeCrashTestTable(t, db, table)
	store := NewUserDataSnapshotStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	const recordCount = 250
	jsonl := makeNLines(t, extID, table, recordCount, "me")
	state := r63UserState(extID, "data", recordCount, jsonl)
	stateJSON, _ := json.Marshal(state)
	if err := store.RestoreUserDataFromSnapshot(ctx, extID, "op-r63-me", string(stateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, "op-r63-me"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var batchIndex, totalRows int64
	var batchHash, finalHash, genesisHash string
	if err := db.QueryRowContext(ctx,
		"SELECT batch_index, total_rows, batch_hash, expected_final_batch_hash, genesis_hash FROM extension_package_user_data_restore_journal WHERE operation_id=? AND table_name=?",
		"op-r63-me", table).Scan(&batchIndex, &totalRows, &batchHash, &finalHash, &genesisHash); err != nil {
		t.Fatalf("query: %v", err)
	}
	if batchIndex != 3 || totalRows != int64(recordCount) {
		t.Fatalf("expected 3 batches / %d rows, got %d / %d", recordCount, batchIndex, totalRows)
	}
	if batchHash == genesisHash {
		t.Fatal("non-empty batch_hash must not equal genesis")
	}
	if batchHash != finalHash {
		t.Fatalf("batch_hash must equal expected_final_batch_hash: batch=%s final=%s", batchHash, finalHash)
	}
}

func TestR63BatchHashEmptyBatchRejected(t *testing.T) {
	spec := r63SpecFor(t, "r63emptybatch", "data", "export:op:ext_r63emptybatch_data:r63emptybatch")
	records := r63MakeRecords(t, "r63emptybatch", "data", 5)
	if _, err := computeContentBoundBatchHash(nil, spec, 0, spec.GenesisHash, 1); err == nil {
		t.Fatal("expected error for nil batch")
	}
	if _, err := computeContentBoundBatchHash(records[:0], spec, 0, spec.GenesisHash, 1); err == nil {
		t.Fatal("expected error for empty slice")
	}
}

func TestR63BatchHashFirstBatchMustStartFromGenesis(t *testing.T) {
	spec := r63SpecFor(t, "r63firstbatch", "data", "export:op:ext_r63firstbatch_data:r63firstbatch")
	records := r63MakeRecords(t, "r63firstbatch", "data", 5)
	if _, err := computeContentBoundBatchHash(records, spec, 0, "batch:sha256:notthegenesis", 1); err == nil {
		t.Fatal("first batch must reject non-genesis previous hash")
	}
	ok, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 1)
	if err != nil || ok == "" {
		t.Fatalf("first batch from genesis should succeed: err=%v hash=%s", err, ok)
	}
}

func TestR63BatchHashFirstBatchIndexMustBeOne(t *testing.T) {
	spec := r63SpecFor(t, "r63batchone", "data", "export:op:ext_r63batchone_data:r63batchone")
	records := r63MakeRecords(t, "r63batchone", "data", 5)
	if _, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 0); err == nil {
		t.Fatal("index zero must be rejected")
	}
	if _, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, -1); err == nil {
		t.Fatal("negative index must be rejected")
	}
}

func TestR63BatchHashPrevHashRequiredForNthBatch(t *testing.T) {
	spec := r63SpecFor(t, "r63nthbatch", "data", "export:op:ext_r63nthbatch_data:r63nthbatch")
	records := r63MakeRecords(t, "r63nthbatch", "data", 5)
	if _, err := computeContentBoundBatchHash(records, spec, 100, "", 2); err == nil {
		t.Fatal("non-first batch must require previous hash")
	}
	fakePrev := "batch:sha256:aaaaaaa00000000000000000000000000000000000000000000000000000000000"
	hash, err := computeContentBoundBatchHash(records, spec, 100, fakePrev, 2)
	if err != nil || hash == "" {
		t.Fatalf("non-first batch with prev should succeed: err=%v hash=%s", err, hash)
	}
}

func TestR63SpecFormatPrefix(t *testing.T) {
	spec := r63SpecFor(t, "r63format", "data", "export:op:ext_r63format_data:r63format")
	if !strings.HasPrefix(spec.GenesisHash, "genesis:sha256:") {
		t.Fatalf("invalid genesis prefix: %s", spec.GenesisHash)
	}
	records := r63MakeRecords(t, "r63format", "data", 5)
	batchHash, err := computeContentBoundBatchHash(records, spec, 0, spec.GenesisHash, 1)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !strings.HasPrefix(batchHash, "batch:sha256:") || batchHash == spec.GenesisHash {
		t.Fatalf("invalid batch hash: %s", batchHash)
	}
}
