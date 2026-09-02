package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type SideEffectKind string

const (
	SideEffectToolCall       SideEffectKind = "tool_call"
	SideEffectSkillInvoke    SideEffectKind = "skill_invoke"
	SideEffectHTTPCall       SideEffectKind = "http_call"
	SideEffectNotification   SideEffectKind = "notification"
	SideEffectScheduleCreate SideEffectKind = "schedule_create"
	SideEffectMemoryWrite    SideEffectKind = "memory_write"
	SideEffectArtifactWrite  SideEffectKind = "artifact_write"
	SideEffectContextWrite   SideEffectKind = "context_write"
)

const (
	SideEffectStatusCompleted = "completed"
	SideEffectStatusFailed    = "failed"
)

type SideEffectRecord struct {
	JournalID      string          `json:"journalId"`
	ExecutionID    string          `json:"executionId,omitempty"`
	WorkflowID     string          `json:"workflowId,omitempty"`
	NodeID         string          `json:"nodeId"`
	Attempt        int             `json:"attempt,omitempty"`
	Generation     int64           `json:"generation,omitempty"`
	DeviceID       string          `json:"deviceId,omitempty"`
	Kind           SideEffectKind  `json:"kind"`
	Target         string          `json:"target,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	Status         string          `json:"status"`
	Duration       time.Duration   `json:"duration"`
	Timestamp      time.Time       `json:"timestamp"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
}

// SideEffectStore is an optional durable extension implemented by the workflow
// execution repository. The executor keeps the in-memory journal API for tests
// and lightweight runtimes, while production repositories persist every entry.
type SideEffectStore interface {
	SaveSideEffect(ctx context.Context, record SideEffectRecord) error
	ListSideEffects(ctx context.Context, executionID string) ([]SideEffectRecord, error)
}

type SideEffectJournalScope struct {
	ExecutionID string
	WorkflowID  string
	Generation  int64
	DeviceID    string
}

type SideEffectJournal struct {
	records []SideEffectRecord
	mu      sync.RWMutex
	store   SideEffectStore
	ctx     context.Context
	scope   SideEffectJournalScope
}

func NewSideEffectJournal() *SideEffectJournal {
	return &SideEffectJournal{}
}

func NewPersistentSideEffectJournal(ctx context.Context, store SideEffectStore, scope SideEffectJournalScope) *SideEffectJournal {
	if ctx == nil {
		ctx = context.Background()
	}
	return &SideEffectJournal{ctx: ctx, store: store, scope: scope}
}

func (j *SideEffectJournal) Record(nodeID string, kind SideEffectKind, target string, input, output json.RawMessage, err string, duration time.Duration) {
	j.RecordAttempt(nodeID, 0, "", kind, target, input, output, err, duration)
}

func (j *SideEffectJournal) RecordAttempt(nodeID string, attempt int, idempotencyKey string, kind SideEffectKind, target string, input, output json.RawMessage, err string, duration time.Duration) {
	if j == nil {
		return
	}
	now := time.Now().UTC()
	completedAt := now
	status := SideEffectStatusCompleted
	if err != "" {
		status = SideEffectStatusFailed
	}
	record := SideEffectRecord{
		JournalID:      fmt.Sprintf("wfse:%s:%s:%d:%d:%d", j.scope.ExecutionID, nodeID, j.scope.Generation, attempt, now.UnixNano()),
		ExecutionID:    j.scope.ExecutionID,
		WorkflowID:     j.scope.WorkflowID,
		NodeID:         nodeID,
		Attempt:        attempt,
		Generation:     j.scope.Generation,
		DeviceID:       j.scope.DeviceID,
		Kind:           kind,
		Target:         target,
		IdempotencyKey: idempotencyKey,
		Input:          redactForJournal(input),
		Output:         redactForJournal(output),
		Error:          err,
		Status:         status,
		Duration:       duration,
		Timestamp:      now,
		CompletedAt:    &completedAt,
	}

	j.mu.Lock()
	j.records = append(j.records, record)
	j.mu.Unlock()

	if j.store != nil {
		persistCtx := j.ctx
		if persistCtx == nil {
			persistCtx = context.Background()
		}
		_ = j.store.SaveSideEffect(persistCtx, record)
	}
}

func (j *SideEffectJournal) Records() []SideEffectRecord {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]SideEffectRecord, len(j.records))
	copy(out, j.records)
	return out
}

func (j *SideEffectJournal) Count() int {
	if j == nil {
		return 0
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	return len(j.records)
}

func (j *SideEffectJournal) ByKind(kind SideEffectKind) []SideEffectRecord {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	var result []SideEffectRecord
	for _, r := range j.records {
		if r.Kind == kind {
			result = append(result, r)
		}
	}
	return result
}

func (j *SideEffectJournal) ByNode(nodeID string) []SideEffectRecord {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	var result []SideEffectRecord
	for _, r := range j.records {
		if r.NodeID == nodeID {
			result = append(result, r)
		}
	}
	return result
}

func redactForJournal(raw json.RawMessage) json.RawMessage {
	if len(raw) > 4096 {
		truncated := make([]byte, 4096)
		copy(truncated, raw[:4096])
		return json.RawMessage(append(truncated, []byte("...[redacted]")...))
	}
	return raw
}
