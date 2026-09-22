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

type ProviderSearchResponse struct {
	Results    []SearchResult
	HasMore    bool
	HTTPStatus int
	RawBytes   int
}

type ProviderSet struct {
	mu        sync.RWMutex
	providers map[string]Provider
	priority  map[string]int
	defaultID string
}

func NewProviderSet(defaultID string) *ProviderSet {
	return &ProviderSet{
		providers: make(map[string]Provider),
		priority:  make(map[string]int),
		defaultID: defaultID,
	}
}

func (s *ProviderSet) Register(id string, p Provider) {
	s.RegisterWithPriority(id, p, 0)
}

func (s *ProviderSet) RegisterWithPriority(id string, p Provider, priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[id] = p
	s.priority[id] = priority
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
