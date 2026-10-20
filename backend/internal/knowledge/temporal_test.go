package knowledge

import (
	"context"
	"testing"
	"time"
)

func seedTemporalSource(t *testing.T, repo *Repository, chunks []Chunk) {
	t.Helper()
	now := time.Now().UTC()
	src := &Source{
		ID: "temporal-source", TenantID: "temporal-tenant", LogicalName: "temporal", Filename: "temporal.jsonl",
		MimeType: "application/jsonl", SourceType: "jsonl", StorageKey: "temporal/r1", SHA256: "temporal-sha", Revision: 1,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: SourceStatusReady, ExpectedUnits: int64(len(chunks)), ProcessedUnits: int64(len(chunks)), IndexedChunks: int64(len(chunks)), CreatedAt: now, UpdatedAt: now,
	}
	job := &IngestJob{ID: "temporal-job", TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Kind: JobKindIngest, Stage: "completed", Status: JobStatusCompleted, CursorJSON: `{}`, LastCommittedOrdinal: int64(len(chunks) - 1), CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateSource(src, job); err != nil {
		t.Fatalf("create temporal source: %v", err)
	}
	if err := repo.UpdateSourceRevision(src.TenantID, src.ID, 1, map[string]any{"status": RevisionStatusReady, "activated_at": now}); err != nil {
		t.Fatalf("ready temporal revision: %v", err)
	}
	for i := range chunks {
		chunks[i].TenantID = src.TenantID
		chunks[i].SourceID = src.ID
		chunks[i].SourceRevision = 1
		chunks[i].ChunkIndex = 0
		chunks[i].EmbeddingText = chunks[i].Content
		chunks[i].LocatorJSON = `{}`
		if chunks[i].MetadataJSON == "" {
			chunks[i].MetadataJSON = `{}`
		}
		chunks[i].EmbeddingVersion = "active"
		chunks[i].IndexVersion = DefaultIndexVersion
		chunks[i].Status = "indexed"
		if chunks[i].CreatedAt.IsZero() {
			chunks[i].CreatedAt = now
		}
		chunks[i].UpdatedAt = now
		unit := DocumentUnit{
			ID: chunks[i].UnitID, TenantID: src.TenantID, SourceID: src.ID, SourceRevision: 1, Ordinal: int64(i), UnitType: "text",
			Title: chunks[i].Title, Content: chunks[i].Content, LocatorJSON: `{}`, MetadataJSON: chunks[i].MetadataJSON,
			ValidFrom: chunks[i].ValidFrom, ValidTo: chunks[i].ValidTo, SupersedesJSON: chunks[i].SupersedesJSON, ConflictGroup: chunks[i].ConflictGroup, CreatedAt: chunks[i].CreatedAt,
		}
		if unit.SupersedesJSON == "" {
			unit.SupersedesJSON = "[]"
			chunks[i].SupersedesJSON = "[]"
		}
		if err := repo.InsertUnitBundle(unit, []Chunk{chunks[i]}, nil); err != nil {
			t.Fatalf("insert temporal chunk %d: %v", i, err)
		}
	}
}

func TestTemporalSearchCurrentHistoricalAndAll(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	pastStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	cutover := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	seedTemporalSource(t, repo, []Chunk{
		{ID: "temporal-old", UnitID: "temporal-u-old", Content: "policy temporal old", ValidFrom: &pastStart, ValidTo: &cutover},
		{ID: "temporal-current", UnitID: "temporal-u-current", Content: "policy temporal current", ValidFrom: &cutover, ValidTo: &future},
	})
	runtime := NewRetrievalRuntime(repo, nil)

	current, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "policy temporal", Limit: 8, TemporalMode: TemporalModeCurrent})
	if err != nil {
		t.Fatalf("current search: %v", err)
	}
	if current.TemporalMode != TemporalModeCurrent || current.AsOf == nil {
		t.Fatalf("current temporal metadata missing: %#v", current)
	}
	if len(current.Evidence) != 1 || current.Evidence[0].ID != "temporal-current" {
		t.Fatalf("current visibility mismatch: %#v", current.Evidence)
	}

	asOf := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	historical, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "policy temporal", Limit: 8, TemporalMode: TemporalModeHistorical, AsOf: &asOf})
	if err != nil {
		t.Fatalf("historical search: %v", err)
	}
	if historical.AsOf == nil || !historical.AsOf.Equal(asOf) || len(historical.Evidence) != 1 || historical.Evidence[0].ID != "temporal-old" {
		t.Fatalf("historical visibility mismatch: %#v", historical)
	}

	all, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "policy temporal", Limit: 8, TemporalMode: TemporalModeAll})
	if err != nil {
		t.Fatalf("all search: %v", err)
	}
	if all.AsOf != nil || len(all.Evidence) != 2 {
		t.Fatalf("all-time search should return both facts: %#v", all)
	}
}

func TestSupersedesUsesExplicitReferenceAndRespectsHistoricalAsOf(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	oldStart := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	newStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	seedTemporalSource(t, repo, []Chunk{
		{ID: "superseded-old", UnitID: "superseded-u-old", Content: "support policy allows email", MetadataJSON: `{"externalId":"support-policy-v1"}`, ValidFrom: &oldStart},
		{ID: "superseding-new", UnitID: "superseding-u-new", Content: "support policy requires portal", ValidFrom: &newStart, SupersedesJSON: `["support-policy-v1"]`},
	})
	runtime := NewRetrievalRuntime(repo, nil)

	current, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "support policy", Limit: 8})
	if err != nil {
		t.Fatalf("current supersession search: %v", err)
	}
	if len(current.Evidence) != 1 || current.Evidence[0].ID != "superseding-new" {
		t.Fatalf("explicit supersession did not hide old current fact: %#v", current.Evidence)
	}

	asOf := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	historical, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "support policy", Limit: 8, TemporalMode: TemporalModeHistorical, AsOf: &asOf})
	if err != nil {
		t.Fatalf("historical supersession search: %v", err)
	}
	if len(historical.Evidence) != 1 || historical.Evidence[0].ID != "superseded-old" {
		t.Fatalf("historical fact was incorrectly erased by future superseder: %#v", historical.Evidence)
	}
}

func TestConflictGroupProducesAmbiguousEvidence(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedTemporalSource(t, repo, []Chunk{
		{ID: "conflict-a", UnitID: "conflict-u-a", Content: "launch date is Monday", ConflictGroup: "launch-date"},
		{ID: "conflict-b", UnitID: "conflict-u-b", Content: "launch date is Tuesday", ConflictGroup: "launch-date"},
		{ID: "other", UnitID: "conflict-u-other", Content: "launch checklist is ready"},
	})
	runtime := NewRetrievalRuntime(repo, nil)
	result, err := runtime.Search(context.Background(), "temporal-tenant", SearchRequest{Query: "launch date", Limit: 2})
	if err != nil {
		t.Fatalf("conflict search: %v", err)
	}
	if !result.Ambiguous || len(result.ConflictGroups) != 1 || result.ConflictGroups[0] != "launch-date" {
		t.Fatalf("explicit conflict was not surfaced: %#v", result)
	}
	if len(result.Evidence) != 2 {
		t.Fatalf("conflicting alternatives should both fit the evidence budget: %#v", result.Evidence)
	}
	for _, evidence := range result.Evidence {
		if evidence.ConflictGroup != "launch-date" || evidence.Metadata["conflictState"] != "ambiguous" {
			t.Fatalf("conflict evidence missing metadata: %#v", evidence)
		}
	}
}

func TestHistoricalModeRequiresAsOf(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	runtime := NewRetrievalRuntime(repo, nil)
	if _, err := runtime.Search(context.Background(), "tenant", SearchRequest{Query: "x", TemporalMode: TemporalModeHistorical}); err == nil {
		t.Fatal("historical mode without asOf must be rejected")
	}
}
