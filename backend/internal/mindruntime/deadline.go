package mindruntime

import (
	"context"
	"sync"
	"time"
)

type DeadlineStage string

const (
	DeadlineStageQueue       DeadlineStage = "queue"
	DeadlineStagePersonality DeadlineStage = "personality"
	DeadlineStageAppraisal   DeadlineStage = "appraisal"
	DeadlineStageGeneration  DeadlineStage = "generation"
	DeadlineStageDelivery    DeadlineStage = "delivery"
	DeadlineStagePersist     DeadlineStage = "persist"
)

type Deadline struct {
	Total            time.Duration
	Remaining        time.Duration
	Stage            DeadlineStage
	CreatedAt        time.Time
	Deadline         time.Time
	Propagated       bool
	LastPropagatedAt time.Time
	CancelReason     string
}

type DeadlineConfig struct {
	TotalTimeout       time.Duration
	QueueTimeout       time.Duration
	PersonalityTimeout time.Duration
	AppraisalTimeout   time.Duration
	GenerationTimeout  time.Duration
	DeliveryTimeout    time.Duration
	PersistTimeout     time.Duration
}

type DeadlinePropagator struct {
	mu     sync.Mutex
	config DeadlineConfig
	active map[string]*Deadline
}

var DefaultDeadlineConfig = DeadlineConfig{
	TotalTimeout:       60 * time.Second,
	QueueTimeout:       10 * time.Second,
	PersonalityTimeout: 15 * time.Second,
	AppraisalTimeout:   15 * time.Second,
	GenerationTimeout:  30 * time.Second,
	DeliveryTimeout:    10 * time.Second,
	PersistTimeout:     5 * time.Second,
}

func NewDeadlinePropagator(cfg DeadlineConfig) *DeadlinePropagator {
	return &DeadlinePropagator{
		config: cfg,
		active: make(map[string]*Deadline),
	}
}

func (dp *DeadlinePropagator) NewDeadline(requestID string) *Deadline {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	now := time.Now().UTC()
	d := &Deadline{
		Total:     dp.config.TotalTimeout,
		Remaining: dp.config.TotalTimeout,
		Stage:     DeadlineStageQueue,
		CreatedAt: now,
		Deadline:  now.Add(dp.config.TotalTimeout),
	}
	dp.active[requestID] = d
	return d
}

func (dp *DeadlinePropagator) Propagate(requestID string, stage DeadlineStage) *Deadline {
	dp.mu.Lock()
	d, ok := dp.active[requestID]
	dp.mu.Unlock()

	if !ok {
		d = dp.NewDeadline(requestID)
	}

	dp.mu.Lock()
	d.Remaining = time.Until(d.Deadline)
	if d.Remaining < 0 {
		d.Remaining = 0
	}
	d.LastPropagatedAt = time.Now().UTC()
	d.Stage = stage
	d.Propagated = true
	dp.mu.Unlock()

	return d
}

func (dp *DeadlinePropagator) ContextWithDeadline(ctx context.Context, requestID string, stage DeadlineStage) (context.Context, context.CancelFunc) {
	d := dp.Propagate(requestID, stage)

	if d.Remaining <= 0 {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		return cctx, cancel
	}

	return context.WithDeadline(ctx, d.Deadline)
}

func (dp *DeadlinePropagator) IsExpired(requestID string) bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	d, ok := dp.active[requestID]
	if !ok {
		return true
	}

	return time.Now().UTC().After(d.Deadline)
}

func (dp *DeadlinePropagator) Remaining(requestID string) time.Duration {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	d, ok := dp.active[requestID]
	if !ok {
		return 0
	}

	remaining := time.Until(d.Deadline)
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

func (dp *DeadlinePropagator) Cancel(requestID, reason string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	d, ok := dp.active[requestID]
	if !ok {
		return
	}

	d.CancelReason = reason
	d.Remaining = 0
}

func (dp *DeadlinePropagator) Superseded(requestID, supersededBy string) {
	dp.Cancel(requestID, "SUPERSEDED by "+supersededBy)
}

func (dp *DeadlinePropagator) ValidateBeforePersist(requestID string) bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	d, ok := dp.active[requestID]
	if !ok {
		return true
	}

	if d.CancelReason != "" {
		return false
	}

	if time.Now().UTC().After(d.Deadline) && d.Stage == DeadlineStagePersist {
		return false
	}

	return true
}

func (dp *DeadlinePropagator) Remove(requestID string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	delete(dp.active, requestID)
}
