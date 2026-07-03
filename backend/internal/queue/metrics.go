package queue

import (
	"sync"
	"time"
)

type QueueMetricsRecord struct {
	mu                 sync.Mutex
	CacheInvalidations int64
	QueueDepth         int64
	MaxQueueDepth      int64
	TaskAgeSum         time.Duration
	TaskAgeCount       int64
	MaxTaskAge         time.Duration
	MergeReasons       map[string]int64
	DropReasons        map[string]int64
	TotalEnqueued      int64
	TotalDequeued      int64
	TotalCompleted     int64
	TotalDropped       int64
	TotalCancelled     int64
}

type QueueMetricsSnapshot struct {
	CacheInvalidations int64            `json:"cacheInvalidations"`
	QueueDepth         int64            `json:"queueDepth"`
	MaxQueueDepth      int64            `json:"maxQueueDepth"`
	AvgTaskAgeMs       float64          `json:"avgTaskAgeMs"`
	MaxTaskAgeMs       float64          `json:"maxTaskAgeMs"`
	MergeReasons       map[string]int64 `json:"mergeReasons"`
	DropReasons        map[string]int64 `json:"dropReasons"`
	TotalEnqueued      int64            `json:"totalEnqueued"`
	TotalDequeued      int64            `json:"totalDequeued"`
	TotalCompleted     int64            `json:"totalCompleted"`
	TotalDropped       int64            `json:"totalDropped"`
	TotalCancelled     int64            `json:"totalCancelled"`
	ThroughputPerSec   float64          `json:"throughputPerSec"`
	CollectedAt        string           `json:"collectedAt"`
}

func NewQueueMetricsRecord() *QueueMetricsRecord {
	return &QueueMetricsRecord{
		MergeReasons: make(map[string]int64),
		DropReasons:  make(map[string]int64),
	}
}

func (qm *QueueMetricsRecord) RecordEnqueue() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TotalEnqueued++
}

func (qm *QueueMetricsRecord) RecordDequeue() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TotalDequeued++
}

func (qm *QueueMetricsRecord) RecordComplete() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TotalCompleted++
}

func (qm *QueueMetricsRecord) RecordDrop(reason string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TotalDropped++
	qm.DropReasons[reason]++
}

func (qm *QueueMetricsRecord) RecordCancel() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TotalCancelled++
}

func (qm *QueueMetricsRecord) RecordMerge(reason string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.MergeReasons[reason]++
}

func (qm *QueueMetricsRecord) RecordCacheInvalidation() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.CacheInvalidations++
}

func (qm *QueueMetricsRecord) RecordQueueDepth(depth int64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.QueueDepth = depth
	if depth > qm.MaxQueueDepth {
		qm.MaxQueueDepth = depth
	}
}

func (qm *QueueMetricsRecord) RecordTaskAge(age time.Duration) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.TaskAgeSum += age
	qm.TaskAgeCount++
	if age > qm.MaxTaskAge {
		qm.MaxTaskAge = age
	}
}

func (qm *QueueMetricsRecord) Snapshot(lastCollectedAt time.Time) QueueMetricsSnapshot {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now().UTC()
	s := QueueMetricsSnapshot{
		CacheInvalidations: qm.CacheInvalidations,
		QueueDepth:         qm.QueueDepth,
		MaxQueueDepth:      qm.MaxQueueDepth,
		MaxTaskAgeMs:       float64(qm.MaxTaskAge.Milliseconds()),
		TotalEnqueued:      qm.TotalEnqueued,
		TotalDequeued:      qm.TotalDequeued,
		TotalCompleted:     qm.TotalCompleted,
		TotalDropped:       qm.TotalDropped,
		TotalCancelled:     qm.TotalCancelled,
		CollectedAt:        now.Format(time.RFC3339Nano),
	}

	if qm.TaskAgeCount > 0 {
		s.AvgTaskAgeMs = float64(qm.TaskAgeSum.Milliseconds()) / float64(qm.TaskAgeCount)
	}

	if !lastCollectedAt.IsZero() {
		elapsed := now.Sub(lastCollectedAt).Seconds()
		if elapsed > 0 {
			s.ThroughputPerSec = float64(qm.TotalCompleted) / elapsed
		}
	}

	s.MergeReasons = make(map[string]int64)
	for k, v := range qm.MergeReasons {
		s.MergeReasons[k] = v
	}
	s.DropReasons = make(map[string]int64)
	for k, v := range qm.DropReasons {
		s.DropReasons[k] = v
	}

	return s
}
