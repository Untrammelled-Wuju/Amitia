package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func TestPhase33RecheckSameSampleIsReadOnly(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(43)
	now := time.Now().UTC().Truncate(time.Second)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-A", first.AuditID, now, []string{"collection-A"}, scan, load)
	if err != nil || checked.Status != "matched" || !checked.ChainAnchored || !checked.SampleMatched || !checked.CountsMatched || checked.Page != 2 {
		t.Fatalf("expected independent sample match: %+v %v", checked, err)
	}
	after, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil || before.Page != after.Page || before.LastResultJSON != after.LastResultJSON || before.NextCursor != after.NextCursor || second.Page != after.Page {
		t.Fatalf("recheck modified audit checkpoint: %+v %+v %v", before, after, err)
	}
}

func TestPhase33RecheckDriftAndTenantFailClosed(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(16)
	now := time.Now().UTC().Truncate(time.Second)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	changedScan := func(ctx context.Context, tenant, collection string, limit int, offset string) ([]qdrantDB.VectorMetadata, string, error) {
		points, next, err := scan(ctx, tenant, collection, limit, offset)
		if len(points) > 0 {
			points[0].Payload = map[string]string{"tenant_id": tenant, "chunk_id": points[0].ID, "index_version": "tampered"}
		}
		return points, next, err
	}
	changed, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-A", first.AuditID, now, []string{"collection-A"}, changedScan, load)
	if err != nil || changed.Status != "changed" || changed.SampleMatched {
		t.Fatalf("vector payload drift went unnoticed: %+v %v", changed, err)
	}
	if _, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-B", first.AuditID, now, []string{"collection-A"}, scan, load); err == nil {
		t.Fatal("foreign tenant rechecked source audit")
	}
	different, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-A", first.AuditID, now, []string{"collection-other"}, scan, load)
	if err != nil || different.Status != "collections_changed" {
		t.Fatalf("collection scope drift accepted: %+v %v", different, err)
	}
	failScan := func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
		return nil, "", errors.New("qdrant down")
	}
	if _, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-A", first.AuditID, now, []string{"collection-A"}, failScan, load); err == nil {
		t.Fatal("unavailable Qdrant was treated as a successful observation")
	}
}

func TestPhase33LegacyAndCorruptReceiptCannotBeCertified(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(11)
	now := time.Now().UTC().Truncate(time.Second)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"auditId":"` + first.AuditID + `","page":1,"totals":{"auditId":"` + first.AuditID + `","pages":1}}`
	if err := repo.db.Table("knowledge_vector_audit_pages").Where("audit_id = ?", first.AuditID).Update("result_json", raw).Error; err != nil {
		t.Fatal(err)
	}
	result, err := repo.recheckOrphanAuditLatestWithSources(context.Background(), "tenant-A", first.AuditID, now, []string{"collection-A"}, scan, load)
	if err != nil || result.Status != "unanchored" {
		t.Fatalf("corrupt receipt was certified: %+v %v", result, err)
	}
}

func TestPhase33ReceiptPaginationVerifiesEarlierChain(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(80)
	now := time.Now().UTC().Truncate(time.Second)
	cursor := ""
	auditID := ""
	for i := 0; i < 4; i++ {
		page, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, cursor, now, scan, load)
		if err != nil {
			t.Fatal(err)
		}
		auditID, cursor = page.AuditID, page.NextCursor
	}
	svc := &service{repo: repo}
	latter, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", auditID, 2, 2)
	if err != nil || !latter.ChainAnchored || len(latter.Items) != 2 || !latter.Items[0].Verified || !latter.Items[1].Verified {
		t.Fatalf("valid paginated chain was not anchored: %+v %v", latter, err)
	}
	if err := repo.db.Table("knowledge_vector_audit_pages").Where("audit_id = ? AND page = 1", auditID).Update("result_hash", orphanAuditGenesisHash).Error; err != nil {
		t.Fatal(err)
	}
	broken, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", auditID, 2, 2)
	if err != nil || broken.ChainAnchored || broken.Items[0].Verified || broken.Items[1].Verified {
		t.Fatalf("corrupt earlier chain was missed: %+v %v", broken, err)
	}
}
