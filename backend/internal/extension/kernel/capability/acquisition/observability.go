package acquisition

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Collector defines the interface through which acquisition events and metrics
// are recorded and later exported.
type Collector interface {
	RecordAcquisitionStart(ctx context.Context, request AcquisitionRequest, candidateID string)
	RecordAcquisitionEnd(ctx context.Context, result *AcquisitionResult, d time.Duration)
	RecordAcquisitionFailure(ctx context.Context, candidateID string, err error)
	RecordPolicyDecision(ctx context.Context, candidateID string, decision PolicyDecision)
	RecordIdempotencyHit(ctx context.Context, key string, isRepeat bool)
	Snapshot() CollectorSnapshot
}

// CollectorSnapshot captures a point-in-time view of all collected metrics.
type CollectorSnapshot struct {
	TotalAttempts      int64 `json:"totalAttempts"`
	TotalSuccess       int64 `json:"totalSuccess"`
	TotalFailures      int64 `json:"totalFailures"`
	ActiveAcquisitions int64 `json:"activeAcquisitions"`
	AverageDurationMs  int64 `json:"averageDurationMs,omitempty"`
	IdempotencyHits    int64 `json:"idempotencyHits"`
}

// CollectorExporter defines how snapshots are persisted or forwarded to an
// external observability backend.
type CollectorExporter interface {
	Export(snapshot CollectorSnapshot) error
}

// defaultCollector implements Collector with atomic counters for lock-free
// writes and a RWMutex-guarded snapshot for lock-free reads.
type defaultCollector struct {
	mu                 sync.RWMutex
	totalAttempts      atomic.Int64
	totalSuccess       atomic.Int64
	totalFailures      atomic.Int64
	activeAcquisitions atomic.Int64
	totalDurationMs    atomic.Int64
	idempotencyHits    atomic.Int64
}

// NewCollector returns a ready-to-use Collector backed by defaultCollector.
func NewCollector() Collector {
	return &defaultCollector{}
}

// RecordAcquisitionStart increments the total-attempts and active-acquisitions
// counters when a new acquisition begins.
func (c *defaultCollector) RecordAcquisitionStart(ctx context.Context, request AcquisitionRequest, candidateID string) {
	c.totalAttempts.Add(1)
	c.activeAcquisitions.Add(1)
}

// RecordAcquisitionEnd records a successful acquisition, decrementing the
// active counter, incrementing the success counter, and accumulating duration.
func (c *defaultCollector) RecordAcquisitionEnd(ctx context.Context, result *AcquisitionResult, d time.Duration) {
	c.activeAcquisitions.Add(-1)
	c.totalSuccess.Add(1)
	c.totalDurationMs.Add(int64(d / time.Millisecond))
}

// RecordAcquisitionFailure records a failed acquisition, decrementing the active
// counter, incrementing the failure counter, and accumulating an approximate
// duration (0 if unknown).
func (c *defaultCollector) RecordAcquisitionFailure(ctx context.Context, candidateID string, err error) {
	c.activeAcquisitions.Add(-1)
	c.totalFailures.Add(1)
	// Duration is unknown for synchronous failures; accumulation stays unchanged.
}

// RecordPolicyDecision records that a policy decision was rendered for the given
// candidate. The decision itself is not stored; this method exists as a hook
// point for custom Collectors that wish to log policy outcomes.
func (c *defaultCollector) RecordPolicyDecision(ctx context.Context, candidateID string, decision PolicyDecision) {
	// No-op for the default collector. Implementors may override to track or
	// forward policy decisions.
}

// RecordIdempotencyHit records an idempotency cache hit (or miss) for the given
// acquisition key.
func (c *defaultCollector) RecordIdempotencyHit(ctx context.Context, key string, isRepeat bool) {
	if isRepeat {
		c.idempotencyHits.Add(1)
	}
}

// Snapshot returns a consistent point-in-time view of all collected metrics.
func (c *defaultCollector) Snapshot() CollectorSnapshot {
	totalAttempts := c.totalAttempts.Load()
	totalSuccess := c.totalSuccess.Load()

	var avgDuration int64
	if totalSuccess > 0 {
		avgDuration = c.totalDurationMs.Load() / totalSuccess
	}

	return CollectorSnapshot{
		TotalAttempts:      totalAttempts,
		TotalSuccess:       totalSuccess,
		TotalFailures:      c.totalFailures.Load(),
		ActiveAcquisitions: c.activeAcquisitions.Load(),
		AverageDurationMs:  avgDuration,
		IdempotencyHits:    c.idempotencyHits.Load(),
	}
}
