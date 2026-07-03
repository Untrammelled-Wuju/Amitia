package scheduler

import (
	"time"
)

type BackpressureState string

const (
	BackpressureNormal   BackpressureState = "normal"
	BackpressureWarning  BackpressureState = "warning"
	BackpressureCritical BackpressureState = "critical"
	BackpressureShedding BackpressureState = "shedding"
)

type BackpressureConfig struct {
	WarningThreshold  float64
	CriticalThreshold float64
	SheddingThreshold float64
	WindowSize        int
	CooldownDuration  time.Duration
}

type BackpressureController struct {
	config          BackpressureConfig
	state           BackpressureState
	recentLoads     []float64
	lastStateChange time.Time
}

type DeferredTaskCategory string

const (
	DeferredEmbedding  DeferredTaskCategory = "embedding"
	DeferredGraph      DeferredTaskCategory = "graph"
	DeferredReflection DeferredTaskCategory = "reflection"
	DeferredStats      DeferredTaskCategory = "stats"
	DeferredProactive  DeferredTaskCategory = "proactive"
)

type DeferredTask struct {
	Category   DeferredTaskCategory
	Task       *Task
	DeferredAt time.Time
}

type OutboxEntry struct {
	ID        string
	Scope     string
	Operation string
	Payload   interface{}
	CreatedAt time.Time
	BatchKey  string
	Priority  PriorityLevel
}

type Outbox struct {
	entries   []OutboxEntry
	maxBatch  int
	flushFunc func([]OutboxEntry) error
}

var DefaultBackpressureConfig = BackpressureConfig{
	WarningThreshold:  0.6,
	CriticalThreshold: 0.8,
	SheddingThreshold: 0.95,
	WindowSize:        20,
	CooldownDuration:  30 * time.Second,
}

func NewBackpressureController(cfg BackpressureConfig) *BackpressureController {
	return &BackpressureController{
		config:          cfg,
		state:           BackpressureNormal,
		recentLoads:     make([]float64, 0, cfg.WindowSize),
		lastStateChange: time.Now().UTC(),
	}
}

func (bc *BackpressureController) RecordLoad(load float64) BackpressureState {
	bc.recentLoads = append(bc.recentLoads, load)
	if len(bc.recentLoads) > bc.config.WindowSize {
		bc.recentLoads = bc.recentLoads[1:]
	}

	avgLoad := bc.averageLoad()
	newState := bc.computeState(avgLoad)
	bc.state = newState
	return newState
}

func (bc *BackpressureController) State() BackpressureState {
	return bc.state
}

func (bc *BackpressureController) ShouldDefer(category DeferredTaskCategory) bool {
	switch bc.state {
	case BackpressureNormal:
		return false
	case BackpressureWarning:
		return category == DeferredEmbedding || category == DeferredGraph
	case BackpressureCritical:
		return category == DeferredEmbedding || category == DeferredGraph || category == DeferredReflection || category == DeferredStats
	case BackpressureShedding:
		return true
	default:
		return false
	}
}

func (bc *BackpressureController) ShouldCancelProactive() bool {
	return bc.state == BackpressureShedding || bc.state == BackpressureCritical
}

func (bc *BackpressureController) averageLoad() float64 {
	if len(bc.recentLoads) == 0 {
		return 0
	}
	var sum float64
	for _, l := range bc.recentLoads {
		sum += l
	}
	return sum / float64(len(bc.recentLoads))
}

func (bc *BackpressureController) computeState(avgLoad float64) BackpressureState {
	now := time.Now().UTC()
	if now.Sub(bc.lastStateChange) < bc.config.CooldownDuration {
		return bc.state
	}

	if avgLoad >= bc.config.SheddingThreshold {
		bc.lastStateChange = now
		return BackpressureShedding
	}
	if avgLoad >= bc.config.CriticalThreshold {
		bc.lastStateChange = now
		return BackpressureCritical
	}
	if avgLoad >= bc.config.WarningThreshold {
		bc.lastStateChange = now
		return BackpressureWarning
	}

	if bc.state != BackpressureNormal {
		bc.lastStateChange = now
	}
	return BackpressureNormal
}

func NewOutbox(maxBatch int, flushFunc func([]OutboxEntry) error) *Outbox {
	if maxBatch <= 0 {
		maxBatch = 100
	}
	return &Outbox{
		maxBatch:  maxBatch,
		flushFunc: flushFunc,
	}
}

func (o *Outbox) Add(entry OutboxEntry) error {
	entry.CreatedAt = time.Now().UTC()
	o.entries = append(o.entries, entry)

	if len(o.entries) >= o.maxBatch {
		return o.Flush()
	}
	return nil
}

func (o *Outbox) Flush() error {
	if len(o.entries) == 0 {
		return nil
	}
	err := o.flushFunc(o.entries)
	o.entries = o.entries[:0]
	return err
}

func (o *Outbox) Size() int {
	return len(o.entries)
}

func (o *Outbox) AggregateByBatchKey() map[string][]OutboxEntry {
	batches := make(map[string][]OutboxEntry)
	for _, e := range o.entries {
		batches[e.BatchKey] = append(batches[e.BatchKey], e)
	}
	return batches
}

func (bc *BackpressureController) ResolveDeferredStrategy() DeferredTaskCategory {
	switch bc.state {
	case BackpressureShedding:
		return DeferredProactive
	case BackpressureCritical:
		return DeferredReflection
	case BackpressureWarning:
		return DeferredGraph
	default:
		return ""
	}
}
