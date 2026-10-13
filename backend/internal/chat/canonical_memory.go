package chat

import (
	"context"
	"github.com/u-ai/backend/internal/memory"
)

type CanonicalMemoryRuntime interface {
	Recall(context.Context, string, string, string, int) ([]memory.HybridSearchResult, error)
	Observe(context.Context, PostProcessPayload) error
}

func (s *service) SetCanonicalMemoryRuntime(runtime CanonicalMemoryRuntime) {
	s.canonicalMemory = runtime
	if runtime != nil {
		s.pipeline = nil
		s.memoryPort = nil
		s.profilePort = nil
		s.episodicPort = nil
	}
}
