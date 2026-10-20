package knowledge

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPhase32ImmutableReceiptChainAndHistoricReplay(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(95)
	now := time.Date(2026, 10, 16, 17, 0, 0, 0, time.UTC)
	var results []OrphanVectorAuditPageResult
	cursor := ""
	for i := 0; i < 4; i++ {
		page, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, cursor, now, scan, load)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, page)
		cursor = page.NextCursor
	}
	svc := &service{repo: repo}
	history, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", results[0].AuditID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !history.ChainAnchored || history.HasMore || len(history.Items) != 4 || history.NextPage != 4 {
		t.Fatalf("unexpected receipts: %+v", history)
	}
	for _, item := range history.Items {
		if !item.Verified {
			t.Fatalf("broken page receipt: %+v", item)
		}
	}
	replay, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, results[0].NextCursor, now, scan, load)
	if err != nil || replay.Page != 2 || replay.Totals.Sampled != 50 {
		t.Fatalf("historic replay mismatch: %+v %v", replay, err)
	}
	segments, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", results[0].AuditID, 2, 1)
	if err != nil || len(segments.Items) != 1 || !segments.Items[0].Verified || !segments.HasMore {
		t.Fatalf("segment mismatch: %+v %v", segments, err)
	}
	foreign, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-B", results[0].AuditID, 0, 10)
	if err == nil || len(foreign.Items) != 0 {
		t.Fatalf("tenant isolation failed: %+v %v", foreign, err)
	}
}

func TestPhase32ReceiptTamperDetectionAndAtomicRollback(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(30)
	now := time.Now().UTC()
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	tx := repo.db.WithContext(context.Background()).Table("knowledge_vector_audit_pages").Where("audit_id = ? AND page = 1", first.AuditID).Update("result_hash", orphanAuditGenesisHash)
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	svc := &service{repo: repo}
	receipts, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", first.AuditID, 0, 10)
	if err != nil || len(receipts.Items) != 1 || receipts.Items[0].Verified || receipts.ChainAnchored {
		t.Fatalf("tamper undetected: %+v %v", receipts, err)
	}
	if _, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, scan, load); err == nil {
		t.Fatal("corrupted predecessor accepted")
	}
	row, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil || row.Page != 1 {
		t.Fatalf("failed commit advanced checkpoint: %+v %v", row, err)
	}
}

func TestPhase32ReceiptWriteFailureRollsBackSession(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(35)
	now := time.Now().UTC()
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`CREATE TRIGGER deny_receipt_insert BEFORE INSERT ON knowledge_vector_audit_pages BEGIN SELECT RAISE(ABORT, 'deny'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, scan, load); err == nil {
		t.Fatal("missing receipt table must fail")
	}
	row, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil || row.Page != 1 {
		t.Fatalf("transaction was not atomic: %+v %v", row, err)
	}
}

func TestPhase32ReceiptNoPayloadLeak(t *testing.T) {
	row := makeAuditReceipt("tenant-A", "test-audit", 1, "cursor-sensitive", orphanAuditGenesisHash, `{"auditId":"test-audit","page":1,"totals":{"auditId":"test-audit","pages":1}}`, time.Now())
	receipt := row.public(true)
	b, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" || string(b) == row.ResultJSON {
		t.Fatal("raw page JSON leaked")
	}
	if row.RequestHash == "cursor-sensitive" {
		t.Fatal("raw continuation leaked")
	}
}

func TestPhase32LegacyAuditResume(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(40)
	now := time.Now().UTC()
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Table("knowledge_vector_audit_pages").Where("audit_id = ?", first.AuditID).Delete(&orphanAuditPageReceiptRow{}).Error; err != nil {
		t.Fatal(err)
	}
	next, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, scan, load)
	if err != nil || !next.Finished {
		t.Fatalf("legacy session cannot continue: %+v %v", next, err)
	}
	svc := &service{repo: repo}
	receipts, err := svc.ListOrphanAuditPageReceipts(context.Background(), "tenant-A", first.AuditID, 0, 10)
	if err != nil || len(receipts.Items) != 1 || receipts.ChainAnchored {
		t.Fatalf("legacy chain incorrectly marked complete: %+v %v", receipts, err)
	}
}
