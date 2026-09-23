package webresearch

import (
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

func BenchmarkFuseResults100(b *testing.B) {
	observations := make([]searchObservation, 0, 100)
	for i := 0; i < 100; i++ {
		observations = append(observations, searchObservation{
			result:      search.SearchResult{Title: fmt.Sprintf("Result %d", i), URL: fmt.Sprintf("https://d%d.example/page/%d?utm_source=bench", i%20, i), Rank: i%10 + 1},
			query:       "agent web research runtime",
			provider:    fmt.Sprintf("p%d", i%3),
			retrievedAt: time.Unix(1700000000, 0),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fuseResults(observations, 20, 3)
	}
}

func BenchmarkMergeResearchFindings(b *testing.B) {
	plan := ResearchPlan{Questions: make([]ResearchQuestion, 0, 12)}
	for i := 0; i < 12; i++ {
		plan.Questions = append(plan.Questions, ResearchQuestion{ID: fmt.Sprintf("q%d", i), Question: fmt.Sprintf("architecture topic %d", i), Required: true})
	}
	citations := make([]Citation, 0, 80)
	for i := 0; i < 80; i++ {
		citations = append(citations, Citation{EvidenceID: fmt.Sprintf("e%d", i), RefID: fmt.Sprintf("r%d", i%20), Text: fmt.Sprintf("architecture topic %d evidence detail", i%12)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mergeResearchFindings(plan, citations)
	}
}
