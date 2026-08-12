package search

import (
	"testing"
	"time"
)

func TestCitationBuilder_Build_DeterministicID(t *testing.T) {
	builder := NewCitationBuilder()
	results := []SearchResult{
		{
			Rank:   1,
			URL:    "https://example.com/page1",
			Domain: "example.com",
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/page1",
				RetrievedAt:  time.Now().UTC(),
			},
		},
		{
			Rank:   2,
			URL:    "https://example.com/page1",
			Domain: "example.com",
			Citation: CitationRef{},
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/page1",
				RetrievedAt:  time.Now().UTC(),
			},
		},
	}

	set := builder.Build(results, SearchKindWeb)
	if len(set.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(set.Citations))
	}
	if set.Citations[0].ID == "" {
		t.Fatal("expected non-empty citation ID")
	}

	set2 := builder.Build(results, SearchKindWeb)
	if set.Citations[0].ID != set2.Citations[0].ID {
		t.Fatalf("citation ID not deterministic: %s vs %s", set.Citations[0].ID, set2.Citations[0].ID)
	}
}

func TestCitationBuilder_Build_DifferentURLs(t *testing.T) {
	builder := NewCitationBuilder()
	results := []SearchResult{
		{
			URL: "https://example.com/a",
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/a",
			},
		},
		{
			URL: "https://example.com/b",
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/b",
			},
		},
	}

	set := builder.Build(results, SearchKindWeb)
	if len(set.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(set.Citations))
	}
	if set.Citations[0].ID == set.Citations[1].ID {
		t.Fatal("different URLs should produce different citation IDs")
	}
}

func TestCitationBuilder_AssignCitations(t *testing.T) {
	builder := NewCitationBuilder()
	results := []SearchResult{
		{
			URL: "https://example.com/a",
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/a",
			},
		},
		{
			URL: "https://example.com/b",
			Source: SearchSourceMetadata{
				Provider:     "brave",
				CanonicalURL: "https://example.com/b",
			},
		},
	}

	set := builder.Build(results, SearchKindWeb)
	builder.AssignCitations(results, set)

	if results[0].Citation.ID == "" {
		t.Fatal("expected citation ID on first result")
	}
	if results[1].Citation.ID == "" {
		t.Fatal("expected citation ID on second result")
	}
	if results[0].Citation.Index != 1 || results[1].Citation.Index != 2 {
		t.Fatalf("unexpected indices: %d, %d", results[0].Citation.Index, results[1].Citation.Index)
	}
}

func TestCitationBuilder_EmptyResults(t *testing.T) {
	builder := NewCitationBuilder()
	set := builder.Build([]SearchResult{}, SearchKindWeb)
	if len(set.Citations) != 0 {
		t.Fatalf("expected 0 citations, got %d", len(set.Citations))
	}
}

func TestNormalizeKind_DefaultWeb(t *testing.T) {
	if NormalizeKind("") != SearchKindWeb {
		t.Fatal("empty kind should normalize to web")
	}
	if NormalizeKind(SearchKindNews) != SearchKindNews {
		t.Fatal("news kind should remain news")
	}
}

func TestValidateSpecializedOptions_Valid(t *testing.T) {
	err := ValidateSpecializedOptions(SearchKindNews, &SpecializedSearchOptions{
		News: &NewsSearchOptions{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSpecializedOptions_Mismatch(t *testing.T) {
	err := ValidateSpecializedOptions(SearchKindWeb, &SpecializedSearchOptions{
		News: &NewsSearchOptions{},
	})
	if err == nil {
		t.Fatal("expected error for mismatched kind and specialized options")
	}
}

func TestNormalizeDomains(t *testing.T) {
	input := []string{"  Example.COM ", "invalid/url.com", ""}
	got := NormalizeDomains(input)
	if len(got) != 1 || got[0] != "example.com" {
		t.Fatalf("unexpected domains: %v", got)
	}
}

func TestSupportsKind(t *testing.T) {
	caps := ProviderCapabilities{
		GeneralWeb:  true,
		SearchKinds: []SearchKind{SearchKindNews, SearchKindAcademic},
	}
	if !SupportsKind(caps, SearchKindWeb) {
		t.Fatal("should support web via GeneralWeb")
	}
	if !SupportsKind(caps, SearchKindNews) {
		t.Fatal("should support news via SearchKinds")
	}
	if SupportsKind(caps, SearchKindImage) {
		t.Fatal("should not support image")
	}
}

func TestAssignRanks_PreservesProviderRank(t *testing.T) {
	n := NewNormalizer()
	results := []SearchResult{
		{Rank: 0, Source: SearchSourceMetadata{ProviderRank: 5}},
		{Rank: 0, Source: SearchSourceMetadata{ProviderRank: 0}},
	}
	n.AssignRanks(results)
	if results[0].Source.ProviderRank != 5 {
		t.Fatalf("expected preserved provider rank 5, got %d", results[0].Source.ProviderRank)
	}
	if results[1].Source.ProviderRank != 2 {
		t.Fatalf("expected assigned provider rank 2, got %d", results[1].Source.ProviderRank)
	}
	if results[0].Rank != 1 || results[1].Rank != 2 {
		t.Fatalf("unexpected ranks: %d, %d", results[0].Rank, results[1].Rank)
	}
}
