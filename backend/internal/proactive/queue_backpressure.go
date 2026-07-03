package proactive

import (
	"log"
	"sync"
	"time"
)

type BackpressureLevel int

const (
	BackpressureNone BackpressureLevel = 0
	BackpressureLow  BackpressureLevel = 1
	BackpressureMed  BackpressureLevel = 2
	BackpressureHigh BackpressureLevel = 3
	BackpressureFull BackpressureLevel = 4
)

func (l BackpressureLevel) String() string {
	switch l {
	case BackpressureNone:
		return "none"
	case BackpressureLow:
		return "low"
	case BackpressureMed:
		return "medium"
	case BackpressureHigh:
		return "high"
	case BackpressureFull:
		return "full"
	default:
		return "unknown"
	}
}

type QueueItem struct {
	ID          string         `json:"id"`
	Priority    OutputPriority `json:"priority"`
	CharacterID string         `json:"characterId"`
	Payload     interface{}    `json:"payload"`
	EnqueuedAt  time.Time      `json:"enqueuedAt"`
	Attempts    int            `json:"attempts"`
}

type QueueBackpressure struct {
	queue      []*QueueItem
	maxSize    int
	highWater  float64
	lowWater   float64
	mu         sync.Mutex
	enqueueCh  chan struct{}
	outboxFunc func(*QueueItem) bool
	stopCh     chan struct{}
}

func NewQueueBackpressure(maxSize int, highWater, lowWater float64, outbox func(*QueueItem) bool) *QueueBackpressure {
	if maxSize <= 0 {
		maxSize = 100
	}
	if highWater <= 0 {
		highWater = 0.8
	}
	if lowWater <= 0 {
		lowWater = 0.5
	}
	if lowWater >= highWater {
		lowWater = highWater * 0.5
	}
	return &QueueBackpressure{
		queue:      make([]*QueueItem, 0, maxSize),
		maxSize:    maxSize,
		highWater:  highWater,
		lowWater:   lowWater,
		enqueueCh:  make(chan struct{}, 1),
		outboxFunc: outbox,
		stopCh:     make(chan struct{}),
	}
}

func (qb *QueueBackpressure) Start() {
	go qb.drainLoop()
	log.Printf("[Backpressure] started maxSize=%d highWater=%.0f%% lowWater=%.0f%%", qb.maxSize, qb.highWater*100, qb.lowWater*100)
}

func (qb *QueueBackpressure) Stop() {
	close(qb.stopCh)
	log.Println("[Backpressure] stopped")
}

func (qb *QueueBackpressure) Enqueue(item *QueueItem) bool {
	qb.mu.Lock()
	defer qb.mu.Unlock()

	if len(qb.queue) >= qb.maxSize {
		if item.Priority <= PriorityLow {
			return false
		}
		qb.dropLowest()
	}

	item.EnqueuedAt = time.Now()
	qb.queue = append(qb.queue, item)

	select {
	case qb.enqueueCh <- struct{}{}:
	default:
	}

	return true
}

func (qb *QueueBackpressure) dropLowest() {
	minIdx := -1
	minPriority := PriorityCrit
	for i, item := range qb.queue {
		if item.Priority < minPriority {
			minPriority = item.Priority
			minIdx = i
		}
	}
	if minIdx >= 0 {
		log.Printf("[Backpressure] dropping item id=%s priority=%s to make room", qb.queue[minIdx].ID, qb.queue[minIdx].Priority)
		qb.queue = append(qb.queue[:minIdx], qb.queue[minIdx+1:]...)
	}
}

func (qb *QueueBackpressure) Level() BackpressureLevel {
	qb.mu.Lock()
	defer qb.mu.Unlock()
	return qb.computeLevel()
}

func (qb *QueueBackpressure) computeLevel() BackpressureLevel {
	if len(qb.queue) == 0 {
		return BackpressureNone
	}
	ratio := float64(len(qb.queue)) / float64(qb.maxSize)
	switch {
	case ratio >= 1.0:
		return BackpressureFull
	case ratio >= qb.highWater:
		return BackpressureHigh
	case ratio >= qb.lowWater:
		return BackpressureMed
	default:
		return BackpressureLow
	}
}

func (qb *QueueBackpressure) ShouldThrottle(priority OutputPriority) bool {
	level := qb.Level()
	switch level {
	case BackpressureFull:
		return priority < PriorityCrit
	case BackpressureHigh:
		return priority <= PriorityLow
	case BackpressureMed:
		return false
	default:
		return false
	}
}

func (qb *QueueBackpressure) drainLoop() {
	for {
		select {
		case <-qb.enqueueCh:
			qb.drainOne()
		case <-time.After(5 * time.Second):
			qb.drainOne()
		case <-qb.stopCh:
			return
		}
	}
}

func (qb *QueueBackpressure) drainOne() {
	qb.mu.Lock()
	if len(qb.queue) == 0 {
		qb.mu.Unlock()
		return
	}
	bestIdx := 0
	bestPriority := qb.queue[0].Priority
	for i := 1; i < len(qb.queue); i++ {
		if qb.queue[i].Priority > bestPriority {
			bestPriority = qb.queue[i].Priority
			bestIdx = i
		}
	}
	item := qb.queue[bestIdx]
	qb.queue = append(qb.queue[:bestIdx], qb.queue[bestIdx+1:]...)
	qb.mu.Unlock()

	if item.Attempts > 10 {
		log.Printf("[Backpressure] max attempts reached for id=%s", item.ID)
		return
	}

	item.Attempts++
	success := qb.outboxFunc(item)
	if !success && item.Priority >= PriorityNormal {
		qb.mu.Lock()
		qb.queue = append(qb.queue, item)
		qb.mu.Unlock()
	}
}

func (qb *QueueBackpressure) Size() int {
	qb.mu.Lock()
	defer qb.mu.Unlock()
	return len(qb.queue)
}

func (qb *QueueBackpressure) Reset() {
	qb.mu.Lock()
	defer qb.mu.Unlock()
	qb.queue = make([]*QueueItem, 0, qb.maxSize)
}
