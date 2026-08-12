package deepsearch

import (
	"strings"
	"testing"
)

func TestValidateRequest_EmptyQuery(t *testing.T) {
	req := &DeepSearchRequest{Query: ""}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err == nil || err.Code != ErrDeepSearchInvalidQuery {
		t.Fatalf("expected invalid query error, got %v", err)
	}
}

func TestValidateRequest_WhitespaceQuery(t *testing.T) {
	req := &DeepSearchRequest{Query: "   "}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err == nil || err.Code != ErrDeepSearchInvalidQuery {
		t.Fatalf("expected invalid query error for whitespace, got %v", err)
	}
}

func TestValidateRequest_TooLongQuery(t *testing.T) {
	req := &DeepSearchRequest{Query: strings.Repeat("a", MaxQueryRunes+1)}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err == nil || err.Code != ErrDeepSearchInvalidQuery {
		t.Fatalf("expected invalid query error for too long, got %v", err)
	}
}

func TestValidateRequest_ValidQuery(t *testing.T) {
	req := &DeepSearchRequest{Query: "test query"}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Query != "test query" {
		t.Fatalf("query mismatch: %q", req.Query)
	}
}

func TestValidateRequest_DefaultsApplied(t *testing.T) {
	req := &DeepSearchRequest{Query: "test"}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.MaxRounds != 3 {
		t.Fatalf("expected default MaxRounds=3, got %d", req.MaxRounds)
	}
	if req.MaxQueriesPerRound != 4 {
		t.Fatalf("expected default MaxQueriesPerRound=4, got %d", req.MaxQueriesPerRound)
	}
	if req.ResultsPerQuery != 8 {
		t.Fatalf("expected default ResultsPerQuery=8, got %d", req.ResultsPerQuery)
	}
	if req.MaxSources != 40 {
		t.Fatalf("expected default MaxSources=40, got %d", req.MaxSources)
	}
}

func TestValidateRequest_ClampMaxRounds(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", MaxRounds: 100}
	ValidateRequest(req, DefaultDeepSearchPolicy())
	if req.MaxRounds != 5 {
		t.Fatalf("expected MaxRounds clamped to 5, got %d", req.MaxRounds)
	}
}

func TestValidateRequest_ClampQueriesPerRound(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", MaxQueriesPerRound: 50}
	ValidateRequest(req, DefaultDeepSearchPolicy())
	if req.MaxQueriesPerRound != 6 {
		t.Fatalf("expected MaxQueriesPerRound clamped to 6, got %d", req.MaxQueriesPerRound)
	}
}

func TestValidateRequest_ClampResultsPerQuery(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", ResultsPerQuery: 100}
	ValidateRequest(req, DefaultDeepSearchPolicy())
	if req.ResultsPerQuery != 20 {
		t.Fatalf("expected ResultsPerQuery clamped to 20, got %d", req.ResultsPerQuery)
	}
}

func TestValidateRequest_ClampMaxSources(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", MaxSources: 500}
	ValidateRequest(req, DefaultDeepSearchPolicy())
	if req.MaxSources != 100 {
		t.Fatalf("expected MaxSources clamped to 100, got %d", req.MaxSources)
	}
}

func TestValidateRequest_EmptyFocusArea(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", FocusAreas: []string{""}}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err == nil || err.Code != ErrDeepSearchInvalidFocusArea {
		t.Fatalf("expected invalid focus area error, got %v", err)
	}
}

func TestValidateRequest_TooManyFocusAreas(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", FocusAreas: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}}
	ValidateRequest(req, DefaultDeepSearchPolicy())
	if len(req.FocusAreas) != 8 {
		t.Fatalf("expected FocusAreas truncated to 8, got %d", len(req.FocusAreas))
	}
}

func TestValidateRequest_FocusAreaTooLong(t *testing.T) {
	req := &DeepSearchRequest{Query: "test", FocusAreas: []string{strings.Repeat("x", MaxFocusAreaChars+1)}}
	err := ValidateRequest(req, DefaultDeepSearchPolicy())
	if err == nil || err.Code != ErrDeepSearchInvalidFocusArea {
		t.Fatalf("expected invalid focus area error for too long, got %v", err)
	}
}

func TestSanitizeRequest_ValidValues(t *testing.T) {
	policy := DefaultDeepSearchPolicy()
	req := &DeepSearchRequest{Query: "test", MaxRounds: 2, MaxQueriesPerRound: 3, ResultsPerQuery: 5, MaxSources: 20}
	policy.SanitizeRequest(req)
	if req.MaxRounds != 2 {
		t.Fatalf("expected MaxRounds=2, got %d", req.MaxRounds)
	}
	if req.MaxQueriesPerRound != 3 {
		t.Fatalf("expected MaxQueriesPerRound=3, got %d", req.MaxQueriesPerRound)
	}
}
