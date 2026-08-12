package search

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
var entityPattern = regexp.MustCompile(`&(?:amp|lt|gt|quot|#39|nbsp);`)
var controlCharPattern = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
var multiSpacePattern = regexp.MustCompile(`\s+`)

func sanitizeText(s string, maxRunes int) string {
	if s == "" {
		return ""
	}
	s = htmlTagPattern.ReplaceAllString(s, "")
	s = entityPattern.ReplaceAllStringFunc(s, func(entity string) string {
		switch entity {
		case "&amp;":
			return "&"
		case "&lt;":
			return "<"
		case "&gt;":
			return ">"
		case "&quot;":
			return "\""
		case "&#39;":
			return "'"
		case "&nbsp;":
			return " "
		}
		return ""
	})
	s = controlCharPattern.ReplaceAllString(s, "")
	s = multiSpacePattern.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxRunes {
		s = string([]rune(s)[:maxRunes])
	}
	return s
}

func sanitizeTitle(s string) string {
	return sanitizeText(s, MaxTitleRunes)
}

func sanitizeSnippet(s string) string {
	return sanitizeText(s, MaxSnippetRunes)
}

var blockedSchemes = map[string]bool{
	"javascript": true,
	"file":       true,
	"data":       true,
	"blob":       true,
	"chrome":     true,
	"intent":     true,
	"about":      true,
}

func validateResultURL(rawURL string) (*url.URL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, false
	}
	if blockedSchemes[scheme] {
		return nil, false
	}
	if parsed.Host == "" {
		return nil, false
	}
	if parsed.User != nil {
		return nil, false
	}
	return parsed, true
}

func canonicalizeURL(u *url.URL) string {
	cloned := *u
	cloned.Scheme = strings.ToLower(cloned.Scheme)
	cloned.Host = canonicalHost(cloned.Host)
	cloned.Fragment = ""
	if (cloned.Scheme == "http" && cloned.Port() == "80") ||
		(cloned.Scheme == "https" && cloned.Port() == "443") {
		cloned.Host = cloned.Hostname()
	}
	return cloned.String()
}

func canonicalHost(host string) string {
	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")
	return host
}

func extractDomain(u *url.URL) string {
	if u == nil {
		return ""
	}
	return canonicalHost(u.Hostname())
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

type Normalizer struct {
	now func() time.Time
}

func NewNormalizer() *Normalizer {
	return &Normalizer{now: time.Now}
}

func (n *Normalizer) NormalizeResults(results []SearchResult, provider string) ([]SearchResult, int) {
	normalized := make([]SearchResult, 0, len(results))
	discarded := 0
	for _, r := range results {
		originalURL := r.URL
		if r.Source.OriginalURL == "" {
			r.Source.OriginalURL = originalURL
		}
		parsed, ok := validateResultURL(r.URL)
		if !ok {
			discarded++
			continue
		}
		r.Title = sanitizeTitle(r.Title)
		r.Snippet = sanitizeSnippet(r.Snippet)
		r.Domain = extractDomain(parsed)
		r.URL = parsed.String()
		r.Source.CanonicalURL = canonicalizeURL(parsed)
		if r.Source.Provider == "" {
			r.Source.Provider = provider
		}
		if r.Source.RetrievedAt.IsZero() {
			r.Source.RetrievedAt = n.now()
		}
		n.NormalizeMetadata(&r.Metadata)
		normalized = append(normalized, r)
	}
	return normalized, discarded
}

func (n *Normalizer) NormalizeMetadata(meta *SearchResultMetadata) {
	if meta == nil {
		return
	}
	meta.DOI = sanitizeText(meta.DOI, 256)
	meta.Journal = sanitizeText(meta.Journal, 512)
	meta.Repository = sanitizeText(meta.Repository, 512)
	meta.Path = sanitizeText(meta.Path, 1024)
	meta.License = sanitizeText(meta.License, 128)
	meta.Address = sanitizeText(meta.Address, 512)
	meta.Merchant = sanitizeText(meta.Merchant, 256)
	meta.Currency = sanitizeText(meta.Currency, 16)
	meta.Availability = sanitizeText(meta.Availability, 64)
	meta.ProductID = sanitizeText(meta.ProductID, 128)
	meta.Type = sanitizeText(meta.Type, 64)
	if meta.ThumbnailURL != "" {
		if _, ok := validateResultURL(meta.ThumbnailURL); !ok {
			meta.ThumbnailURL = ""
		}
	}
	if meta.MediaURL != "" {
		if _, ok := validateResultURL(meta.MediaURL); !ok {
			meta.MediaURL = ""
		}
	}
	if meta.Width < 0 {
		meta.Width = 0
	}
	if meta.Height < 0 {
		meta.Height = 0
	}
	if meta.DurationSeconds < 0 {
		meta.DurationSeconds = 0
	}
	if meta.ReviewCount < 0 {
		meta.ReviewCount = 0
	}
	if meta.Latitude != nil {
		if *meta.Latitude < -90 || *meta.Latitude > 90 {
			meta.Latitude = nil
		}
	}
	if meta.Longitude != nil {
		if *meta.Longitude < -180 || *meta.Longitude > 180 {
			meta.Longitude = nil
		}
	}
	if meta.Rating != nil {
		if *meta.Rating < 0 {
			meta.Rating = nil
		}
	}
	if meta.Price != nil {
		if *meta.Price < 0 {
			meta.Price = nil
		}
	}
	for i := range meta.Authors {
		meta.Authors[i] = sanitizeText(meta.Authors[i], 256)
	}
	var cleanedAuthors []string
	for _, a := range meta.Authors {
		if a != "" {
			cleanedAuthors = append(cleanedAuthors, a)
		}
	}
	meta.Authors = cleanedAuthors
}

func (n *Normalizer) AssignRanks(results []SearchResult) {
	for i := range results {
		results[i].Rank = i + 1
		if results[i].Source.ProviderRank <= 0 {
			results[i].Source.ProviderRank = i + 1
		}
	}
}
