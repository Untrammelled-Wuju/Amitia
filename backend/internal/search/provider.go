package search

import (
	"context"
	"sync"
)

type Provider interface {
	ID() string
	Capabilities() ProviderCapabilities
	Search(ctx context.Context, request GeneralSearchRequest) (ProviderSearchResponse, error)
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
	defaultID string
}

func NewProviderSet(defaultID string) *ProviderSet {
	return &ProviderSet{
		providers: make(map[string]Provider),
		defaultID: defaultID,
	}
}

func (s *ProviderSet) Register(id string, p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[id] = p
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
