package search

import (
	"strings"
	"testing"
)

func TestSanitizeQuery_Empty(t *testing.T) {
	_, err := sanitizeQuery("")
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY, got %v", err)
	}
}

func TestSanitizeQuery_WhitespaceOnly(t *testing.T) {
	_, err := sanitizeQuery("   \t\n  ")
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY for whitespace, got %v", err)
	}
}

func TestSanitizeQuery_Valid(t *testing.T) {
	q, err := sanitizeQuery(" hello world ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != "hello world" {
		t.Fatalf("expected trimmed query, got %q", q)
	}
}

func TestSanitizeQuery_TooLongRunes(t *testing.T) {
	long := strings.Repeat("a", MaxQueryRunes+1)
	_, err := sanitizeQuery(long)
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY for long query, got %v", err)
	}
}

func TestSanitizeQuery_MaxBoundary(t *testing.T) {
	q := strings.Repeat("a", MaxQueryRunes)
	_, err := sanitizeQuery(q)
	if err != nil {
		t.Fatalf("expected valid at rune boundary: %v", err)
	}
}

func TestSanitizeQuery_TooLongBytes(t *testing.T) {
	long := strings.Repeat("あ", MaxQueryBytes/2+1)
	_, err := sanitizeQuery(long)
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY for byte overflow, got %v", err)
	}
}

func TestValidateLimit_Valid(t *testing.T) {
	if err := validateLimit(1); err != nil {
		t.Fatalf("limit 1 should be valid: %v", err)
	}
	if err := validateLimit(20); err != nil {
		t.Fatalf("limit 20 should be valid: %v", err)
	}
}

func TestValidateLimit_Invalid(t *testing.T) {
	if err := validateLimit(0); err == nil || err.Code != SEARCH_INVALID_LIMIT {
		t.Fatalf("limit 0 should be invalid")
	}
	if err := validateLimit(21); err == nil || err.Code != SEARCH_INVALID_LIMIT {
		t.Fatalf("limit 21 should be invalid")
	}
	if err := validateLimit(-1); err == nil || err.Code != SEARCH_INVALID_LIMIT {
		t.Fatalf("limit -1 should be invalid")
	}
}

func TestValidateOffset_Valid(t *testing.T) {
	if err := validateOffset(0); err != nil {
		t.Fatalf("offset 0 should be valid: %v", err)
	}
	if err := validateOffset(100); err != nil {
		t.Fatalf("offset 100 should be valid: %v", err)
	}
}

func TestValidateOffset_Invalid(t *testing.T) {
	if err := validateOffset(101); err == nil || err.Code != SEARCH_INVALID_OFFSET {
		t.Fatalf("offset 101 should be invalid")
	}
	if err := validateOffset(-1); err == nil || err.Code != SEARCH_INVALID_OFFSET {
		t.Fatalf("offset -1 should be invalid")
	}
}

func TestValidateCountry_Valid(t *testing.T) {
	if err := validateCountry("US"); err != nil {
		t.Fatalf("US should be valid: %v", err)
	}
	if err := validateCountry(""); err != nil {
		t.Fatalf("empty country should be valid: %v", err)
	}
}

func TestValidateCountry_Invalid(t *testing.T) {
	if err := validateCountry("USA"); err == nil || err.Code != SEARCH_INVALID_COUNTRY {
		t.Fatalf("USA should be invalid")
	}
	if err := validateCountry("1"); err == nil || err.Code != SEARCH_INVALID_COUNTRY {
		t.Fatalf("1 char should be invalid")
	}
	if err := validateCountry("中国"); err == nil || err.Code != SEARCH_INVALID_COUNTRY {
		t.Fatalf("non-ascii country should be invalid")
	}
}

func TestValidateSafeSearch(t *testing.T) {
	if err := validateSafeSearch(""); err != nil {
		t.Fatalf("empty should be valid")
	}
	if err := validateSafeSearch(SafeSearchModerate); err != nil {
		t.Fatalf("moderate should be valid")
	}
	if err := validateSafeSearch("INVALID"); err == nil || err.Code != SEARCH_INVALID_SAFE_SEARCH {
		t.Fatalf("invalid mode should fail")
	}
}

func TestNormalizeLimit_Default(t *testing.T) {
	if l := normalizeLimit(0); l != DefaultLimit {
		t.Fatalf("expected default limit %d, got %d", DefaultLimit, l)
	}
	if l := normalizeLimit(-5); l != DefaultLimit {
		t.Fatalf("expected default for negative, got %d", l)
	}
}

func TestNormalizeOffset_Negative(t *testing.T) {
	if o := normalizeOffset(-1); o != 0 {
		t.Fatalf("expected 0 for negative, got %d", o)
	}
}

func TestValidateAndNormalize_Full(t *testing.T) {
	in := &ToolInput{
		Query:      "  test query  ",
		Limit:      5,
		Offset:     10,
		Language:   "en",
		Country:    "US",
		SafeSearch: SafeSearchStrict,
	}
	v, err := validateAndNormalize(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.query != "test query" {
		t.Fatalf("query not trimmed: %q", v.query)
	}
	if v.limit != 5 {
		t.Fatalf("limit mismatch: %d", v.limit)
	}
	if v.offset != 10 {
		t.Fatalf("offset mismatch: %d", v.offset)
	}
	if v.language != "en" {
		t.Fatalf("language mismatch: %q", v.language)
	}
	if v.country != "US" {
		t.Fatalf("country mismatch: %q", v.country)
	}
	if v.safeSearch != SafeSearchStrict {
		t.Fatalf("safeSearch mismatch: %q", v.safeSearch)
	}
}

func TestValidateAndNormalize_DefaultSafeSearch(t *testing.T) {
	in := &ToolInput{Query: "test"}
	v, err := validateAndNormalize(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.safeSearch != SafeSearchModerate {
		t.Fatalf("expected default moderate, got %q", v.safeSearch)
	}
}

func TestValidateAndNormalize_RejectBadQuery(t *testing.T) {
	in := &ToolInput{Query: ""}
	_, err := validateAndNormalize(in)
	if err == nil || err.Code != SEARCH_INVALID_QUERY {
		t.Fatalf("expected SEARCH_INVALID_QUERY")
	}
}

func TestValidateAndNormalize_RejectBadLimit(t *testing.T) {
	in := &ToolInput{Query: "test", Limit: 1000}
	_, err := validateAndNormalize(in)
	if err == nil || err.Code != SEARCH_INVALID_LIMIT {
		t.Fatalf("expected SEARCH_INVALID_LIMIT")
	}
}

func TestSafeSearchMode_Valid(t *testing.T) {
	if !SafeSearchOff.Valid() {
		t.Fatal("SafeSearchOff should be valid")
	}
	if !SafeSearchModerate.Valid() {
		t.Fatal("SafeSearchModerate should be valid")
	}
	if !SafeSearchStrict.Valid() {
		t.Fatal("SafeSearchStrict should be valid")
	}
	if SafeSearchMode("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestHTTPStatusHelpers(t *testing.T) {
	if !IsAuthError(401) {
		t.Fatal("401 should be auth error")
	}
	if !IsAuthError(403) {
		t.Fatal("403 should be auth error")
	}
	if IsAuthError(400) {
		t.Fatal("400 should not be auth error")
	}
	if !IsRateLimited(429) {
		t.Fatal("429 should be rate limited")
	}
	if !IsServerError(500) {
		t.Fatal("500 should be server error")
	}
	if IsServerError(501) {
		t.Fatal("501 should not be server error")
	}
	if !IsClientError(400) {
		t.Fatal("400 should be client error")
	}
	if !IsClientError(422) {
		t.Fatal("422 should be client error")
	}
	if IsClientError(401) {
		t.Fatal("401 should not be client error")
	}
	if IsClientError(429) {
		t.Fatal("429 should not be client error")
	}
	if IsRetryableStatus(429) {
		t.Fatal("429 should not be retryable")
	}
	if IsRetryableStatus(401) {
		t.Fatal("401 should not be retryable")
	}
	if IsRetryableStatus(503) {
		t.Fatal("503 should NOT be retryable (in explicit denylist)")
	}
	if !IsRetryableStatus(505) {
		t.Fatal("505 should be retryable")
	}
}
