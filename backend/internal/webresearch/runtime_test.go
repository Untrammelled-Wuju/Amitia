package webresearch

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type runtimeTestProvider struct {
	id      string
	results []search.SearchResult
	err     error
}

func (p *runtimeTestProvider) ID() string { return p.id }

func (p *runtimeTestProvider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{
		GeneralWeb:      true,
		SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindNews, search.SearchKindCode},
		LanguageFilter:  true,
		CountryFilter:   true,
		SafeSearch:      true,
		Pagination:      true,
		TimeRangeFilter: true,
		DomainFilter:    true,
		MaxResults:      20,
	}
}

func (p *runtimeTestProvider) Search(_ context.Context, _ search.SearchRequest) (search.ProviderSearchResponse, error) {
	if p.err != nil {
		return search.ProviderSearchResponse{}, p.err
	}
	return search.ProviderSearchResponse{Results: append([]search.SearchResult(nil), p.results...)}, nil
}

func (p *runtimeTestProvider) Health(context.Context) search.ProviderHealth {
	return search.ProviderHealthReady
}

func TestValidateInputRejectsMultipleCommandFamilies(t *testing.T) {
	err := validateInput(ToolInput{
		SearchQuery: []SearchQueryCommand{{Q: "amitia"}},
		Open:        []OpenCommand{{RefID: "web_s_1"}},
	})
	if err == nil || err.Code != ErrInvalidInput {
		t.Fatalf("expected invalid input, got %#v", err)
	}
}

func TestValidateInputRequiresReferenceXORURL(t *testing.T) {
	err := validateInput(ToolInput{Open: []OpenCommand{{RefID: "web_s_1", URL: "https://example.com"}}})
	if err == nil || err.Code != ErrInvalidInput {
		t.Fatalf("expected invalid input for ref/url pair, got %#v", err)
	}
}

func TestValidateInputRejectsUnsafeDomainFilter(t *testing.T) {
	err := validateInput(ToolInput{SearchQuery: []SearchQueryCommand{{Q: "amitia", Domains: []string{"example.com OR site:internal"}}}})
	if err == nil || err.Code != ErrInvalidInput {
		t.Fatalf("expected invalid domain input, got %#v", err)
	}
}

func TestNormalizeModeAuto(t *testing.T) {
	if got := normalizeMode(ModeAuto, ToolInput{SearchQuery: []SearchQueryCommand{{Q: "one"}}}); got != ModeFast {
		t.Fatalf("single query auto should be fast, got %s", got)
	}
	if got := normalizeMode(ModeAuto, ToolInput{SearchQuery: []SearchQueryCommand{{Q: "one"}}, FocusAreas: []string{"architecture"}}); got != ModeResearch {
		t.Fatalf("focus area auto should be research, got %s", got)
	}
}

func TestRuntimeFastFallsBackToNextProvider(t *testing.T) {
	providers := search.NewProviderSet("primary")
	providers.RegisterWithPriority("primary", &runtimeTestProvider{
		id:  "primary",
		err: search.NewError(search.SEARCH_PROVIDER_UNAVAILABLE, "primary", true, errors.New("down")),
	}, 100)
	providers.RegisterWithPriority("backup", &runtimeTestProvider{
		id: "backup",
		results: []search.SearchResult{{
			Title:   "Amitia",
			URL:     "https://example.com/amitia",
			Snippet: "result",
		}},
	}, 90)
	searchService := search.NewService(search.Config{
		Enabled:         true,
		DefaultProvider: "primary",
		DefaultLimit:    8,
		MaxLimit:        20,
		Timeout:         time.Second,
		Providers: map[string]search.ProviderConfig{
			"primary": {Enabled: true},
			"backup":  {Enabled: true},
		},
	}, providers)
	cfg := DefaultConfig()
	cfg.SearchTimeout = time.Second
	runtime := NewRuntime(cfg, searchService, NewStore(nil), nil)
	raw := json.RawMessage(`{"mode":"fast","search_query":[{"q":"amitia"}]}`)
	out, err := runtime.Execute(context.Background(), Scope{ConversationID: "c1", TurnID: "t1", InvocationID: "i1"}, raw, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Stats.SearchCalls != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", out.Stats.SearchCalls)
	}
	if len(out.Search) != 1 {
		t.Fatalf("expected one result, got %d", len(out.Search))
	}
	if out.Search[0].Provider != "backup" {
		t.Fatalf("expected backup provider, got %q", out.Search[0].Provider)
	}
}

func TestStoreReferenceScopeIsolation(t *testing.T) {
	store := NewStore(nil)
	now := time.Now().UTC()
	ref := Reference{RefID: "web_s_scope", ConversationID: "a", Kind: "search", URL: "https://example.com", CanonicalURL: "https://example.com", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.PutReference(context.Background(), ref); err != nil {
		t.Fatalf("put: %v", err)
	}
	_, err := store.GetReference(context.Background(), "b", ref.RefID)
	var webErr *Error
	if !errors.As(err, &webErr) || webErr.Code != ErrReferenceScope {
		t.Fatalf("expected scope error, got %v", err)
	}
}

func TestStoreCleanupExpiredMemoryEntries(t *testing.T) {
	store := NewStore(nil)
	now := time.Now().UTC()
	ref := Reference{RefID: "web_p_expired", ConversationID: "c", Kind: "page", URL: "https://example.com", CanonicalURL: "https://example.com", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}
	page := Page{RefID: ref.RefID, ConversationID: "c", URL: ref.URL, CanonicalURL: ref.CanonicalURL, ContentType: "text/html", Content: "body", ContentHash: "hash", FetchedAt: now.Add(-time.Hour)}
	evidence := Evidence{ID: "ev_expired", ConversationID: "c", PageRefID: ref.RefID, Text: "body", TextHash: "hash", CreatedAt: now.Add(-time.Hour)}
	if err := store.PutReference(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if err := store.PutPage(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	if err := store.PutEvidence(context.Background(), []Evidence{evidence}); err != nil {
		t.Fatal(err)
	}
	if err := store.MaybeCleanupExpired(context.Background(), now); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := store.GetReference(context.Background(), "c", ref.RefID); err == nil {
		t.Fatal("expired reference should be removed")
	}
	if _, err := store.GetPage(context.Background(), "c", page.RefID); err == nil {
		t.Fatal("expired page should be removed")
	}
}

func TestSlicePageAroundLine(t *testing.T) {
	content := "one\ntwo\nthree\nfour\nfive\n"
	page := Page{Content: content}
	got := slicePageAroundLine(page, 4, "short")
	if !strings.Contains(got.Content, "four") {
		t.Fatalf("expected requested line in content: %q", got.Content)
	}
}

func TestCompactToolOutputBoundsLargePayload(t *testing.T) {
	large := strings.Repeat("证据内容", 12000)
	out := &ToolOutput{
		Pages: []Page{{Content: large}, {Content: large}, {Content: large}, {Content: large}},
		Citations: []Citation{
			{EvidenceID: "1", Text: large}, {EvidenceID: "2", Text: large}, {EvidenceID: "3", Text: large},
			{EvidenceID: "4", Text: large}, {EvidenceID: "5", Text: large}, {EvidenceID: "6", Text: large},
		},
	}
	compactToolOutput(out, "medium")
	pageChars := 0
	for _, page := range out.Pages {
		pageChars += len([]rune(page.Content))
	}
	if pageChars > 90000 {
		t.Fatalf("page payload remained too large: %d", pageChars)
	}
	for _, citation := range out.Citations {
		if len([]rune(citation.Text)) > 1800 {
			t.Fatalf("citation not compacted: %d", len([]rune(citation.Text)))
		}
	}
}

func TestFuseResultsDeduplicatesNearDuplicateTitlesOnSameDomain(t *testing.T) {
	now := time.Now().UTC()
	observations := []searchObservation{
		{result: search.SearchResult{Rank: 1, Title: "Amitia Web Research Runtime Architecture", URL: "https://example.com/a"}, query: "amitia architecture", provider: "p1", retrievedAt: now},
		{result: search.SearchResult{Rank: 2, Title: "Amitia Web Research Runtime Architecture", URL: "https://example.com/b"}, query: "amitia web runtime", provider: "p2", retrievedAt: now},
	}
	got := fuseResults(observations, 10)
	if len(got) != 1 {
		t.Fatalf("expected near-duplicate title on same domain to collapse, got %d", len(got))
	}
	if got[0].seenCount != 2 {
		t.Fatalf("expected seenCount=2, got %d", got[0].seenCount)
	}
}

func TestFuseResultsBoostsIndependentQueryAndProviderSupport(t *testing.T) {
	now := time.Now().UTC()
	observations := []searchObservation{
		{result: search.SearchResult{Rank: 2, Title: "Shared", URL: "https://example.com/shared"}, query: "query one", provider: "p1", retrievedAt: now},
		{result: search.SearchResult{Rank: 2, Title: "Shared", URL: "https://example.com/shared"}, query: "query two", provider: "p2", retrievedAt: now},
		{result: search.SearchResult{Rank: 1, Title: "Single", URL: "https://other.example/single"}, query: "query one", provider: "p1", retrievedAt: now},
	}
	got := fuseResults(observations, 10)
	if len(got) != 2 {
		t.Fatalf("expected two fused results, got %d", len(got))
	}
	if got[0].result.URL != "https://example.com/shared" {
		t.Fatalf("independent support should boost shared source, got first=%s score=%f", got[0].result.URL, got[0].score)
	}
}

func TestStoreEvidenceIsQuerySpecific(t *testing.T) {
	store := NewStore(nil)
	items := []Evidence{
		{ID: "ev_a", ConversationID: "c", PageRefID: "p", Query: normalizedQuery("provider routing"), Text: "provider routing evidence", Relevance: 1, CreatedAt: time.Now().UTC()},
		{ID: "ev_b", ConversationID: "c", PageRefID: "p", Query: normalizedQuery("browser runtime"), Text: "browser runtime evidence", Relevance: 1, CreatedAt: time.Now().UTC()},
	}
	if err := store.PutEvidence(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	got, err := store.EvidenceForPageQuery(context.Background(), "c", "p", "provider routing")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ev_a" {
		t.Fatalf("unexpected query evidence: %#v", got)
	}
}

func TestFitPageForQuerySelectsRelevantBlocks(t *testing.T) {
	page := Page{Content: strings.Join([]string{
		"Gardening soil and flower care notes with unrelated background details.",
		"Amitia provider routing uses deterministic fallback across search providers.",
		"Browser automation is only used as a dynamic page fallback mechanism.",
	}, "\n\n")}
	got := fitPageForQuery(page, "provider routing fallback", "short")
	if !strings.Contains(got.Content, "provider routing") {
		t.Fatalf("relevant block missing: %q", got.Content)
	}
}

func TestStoreCitationNumbersAreStableWithinTurn(t *testing.T) {
	store := NewStore(nil)
	ctx := context.Background()
	first, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "web_p_a")
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "web_p_a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.GetOrAssignCitationNumber(ctx, "turn-1", "web_p_b")
	if err != nil {
		t.Fatal(err)
	}
	otherTurn, err := store.GetOrAssignCitationNumber(ctx, "turn-2", "web_p_a")
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 || again != 1 || second != 2 || otherTurn != 1 {
		t.Fatalf("unexpected citation numbering: first=%d again=%d second=%d other=%d", first, again, second, otherTurn)
	}
}

func TestWithDocumentPageFragmentUsesZeroBasedPage(t *testing.T) {
	got := withDocumentPageFragment("https://example.com/report.pdf?download=1#old", 2)
	if got != "https://example.com/report.pdf?download=1#page=3" {
		t.Fatalf("unexpected page fragment: %q", got)
	}
}

func TestRuntimeRejectsDeepResearchWhenFeatureFlagDisabled(t *testing.T) {
	providers := search.NewProviderSet("primary")
	providers.RegisterWithPriority("primary", &runtimeTestProvider{id: "primary"}, 100)
	searchService := search.NewService(search.Config{
		Enabled:         true,
		DefaultProvider: "primary",
		Providers: map[string]search.ProviderConfig{
			"primary": {Enabled: true},
		},
	}, providers)
	cfg := DefaultConfig()
	cfg.DeepResearchEnabled = false
	runtime := NewRuntime(cfg, searchService, NewStore(nil), nil)
	_, err := runtime.Execute(context.Background(), Scope{ConversationID: "c", TurnID: "t", InvocationID: "i"}, json.RawMessage(`{"mode":"deep_research","search_query":[{"q":"amitia"}]}`), nil)
	if err == nil || err.Code != ErrNotConfigured {
		t.Fatalf("expected feature flag error, got %#v", err)
	}
}

func TestFuseResultsRespectsDomainDiversityLimit(t *testing.T) {
	now := time.Now().UTC()
	observations := []searchObservation{
		{result: search.SearchResult{Rank: 1, Title: "One", URL: "https://same.example/1"}, query: "q", provider: "p", retrievedAt: now},
		{result: search.SearchResult{Rank: 2, Title: "Two", URL: "https://same.example/2"}, query: "q", provider: "p", retrievedAt: now},
		{result: search.SearchResult{Rank: 3, Title: "Three", URL: "https://same.example/3"}, query: "q", provider: "p", retrievedAt: now},
		{result: search.SearchResult{Rank: 4, Title: "Other", URL: "https://other.example/1"}, query: "q", provider: "p", retrievedAt: now},
	}
	got := fuseResults(observations, 3, 2)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	count := 0
	for _, item := range got {
		if domainOf(item.result.URL) == "same.example" {
			count++
		}
	}
	if count > 2 {
		t.Fatalf("domain diversity limit not respected: %d", count)
	}
}

func TestCanonicalizeURLRemovesTrackingAndDefaultPort(t *testing.T) {
	got := canonicalizeURL("HTTPS://Example.COM:443/path?id=7&utm_source=test&fbclid=abc#section")
	if got != "https://example.com/path?id=7" {
		t.Fatalf("unexpected canonical URL: %q", got)
	}
}

func TestToolOutputUsesExplicitExternalContentTrustLabel(t *testing.T) {
	out := &ToolOutput{
		ContentTrust:             ExternalContentTrustLabel,
		UntrustedExternalContent: true,
	}
	security := assessExternalContent(out)
	if out.ContentTrust != ExternalContentTrustLabel {
		t.Fatalf("unexpected output trust label %q", out.ContentTrust)
	}
	if security.TrustLabel != ExternalContentTrustLabel || !security.UntrustedExternalContent {
		t.Fatalf("unexpected security summary %#v", security)
	}
}
