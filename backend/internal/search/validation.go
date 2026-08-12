package search

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxQueryRunes   = 2048
	MaxQueryBytes   = 8192
	MaxLimit        = 20
	DefaultLimit    = 8
	MaxOffset       = 100
	MaxTitleRunes   = 512
	MaxSnippetRunes = 4096
)

type asciiOnlyRe struct{}

func (a *asciiOnlyRe) MatchString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}


func hasNonASCII(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func sanitizeQuery(query string) (string, *Error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "", &Error{Code: SEARCH_INVALID_QUERY}
	}
	if !utf8.ValidString(trimmed) {
		return "", &Error{Code: SEARCH_INVALID_QUERY}
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == 0 {
			return "", &Error{Code: SEARCH_INVALID_QUERY}
		}
	}
	if utf8.RuneCountInString(trimmed) > MaxQueryRunes {
		return "", &Error{Code: SEARCH_INVALID_QUERY}
	}
	if len(trimmed) > MaxQueryBytes {
		return "", &Error{Code: SEARCH_INVALID_QUERY}
	}
	return trimmed, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	return limit
}

func validateLimit(limit int) *Error {
	if limit < 1 || limit > MaxLimit {
		return &Error{Code: SEARCH_INVALID_LIMIT}
	}
	return nil
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func validateOffset(offset int) *Error {
	if offset < 0 || offset > MaxOffset {
		return &Error{Code: SEARCH_INVALID_OFFSET}
	}
	return nil
}

func validateLanguage(lang string) *Error {
	if lang == "" {
		return nil
	}
	if hasNonASCII(lang) {
		return &Error{Code: SEARCH_INVALID_LANGUAGE}
	}
	return nil
}

func validateCountry(country string) *Error {
	if country == "" {
		return nil
	}
	if len(country) != 2 {
		return &Error{Code: SEARCH_INVALID_COUNTRY}
	}
	if hasNonASCII(country) {
		return &Error{Code: SEARCH_INVALID_COUNTRY}
	}
	return nil
}

func validateSafeSearch(mode SafeSearchMode) *Error {
	if mode == "" {
		return nil
	}
	if !mode.Valid() {
		return &Error{Code: SEARCH_INVALID_SAFE_SEARCH}
	}
	return nil
}

type validatedInput struct {
	query      string
	limit      int
	offset     int
	language   string
	country    string
	safeSearch SafeSearchMode
}

func validateAndNormalize(in *ToolInput) (*validatedInput, *Error) {
	query, err := sanitizeQuery(in.Query)
	if err != nil {
		return nil, err
	}
	limit := normalizeLimit(in.Limit)
	if err := validateLimit(limit); err != nil {
		return nil, err
	}
	offset := normalizeOffset(in.Offset)
	if err := validateOffset(offset); err != nil {
		return nil, err
	}
	lang := strings.TrimSpace(in.Language)
	if err := validateLanguage(lang); err != nil {
		return nil, err
	}
	country := strings.TrimSpace(in.Country)
	if err := validateCountry(country); err != nil {
		return nil, err
	}
	if err := validateSafeSearch(in.SafeSearch); err != nil {
		return nil, err
	}
	if in.SafeSearch == "" {
		in.SafeSearch = SafeSearchModerate
	}
	return &validatedInput{
		query:      query,
		limit:      limit,
		offset:     offset,
		language:   lang,
		country:    country,
		safeSearch: in.SafeSearch,
	}, nil
}
