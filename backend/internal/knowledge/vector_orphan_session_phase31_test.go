package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func phase31Repository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:phase31-"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE knowledge_vector_audit_sessions (
        id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, collections_hash TEXT NOT NULL,
        page_limit INTEGER NOT NULL, page INTEGER NOT NULL, next_cursor TEXT NOT NULL,
        last_cursor TEXT NOT NULL, last_result_json TEXT NOT NULL,
        sampled INTEGER NOT NULL, suspected_orphans INTEGER NOT NULL,
        metadata_mismatches INTEGER NOT NULL, in_flight INTEGER NOT NULL,
        status TEXT NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
        expires_at DATETIME NOT NULL
    )`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE knowledge_vector_audit_pages (
        audit_id TEXT NOT NULL, tenant_id TEXT NOT NULL, page INTEGER NOT NULL,
        request_hash TEXT NOT NULL, result_json TEXT NOT NULL, result_hash TEXT NOT NULL,
        previous_hash TEXT NOT NULL, receipt_hash TEXT NOT NULL, created_at DATETIME NOT NULL,
        PRIMARY KEY(audit_id, page)
    )`).Error; err != nil {
		t.Fatal(err)
	}
	return &Repository{db: db}
}

func TestPhase31PersistedAuditPaginationAndReplay(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(62)
	now := time.Date(2026, 10, 16, 17, 0, 0, 0, time.UTC)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 50, "", now, scan, load)
	if err != nil || first.Totals == nil || first.Totals.Sampled != 50 || first.Totals.Pages != 1 || first.NextCursor == "" {
		t.Fatalf("first persisted page: %+v %v", first, err)
	}
	second, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 50, first.NextCursor, now, scan, load)
	if err != nil || second.Totals == nil || second.Totals.Sampled != 62 || second.Totals.Pages != 2 || second.Totals.SuspectedOrphans != 62 || !second.Finished {
		t.Fatalf("second persisted page: %+v %v", second, err)
	}
	failScan := func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
		return nil, "", errors.New("a replay must not rescan")
	}
	replay, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 50, first.NextCursor, now, failScan, load)
	if err != nil || replay.Totals == nil || replay.Totals.Sampled != 62 || replay.Page != 2 || replay.AuditID != first.AuditID {
		t.Fatalf("repeated page must return committed result: %+v %v", replay, err)
	}
	row, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil || row.Status != "finished" || row.Page != 2 || row.Sampled != 62 {
		t.Fatalf("durable session mismatch: %+v %v", row, err)
	}
}

func TestPhase31RejectsForgedContinuationAndScopeChange(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(62)
	now := time.Date(2026, 10, 16, 17, 0, 0, 0, time.UTC)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	state, err := readOrphanAuditCursor(first.NextCursor, "tenant-A", []string{"collection-A"}, now)
	if err != nil {
		t.Fatal(err)
	}
	state.Page = 3
	fake, err := makeOrphanAuditCursor(state)
	if err != nil {
		t.Fatal(err)
	}
	state.Page = 1
	state.Offset = "11111111-1111-4111-8111-000000000045"
	changedOffset, err := makeOrphanAuditCursor(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name, tenant, collection, cursor string
		limit                            int
		time                             time.Time
	}{
		{"forged-page", "tenant-A", "collection-A", fake, 25, now},
		{"forged-offset", "tenant-A", "collection-A", changedOffset, 25, now},
		{"foreign-tenant", "tenant-B", "collection-A", first.NextCursor, 25, now},
		{"changed-collection", "tenant-A", "collection-B", first.NextCursor, 25, now},
		{"changed-limit", "tenant-A", "collection-A", first.NextCursor, 50, now},
		{"expired", "tenant-A", "collection-A", first.NextCursor, 25, now.Add(31 * time.Minute)},
	} {
		t.Run(item.name, func(t *testing.T) {
			if _, err := repo.auditOrphanPageWithStore(context.Background(), item.tenant, []string{item.collection}, item.limit, item.cursor, item.time, scan, load); err == nil {
				t.Fatal("unsafe cursor accepted")
			}
		})
	}
}

func TestPhase31FailedPageDoesNotCommit(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(40)
	now := time.Date(2026, 10, 16, 17, 0, 0, 0, time.UTC)
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	badScan := func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
		return nil, "", fmt.Errorf("Qdrant unavailable")
	}
	if _, err = repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, badScan, load); err == nil {
		t.Fatal("failed scan succeeded")
	}
	row, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", first.AuditID)
	if err != nil || row.Page != 1 || row.Sampled != 25 || row.NextCursor != first.NextCursor {
		t.Fatalf("failed page modified checkpoint: %+v %v", row, err)
	}
	retried, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, first.NextCursor, now, scan, load)
	if err != nil || retried.Totals == nil || retried.Totals.Pages != 2 || retried.Totals.Sampled != 40 {
		t.Fatalf("resume after transient failure: %+v %v", retried, err)
	}
}

func TestPhase31AuditHistoryTenantIsolation(t *testing.T) {
	repo := phase31Repository(t)
	_, scan, load := phase30PageFixture(5)
	now := time.Now().UTC()
	first, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil {
		t.Fatal(err)
	}
	s := &service{repo: repo}
	items, err := s.ListOrphanAuditSessions(context.Background(), "tenant-A", 10)
	if err != nil || len(items) != 1 || items[0].AuditID != first.AuditID || items[0].Sampled != 5 {
		t.Fatalf("missing own history: %+v %v", items, err)
	}
	foreign, err := s.ListOrphanAuditSessions(context.Background(), "tenant-B", 10)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign history leaked: %+v %v", foreign, err)
	}
	b, err := json.Marshal(items[0])
	if err != nil || strings.Contains(string(b), "nextCursor") || strings.Contains(string(b), "lastResultJSON") {
		t.Fatal("history leaked continuation or raw result")
	}
}

func TestPhase31EmptyCollectionSession(t *testing.T) {
	repo := phase31Repository(t)
	now := time.Now().UTC()
	failScan := func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
		return nil, "", errors.New("unexpected Qdrant scan")
	}
	failLoad := func(context.Context, string, []qdrantDB.VectorMetadata) (orphanVectorState, error) {
		return orphanVectorState{}, errors.New("unexpected canonical read")
	}
	result, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", nil, 25, "", now, failScan, failLoad)
	if err != nil || !result.Finished || result.Page != 1 || result.Totals == nil || result.Totals.Pages != 1 || result.Totals.Sampled != 0 {
		t.Fatalf("empty collection audit must finish with persistent receipt: %+v %v", result, err)
	}
	row, err := repo.getOrphanAuditSession(context.Background(), "tenant-A", result.AuditID)
	if err != nil || row.Status != "finished" || row.Sampled != 0 {
		t.Fatalf("empty audit receipt missing: %+v %v", row, err)
	}
}
