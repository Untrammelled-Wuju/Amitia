package search

import (
	"strings"
)

func NormalizeKind(kind SearchKind) SearchKind {
	if kind == "" {
		return SearchKindWeb
	}
	return kind
}

func ValidateSpecializedOptions(kind SearchKind, opts *SpecializedSearchOptions) *Error {
	if opts == nil {
		return nil
	}
	count := 0
	switch {
	case opts.News != nil:
		count++
		if kind != SearchKindNews {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Academic != nil:
		count++
		if kind != SearchKindAcademic {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Code != nil:
		count++
		if kind != SearchKindCode {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Image != nil:
		count++
		if kind != SearchKindImage {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Video != nil:
		count++
		if kind != SearchKindVideo {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Places != nil:
		count++
		if kind != SearchKindPlaces {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
		if opts.Places.Location != nil {
			if opts.Places.Location.Latitude < -90 || opts.Places.Location.Latitude > 90 {
				return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
			}
			if opts.Places.Location.Longitude < -180 || opts.Places.Location.Longitude > 180 {
				return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
			}
		}
		if opts.Places.RadiusM < 0 || opts.Places.RadiusM > 50000 {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	case opts.Product != nil:
		count++
		if kind != SearchKindProduct {
			return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
		}
	}
	if count > 1 {
		return NewError(SEARCH_SPECIALIZED_OPTIONS_INVALID, "", false, nil)
	}
	return nil
}

func NormalizeDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	var result []string
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		d = strings.TrimSuffix(d, ".")
		if d == "" {
			continue
		}
		if strings.Contains(d, "/") || strings.Contains(d, ":") {
			continue
		}
		result = append(result, d)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ProviderSupportsFilter(caps ProviderCapabilities, kind SearchKind, hasLanguage, hasCountry, hasSafeSearch bool, hasTimeRange, hasDomains bool) *Error {
	if hasLanguage && !caps.LanguageFilter {
		return NewError(SEARCH_FILTER_UNSUPPORTED, "", false, nil)
	}
	if hasCountry && !caps.CountryFilter {
		return NewError(SEARCH_FILTER_UNSUPPORTED, "", false, nil)
	}
	if hasSafeSearch && !caps.SafeSearch {
		return NewError(SEARCH_FILTER_UNSUPPORTED, "", false, nil)
	}
	if hasTimeRange && !caps.TimeRangeFilter {
		return NewError(SEARCH_TIME_RANGE_UNSUPPORTED, "", false, nil)
	}
	if hasDomains && !caps.DomainFilter {
		return NewError(SEARCH_DOMAIN_FILTER_UNSUPPORTED, "", false, nil)
	}
	if !SupportsKind(caps, kind) {
		return NewError(SEARCH_KIND_UNSUPPORTED, "", false, nil)
	}
	return nil
}
