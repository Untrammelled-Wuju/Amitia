package engines

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/search"
)

func TestDefaultRegistryMeetsCapabilityFilterMatrix(t *testing.T) {
	registry := DefaultRegistry(nil)
	type requirements struct {
		kind       search.SearchKind
		generalWeb bool
		language   bool
		country    bool
		safe       bool
		pagination bool
		timeRange  bool
		domains    bool
	}
	matrix := []requirements{
		{kind: search.SearchKindWeb, generalWeb: true, language: true, country: true, safe: true, pagination: true, timeRange: true, domains: true},
		{kind: search.SearchKindNews, language: true, country: true, safe: true, pagination: true, timeRange: true, domains: true},
		{kind: search.SearchKindAcademic, pagination: true, timeRange: true},
		{kind: search.SearchKindCode, pagination: true},
		{kind: search.SearchKindImage, language: true, country: true, safe: true, pagination: true, timeRange: true, domains: true},
		{kind: search.SearchKindVideo, language: true, country: true, safe: true, pagination: true, timeRange: true, domains: true},
		{kind: search.SearchKindPlaces, language: true, country: true},
		{kind: search.SearchKindProduct, country: true, pagination: true},
		{kind: search.SearchKindMusic, language: true, country: true, pagination: true},
		{kind: search.SearchKindFiles, pagination: true},
		{kind: search.SearchKindSocial, pagination: true},
		{kind: search.SearchKindSoftware, pagination: true},
		{kind: search.SearchKindTranslation},
		{kind: search.SearchKindDictionary},
		{kind: search.SearchKindWeather, language: true},
		{kind: search.SearchKindCurrency},
	}
	for _, requirement := range matrix {
		matches := 0
		for _, engine := range registry.All() {
			caps := engine.Descriptor().Capabilities
			if !search.SupportsKind(caps, requirement.kind) {
				continue
			}
			if requirement.generalWeb && !caps.GeneralWeb {
				continue
			}
			if requirement.language && !caps.LanguageFilter {
				continue
			}
			if requirement.country && !caps.CountryFilter {
				continue
			}
			if requirement.safe && !caps.SafeSearch {
				continue
			}
			if requirement.pagination && !caps.Pagination {
				continue
			}
			if requirement.timeRange && !caps.TimeRangeFilter {
				continue
			}
			if requirement.domains && !caps.DomainFilter {
				continue
			}
			matches++
		}
		require.Greaterf(t, matches, 0, "no engine satisfies the filter matrix for kind %s", requirement.kind)
	}
}
