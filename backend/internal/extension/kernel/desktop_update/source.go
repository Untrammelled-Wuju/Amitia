package desktop_update

import (
	"fmt"
	"sort"
	"sync"
)

type SourceType string

const (
	SourceTypeLocalFile         SourceType = "local_file"
	SourceTypeOfficialRegistry  SourceType = "official_registry"
	SourceTypePublisherRegistry SourceType = "publisher_registry"
	SourceTypeCustomRegistry    SourceType = "custom_registry"
)

const (
	TrustPolicyStrict  = "strict"
	TrustPolicyLenient = "lenient"
	TrustPolicyNone    = "none"
)

type ExtensionUpdateSource struct {
	SourceID    string
	SourceType  SourceType
	BaseURL     string
	PublisherID string
	TrustPolicy string
	Enabled     bool
	Priority    int
}

func (s *ExtensionUpdateSource) Validate() error {
	if s.SourceID == "" {
		return fmt.Errorf("%w: source id required", ErrInvalidUpdateSource)
	}
	switch s.SourceType {
	case SourceTypeLocalFile, SourceTypeOfficialRegistry, SourceTypePublisherRegistry, SourceTypeCustomRegistry:
	default:
		return fmt.Errorf("%w: invalid source type %s", ErrInvalidUpdateSource, s.SourceType)
	}
	if s.SourceType != SourceTypeLocalFile && s.BaseURL == "" {
		return fmt.Errorf("%w: base url required for %s", ErrInvalidUpdateSource, s.SourceType)
	}
	switch s.TrustPolicy {
	case "", TrustPolicyStrict, TrustPolicyLenient, TrustPolicyNone:
	default:
		return fmt.Errorf("%w: invalid trust policy %s", ErrInvalidUpdateSource, s.TrustPolicy)
	}
	if s.SourceType == SourceTypeCustomRegistry && s.TrustPolicy == TrustPolicyNone {
		return fmt.Errorf("%w: custom registry cannot use trust policy none", ErrInvalidUpdateSource)
	}
	return nil
}

func (s *ExtensionUpdateSource) IsTrusted() bool {
	switch s.SourceType {
	case SourceTypeOfficialRegistry:
		return true
	case SourceTypeLocalFile:
		return true
	case SourceTypePublisherRegistry:
		return s.TrustPolicy == TrustPolicyStrict || s.TrustPolicy == TrustPolicyLenient
	case SourceTypeCustomRegistry:
		return s.TrustPolicy == TrustPolicyStrict
	default:
		return false
	}
}

type UpdateSourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]*ExtensionUpdateSource
}

func NewUpdateSourceRegistry() *UpdateSourceRegistry {
	r := &UpdateSourceRegistry{
		sources: make(map[string]*ExtensionUpdateSource),
	}
	r.Register(&ExtensionUpdateSource{
		SourceID:    "local-file",
		SourceType:  SourceTypeLocalFile,
		TrustPolicy: TrustPolicyStrict,
		Enabled:     true,
		Priority:    100,
	})
	r.Register(&ExtensionUpdateSource{
		SourceID:    "official-registry",
		SourceType:  SourceTypeOfficialRegistry,
		BaseURL:     "https://registry.amitia.dev",
		TrustPolicy: TrustPolicyStrict,
		Enabled:     true,
		Priority:    50,
	})
	return r
}

func (r *UpdateSourceRegistry) Register(source *ExtensionUpdateSource) error {
	if err := source.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[source.SourceID] = source
	return nil
}

func (r *UpdateSourceRegistry) Get(sourceID string) (*ExtensionUpdateSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[sourceID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

func (r *UpdateSourceRegistry) List() []ExtensionUpdateSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionUpdateSource, 0, len(r.sources))
	for _, s := range r.sources {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].SourceID < out[j].SourceID
	})
	return out
}

func (r *UpdateSourceRegistry) ListEnabled() []ExtensionUpdateSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExtensionUpdateSource, 0, len(r.sources))
	for _, s := range r.sources {
		if s.Enabled {
			out = append(out, *s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].SourceID < out[j].SourceID
	})
	return out
}

func (r *UpdateSourceRegistry) Remove(sourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sources[sourceID]; !ok {
		return fmt.Errorf("%w: source %s not found", ErrUpdateNotFound, sourceID)
	}
	if sourceID == "local-file" || sourceID == "official-registry" {
		return fmt.Errorf("%w: cannot remove built-in source %s", ErrInvalidUpdateSource, sourceID)
	}
	delete(r.sources, sourceID)
	return nil
}

func (r *UpdateSourceRegistry) Enable(sourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[sourceID]
	if !ok {
		return fmt.Errorf("%w: source %s not found", ErrUpdateNotFound, sourceID)
	}
	s.Enabled = true
	return nil
}

func (r *UpdateSourceRegistry) Disable(sourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[sourceID]
	if !ok {
		return fmt.Errorf("%w: source %s not found", ErrUpdateNotFound, sourceID)
	}
	if sourceID == "local-file" || sourceID == "official-registry" {
		return fmt.Errorf("%w: cannot disable built-in source %s", ErrInvalidUpdateSource, sourceID)
	}
	s.Enabled = false
	return nil
}

func (r *UpdateSourceRegistry) IsTrusted(sourceID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[sourceID]
	if !ok {
		return false
	}
	return s.IsTrusted()
}
