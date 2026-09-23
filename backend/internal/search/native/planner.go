package native

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

type PlannedEngine struct {
	Engine     Engine
	Descriptor EngineDescriptor
	Timeout    time.Duration
}

type EngineSkip struct {
	EngineID string `json:"engineId"`
	Reason   string `json:"reason"`
}

type EnginePlan struct {
	Engines []PlannedEngine `json:"-"`
	Skipped []EngineSkip    `json:"skipped,omitempty"`
}

func (p *Provider) plan(ctx context.Context, kind search.SearchKind, request search.SearchRequest) EnginePlan {
	type candidate struct {
		planned       PlannedEngine
		priority      int
		weight        float64
		safePreferred bool
	}
	plan := EnginePlan{}
	if p == nil || p.registry == nil {
		return plan
	}
	now := time.Now()
	items := make([]candidate, 0, p.registry.Count())
	for _, engine := range p.registry.All() {
		descriptor := normalizeDescriptor(engine.Descriptor())
		skip := func(reason string) {
			plan.Skipped = append(plan.Skipped, EngineSkip{EngineID: descriptor.ID, Reason: reason})
		}
		if descriptor.ID == "" {
			skip("invalid_descriptor")
			continue
		}
		if !engineEnabled(p.runtimeConfig, descriptor) {
			skip("disabled")
			continue
		}
		if !p.engineCircuitAllows(descriptor.ID, now) {
			skip("circuit_open")
			continue
		}
		if !search.SupportsKind(descriptor.Capabilities, kind) {
			skip("kind_unsupported")
			continue
		}
		if request.Language != "" && !descriptor.Capabilities.LanguageFilter {
			skip("language_filter_unsupported")
			continue
		}
		if request.Country != "" && !descriptor.Capabilities.CountryFilter {
			skip("country_filter_unsupported")
			continue
		}
		if requiresNativeSafeSearch(kind, request.SafeSearch) && !descriptor.Capabilities.SafeSearch {
			skip("safe_search_unsupported")
			continue
		}
		if request.Offset > 0 && !descriptor.Capabilities.Pagination {
			skip("pagination_unsupported")
			continue
		}
		if request.TimeRange != nil && !descriptor.Capabilities.TimeRangeFilter {
			skip("time_filter_unsupported")
			continue
		}
		if len(request.Domains) > 0 && !descriptor.Capabilities.DomainFilter {
			skip("domain_filter_unsupported")
			continue
		}
		if len(request.ExcludeDomains) > 0 && !descriptor.Capabilities.ExcludeDomainFilter {
			skip("exclude_domain_filter_unsupported")
			continue
		}
		credentialsReady := true
		for _, credentialID := range descriptor.CredentialIDs {
			if !search.EngineCredentialAvailable(ctx, credentialID) {
				credentialsReady = false
				break
			}
		}
		if !credentialsReady {
			skip("credential_missing")
			continue
		}
		if !p.engineRateCandidateAllowed(descriptor.ID, now) {
			skip("rate_limited")
			continue
		}
		items = append(items, candidate{
			planned:       PlannedEngine{Engine: engine, Descriptor: descriptor, Timeout: effectiveEngineTimeout(p.runtimeConfig, descriptor)},
			priority:      descriptor.Priority,
			weight:        descriptor.Weight,
			safePreferred: prefersNativeSafeSearch(kind, request.SafeSearch) && descriptor.Capabilities.SafeSearch,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].safePreferred != items[j].safePreferred {
			return items[i].safePreferred
		}
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		if items[i].weight != items[j].weight {
			return items[i].weight > items[j].weight
		}
		return strings.Compare(items[i].planned.Descriptor.ID, items[j].planned.Descriptor.ID) < 0
	})
	limit := p.maxEngines
	if p.runtimeConfig.MaxEngines > 0 {
		limit = p.runtimeConfig.MaxEngines
	}
	if limit > 0 && len(items) > limit {
		for _, item := range items[limit:] {
			plan.Skipped = append(plan.Skipped, EngineSkip{EngineID: item.planned.Descriptor.ID, Reason: "max_engines"})
		}
		items = items[:limit]
	}
	plan.Engines = make([]PlannedEngine, 0, len(items))
	for _, item := range items {
		plan.Engines = append(plan.Engines, item.planned)
	}
	return plan
}

// requiresNativeSafeSearch distinguishes content domains where a provider-side
// safe-search control is materially required from constrained/structured
// verticals (code, academic, package indexes, dictionaries, weather, etc.).
// Tool input defaults to "moderate" globally; treating that default as a hard
// capability for every engine would otherwise disable most useful vertical
// engines even though safe-search has no meaningful API semantics there.
func requiresNativeSafeSearch(kind search.SearchKind, mode search.SafeSearchMode) bool {
	// "moderate" is the system default, so treating it as a hard capability
	// would make whole verticals unavailable whenever their public APIs do not
	// expose a provider-side safe-search switch. Strict is the only mode that
	// must fail closed. Moderate remains a planner preference.
	if mode != search.SafeSearchStrict {
		return false
	}
	return nativeSafeSearchRelevant(kind)
}

func prefersNativeSafeSearch(kind search.SearchKind, mode search.SafeSearchMode) bool {
	return mode != "" && mode != search.SafeSearchOff && nativeSafeSearchRelevant(kind)
}

func nativeSafeSearchRelevant(kind search.SearchKind) bool {
	switch kind {
	case search.SearchKindWeb,
		search.SearchKindImage,
		search.SearchKindVideo,
		search.SearchKindSocial,
		search.SearchKindProduct:
		return true
	default:
		return false
	}
}
