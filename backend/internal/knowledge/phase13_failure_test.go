package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPhase13RetrievalFallsBackWhenDenseUnavailable(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	unit := seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", "fallback-marker exact sparse fact", `{"page":1}`)
	chunk := Chunk{
		ID: "chunk-a", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, UnitID: unit.ID,
		ChunkIndex: 0, Content: "fallback-marker exact sparse fact", EmbeddingText: "fallback-marker exact sparse fact",
		LocatorJSON: `{"page":1}`, MetadataJSON: `{}`, Checksum: "chunk-a-sum",
		EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.DB().Create(&chunk).Error; err != nil {
		t.Fatalf("seed chunk: %v", err)
	}

	runtime := NewRetrievalRuntime(repo, nil, defaultRuntimeSettings())
	resp, err := runtime.Search(context.Background(), "tenant-a", SearchRequest{
		Query: "fallback-marker", RetrievalLevel: RetrievalLevelFast, Limit: 4, SkipTrace: true,
	})
	if err != nil {
		t.Fatalf("retrieval must degrade instead of failing: %v", err)
	}
	if len(resp.Evidence) == 0 {
		t.Fatalf("sparse/exact fallback returned no evidence: %+v", resp)
	}
	joined := strings.Join(resp.Warnings, "\n")
	if !strings.Contains(joined, "向量检索不可用") {
		t.Fatalf("dense degradation must be explicit, warnings=%q", joined)
	}
}

func TestPhase13CanonicalReaderHonorsCancellationDuringLargeLocatorScan(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "cancel-source", 1, RevisionStatusReady)
	now := time.Now().UTC()
	units := make([]DocumentUnit, 0, 4096)
	for i := 0; i < 4096; i++ {
		units = append(units, DocumentUnit{
			ID: fmt.Sprintf("cancel-unit-%04d", i), TenantID: "tenant-a", SourceID: "cancel-source", SourceRevision: 1,
			Ordinal: int64(i), UnitType: "page", Content: "decoy", LocatorJSON: `{"page":1}`, MetadataJSON: `{}`,
			Checksum: "cancel-sum", CreatedAt: now,
		})
	}
	if err := repo.DB().CreateInBatches(&units, 256).Error; err != nil {
		t.Fatalf("seed units: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&service{repo: repo}).Read(ctx, "tenant-a", ReadRequest{
		SourceID: "cancel-source", SourceRevision: 1, Locator: map[string]any{"page": 999999}, MaxUnits: 1,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("reader must return context cancellation, got %v", err)
	}
}

func TestPhase13ReadyDegradedRevisionRemainsReadable(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "ocr-source", 1, RevisionStatusReadyDegraded)
	unit := seedReaderUnit(t, repo, "tenant-a", "ocr-source", 1, 0, "ocr-unit", "OCR degraded but canonical text remains readable", `{"page":9}`)

	got, err := (&service{repo: repo}).Read(context.Background(), "tenant-a", ReadRequest{UnitID: unit.ID, MaxUnits: 1, MaxRunes: 512})
	if err != nil {
		t.Fatalf("ready_degraded revision must remain readable: %v", err)
	}
	if got.AnchorUnitID != unit.ID || len(got.Units) != 1 {
		t.Fatalf("unexpected degraded read result: %+v", got)
	}
}
