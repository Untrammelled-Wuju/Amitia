package chat

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/business"
)

type CanonicalMemoryRuntime interface {
	Recall(context.Context, string, string, string, int) ([]business.HybridSearchResult, error)
	Observe(context.Context, PostProcessPayload) error
}

func (s *service) SetCanonicalMemoryRuntime(runtime CanonicalMemoryRuntime) {
	s.canonicalMemory = runtime
}
