package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestPhase20DeletedSourceCannotBeSearchedReadOrOutlined(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusReady)
	unit := seedReaderUnit(t, repo, "tenant-A", "source-A", 1, 0, "unit-A", "unpublished-secret-needle", `{"sheetName":"Data","rowStart":146}`)
	chunk := Chunk{
		ID: "chunk-A", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, UnitID: unit.ID,
		ChunkIndex: 0, Content: "unpublished-secret-needle", EmbeddingText: "unpublished-secret-needle",
		LocatorJSON: `{"sheetName":"Data","rowStart":146}`, MetadataJSON: `{}`,
		EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed", CreatedAt: time.Now().UTC(),
	}
	if err := repo.DB().Create(&chunk).Error; err != nil {
		t.Fatal(err)
	}
	svc := &service{repo: repo}
	before, err := repo.ExactCandidates("tenant-A", "unpublished-secret-needle", nil, 10)
	if err != nil || len(before) != 1 {
		t.Fatalf("live evidence missing before deletion: %+v %v", before, err)
	}
	if _, err := svc.Read(context.Background(), "tenant-A", ReadRequest{SourceID: "source-A", SourceRevision: 1, Locator: map[string]any{"sheet": "Data", "row": 146}, MaxUnits: 1}); err != nil {
		t.Fatalf("live worksheet locator unreadable: %v", err)
	}
	if err := repo.SoftDeleteSource("tenant-A", "source-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSource("tenant-A", "source-A"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tombstone did not hide source: %v", err)
	}
	if after, err := repo.ExactCandidates("tenant-A", "unpublished-secret-needle", nil, 10); err != nil || len(after) != 0 {
		t.Fatalf("deleted evidence leaked to exact retrieval: %+v %v", after, err)
	}
	if after, err := repo.VisibleChunksByIDs("tenant-A", []string{chunk.ID}); err != nil || len(after) != 0 {
		t.Fatalf("deleted evidence leaked from vector candidate hydration: %+v %v", after, err)
	}
	for name, req := range map[string]ReadRequest{
		"unit":     {UnitID: unit.ID, MaxUnits: 1},
		"evidence": {EvidenceID: chunk.ID, MaxUnits: 1},
		"locator":  {SourceID: "source-A", SourceRevision: 1, Locator: map[string]any{"sheet": "Data", "row": 146}, MaxUnits: 1},
	} {
		if _, err := svc.Read(context.Background(), "tenant-A", req); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("deleted source still readable by %s: %v", name, err)
		}
	}
	if available, err := repo.RevisionIndexable("tenant-A", "source-A", 1); err != nil || available {
		t.Fatalf("deleted revision accepted new vector indexing: available=%v err=%v", available, err)
	}
	if _, err := repo.FindUnitsByLocator(context.Background(), "tenant-A", "source-A", 1, map[string]any{"sheet": "Data", "row": 146}, 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted source still searchable by repository locator: %v", err)
	}
	if _, err := svc.GetSourceOutline(context.Background(), "tenant-A", "source-A", 1, -1, 10); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted source still exposes outlines: %v", err)
	}
}
