package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func phase34Repository(t *testing.T) *Repository {
	t.Helper()
	repo := phase31Repository(t)
	statements := []string{
		`CREATE TABLE knowledge_vector_audit_recheck_leases (audit_id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, lease_token TEXT NOT NULL, lease_expires_at DATETIME NOT NULL, next_allowed_at DATETIME NOT NULL)`,
		`CREATE TABLE knowledge_vector_audit_recheck_results (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, audit_id TEXT NOT NULL, page INTEGER NOT NULL, status TEXT NOT NULL, result_hash TEXT NOT NULL, receipt_hash TEXT NOT NULL, observed_at DATETIME NOT NULL, created_at DATETIME NOT NULL)`,
	}
	for _, sql := range statements {
		if err := repo.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestPhase34RecheckCooldownAndImmutableHistory(t *testing.T) {
	repo := phase34Repository(t)
	_, scan, load := phase30PageFixture(10)
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	initial, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context) ([]string, error) { return []string{"collection-A"}, nil }
	first, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", initial.AuditID, now, lookup, scan, load)
	if err != nil || first.Status != "matched" || first.RecheckID == "" || first.ManualRecoveryAllowed {
		t.Fatalf("first recheck: %+v %v", first, err)
	}
	if _, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", initial.AuditID, now.Add(10*time.Second), lookup, scan, load); err == nil {
		t.Fatal("cooldown allowed a duplicate scan")
	}
	svc := &service{repo: repo}
	items, err := svc.ListOrphanAuditRechecks(context.Background(), "tenant-A", initial.AuditID, 10)
	if err != nil || len(items) != 1 || items[0].ID != first.RecheckID || items[0].Status != "matched" || len(items[0].ReceiptHash) != 64 || len(items[0].ResultHash) != 64 {
		t.Fatalf("durable recheck receipt: %+v %v", items, err)
	}
	if _, err := svc.ListOrphanAuditRechecks(context.Background(), "tenant-B", initial.AuditID, 10); err == nil {
		t.Fatal("cross-tenant recheck history access")
	}
	second, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", initial.AuditID, now.Add(31*time.Second), lookup, scan, load)
	if err != nil || second.RecheckID == first.RecheckID {
		t.Fatalf("new recheck after cooldown: %+v %v", second, err)
	}
	items, err = svc.ListOrphanAuditRechecks(context.Background(), "tenant-A", initial.AuditID, 10)
	if err != nil || len(items) != 2 || items[1].ID != first.RecheckID {
		t.Fatalf("history should be append-only: %+v %v", items, err)
	}
}

func TestPhase34UnavailableScanIsRecordedAndNotApproved(t *testing.T) {
	repo := phase34Repository(t)
	_, scan, load := phase30PageFixture(1)
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	initial, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	failScan := func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
		return nil, "", errors.New("private connection details")
	}
	lookup := func(context.Context) ([]string, error) { return []string{"collection-A"}, nil }
	checked, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", initial.AuditID, now, lookup, failScan, load)
	if err == nil || checked.Status != "unavailable" || checked.ManualRecoveryAllowed {
		t.Fatalf("failed Qdrant scan incorrectly accepted: %+v %v", checked, err)
	}
	items, err := (&service{repo: repo}).ListOrphanAuditRechecks(context.Background(), "tenant-A", initial.AuditID, 10)
	if err != nil || len(items) != 1 || items[0].Status != "unavailable" {
		t.Fatalf("failure should be auditable: %+v %v", items, err)
	}
	if _, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", initial.AuditID, now.Add(time.Second), lookup, scan, load); err == nil {
		t.Fatal("failure bypassed cooldown")
	}
}

func TestPhase34RejectsForeignTenantBeforeReservingLease(t *testing.T) {
	repo := phase34Repository(t)
	_, scan, load := phase30PageFixture(1)
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	initial, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context) ([]string, error) { return []string{"collection-A"}, nil }
	if _, err := repo.recheckAuditWithLease(context.Background(), "tenant-B", initial.AuditID, now, lookup, scan, load); err == nil {
		t.Fatal("foreign tenant reserved an audit recheck")
	}
	var count int64
	if err := repo.db.Table("knowledge_vector_audit_recheck_leases").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("foreign reservation persisted: %d %v", count, err)
	}
}
