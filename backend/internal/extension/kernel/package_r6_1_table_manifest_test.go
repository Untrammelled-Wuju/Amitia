package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

type r61Row struct {
	EntityID string
	Value    string
	Amount   int
}

func newR61CaptureDatabase(
	t *testing.T,
	extensionID string,
	rows []r61Row,
) (
	*sql.DB,
	string,
) {
	t.Helper()

	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "r61.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	table := migration.ExtensionNamespacePrefix(extensionID) + "records"

	_, err = database.Exec(fmt.Sprintf(`CREATE TABLE %s (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL,
		amount INTEGER NOT NULL
	)`, quoteIdentifier(table)))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		_, err = database.Exec(fmt.Sprintf(`INSERT INTO %s (entity_id, entity_value, amount) VALUES (?, ?, ?)`, quoteIdentifier(table)),
			row.EntityID, row.Value, row.Amount)
		if err != nil {
			t.Fatal(err)
		}
	}

	return database, table
}

func TestR61CaptureNonEmptyTableUsesActualAggregateHash(t *testing.T) {
	const extensionID = "com.example.r61.aggregate"

	database, table := newR61CaptureDatabase(t, extensionID, []r61Row{
		{EntityID: "record-2", Value: "second", Amount: 2},
		{EntityID: "record-1", Value: "first", Amount: 1},
	})

	result, err := captureUserDataTableSnapshot(context.Background(), database, extensionID, table)
	if err != nil {
		t.Fatal(err)
	}

	rawRecords, parsedRecords, err := parseAndValidateJSONL(result.jsonl, extensionID)
	if err != nil {
		t.Fatal(err)
	}

	if len(rawRecords) != 2 || len(parsedRecords) != 2 {
		t.Fatalf("unexpected captured record count: raw=%d parsed=%d", len(rawRecords), len(parsedRecords))
	}

	expectedAggregateHash := computeUserDataAggregateHashFromRecords(parsedRecords)
	if result.manifest.AggregateHash != expectedAggregateHash {
		t.Fatalf("aggregate hash mismatch: manifest=%s expected=%s", result.manifest.AggregateHash, expectedAggregateHash)
	}

	emptyAggregateHash := computeUserDataAggregateHashFromRecords(nil)
	if result.manifest.AggregateHash == emptyAggregateHash {
		t.Fatal("non-empty table uses empty aggregate hash")
	}
}

func TestR61CaptureNonEmptyTableComputesFinalBatchHash(t *testing.T) {
	const extensionID = "com.example.r61.batch"

	rows := make([]r61Row, 0, 205)
	for index := 0; index < 205; index++ {
		rows = append(rows, r61Row{
			EntityID: fmt.Sprintf("record-%04d", index),
			Value:    fmt.Sprintf("value-%04d", index),
			Amount:   index,
		})
	}

	database, table := newR61CaptureDatabase(t, extensionID, rows)

	result, err := captureUserDataTableSnapshot(context.Background(), database, extensionID, table)
	if err != nil {
		t.Fatal(err)
	}

	rawRecords, _, err := parseAndValidateJSONL(result.jsonl, extensionID)
	if err != nil {
		t.Fatal(err)
	}

	chainSpec := UserDataBatchChainSpec{
		Identity: UserDataTableIdentity{
			Domain:                userDataBatchGenesisDomain,
			SchemaVersion:         userDataRecordSchemaVersion,
			ExtensionID:           extensionID,
			CanonicalTable:        table,
			EntityType:            result.manifest.EntityType,
			NamespaceHash:         result.manifest.NamespaceHash,
			BatchAlgorithmVersion: userDataBatchHashAlgorithmVersion,
		},
		DataExportReference: result.manifest.DataExportReference,
		BatchSize:           int64(userDataRestoreBatchSize),
		GenesisHash:         result.manifest.GenesisHash,
	}

	chainResult, err := recalculateBatchHashChain(
		rawRecords,
		chainSpec,
	)
	if err != nil {
		t.Fatal(err)
	}

	if chainResult.BatchCount != 3 {
		t.Fatalf("expected 3 batches, got %d", chainResult.BatchCount)
	}

	if result.manifest.FinalBatchHash != chainResult.FinalHash {
		t.Fatalf("final batch hash mismatch: manifest=%s expected=%s", result.manifest.FinalBatchHash, chainResult.FinalHash)
	}

	if result.manifest.FinalBatchHash == result.manifest.GenesisHash {
		t.Fatal("non-empty final batch hash must not equal genesis")
	}
}

func TestR61CaptureEmptyTableUsesEmptyAggregateAndGenesis(t *testing.T) {
	const extensionID = "com.example.r61.empty"

	database, table := newR61CaptureDatabase(t, extensionID, nil)

	result, err := captureUserDataTableSnapshot(context.Background(), database, extensionID, table)
	if err != nil {
		t.Fatal(err)
	}

	if result.count != 0 {
		t.Fatalf("expected empty snapshot, got %d records", result.count)
	}

	if result.manifest.AggregateHash != computeUserDataAggregateHashFromRecords(nil) {
		t.Fatal("empty aggregate hash mismatch")
	}

	if result.manifest.FinalBatchHash != result.manifest.GenesisHash {
		t.Fatal("empty final batch hash must equal genesis")
	}

	if result.manifest.DataExportReference == "" {
		t.Fatal("empty snapshot export reference missing")
	}
}

func TestR61ExportReferenceChangesWithJSONLContent(t *testing.T) {
	const extensionID = "com.example.r61.export"

	firstDatabase, firstTable := newR61CaptureDatabase(t, extensionID, []r61Row{
		{EntityID: "record-1", Value: "value-a", Amount: 1},
	})

	first, err := captureUserDataTableSnapshot(context.Background(), firstDatabase, extensionID, firstTable)
	if err != nil {
		t.Fatal(err)
	}

	secondDatabase, secondTable := newR61CaptureDatabase(t, extensionID, []r61Row{
		{EntityID: "record-1", Value: "value-b", Amount: 1},
	})

	second, err := captureUserDataTableSnapshot(context.Background(), secondDatabase, extensionID, secondTable)
	if err != nil {
		t.Fatal(err)
	}

	if first.manifest.DataExportReference == second.manifest.DataExportReference {
		t.Fatal("JSONL content drift must change DataExportReference")
	}
}

func TestR61CaptureIsStableAcrossInsertionOrder(t *testing.T) {
	const extensionID = "com.example.r61.order"

	firstRows := []r61Row{
		{EntityID: "record-3", Value: "three", Amount: 3},
		{EntityID: "record-1", Value: "one", Amount: 1},
		{EntityID: "record-2", Value: "two", Amount: 2},
	}

	secondRows := []r61Row{
		firstRows[1],
		firstRows[2],
		firstRows[0],
	}

	firstDatabase, firstTable := newR61CaptureDatabase(t, extensionID, firstRows)
	secondDatabase, secondTable := newR61CaptureDatabase(t, extensionID, secondRows)

	first, err := captureUserDataTableSnapshot(context.Background(), firstDatabase, extensionID, firstTable)
	if err != nil {
		t.Fatal(err)
	}

	second, err := captureUserDataTableSnapshot(context.Background(), secondDatabase, extensionID, secondTable)
	if err != nil {
		t.Fatal(err)
	}

	if first.jsonl != second.jsonl {
		t.Fatalf("canonical JSONL depends on insertion order:\nfirst=%s\nsecond=%s", first.jsonl, second.jsonl)
	}

	if first.manifest.AggregateHash != second.manifest.AggregateHash {
		t.Fatal("aggregate hash depends on insertion order")
	}

	if first.manifest.FinalBatchHash != second.manifest.FinalBatchHash {
		t.Fatal("final batch hash depends on insertion order")
	}

	if first.manifest.DataExportReference != second.manifest.DataExportReference {
		t.Fatal("export reference depends on insertion order")
	}
}

func TestR61LegacyExportReferenceRejectsEmptyOperationID(t *testing.T) {
	reference := computeUserDataExportReference("", "ext_test_records", "com.example.test")
	if reference != "" {
		t.Fatalf("empty operation id produced export reference %q", reference)
	}

	if strings.Contains(reference, "export::") {
		t.Fatalf("invalid empty-operation export reference produced: %s", reference)
	}
}
