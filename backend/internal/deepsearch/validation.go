package deepsearch

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxQueryRunes      = 2048
	MaxQueryBytes      = 8192
	MaxFocusAreaChars  = 256
)

func ValidateRequest(req *DeepSearchRequest, policy DeepSearchPolicy) *Error {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return NewError(ErrDeepSearchInvalidQuery, "query is required", ErrInvalidQuery)
	}
	if !utf8.ValidString(req.Query) {
		return NewError(ErrDeepSearchInvalidQuery, "query is not valid UTF-8", ErrInvalidQuery)
	}
	if utf8.RuneCountInString(req.Query) > MaxQueryRunes {
		return NewError(ErrDeepSearchInvalidQuery, "query exceeds maximum length", ErrInvalidQuery)
	}
	if len([]byte(req.Query)) > MaxQueryBytes {
		return NewError(ErrDeepSearchInvalidQuery, "query exceeds maximum byte size", ErrInvalidQuery)
	}

	policy.SanitizeRequest(req)

	if len(req.FocusAreas) > policy.MaxFocusAreas {
		req.FocusAreas = req.FocusAreas[:policy.MaxFocusAreas]
	}
	for i, fa := range req.FocusAreas {
		fa = strings.TrimSpace(fa)
		if fa == "" {
			return NewError(ErrDeepSearchInvalidFocusArea, "focus area cannot be empty", ErrInvalidFocusArea)
		}
		if len(fa) > MaxFocusAreaChars {
			return NewError(ErrDeepSearchInvalidFocusArea, "focus area too long", ErrInvalidFocusArea)
		}
		req.FocusAreas[i] = fa
	}

	req.Language = strings.TrimSpace(req.Language)
	req.Country = strings.TrimSpace(req.Country)
	req.SafeSearch = strings.TrimSpace(req.SafeSearch)

	return nil
}
