package knowledge

import (
	"context"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func TestPhase29OrphanAuditLifecycleParity(t *testing.T) {
	cases := []struct {
		name           string
		sourceStatus   string
		revisionStatus string
		chunkStatus    string
		chunkIndex     int
		unindexable    int
	}{
		{name: "ready", sourceStatus: SourceStatusReady, revisionStatus: RevisionStatusReady, chunkStatus: "indexed", chunkIndex: 0},
		{name: "degraded", sourceStatus: SourceStatusReadyDegraded, revisionStatus: RevisionStatusReadyDegraded, chunkStatus: "indexed", chunkIndex: 0},
		{name: "source_failed", sourceStatus: SourceStatusFailed, revisionStatus: RevisionStatusReady, chunkStatus: "indexed", chunkIndex: 0, unindexable: 1},
		{name: "revision_failed", sourceStatus: SourceStatusReady, revisionStatus: RevisionStatusFailed, chunkStatus: "indexed", chunkIndex: 0, unindexable: 1},
		{name: "revision_processing", sourceStatus: SourceStatusReady, revisionStatus: RevisionStatusProcessing, chunkStatus: "indexed", chunkIndex: 0, unindexable: 1},
		{name: "chunk_pending", sourceStatus: SourceStatusReady, revisionStatus: RevisionStatusReady, chunkStatus: "pending", chunkIndex: 0, unindexable: 1},
		{name: "invalid_chunk_index", sourceStatus: SourceStatusReady, revisionStatus: RevisionStatusReady, chunkStatus: "indexed", chunkIndex: -1, unindexable: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newKnowledgeTestRepository(t)
			seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, tc.revisionStatus)
			if err := repo.DB().Model(&Source{}).Where("tenant_id = ? AND id = ?", "tenant-A", "source-A").Update("status", tc.sourceStatus).Error; err != nil {
				t.Fatal(err)
			}
			chunk := Chunk{ID: "chunk-A", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, UnitID: "unit-A", ChunkIndex: tc.chunkIndex, Status: tc.chunkStatus, IndexVersion: DefaultIndexVersion, LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			if err := repo.DB().Create(&chunk).Error; err != nil {
				t.Fatal(err)
			}
			point := phase27Point("chunk-A", "tenant-A", "source-A", "1", DefaultIndexVersion)
			state, err := repo.orphanAuditState(context.Background(), "tenant-A", []qdrantDB.VectorMetadata{point})
			if err != nil {
				t.Fatal(err)
			}
			if got := state.Chunks[chunk.ID].Unindexable; got != tc.unindexable {
				t.Fatalf("unindexable=%d, expected %d", got, tc.unindexable)
			}
			eligible, err := repo.vectorAuditCandidates(context.Background(), "tenant-A", 10)
			if err != nil {
				t.Fatal(err)
			}
			if want := tc.unindexable == 0; (len(eligible) == 1) != want {
				t.Fatalf("forward indexable=%v, reverse unindexable=%d", len(eligible) == 1, tc.unindexable)
			}
			orphans, mismatches, inflight, err := classifyOrphanVectorPoints([]qdrantDB.VectorMetadata{point}, "tenant-A", knowledgeCollectionName(DefaultIndexVersion), state)
			if err != nil || orphans != tc.unindexable || mismatches != 0 || inflight != 0 {
				t.Fatalf("unexpected classification: orphans=%d mismatches=%d inflight=%d err=%v", orphans, mismatches, inflight, err)
			}
		})
	}
}

func TestPhase29OrphanAuditPendingDeletionAndTenantBoundary(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusFailed)
	seedReadableKnowledgeSource(t, repo, "tenant-B", "source-B", 1, RevisionStatusReady)
	for _, row := range []struct{ tenant, source, chunk string }{
		{"tenant-A", "source-A", "chunk-A"},
		{"tenant-B", "source-B", "chunk-B"},
	} {
		chunk := Chunk{ID: row.chunk, TenantID: row.tenant, SourceID: row.source, SourceRevision: 1, UnitID: row.chunk, ChunkIndex: 0, Status: "indexed", IndexVersion: DefaultIndexVersion, LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := repo.DB().Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
	}
	point := phase27Point("chunk-A", "tenant-A", "source-A", "1", DefaultIndexVersion)
	state, err := repo.orphanAuditState(context.Background(), "tenant-A", []qdrantDB.VectorMetadata{point})
	if err != nil || state.Chunks["chunk-A"].Unindexable != 1 || len(state.Pending) != 0 {
		t.Fatalf("failed revision should be a suspected orphan: %#v, %v", state, err)
	}
	outbox := phase23Outbox("delete-A", "tenant-A", "source-A", "delete", 1, time.Now().UTC())
	if err := repo.DB().Create(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	state, err = repo.orphanAuditState(context.Background(), "tenant-A", []qdrantDB.VectorMetadata{point})
	if err != nil {
		t.Fatal(err)
	}
	orphans, mismatches, inflight, err := classifyOrphanVectorPoints([]qdrantDB.VectorMetadata{point}, "tenant-A", knowledgeCollectionName(DefaultIndexVersion), state)
	if err != nil || orphans != 0 || mismatches != 0 || inflight != 1 {
		t.Fatalf("pending delete should suppress suspected orphan: %d %d %d %v", orphans, mismatches, inflight, err)
	}
	foreign := phase27Point("chunk-B", "tenant-B", "source-B", "1", DefaultIndexVersion)
	if _, err := repo.orphanAuditState(context.Background(), "tenant-A", []qdrantDB.VectorMetadata{foreign}); err == nil {
		t.Fatal("cross-tenant audit point accepted")
	}
}
