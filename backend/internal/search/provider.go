package search

import (
	"context"
	"sort"
	"sync"
)

type Provider interface {
	ID() string
	Capabilities() ProviderCapabilities
	Search(ctx context.Context, request SearchRequest) (ProviderSearchResponse, error)
	Health(ctx context.Context) ProviderHealth
}

// ContextCapabilitiesProvider exposes capabilities that are actually usable for
// the current request context (for example after credential availability and
// circuit state are known).
type ContextCapabilitiesProvider interface {
	CapabilitiesForContext(ctx context.Context) ProviderCapabilities
}

// RequestCapabilityProvider validates one fully-specified search request against
// the provider's current runtime state. This is more precise than static
// capabilities for metasearch providers whose usable engines depend on
// credentials, health, filters, pagination and runtime policy.
type RequestCapabilityProvider interface {
	ValidateSearchRequest(ctx context.Context, request SearchRequest) *Error
}

type ProviderUsage struct {
	CostUSD float64 `json:"costUsd,omitempty"`
	Credits float64 `json:"credits,omitempty"`
}

func (u ProviderUsage) Add(other ProviderUsage) ProviderUsage {
	u.CostUSD += other.CostUSD
	u.Credits += other.Credits
	return u
}

type ProviderSearchResponse struct {
	Results    []SearchResult
	HasMore    bool
	HTTPStatus int
	RawBytes   int
	Usage      ProviderUsage
}

type ProviderSet struct {
	mu        sync.RWMutex
	providers map[string]Provider
	priority  map[string]int
	manifests map[string]ProviderManifest
	defaultID string
}

func NewProviderSet(defaultID string) *ProviderSet {
	return &ProviderSet{
		providers: make(map[string]Provider),
		priority:  make(map[string]int),
		manifests: make(map[string]ProviderManifest),
		defaultID: defaultID,
	}
}

func (s *ProviderSet) Register(id string, p Provider) {
	s.RegisterWithPriority(id, p, 0)
}

func (s *ProviderSet) RegisterWithPriority(id string, p Provider, priority int) {
	_ = s.RegisterWithManifest(id, p, priority, ProviderManifest{})
}

func (s *ProviderSet) RegisterWithManifest(id string, p Provider, priority int, manifest ProviderManifest) error {
	normalized, err := normalizeProviderManifest(id, p, manifest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[id] = p
	s.priority[id] = priority
	s.manifests[id] = normalized
	return nil
}

func (s *ProviderSet) Unregister(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.providers[id]; !ok {
		return false
	}
	delete(s.providers, id)
	delete(s.priority, id)
	delete(s.manifests, id)
	if s.defaultID == id {
		s.defaultID = ""
	}
	return true
}

func (s *ProviderSet) Resolve(id string) (Provider, bool) {
	if id == "" {
		return s.Default()
	}
	return s.Get(id)
}

func (s *ProviderSet) Priority(id string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.providers[id]; !ok {
		return 0, false
	}
	return s.priority[id], true
}

func (s *ProviderSet) Get(id string) (Provider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.providers[id]
	return p, ok
}

func (s *ProviderSet) Default() (Provider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.providers[s.defaultID]
	return p, ok
}

func (s *ProviderSet) SetDefault(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.providers[id]; !ok {
		return false
	}
	s.defaultID = id
	return true
}

func (s *ProviderSet) DefaultID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaultID
}

func (s *ProviderSet) Has(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.providers[id]
	return ok
}

func (s *ProviderSet) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.providers)
}

func (s *ProviderSet) All() map[string]Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Provider, len(s.providers))
	for k, v := range s.providers {
		out[k] = v
	}
	return out
}

func (s *ProviderSet) Candidates(kind SearchKind) []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type candidate struct {
		id       string
		priority int
		provider Provider
	}
	items := make([]candidate, 0, len(s.providers))
	for id, p := range s.providers {
		if SupportsKind(p.Capabilities(), kind) {
			items = append(items, candidate{id: id, priority: s.priority[id], provider: p})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		iDefault := items[i].id == s.defaultID
		jDefault := items[j].id == s.defaultID
		if iDefault != jDefault {
			return iDefault
		}
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		return items[i].id < items[j].id
	})
	result := make([]Provider, 0, len(items))
	for _, item := range items {
		result = append(result, item.provider)
	}
	return result
}

func (s *ProviderSet) CandidateIDs(kind SearchKind) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type candidate struct {
		id       string
		priority int
	}
	items := make([]candidate, 0, len(s.providers))
	for id, p := range s.providers {
		if SupportsKind(p.Capabilities(), kind) {
			items = append(items, candidate{id: id, priority: s.priority[id]})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		iDefault := items[i].id == s.defaultID
		jDefault := items[j].id == s.defaultID
		if iDefault != jDefault {
			return iDefault
		}
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		return items[i].id < items[j].id
	})
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.id)
	}
	return result
}

func SupportsKind(caps ProviderCapabilities, kind SearchKind) bool {
	if kind == SearchKindWeb || kind == "" {
		return caps.GeneralWeb
	}
	for _, k := range caps.SearchKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (s *ProviderSet) Manifest(id string) (ProviderManifest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	manifest, ok := s.manifests[id]
	return manifest, ok
}

func (s *ProviderSet) Manifests() []ProviderManifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.manifests))
	for id := range s.manifests {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ProviderManifest, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.manifests[id])
	}
	return out
}
