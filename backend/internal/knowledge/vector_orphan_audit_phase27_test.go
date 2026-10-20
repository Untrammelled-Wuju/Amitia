package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func phase27Point(id, tenant, source, revision, version string) qdrantDB.VectorMetadata {
	return qdrantDB.VectorMetadata{ID: id, Payload: map[string]string{
		"chunk_id": id, "tenant_id": tenant, "source_id": source, "source_revision": revision, "index_version": version,
	}}
}

func TestPhase27OrphanClassification(t *testing.T) {
	points := []qdrantDB.VectorMetadata{
		phase27Point("gone", "tenant-A", "source-1", "1", "v1"),
		phase27Point("wrong", "tenant-A", "source-1", "1", "v1"),
		phase27Point("pending", "tenant-A", "source-2", "2", "v1"),
		phase27Point("deleted", "tenant-A", "source-3", "1", "v1"),
		phase27Point("valid", "tenant-A", "source-1", "1", "v1"),
	}
	state := orphanVectorState{
		Chunks: map[string]orphanVectorRow{
			"wrong":   {ID: "wrong", SourceID: "other-source", SourceRevision: 1, IndexVersion: "v1"},
			"pending": {ID: "pending", SourceID: "source-2", SourceRevision: 2, IndexVersion: "v1"},
			"deleted": {ID: "deleted", SourceID: "source-3", SourceRevision: 1, IndexVersion: "v1", Unindexable: 1},
			"valid":   {ID: "valid", SourceID: "source-1", SourceRevision: 1, IndexVersion: "v1"},
		},
		Pending: map[string]bool{orphanSourceRevisionKey("source-2", 2): true},
	}
	orphans, mismatches, inflight, err := classifyOrphanVectorPoints(points, "tenant-A", knowledgeCollectionName("v1"), state)
	if err != nil || orphans != 2 || mismatches != 1 || inflight != 1 {
		t.Fatalf("unexpected classification: orphans=%d mismatches=%d inflight=%d err=%v", orphans, mismatches, inflight, err)
	}
	points[0].Payload["tenant_id"] = "tenant-B"
	if _, _, _, err = classifyOrphanVectorPoints(points, "tenant-A", knowledgeCollectionName("v1"), state); err == nil {
		t.Fatal("cross-tenant point accepted")
	}
}

func TestPhase27OrphanAuditBoundedAndFailClosed(t *testing.T) {
	versions := []string{knowledgeCollectionName("v1"), knowledgeCollectionName("v2"), knowledgeCollectionName("v3"), knowledgeCollectionName("v4"), knowledgeCollectionName("v5")}
	calls := 0
	scan := func(_ context.Context, tenant, collection string, limit int) ([]qdrantDB.VectorMetadata, error) {
		calls++
		if tenant != "tenant-A" || limit < 1 || limit > 25 {
			t.Fatalf("invalid scan request: %q %d", tenant, limit)
		}
		return []qdrantDB.VectorMetadata{phase27Point(collection, tenant, "source-1", "1", "v1")}, nil
	}
	load := func(_ context.Context, tenant string, points []qdrantDB.VectorMetadata) (orphanVectorState, error) {
		return orphanVectorState{Chunks: map[string]orphanVectorRow{}, Pending: map[string]bool{}}, nil
	}
	result, err := auditTenantOrphanVectors(context.Background(), "tenant-A", versions, 50, scan, load)
	if err != nil || result.Status != "suspected" || result.Sampled != 4 || result.SuspectedOrphans != 4 || result.CollectionsChecked != 4 || result.CollectionsKnown != 5 || result.Exhaustive || calls != 4 {
		t.Fatalf("bounded result: %#v err=%v calls=%d", result, err, calls)
	}
	if result.ObservedAt.IsZero() {
		t.Fatal("missing observation timestamp")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if _, err := auditTenantOrphanVectors(ctx, "tenant-A", versions, 50, scan, load); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled scan started: %v", err)
	}
	badScan := func(context.Context, string, string, int) ([]qdrantDB.VectorMetadata, error) {
		return nil, errors.New("qdrant unavailable")
	}
	result, err = auditTenantOrphanVectors(context.Background(), "tenant-A", versions, 50, badScan, load)
	if err == nil || result.Status != "unavailable" {
		t.Fatalf("failed scan treated as success: %#v %v", result, err)
	}
	if _, err := auditTenantOrphanVectors(context.Background(), "", versions, 50, scan, load); err == nil {
		t.Fatal("empty tenant accepted")
	}
	if _, err := auditTenantOrphanVectors(context.Background(), "tenant-A", versions, 101, scan, load); err == nil {
		t.Fatal("unbounded scan accepted")
	}
	foreign := func(context.Context, string, string, int) ([]qdrantDB.VectorMetadata, error) {
		return []qdrantDB.VectorMetadata{phase27Point("id", "tenant-B", "source-1", "1", "v1")}, nil
	}
	if _, err := auditTenantOrphanVectors(context.Background(), "tenant-A", versions, 50, foreign, load); err == nil {
		t.Fatal("foreign point accepted")
	}
}

func TestPhase27OrphanAuditCanonicalIsolation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-A", "source-A", 1, RevisionStatusReady)
	seedReadableKnowledgeSource(t, repo, "tenant-B", "source-B", 1, RevisionStatusReady)
	for _, row := range []struct{ id, tenant, source string }{
		{"chunk-A", "tenant-A", "source-A"},
		{"chunk-B", "tenant-B", "source-B"},
	} {
		chunk := Chunk{ID: row.id, TenantID: row.tenant, SourceID: row.source, SourceRevision: 1, UnitID: row.id, ChunkIndex: 0, Status: "indexed", IndexVersion: "v1", LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := repo.DB().Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
	}
	points := []qdrantDB.VectorMetadata{phase27Point("chunk-A", "tenant-A", "source-A", "1", "v1")}
	state, err := repo.orphanAuditState(context.Background(), "tenant-A", points)
	if err != nil || len(state.Chunks) != 1 || state.Chunks["chunk-A"].Unindexable != 0 {
		t.Fatalf("tenant A lookup: %+v %v", state, err)
	}
	if _, err := repo.orphanAuditState(context.Background(), "tenant-B", points); err == nil {
		t.Fatal("cross-tenant read allowed")
	}
	ob := phase23Outbox("pending-27", "tenant-A", "source-A", "delete", 1, time.Now().UTC())
	if err := repo.DB().Create(&ob).Error; err != nil {
		t.Fatal(err)
	}
	state, err = repo.orphanAuditState(context.Background(), "tenant-A", points)
	if err != nil || !state.Pending[orphanSourceRevisionKey("source-A", 1)] {
		t.Fatalf("missing pending tombstone: %+v %v", state, err)
	}
	if err := repo.DB().Delete(&ob).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDeleteSource("tenant-A", "source-A"); err != nil {
		t.Fatal(err)
	}
	state, err = repo.orphanAuditState(context.Background(), "tenant-A", points)
	if err != nil || state.Chunks["chunk-A"].Unindexable != 1 {
		t.Fatalf("soft-deleted source not classified: %+v %v", state, err)
	}
}

func TestPhase27RotatingSampleWindow(t *testing.T) {
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	if vectorAuditDailyOffset(10, 50, "tenant", now) != 0 {
		t.Fatal("small corpus should not offset")
	}
	a := vectorAuditDailyOffset(1000, 50, "tenant", now)
	b := vectorAuditDailyOffset(1000, 50, "tenant", now.Add(30*time.Minute))
	if a != b || a < 0 || a > 950 {
		t.Fatalf("invalid deterministic audit window: %d %d", a, b)
	}
	foundDifferentDay := false
	for day := 1; day <= 10; day++ {
		if vectorAuditDailyOffset(1000, 50, "tenant", now.Add(time.Duration(day)*24*time.Hour)) != a {
			foundDifferentDay = true
		}
	}
	if !foundDifferentDay {
		t.Fatal("daily window does not rotate")
	}
}
