package control

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type InMemoryMetricsSink struct {
	mu        sync.Mutex
	decisions map[OutputDecisionReason]uint64
}

func NewInMemoryMetricsSink() *InMemoryMetricsSink {
	return &InMemoryMetricsSink{decisions: make(map[OutputDecisionReason]uint64)}
}

func (s *InMemoryMetricsSink) RecordOutputDecision(_ domain.RuntimeInstanceID, _ ControlOutputKind, reason OutputDecisionReason, _ bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[reason]++
}

type DiscardMetricsSink struct{}

func NewDiscardMetricsSink() *DiscardMetricsSink {
	return &DiscardMetricsSink{}
}

func (*DiscardMetricsSink) RecordOutputDecision(domain.RuntimeInstanceID, ControlOutputKind, OutputDecisionReason, bool) {
}

var _ MetricsSink = (*InMemoryMetricsSink)(nil)
var _ MetricsSink = (*DiscardMetricsSink)(nil)
