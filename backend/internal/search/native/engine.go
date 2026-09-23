package native

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/search"
)

const ProviderID = search.ProviderNative

type EngineDescriptor struct {
	ID           string
	Name         string
	Priority     int
	Weight       float64
	Capabilities search.ProviderCapabilities

	// Runtime policy metadata. Zero values are intentionally permissive for
	// legacy/free engines. Credentialed engines declare their requirements via
	// BuiltinEnginePolicy when they do not set these fields explicitly.
	CredentialIDs      []string
	DefaultDisabled    bool
	Commercial         bool
	Group              string
	Timeout            time.Duration
	RateLimitPerMinute int
	Burst              int
	EstimatedUsage     search.ProviderUsage
}

type Engine interface {
	Descriptor() EngineDescriptor
	Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error)
}

type Registry struct {
	engines []Engine
}

func NewRegistry(engines ...Engine) *Registry {
	filtered := make([]Engine, 0, len(engines))
	for _, engine := range engines {
		if engine != nil {
			filtered = append(filtered, engine)
		}
	}
	return &Registry{engines: filtered}
}

func (r *Registry) All() []Engine {
	if r == nil {
		return nil
	}
	result := make([]Engine, len(r.engines))
	copy(result, r.engines)
	return result
}

func (r *Registry) Count() int {
	if r == nil {
		return 0
	}
	return len(r.engines)
}
