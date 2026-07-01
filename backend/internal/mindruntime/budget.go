package mindruntime

import (
	"fmt"
	"sync"
	"time"
)

type BudgetStatus string

const (
	BudgetNormal    BudgetStatus = "normal"
	BudgetWarning   BudgetStatus = "warning"
	BudgetExhausted BudgetStatus = "exhausted"
)

type BudgetExhaustionPolicy string

const (
	BudgetPolicyDegrade    BudgetExhaustionPolicy = "degrade"
	BudgetPolicyPrioritize BudgetExhaustionPolicy = "prioritize"
)

type BudgetConfig struct {
	MaxModelCalls  int
	MaxInputTokens int
	MaxQueueMillis int64
	MaxTotalMillis int64
	Paths          []string
}

type BudgetSnapshot struct {
	Path           string
	ActualCalls    int
	CacheHits      int
	InputTokens    int
	QueueMillis    int64
	TotalMillis    int64
	CancelReason   string
	TimeoutStage   string
	Status         BudgetStatus
	DegradeReason  string
	MaxCalls       int
	MaxTokens      int
	MaxQueueMillis int64
	MaxTotalMillis int64
}

type BudgetTracker struct {
	mu       sync.Mutex
	budgets  map[string]*BudgetRecord
	active   map[string]*BudgetRecord
	snapshots []BudgetSnapshot
}

type BudgetRecord struct {
	Config      BudgetConfig
	ActualCalls int
	CacheHits   int
	InputTokens int
	StartTime   time.Time
	QueueStart  time.Time
	Deadline    time.Time
	Sequence    int
}

func NewBudgetTracker() *BudgetTracker {
	return &BudgetTracker{
		budgets:  make(map[string]*BudgetRecord),
		active:   make(map[string]*BudgetRecord),
		snapshots: make([]BudgetSnapshot, 0),
	}
}

func (bt *BudgetTracker) RegisterBudget(path string, config BudgetConfig) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.budgets[path] = &BudgetRecord{
		Config: config,
	}
}

func (bt *BudgetTracker) StartCall(path string, inputTokens int) BudgetStatus {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return BudgetNormal
	}

	now := time.Now().UTC()

	if record.Deadline.IsZero() {
		record.Deadline = now
	}
	if record.Config.MaxTotalMillis > 0 {
		record.Deadline = now.Add(time.Duration(record.Config.MaxTotalMillis) * time.Millisecond)
	}

	record.ActualCalls++
	record.InputTokens += inputTokens
	if record.QueueStart.IsZero() {
		record.QueueStart = now
	}

	record.Sequence++
	activeCopy := *record
	bt.active[path] = &activeCopy

	if record.Config.MaxModelCalls > 0 && record.ActualCalls > record.Config.MaxModelCalls {
		return BudgetExhausted
	}
	if record.Config.MaxInputTokens > 0 && record.InputTokens > record.Config.MaxInputTokens {
		return BudgetWarning
	}
	if !record.Deadline.IsZero() && now.After(record.Deadline) {
		return BudgetExhausted
	}

	return BudgetNormal
}

func (bt *BudgetTracker) EndCall(path string, cacheHit bool) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return
	}

	if cacheHit {
		record.CacheHits++
	}

	delete(bt.active, path)
}

func (bt *BudgetTracker) RecordCancellation(path, reason, stage string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return
	}

	snap := BudgetSnapshot{
		Path:         path,
		ActualCalls:  record.ActualCalls,
		CacheHits:    record.CacheHits,
		InputTokens:  record.InputTokens,
		CancelReason: reason,
		TimeoutStage: stage,
		Status:       BudgetExhausted,
		MaxCalls:     record.Config.MaxModelCalls,
		MaxTokens:    record.Config.MaxInputTokens,
	}
	bt.snapshots = append(bt.snapshots, snap)
}

func (bt *BudgetTracker) Snapshot(path string) BudgetSnapshot {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return BudgetSnapshot{Path: path}
	}

	now := time.Now().UTC()
	queueMillis := int64(0)
	if !record.QueueStart.IsZero() {
		queueMillis = now.Sub(record.QueueStart).Milliseconds()
	}

	return BudgetSnapshot{
		Path:           path,
		ActualCalls:    record.ActualCalls,
		CacheHits:      record.CacheHits,
		InputTokens:    record.InputTokens,
		QueueMillis:    queueMillis,
		MaxCalls:       record.Config.MaxModelCalls,
		MaxTokens:      record.Config.MaxInputTokens,
		MaxQueueMillis: record.Config.MaxQueueMillis,
		MaxTotalMillis: record.Config.MaxTotalMillis,
	}
}

func (bt *BudgetTracker) IsExhausted(path string) (bool, string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return false, ""
	}

	now := time.Now().UTC()

	if record.Config.MaxModelCalls > 0 && record.ActualCalls >= record.Config.MaxModelCalls {
		return true, fmt.Sprintf("max calls exceeded: %d/%d", record.ActualCalls, record.Config.MaxModelCalls)
	}
	if record.Config.MaxInputTokens > 0 && record.InputTokens >= record.Config.MaxInputTokens {
		return true, fmt.Sprintf("max input tokens exceeded: %d/%d", record.InputTokens, record.Config.MaxInputTokens)
	}
	if !record.Deadline.IsZero() && now.After(record.Deadline) {
		return true, "deadline exceeded"
	}

	queueMillis := int64(0)
	if !record.QueueStart.IsZero() {
		queueMillis = now.Sub(record.QueueStart).Milliseconds()
	}
	if record.Config.MaxQueueMillis > 0 && queueMillis >= record.Config.MaxQueueMillis {
		return true, fmt.Sprintf("max queue time exceeded: %d/%d ms", queueMillis, record.Config.MaxQueueMillis)
	}

	return false, ""
}

func (bt *BudgetTracker) DegradeReason(path string) string {
	exhausted, reason := bt.IsExhausted(path)
	if exhausted {
		return reason
	}
	return ""
}

func (bt *BudgetTracker) AllowedForReply(path string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	_, ok := bt.budgets[path]
	if !ok {
		return true
	}

	if path == "reply" || path == "safety_check" || path == "sqlite_commit" {
		return true
	}

	return false
}

func (bt *BudgetTracker) Reset(path string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	record, ok := bt.budgets[path]
	if !ok {
		return
	}

	snap := BudgetSnapshot{
		Path:         path,
		ActualCalls:  record.ActualCalls,
		CacheHits:    record.CacheHits,
		InputTokens:  record.InputTokens,
		Status:       BudgetNormal,
		MaxCalls:     record.Config.MaxModelCalls,
		MaxTokens:    record.Config.MaxInputTokens,
	}
	bt.snapshots = append(bt.snapshots, snap)

	record.ActualCalls = 0
	record.CacheHits = 0
	record.InputTokens = 0
	record.QueueStart = time.Time{}
	record.Deadline = time.Time{}
}

func (bt *BudgetTracker) AllSnapshots() []BudgetSnapshot {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	result := make([]BudgetSnapshot, len(bt.snapshots))
	copy(result, bt.snapshots)
	return result
}
