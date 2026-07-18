package temporal

import (
	"sync/atomic"
	"time"
)

type MetricsSnapshot struct {
	SnapshotCount             uint64  `json:"snapshotCount"`
	SnapshotErrors            uint64  `json:"snapshotErrors"`
	SnapshotLatencyTotalMs    uint64  `json:"snapshotLatencyTotalMs"`
	SnapshotLatencyAverageMs  float64 `json:"snapshotLatencyAverageMs"`
	AnchorEvents              uint64  `json:"anchorEvents"`
	AnchorDeduplicated        uint64  `json:"anchorDeduplicated"`
	AnchorRecoveryExpired     uint64  `json:"anchorRecoveryExpired"`
	ProactiveCandidates       uint64  `json:"proactiveCandidates"`
	ProactiveCandidateErrors  uint64  `json:"proactiveCandidateErrors"`
	MemoryRerankCandidates    uint64  `json:"memoryRerankCandidates"`
	MemoryTemporalBoostMicros uint64  `json:"memoryTemporalBoostMicros"`
}

type temporalMetrics struct {
	snapshotCount             atomic.Uint64
	snapshotErrors            atomic.Uint64
	snapshotLatencyTotalMs    atomic.Uint64
	anchorEvents              atomic.Uint64
	anchorDeduplicated        atomic.Uint64
	anchorRecoveryExpired     atomic.Uint64
	proactiveCandidates       atomic.Uint64
	proactiveCandidateErrors  atomic.Uint64
	memoryRerankCandidates    atomic.Uint64
	memoryTemporalBoostMicros atomic.Uint64
}

func (m *temporalMetrics) recordSnapshot(duration time.Duration, err error) {
	m.snapshotCount.Add(1)
	if err != nil {
		m.snapshotErrors.Add(1)
	}
	if duration > 0 {
		m.snapshotLatencyTotalMs.Add(uint64(duration.Milliseconds()))
	}
}

func (m *temporalMetrics) snapshot() MetricsSnapshot {
	count := m.snapshotCount.Load()
	total := m.snapshotLatencyTotalMs.Load()
	average := 0.0
	if count > 0 {
		average = float64(total) / float64(count)
	}
	return MetricsSnapshot{
		SnapshotCount: count, SnapshotErrors: m.snapshotErrors.Load(), SnapshotLatencyTotalMs: total,
		SnapshotLatencyAverageMs: average, AnchorEvents: m.anchorEvents.Load(), AnchorDeduplicated: m.anchorDeduplicated.Load(),
		AnchorRecoveryExpired: m.anchorRecoveryExpired.Load(), ProactiveCandidates: m.proactiveCandidates.Load(),
		ProactiveCandidateErrors: m.proactiveCandidateErrors.Load(), MemoryRerankCandidates: m.memoryRerankCandidates.Load(),
		MemoryTemporalBoostMicros: m.memoryTemporalBoostMicros.Load(),
	}
}

func (s *Service) Metrics() MetricsSnapshot { return s.metrics.snapshot() }
