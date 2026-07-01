package scheduler

import (
	"time"
)

type BatchOperation string

const (
	BatchEmbedding    BatchOperation = "embedding"
	BatchGraphSync    BatchOperation = "graph_sync"
	BatchDeleteClean  BatchOperation = "delete_cleanup"
	BatchReflection   BatchOperation = "reflection"
	BatchStats        BatchOperation = "stats"
)

type BatchRequest struct {
	Operation BatchOperation
	Scope     string
	Payload   interface{}
	Priority  PriorityLevel
	Deadline  time.Time
	BatchKey  string
}

type BatchResult struct {
	Operation BatchOperation
	Succeeded int
	Failed    int
	Errors    []error
	Duration  time.Duration
}

type BatchProcessor struct {
	aggr     *Outbox
	maxBatch int
}

func NewBatchProcessor(maxBatch int, flushFunc func([]OutboxEntry) error) *BatchProcessor {
	return &BatchProcessor{
		aggr:     NewOutbox(maxBatch, flushFunc),
		maxBatch: maxBatch,
	}
}

func (bp *BatchProcessor) Submit(req BatchRequest) error {
	key := req.BatchKey
	if key == "" {
		key = string(req.Operation) + ":" + req.Scope
	}

	return bp.aggr.Add(OutboxEntry{
		ID:        req.Scope + ":" + string(req.Operation) + ":" + time.Now().UTC().Format(time.RFC3339Nano),
		Scope:     req.Scope,
		Operation: string(req.Operation),
		Payload:   req.Payload,
		BatchKey:  key,
		Priority:  req.Priority,
	})
}

func (bp *BatchProcessor) Flush() error {
	return bp.aggr.Flush()
}

func (bp *BatchProcessor) Size() int {
	return bp.aggr.Size()
}

func (bp *BatchProcessor) AggregateByBatchKey() map[string][]OutboxEntry {
	return bp.aggr.AggregateByBatchKey()
}

func (bp *BatchProcessor) ProcessBatch(entries []OutboxEntry) error {
	return bp.aggr.flushFunc(entries)
}
