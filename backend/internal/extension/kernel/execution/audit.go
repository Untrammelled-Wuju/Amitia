package execution

import (
	"context"
	"sync"
	"time"
)

func NewAuditRecorder() *AuditRecorder {
	return &AuditRecorder{
		records: make(map[string]*AuditEntry),
	}
}

type AuditEntry struct {
	InvocationID string
	ToolID       string
	Status       string
	ErrorCode    string
	ErrorMsg     string
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	RetryCount   int
	Cancelled    bool
	CircuitOpen  bool
}

type AuditRecorder struct {
	records map[string]*AuditEntry
	mu      sync.RWMutex
}

func (a *AuditRecorder) RecordStart(ctx context.Context, invID, toolID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.records[invID] = &AuditEntry{
		InvocationID: invID,
		ToolID:       toolID,
		StartTime:    time.Now(),
		Status:       "running",
	}
}

func (a *AuditRecorder) RecordFinish(ctx context.Context, invID, toolID, status string, duration time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if entry, ok := a.records[invID]; ok {
		entry.Status = status
		entry.EndTime = time.Now()
		entry.Duration = duration
	}
}

func (a *AuditRecorder) RecordDenied(ctx context.Context, invID, toolID, code, msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.records[invID] = &AuditEntry{
		InvocationID: invID,
		ToolID:       toolID,
		Status:       "denied",
		ErrorCode:    code,
		ErrorMsg:     msg,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}
}

func (a *AuditRecorder) RecordRetry(invID string, attempt int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if entry, ok := a.records[invID]; ok {
		entry.RetryCount = attempt
	}
}

func (a *AuditRecorder) GetEntry(invID string) *AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.records[invID]
}

func (a *AuditRecorder) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.records)
}
