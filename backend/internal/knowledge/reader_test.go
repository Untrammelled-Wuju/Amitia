package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

func seedReadableKnowledgeSource(t *testing.T, repo *Repository, tenantID, sourceID string, revision int64, status string) {
	t.Helper()
	now := time.Now().UTC()
	source := Source{
		ID: sourceID, TenantID: tenantID, LogicalName: "reader", Filename: "reader.pdf",
		MimeType: "application/pdf", SourceType: "pdf", StorageKey: "reader/key", SHA256: "source-sha", Revision: revision,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: SourceStatusReady, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&source).Error; err != nil {
		t.Fatalf("seed source: %v", err)
	}
	rev := SourceRevision{
		ID: sourceRevisionID(sourceID, revision), TenantID: tenantID, SourceID: sourceID, Revision: revision,
		Filename: source.Filename, MimeType: source.MimeType, SourceType: source.SourceType, StorageKey: source.StorageKey, SHA256: source.SHA256,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: status, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&rev).Error; err != nil {
		t.Fatalf("seed revision: %v", err)
	}
}

func seedReaderUnit(t *testing.T, repo *Repository, tenantID, sourceID string, revision, ordinal int64, id, content, locator string) DocumentUnit {
	t.Helper()
	unit := DocumentUnit{
		ID: id, TenantID: tenantID, SourceID: sourceID, SourceRevision: revision, Ordinal: ordinal,
		UnitType: "page", Title: "Page", Content: content, LocatorJSON: locator, MetadataJSON: `{}`,
		Checksum: "sum-" + id, CreatedAt: time.Now().UTC(),
	}
	if err := repo.DB().Create(&unit).Error; err != nil {
		t.Fatalf("seed unit %s: %v", id, err)
	}
	return unit
}

func TestCanonicalReaderResolvesEvidenceAndCentersTruncation(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	prefix := strings.Repeat("前", 180)
	anchor := "这是必须保留的精确命中片段"
	suffix := strings.Repeat("后", 180)
	unit := seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", prefix+anchor+suffix, `{"page":37}`)
	chunk := Chunk{
		ID: "chunk-a", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, UnitID: unit.ID,
		ChunkIndex: 0, Content: anchor, EmbeddingText: anchor, LocatorJSON: `{"page":37}`, MetadataJSON: `{}`,
		Checksum: "chunk-sum", EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := repo.DB().Create(&chunk).Error; err != nil {
		t.Fatalf("seed chunk: %v", err)
	}

	svc := &service{repo: repo}
	got, err := svc.Read(context.Background(), "tenant-a", ReadRequest{EvidenceID: chunk.ID, MaxUnits: 1, MaxRunes: 96})
	if err != nil {
		t.Fatalf("read by evidence: %v", err)
	}
	if got.AnchorUnitID != unit.ID || len(got.Units) != 1 || !got.Truncated {
		t.Fatalf("unexpected canonical read result: %#v", got)
	}
	if !strings.Contains(got.Units[0].Content, anchor) {
		t.Fatalf("truncation lost retrieval anchor: %q", got.Units[0].Content)
	}
	if got.TotalRunes > 98 { // ellipsis can add at most two runes around the bounded slice.
		t.Fatalf("read exceeded rune budget: %d", got.TotalRunes)
	}
}

func TestCanonicalReaderIsTenantAndRevisionScoped(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	unit := seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", "private canonical text", `{"page":1}`)
	svc := &service{repo: repo}

	if _, err := svc.Read(context.Background(), "tenant-b", ReadRequest{UnitID: unit.ID, MaxUnits: 1}); err == nil {
		t.Fatal("cross-tenant canonical unit read must be rejected")
	}
	if _, err := svc.Read(context.Background(), "tenant-a", ReadRequest{UnitID: unit.ID, SourceID: "other-source", MaxUnits: 1}); err == nil {
		t.Fatal("source substitution must be rejected")
	}
	if _, err := svc.Read(context.Background(), "tenant-a", ReadRequest{UnitID: unit.ID, SourceRevision: 2, MaxUnits: 1}); err == nil {
		t.Fatal("revision substitution must be rejected")
	}
}

func TestCanonicalReaderRejectsPurgedOrUnreadyRevision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
		purged bool
	}{
		{name: "processing", status: RevisionStatusProcessing},
		{name: "purged_status", status: RevisionStatusPurged},
		{name: "purged_timestamp", status: RevisionStatusReady, purged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newKnowledgeTestRepository(t)
			seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, tc.status)
			if tc.purged {
				now := time.Now().UTC()
				if err := repo.DB().Model(&SourceRevision{}).
					Where("tenant_id = ? AND source_id = ? AND revision = ?", "tenant-a", "source-a", 1).
					Update("purged_at", now).Error; err != nil {
					t.Fatalf("mark purged: %v", err)
				}
			}
			unit := seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", "text", `{"page":1}`)
			svc := &service{repo: repo}
			if _, err := svc.Read(context.Background(), "tenant-a", ReadRequest{UnitID: unit.ID, MaxUnits: 1}); err == nil {
				t.Fatalf("revision status %q purged=%v must not be readable", tc.status, tc.purged)
			}
		})
	}
}

func TestCanonicalReaderFindsStructuredLocatorAliases(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "sheet-source", 1, RevisionStatusReadyDegraded)
	seedReaderUnit(t, repo, "tenant-a", "sheet-source", 1, 0, "row-145", "wrong row", `{"sheetName":"Data","rowStart":145,"rowEnd":145}`)
	want := seedReaderUnit(t, repo, "tenant-a", "sheet-source", 1, 1, "row-146", "target row", `{"sheetName":"Data","rowStart":146,"rowEnd":146}`)

	svc := &service{repo: repo}
	got, err := svc.Read(context.Background(), "tenant-a", ReadRequest{
		SourceID: "sheet-source", SourceRevision: 1, Locator: map[string]any{"sheet": "Data", "row": 146}, MaxUnits: 1, MaxRunes: 500,
	})
	if err != nil {
		t.Fatalf("read locator: %v", err)
	}
	if got.AnchorUnitID != want.ID || len(got.Units) != 1 || got.Units[0].ID != want.ID {
		t.Fatalf("locator resolved wrong canonical unit: %#v", got)
	}
}

func TestCanonicalReaderLocatorScansBeyondFirstBatch(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "large-source", 1, RevisionStatusReady)
	now := time.Now().UTC()
	units := make([]DocumentUnit, 0, 2051)
	for i := 0; i < 2050; i++ {
		units = append(units, DocumentUnit{
			ID: fmt.Sprintf("large-unit-%04d", i), TenantID: "tenant-a", SourceID: "large-source", SourceRevision: 1,
			Ordinal: int64(i), UnitType: "page", Title: "Page", Content: "decoy",
			LocatorJSON: fmt.Sprintf(`{"page":%d}`, i+1), MetadataJSON: `{}`, Checksum: fmt.Sprintf("sum-%d", i), CreatedAt: now,
		})
	}
	units = append(units, DocumentUnit{
		ID: "large-unit-target", TenantID: "tenant-a", SourceID: "large-source", SourceRevision: 1,
		Ordinal: 3000, UnitType: "page", Title: "Page 3001", Content: "target beyond first locator scan batch",
		LocatorJSON: `{"page":3001}`, MetadataJSON: `{}`, Checksum: "sum-target", CreatedAt: now,
	})
	if err := repo.DB().CreateInBatches(&units, 250).Error; err != nil {
		t.Fatalf("seed large canonical units: %v", err)
	}

	svc := &service{repo: repo}
	got, err := svc.Read(context.Background(), "tenant-a", ReadRequest{
		SourceID: "large-source", SourceRevision: 1, Locator: map[string]any{"page": 3001}, MaxUnits: 1, MaxRunes: 500,
	})
	if err != nil {
		t.Fatalf("read locator beyond first batch: %v", err)
	}
	if got.AnchorUnitID != "large-unit-target" || len(got.Units) != 1 || got.Units[0].Content != "target beyond first locator scan batch" {
		t.Fatalf("large locator scan resolved wrong unit: %#v", got)
	}
}

func TestPhase21CanonicalReaderRejectsInvalidLocatorBeforeResolvingUnits(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "private-unit", "private document", `{"page":1}`)
	svc := &service{repo: repo}
	cases := []struct {
		name    string
		locator map[string]any
	}{
		{"empty key", map[string]any{"": "anything"}},
		{"whitespace key", map[string]any{" ": "anything"}},
		{"trimmed key collision", map[string]any{" page ": 1}},
		{"nested object", map[string]any{"page": map[string]any{"number": 1}}},
		{"array", map[string]any{"page": []int{1}}},
		{"empty string", map[string]any{"page": ""}},
		{"control byte", map[string]any{"page": "1\n2"}},
		{"too many fields", func() map[string]any {
			m := map[string]any{}
			for i := 0; i < 17; i++ {
				m[fmt.Sprintf("extra-%d", i)] = "x"
			}
			return m
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Read(context.Background(), "tenant-a", ReadRequest{
				SourceID: "source-a", SourceRevision: 1, Locator: tc.locator, MaxUnits: 1,
			}); err == nil {
				t.Fatalf("invalid locator matched canonical source: %+v", tc.locator)
			}
			if matches, err := repo.FindUnitsByLocator(context.Background(), "tenant-a", "source-a", 1, tc.locator, 1); err == nil || len(matches) != 0 {
				t.Fatalf("repository bypassed input validation: %+v err=%v", matches, err)
			}
		})
	}
	if _, err := svc.Read(context.Background(), "tenant-a", ReadRequest{
		SourceID: "source-a", SourceRevision: 1, Locator: map[string]any{"page": float64(1)}, MaxUnits: 1,
	}); err != nil {
		t.Fatalf("canonical numeric locator regression: %v", err)
	}
}

func TestPhase21CanonicalReaderRuneBudgetIncludesEllipses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		anchor  string
		limit   int
	}{
		{"unicode near start", "正文" + strings.Repeat("后", 120), "正文", 18},
		{"unicode near center", strings.Repeat("前", 60) + "关键引用" + strings.Repeat("后", 60), "关键引用", 20},
		{"unicode near end", strings.Repeat("前", 120) + "关键引用", "关键引用", 18},
		{"ascii minimum", "abcdefghijklmnop", "", 1},
		{"ascii two", "abcdefghijklmnop", "", 2},
		{"short excerpt", "abc", "", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := truncateAroundAnchor(tc.content, tc.anchor, tc.limit)
			if utf8.RuneCountInString(actual) > tc.limit {
				t.Fatalf("truncated excerpt exceeded rune budget: %d > %d", utf8.RuneCountInString(actual), tc.limit)
			}
			if tc.anchor != "" && tc.limit >= utf8.RuneCountInString(tc.anchor)+2 && !strings.Contains(actual, tc.anchor) {
				t.Fatalf("truncation lost anchor: %q", actual)
			}
		})
	}
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	unit := seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "long-unit", strings.Repeat("前", 150)+"精确引用"+strings.Repeat("后", 150), `{"page":42}`)
	if err := repo.DB().Create(&Chunk{
		ID: "long-chunk", TenantID: "tenant-a", SourceID: "source-a", SourceRevision: 1, UnitID: unit.ID,
		ChunkIndex: 0, Content: "精确引用", EmbeddingText: "精确引用", LocatorJSON: `{"page":42}`, MetadataJSON: `{}`,
		EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion, Status: "indexed", CreatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := (&service{repo: repo}).Read(context.Background(), "tenant-a", ReadRequest{EvidenceID: "long-chunk", MaxUnits: 1, MaxRunes: 24})
	if err != nil || len(got.Units) != 1 {
		t.Fatalf("reader failed to bound excerpts: %+v err=%v", got, err)
	}
	if got.TotalRunes > 24 || utf8.RuneCountInString(got.Units[0].Content) > 24 || !strings.Contains(got.Units[0].Content, "精确引用") {
		t.Fatalf("read budget or anchor mismatch: %+v", got)
	}
}

func TestPhase21CancelledCanonicalReadDoesNotResolveSource(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", "private data", `{"page":1}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&service{repo: repo}).Read(ctx, "tenant-a", ReadRequest{UnitID: "unit-a", MaxUnits: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled canonical read must stop before database access: %v", err)
	}
}

func TestPhase21LocatorRepositoryRejectsUnreadableRevisions(t *testing.T) {
	for _, status := range []string{RevisionStatusProcessing, RevisionStatusFailed, RevisionStatusCancelled, RevisionStatusPurged} {
		t.Run(status, func(t *testing.T) {
			repo := newKnowledgeTestRepository(t)
			seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, status)
			seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-a", "private data", `{"page":1}`)
			got, err := repo.FindUnitsByLocator(context.Background(), "tenant-a", "source-a", 1, map[string]any{"page": 1}, 1)
			if !errors.Is(err, gorm.ErrRecordNotFound) || len(got) != 0 {
				t.Fatalf("unreadable revision exposed canonical locator units: %+v err=%v", got, err)
			}
		})
	}
}
