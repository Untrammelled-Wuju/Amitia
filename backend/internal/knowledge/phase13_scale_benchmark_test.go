package knowledge

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func benchmarkCandidates(size int) []evidenceCandidate {
	candidates := make([]evidenceCandidate, 0, size)
	for i := 0; i < size; i++ {
		content := fmt.Sprintf("amitia phase13 scale evidence %06d fact-group-%04d source-group-%04d stable benchmark tokens", i, i%1024, i%512)
		candidates = append(candidates, evidenceCandidate{
			matched:   Chunk{ID: fmt.Sprintf("scale-chunk-%06d", i), SourceID: fmt.Sprintf("scale-source-%04d", i%512), Content: content},
			readID:    fmt.Sprintf("scale-chunk-%06d", i),
			relevance: 1 - float64(i)/float64(size+1),
			features:  similarityFeatures(content),
		})
	}
	return candidates
}

func BenchmarkKnowledgeMMRScale(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		size := size
		b.Run(fmt.Sprintf("chunks_%d", size), func(b *testing.B) {
			if size >= 100_000 && os.Getenv("AMITIA_HEAVY_BENCH") != "1" {
				b.Skip("set AMITIA_HEAVY_BENCH=1 to run 100k scale benchmark")
			}
			candidates := benchmarkCandidates(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got := selectMMR(candidates, 24)
				if len(got) != 24 {
					b.Fatalf("expected 24 selected evidence items, got %d", len(got))
				}
			}
		})
	}
}

func newKnowledgeBenchmarkRepository(b *testing.B) *Repository {
	b.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		b.Fatalf("open sqlite benchmark database: %v", err)
	}
	if err := db.AutoMigrate(&Source{}, &SourceRevision{}, &DocumentUnit{}, &Chunk{}, &IngestJob{}, &IndexOutbox{}, &IndexAlias{}); err != nil {
		b.Fatalf("migrate benchmark schema: %v", err)
	}
	return NewRepository(db)
}

func seedBenchmarkLocatorUnits(b *testing.B, repo *Repository, count int) {
	b.Helper()
	now := time.Now().UTC()
	source := Source{
		ID: "phase13-reader-source", TenantID: "phase13-tenant", LogicalName: "phase13-reader", Filename: "huge.pdf",
		MimeType: "application/pdf", SourceType: "pdf", StorageKey: "phase13/huge.pdf", SHA256: "phase13-source-sha", Revision: 1,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: SourceStatusReady, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&source).Error; err != nil {
		b.Fatalf("seed benchmark source: %v", err)
	}
	revision := SourceRevision{
		ID: sourceRevisionID(source.ID, 1), TenantID: source.TenantID, SourceID: source.ID, Revision: 1,
		Filename: source.Filename, MimeType: source.MimeType, SourceType: source.SourceType, StorageKey: source.StorageKey, SHA256: source.SHA256,
		ParserVersion: ParserVersion, ChunkSchemaVersion: ChunkSchemaVersion, EmbeddingVersion: "active", IndexVersion: DefaultIndexVersion,
		Status: RevisionStatusReady, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.DB().Create(&revision).Error; err != nil {
		b.Fatalf("seed benchmark revision: %v", err)
	}
	const batchSize = 500
	for start := 0; start < count; start += batchSize {
		end := start + batchSize
		if end > count {
			end = count
		}
		units := make([]DocumentUnit, 0, end-start)
		for i := start; i < end; i++ {
			units = append(units, DocumentUnit{
				ID: fmt.Sprintf("phase13-unit-%06d", i), TenantID: source.TenantID, SourceID: source.ID, SourceRevision: 1,
				Ordinal: int64(i), UnitType: "page", Title: fmt.Sprintf("Page %d", i+1), Content: fmt.Sprintf("canonical page %d", i+1),
				LocatorJSON: fmt.Sprintf(`{"page":%d}`, i+1), MetadataJSON: `{}`, Checksum: fmt.Sprintf("sum-%d", i), CreatedAt: now,
			})
		}
		if err := repo.DB().CreateInBatches(&units, batchSize).Error; err != nil {
			b.Fatalf("seed locator units %d..%d: %v", start, end, err)
		}
	}
}

func BenchmarkCanonicalReaderLocatorScale(b *testing.B) {
	for _, size := range []int{1_000, 10_000} {
		size := size
		b.Run(fmt.Sprintf("pages_%d", size), func(b *testing.B) {
			repo := newKnowledgeBenchmarkRepository(b)
			seedBenchmarkLocatorUnits(b, repo, size)
			svc := &service{repo: repo}
			req := ReadRequest{SourceID: "phase13-reader-source", SourceRevision: 1, Locator: map[string]any{"page": size}, MaxUnits: 1, MaxRunes: 512}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := svc.Read(context.Background(), "phase13-tenant", req)
				if err != nil {
					b.Fatalf("read last locator: %v", err)
				}
				if got.AnchorUnitID != fmt.Sprintf("phase13-unit-%06d", size-1) {
					b.Fatalf("reader resolved wrong anchor: %s", got.AnchorUnitID)
				}
			}
		})
	}
}
