package search

import (
	"testing"
	"time"
)

func TestSanitizeTitle_StripHTML(t *testing.T) {
	got := sanitizeTitle("<b>Hello</b> World")
	if got != "Hello World" {
		t.Fatalf("expected stripped HTML, got %q", got)
	}
}

func TestSanitizeTitle_Entities(t *testing.T) {
	got := sanitizeTitle("A &amp; B &lt; C")
	if got != "A & B < C" {
		t.Fatalf("expected decoded entities, got %q", got)
	}
}

func TestSanitizeTitle_ControlChars(t *testing.T) {
	got := sanitizeTitle("hello\x00world\x07!")
	if got != "helloworld!" {
		t.Fatalf("expected control chars removed, got %q", got)
	}
}

func TestSanitizeTitle_Truncate(t *testing.T) {
	long := ""
	for i := 0; i < MaxTitleRunes+10; i++ {
		long += "x"
	}
	got := sanitizeTitle(long)
	if len([]rune(got)) > MaxTitleRunes {
		t.Fatalf("title not truncated: runes=%d", len([]rune(got)))
	}
}

func TestSanitizeSnippet_Truncate(t *testing.T) {
	long := ""
	for i := 0; i < MaxSnippetRunes+100; i++ {
		long += "a"
	}
	got := sanitizeSnippet(long)
	if len([]rune(got)) > MaxSnippetRunes {
		t.Fatalf("snippet not truncated: runes=%d", len([]rune(got)))
	}
}

func TestValidateResultURL_ValidHTTPS(t *testing.T) {
	u, ok := validateResultURL("https://example.com/path?q=1")
	if !ok {
		t.Fatal("https URL should be valid")
	}
	if u.Scheme != "https" {
		t.Fatalf("scheme mismatch: %s", u.Scheme)
	}
}

func TestValidateResultURL_ValidHTTP(t *testing.T) {
	_, ok := validateResultURL("http://example.com")
	if !ok {
		t.Fatal("http URL should be valid")
	}
}

func TestValidateResultURL_RejectJS(t *testing.T) {
	_, ok := validateResultURL("javascript:alert(1)")
	if ok {
		t.Fatal("javascript URL should be rejected")
	}
}

func TestValidateResultURL_RejectFile(t *testing.T) {
	_, ok := validateResultURL("file:///etc/passwd")
	if ok {
		t.Fatal("file URL should be rejected")
	}
}

func TestValidateResultURL_RejectData(t *testing.T) {
	_, ok := validateResultURL("data:text/html,<script>alert(1)</script>")
	if ok {
		t.Fatal("data URL should be rejected")
	}
}

func TestValidateResultURL_RejectUserinfo(t *testing.T) {
	_, ok := validateResultURL("https://user:pass@example.com/")
	if ok {
		t.Fatal("userinfo URL should be rejected")
	}
}

func TestValidateResultURL_RejectEmptyHost(t *testing.T) {
	_, ok := validateResultURL("https:///path")
	if ok {
		t.Fatal("empty host URL should be rejected")
	}
}

func TestCanonicalizeURL_LowercaseSchemeHost(t *testing.T) {
	u, _ := validateResultURL("https://Example.COM/Path")
	got := canonicalizeURL(u)
	if got != "https://example.com/Path" {
		t.Fatalf("expected canonical, got %q", got)
	}
}

func TestCanonicalizeURL_StripFragment(t *testing.T) {
	u, _ := validateResultURL("https://example.com/page#section")
	got := canonicalizeURL(u)
	if got != "https://example.com/page" {
		t.Fatalf("fragment not stripped: %q", got)
	}
}

func TestCanonicalizeURL_StripDefaultPort(t *testing.T) {
	u, _ := validateResultURL("https://example.com:443/page")
	got := canonicalizeURL(u)
	if got != "https://example.com/page" {
		t.Fatalf("default port not stripped: %q", got)
	}
}

func TestCanonicalizeURL_KeepQueryParams(t *testing.T) {
	u, _ := validateResultURL("https://example.com/item?id=123")
	got := canonicalizeURL(u)
	if got != "https://example.com/item?id=123" {
		t.Fatalf("query params removed: %q", got)
	}
}

func TestCanonicalizeURL_KeepNonDefaultPort(t *testing.T) {
	u, _ := validateResultURL("https://example.com:8443/page")
	got := canonicalizeURL(u)
	if got != "https://example.com:8443/page" {
		t.Fatalf("non-default port removed: %q", got)
	}
}

func TestExtractDomain(t *testing.T) {
	u, _ := validateResultURL("https://www.Example.COM/path")
	got := extractDomain(u)
	if got != "www.example.com" {
		t.Fatalf("expected lowered domain, got %q", got)
	}
}

func TestNormalizer_NormalizeResults_DropBadURLs(t *testing.T) {
	n := NewNormalizer()
	input := []SearchResult{
		{Title: "Good", URL: "https://good.com/", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "Bad", URL: "javascript:alert(1)", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "OK", URL: "https://ok.com/page?id=1", Source: SearchSourceMetadata{Provider: "fake"}},
	}
	results, discarded := n.NormalizeResults(input, "fake")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if discarded != 1 {
		t.Fatalf("expected 1 discarded, got %d", discarded)
	}
	if results[0].Domain != "good.com" {
		t.Fatalf("domain mismatch: %q", results[0].Domain)
	}
}

func TestNormalizer_NormalizeResults_AllMalformed(t *testing.T) {
	n := NewNormalizer()
	input := []SearchResult{
		{Title: "Bad1", URL: "javascript:alert(1)", Source: SearchSourceMetadata{Provider: "fake"}},
		{Title: "Bad2", URL: "file:///etc/passwd", Source: SearchSourceMetadata{Provider: "fake"}},
	}
	results, discarded := n.NormalizeResults(input, "fake")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if discarded != 2 {
		t.Fatalf("expected 2 discarded, got %d", discarded)
	}
}

func TestNormalizer_NormalizeResults_HTMLStripped(t *testing.T) {
	n := NewNormalizer()
	input := []SearchResult{
		{Title: "<b>Bold</b> Title", URL: "https://example.com/", Snippet: "<em>emphasized</em> text", Source: SearchSourceMetadata{Provider: "fake"}},
	}
	results, _ := n.NormalizeResults(input, "fake")
	if results[0].Title != "Bold Title" {
		t.Fatalf("title not stripped: %q", results[0].Title)
	}
	if results[0].Snippet != "emphasized text" {
		t.Fatalf("snippet not stripped: %q", results[0].Snippet)
	}
}

func TestNormalizer_AssignRanks(t *testing.T) {
	n := NewNormalizer()
	results := []SearchResult{
		{URL: "https://a.com/"},
		{URL: "https://b.com/"},
		{URL: "https://c.com/"},
	}
	n.AssignRanks(results)
	for i, r := range results {
		if r.Rank != i+1 {
			t.Fatalf("rank[%d] = %d, expected %d", i, r.Rank, i+1)
		}
		if r.Source.ProviderRank != i+1 {
			t.Fatalf("providerRank[%d] = %d, expected %d", i, r.Source.ProviderRank, i+1)
		}
	}
}

func TestNormalizer_NormalizeResults_EmptyInput(t *testing.T) {
	n := NewNormalizer()
	results, discarded := n.NormalizeResults(nil, "fake")
	if len(results) != 0 {
		t.Fatalf("expected 0, got %d", len(results))
	}
	if discarded != 0 {
		t.Fatalf("expected 0 discarded, got %d", discarded)
	}
}

func TestNormalizer_NormalizeResults_RetrievedAtDefault(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	n := &Normalizer{now: func() time.Time { return now }}
	results, _ := n.NormalizeResults([]SearchResult{
		{Title: "T", URL: "https://example.com/", Source: SearchSourceMetadata{Provider: "fake"}},
	}, "fake")
	if !results[0].Source.RetrievedAt.Equal(now) {
		t.Fatalf("retrievedAt not set: got %v", results[0].Source.RetrievedAt)
	}
}

func TestValidateResultURL_EmptyString(t *testing.T) {
	_, ok := validateResultURL("")
	if ok {
		t.Fatal("empty URL should be rejected")
	}
}

func TestValidateResultURL_BlobScheme(t *testing.T) {
	_, ok := validateResultURL("blob:https://example.com/uuid")
	if ok {
		t.Fatal("blob should be rejected")
	}
}
