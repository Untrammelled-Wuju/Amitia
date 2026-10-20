package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func phase30PageFixture(count int) ([]qdrantDB.VectorMetadata, orphanVectorPageScanner, orphanStateLoader) {
	items := make([]qdrantDB.VectorMetadata, count)
	for i := range items {
		id := fmt.Sprintf("11111111-1111-4111-8111-%012x", i+1)
		items[i] = phase27Point(id, "tenant-A", "source-A", "1", DefaultIndexVersion)
	}
	scan := func(_ context.Context, tenant, collection string, limit int, offset string) ([]qdrantDB.VectorMetadata, string, error) {
		if tenant != "tenant-A" || collection != "collection-A" || limit > 25 || limit < 1 {
			return nil, "", fmt.Errorf("unbounded or foreign scan")
		}
		start := 0
		if offset != "" {
			start = -1
			for i, item := range items {
				if item.ID == offset {
					start = i
					break
				}
			}
			if start < 0 {
				return nil, "", fmt.Errorf("unknown offset")
			}
		}
		end := start + limit
		if end > len(items) {
			end = len(items)
		}
		next := ""
		if end < len(items) {
			next = items[end].ID
		}
		return items[start:end], next, nil
	}
	load := func(_ context.Context, tenant string, points []qdrantDB.VectorMetadata) (orphanVectorState, error) {
		if tenant != "tenant-A" {
			return orphanVectorState{}, fmt.Errorf("foreign canonical lookup")
		}
		return orphanVectorState{Chunks: map[string]orphanVectorRow{}, Pending: map[string]bool{}}, nil
	}
	return items, scan, load
}

func TestPhase30OrphanPageBoundedContinuation(t *testing.T) {
	_, scan, load := phase30PageFixture(62)
	now := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	cursor := ""
	seen := 0
	auditID := ""
	for page := 1; page <= 4; page++ {
		result, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", []string{"collection-A"}, 50, cursor, now, scan, load)
		if err != nil {
			t.Fatal(err)
		}
		if result.Page != page || result.AuditID == "" || result.SuspectedOrphans != result.Sampled {
			t.Fatalf("wrong page: %+v", result)
		}
		if auditID != "" && auditID != result.AuditID {
			t.Fatal("audit session changed")
		}
		auditID = result.AuditID
		seen += result.Sampled
		if result.Finished {
			if page != 2 || seen != 62 || result.NextCursor != "" {
				t.Fatalf("did not traverse whole collection: %+v total=%d", result, seen)
			}
			return
		}
		if result.NextCursor == "" || result.Sampled != 50 {
			t.Fatalf("pagination stalled: %+v", result)
		}
		cursor = result.NextCursor
	}
	t.Fatal("scan did not finish")
}

func TestPhase30OrphanPageCollectionWindow(t *testing.T) {
	calls := 0
	collections := []string{"a", "b", "c", "d", "e", "f"}
	scan := func(_ context.Context, tenant, collection string, limit int, offset string) ([]qdrantDB.VectorMetadata, string, error) {
		calls++
		if tenant != "tenant-A" || offset != "" || limit > 25 {
			t.Fatal("invalid scan scope")
		}
		return nil, "", nil
	}
	load := func(context.Context, string, []qdrantDB.VectorMetadata) (orphanVectorState, error) {
		return orphanVectorState{Chunks: map[string]orphanVectorRow{}, Pending: map[string]bool{}}, nil
	}
	now := time.Now().UTC()
	first, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", collections, 25, "", now, scan, load)
	if err != nil || first.CollectionsChecked != 4 || first.Finished || first.NextCursor == "" || calls != 4 {
		t.Fatalf("first collection window: %+v err=%v", first, err)
	}
	second, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", collections, 25, first.NextCursor, now, scan, load)
	if err != nil || second.CollectionsChecked != 2 || !second.Finished || second.NextCursor != "" || calls != 6 {
		t.Fatalf("second collection window: %+v err=%v", second, err)
	}
}

func TestPhase30OrphanCursorIsolationAndExpiry(t *testing.T) {
	_, scan, load := phase30PageFixture(30)
	now := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	first, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", []string{"collection-A"}, 20, "", now, scan, load)
	if err != nil || first.NextCursor == "" {
		t.Fatalf("failed to create cursor: %+v %v", first, err)
	}
	for _, tc := range []struct {
		name        string
		tenant      string
		collections []string
		cursor      string
		now         time.Time
	}{
		{"tenant", "tenant-B", []string{"collection-A"}, first.NextCursor, now},
		{"collections", "tenant-A", []string{"collection-B"}, first.NextCursor, now},
		{"expired", "tenant-A", []string{"collection-A"}, first.NextCursor, now.Add(31 * time.Minute)},
		{"garbage", "tenant-A", []string{"collection-A"}, "not-a-cursor", now},
		{"too_long", "tenant-A", []string{"collection-A"}, strings.Repeat("x", 1025), now},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := auditTenantOrphanVectorPage(context.Background(), tc.tenant, tc.collections, 20, tc.cursor, tc.now, scan, load); err == nil {
				t.Fatal("invalid continuation accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := auditTenantOrphanVectorPage(ctx, "tenant-A", []string{"collection-A"}, 20, "", now, scan, load); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request accepted: %v", err)
	}
}

func TestPhase30OrphanPageScannerFailClosed(t *testing.T) {
	_, scan, load := phase30PageFixture(30)
	now := time.Now()
	badCases := []struct {
		name string
		scan orphanVectorPageScanner
		load orphanStateLoader
	}{
		{"oversized", func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
			return make([]qdrantDB.VectorMetadata, 51), "", nil
		}, load},
		{"invalid_offset", func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
			return []qdrantDB.VectorMetadata{phase27Point("foo", "tenant-A", "source-A", "1", "v1")}, "INVALID", nil
		}, load},
		{"foreign_point", func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
			return []qdrantDB.VectorMetadata{phase27Point("foo", "tenant-B", "source-A", "1", "v1")}, "", nil
		}, load},
		{"scanner_error", func(context.Context, string, string, int, string) ([]qdrantDB.VectorMetadata, string, error) {
			return nil, "", errors.New("qdrant offline")
		}, load},
		{"canonical_error", scan, func(context.Context, string, []qdrantDB.VectorMetadata) (orphanVectorState, error) {
			return orphanVectorState{}, errors.New("sqlite offline")
		}},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, tc.scan, tc.load); err == nil {
				t.Fatal("unsafe scan accepted")
			}
		})
	}
}

func TestPhase30OrphanPageMaxPageBoundary(t *testing.T) {
	_, scan, load := phase30PageFixture(30)
	now := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	first, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", []string{"collection-A"}, 5, "", now, scan, load)
	if err != nil || first.NextCursor == "" {
		t.Fatalf("missing initial continuation: %+v %v", first, err)
	}
	state, err := readOrphanAuditCursor(first.NextCursor, "tenant-A", []string{"collection-A"}, now)
	if err != nil {
		t.Fatal(err)
	}
	state.Page = maxOrphanAuditPages - 1
	cursor, err := makeOrphanAuditCursor(state)
	if err != nil {
		t.Fatal(err)
	}
	last, err := auditTenantOrphanVectorPage(context.Background(), "tenant-A", []string{"collection-A"}, 5, cursor, now, scan, load)
	if err != nil || last.Page != maxOrphanAuditPages || !last.Truncated || last.Finished || last.NextCursor != "" {
		t.Fatalf("max-page boundary misreported: %+v err=%v", last, err)
	}
}
