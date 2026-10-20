package knowledge

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func TestPhase26VectorAuditSamplesMissingMetadataAndVersions(t *testing.T) {
	rows := []vectorAuditCandidate{
		{ID: "00000000-0000-0000-0000-000000000001", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, IndexVersion: "v1"},
		{ID: "00000000-0000-0000-0000-000000000002", TenantID: "tenant-A", SourceID: "source-A", SourceRevision: 1, IndexVersion: "v1"},
		{ID: "00000000-0000-0000-0000-000000000003", TenantID: "tenant-A", SourceID: "source-B", SourceRevision: 2, IndexVersion: "v2"},
	}
	calls := 0
	fetch := func(ctx context.Context, ids []string, collection string) (map[string]qdrantDB.VectorMetadata, error) {
		calls++
		points := make(map[string]qdrantDB.VectorMetadata)
		for _, id := range ids {
			if id == rows[1].ID {
				continue
			}
			var candidate vectorAuditCandidate
			for _, row := range rows {
				if row.ID == id {
					candidate = row
				}
			}
			meta := map[string]string{"chunk_id": id, "tenant_id": candidate.TenantID, "source_id": candidate.SourceID, "source_revision": fmt.Sprint(candidate.SourceRevision), "index_version": candidate.IndexVersion}
			if id == rows[2].ID {
				meta["tenant_id"] = "foreign"
			}
			points[id] = qdrantDB.VectorMetadata{ID: id, Payload: meta}
		}
		return points, nil
	}
	result, err := auditVectorCandidates(context.Background(), rows, 50, fetch)
	if err != nil || result.Status != "degraded" || result.Sampled != 3 || result.Missing != 1 || result.Mismatched != 1 || result.CollectionsChecked != 2 || calls != 2 || result.Exhaustive {
		t.Fatalf("audit result: %#v err=%v calls=%d", result, err, calls)
	}
	if result.ObservedAt.IsZero() {
		t.Fatal("audit observation time missing")
	}
}

func TestPhase26VectorAuditEmptyCancelledAndFailure(t *testing.T) {
	called := false
	fetch := func(context.Context, []string, string) (map[string]qdrantDB.VectorMetadata, error) {
		called = true
		return nil, errors.New("qdrant unavailable")
	}
	result, err := auditVectorCandidates(context.Background(), nil, 50, fetch)
	if err != nil || result.Status != "empty" || called {
		t.Fatalf("empty sample: %#v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = auditVectorCandidates(ctx, []vectorAuditCandidate{{ID: "id", IndexVersion: "v1"}}, 50, fetch)
	if !errors.Is(err, context.Canceled) || called || result.Sampled != 1 {
		t.Fatalf("cancelled request: %#v %v", result, err)
	}
	result, err = auditVectorCandidates(context.Background(), []vectorAuditCandidate{{ID: "id", IndexVersion: "v1"}}, 50, fetch)
	if err == nil || result.Status != "unavailable" || !called {
		t.Fatalf("failed fetch: %#v %v", result, err)
	}
}

func TestPhase26VectorAuditTenantAndLifecycleIsolation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	for _, seed := range []struct{ tenant, source, id string }{
		{"tenant-A", "source-A", "chunk-A"},
		{"tenant-B", "source-B", "chunk-B"},
	} {
		seedReadableKnowledgeSource(t, repo, seed.tenant, seed.source, 1, RevisionStatusReady)
		chunk := Chunk{ID: seed.id, TenantID: seed.tenant, SourceID: seed.source, SourceRevision: 1, UnitID: seed.id, ChunkIndex: 0, Status: "indexed", IndexVersion: DefaultIndexVersion, LocatorJSON: `{}`, MetadataJSON: `{}`, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := repo.DB().Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.vectorAuditCandidates(context.Background(), "tenant-A", 50)
	if err != nil || len(rows) != 1 || rows[0].ID != "chunk-A" {
		t.Fatalf("tenant scope: %+v %v", rows, err)
	}
	if _, err := repo.vectorAuditCandidates(context.Background(), "", 50); err == nil {
		t.Fatal("empty tenant accepted")
	}
	if _, err := repo.vectorAuditCandidates(context.Background(), "tenant-A", 101); err == nil {
		t.Fatal("unbounded sampling accepted")
	}
	row := phase23Outbox("pending-audit", "tenant-A", "source-A", "upsert", 1, time.Now().UTC())
	if err := repo.DB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = repo.vectorAuditCandidates(context.Background(), "tenant-A", 50)
	if err != nil || len(rows) != 0 {
		t.Fatalf("in-flight revision audited as stable: %+v %v", rows, err)
	}
	if err := repo.DB().Delete(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDeleteSource("tenant-A", "source-A"); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.vectorAuditCandidates(context.Background(), "tenant-A", 50)
	if err != nil || len(rows) != 0 {
		t.Fatalf("deleted source audited: %+v %v", rows, err)
	}
	rows, err = repo.vectorAuditCandidates(context.Background(), "tenant-B", 50)
	if err != nil || len(rows) != 1 || rows[0].ID != "chunk-B" {
		t.Fatalf("other tenant altered by deletion: %+v %v", rows, err)
	}
}
