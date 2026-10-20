package knowledge

import (
	"fmt"
	"testing"
)

func BenchmarkMMRSelectCandidates500(b *testing.B) {
	candidates := make([]evidenceCandidate, 0, 500)
	for i := 0; i < 500; i++ {
		content := fmt.Sprintf("knowledge benchmark evidence %03d stable fact group %02d", i, i%40)
		candidates = append(candidates, evidenceCandidate{
			matched: Chunk{ID: fmt.Sprintf("chunk-%03d", i), SourceID: fmt.Sprintf("source-%02d", i%20), Content: content},
			readID:  fmt.Sprintf("chunk-%03d", i), relevance: 1 - float64(i)/1000,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = selectMMR(candidates, 20)
	}
}

func BenchmarkBuildDeepQueryPlan(b *testing.B) {
	req := SearchRequest{Query: "比较 Amitia 知识库 revision rollback 与 reindex 的一致性、延迟和失败恢复差异", RetrievalLevel: RetrievalLevelDeep}
	settings := defaultRuntimeSettings()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildQueryPlan(req, 12, settings); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMMRSelectCandidates5000(b *testing.B) {
	candidates := make([]evidenceCandidate, 0, 5000)
	for i := 0; i < 5000; i++ {
		content := fmt.Sprintf("knowledge stress evidence %05d fact-group-%03d source-group-%03d", i, i%240, i%180)
		candidates = append(candidates, evidenceCandidate{
			matched: Chunk{ID: fmt.Sprintf("stress-chunk-%05d", i), SourceID: fmt.Sprintf("stress-source-%03d", i%180), Content: content},
			readID:  fmt.Sprintf("stress-chunk-%05d", i), relevance: 1 - float64(i)/10000, features: similarityFeatures(content),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := selectMMR(candidates, 24)
		if len(got) != 24 {
			b.Fatalf("expected 24 results, got %d", len(got))
		}
	}
}
