package search

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type CitationBuilder struct{}

func NewCitationBuilder() *CitationBuilder {
	return &CitationBuilder{}
}

func (b CitationBuilder) Build(results []SearchResult, kind SearchKind) CitationSet {
	if len(results) == 0 {
		return CitationSet{}
	}
	seen := make(map[string]int, len(results))
	citations := make([]Citation, 0, len(results))
	for _, r := range results {
		canon := r.Source.CanonicalURL
		if canon == "" {
			canon = r.URL
		}
		identity := citationIdentity(canon, r.Source.Provider)
		if idx, ok := seen[identity]; ok {
			c := &citations[idx]
			if r.Source.RetrievedAt.After(c.RetrievedAt) {
				c.RetrievedAt = r.Source.RetrievedAt
			}
			if r.PublishedAt != nil && c.PublishedAt == nil {
				c.PublishedAt = r.PublishedAt
			}
			continue
		}
		seen[identity] = len(citations)
		citations = append(citations, Citation{
			ID:           identity,
			Index:        len(citations) + 1,
			Title:        r.Title,
			URL:          r.URL,
			CanonicalURL: r.Source.CanonicalURL,
			Domain:       r.Domain,
			Provider:     r.Source.Provider,
			ProviderRank: r.Source.ProviderRank,
			RetrievedAt:  r.Source.RetrievedAt,
			PublishedAt:  r.PublishedAt,
			Snippet:      r.Snippet,
			Kind:         kind,
			Metadata:     r.Metadata,
		})
	}
	return CitationSet{Citations: citations}
}

func (b CitationBuilder) AssignCitations(results []SearchResult, set CitationSet) {
	if len(set.Citations) == 0 {
		return
	}
	refByID := make(map[string]int, len(set.Citations))
	for i, c := range set.Citations {
		refByID[c.ID] = i + 1
	}
	for i := range results {
		canon := results[i].Source.CanonicalURL
		if canon == "" {
			canon = results[i].URL
		}
		identity := citationIdentity(canon, results[i].Source.Provider)
		if idx, ok := refByID[identity]; ok {
			results[i].Citation = CitationRef{
				ID:    identity,
				Index: idx,
			}
		}
	}
}

func citationIdentity(canonicalURL, provider string) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(canonicalURL)))
	h.Write([]byte("\n"))
	h.Write([]byte(strings.TrimSpace(provider)))
	sum := h.Sum(nil)
	return "cit_" + hex.EncodeToString(sum)[:12]
}
