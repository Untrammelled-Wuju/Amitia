package workflow

import (
	"encoding/json"
	"sync"
	"time"
)

type SideEffectKind string

const (
	SideEffectToolCall      SideEffectKind = "tool_call"
	SideEffectSkillInvoke   SideEffectKind = "skill_invoke"
	SideEffectHTTPCall      SideEffectKind = "http_call"
	SideEffectNotification   SideEffectKind = "notification"
	SideEffectScheduleCreate SideEffectKind = "schedule_create"
	SideEffectMemoryWrite   SideEffectKind = "memory_write"
	SideEffectArtifactWrite SideEffectKind = "artifact_write"
	SideEffectContextWrite  SideEffectKind = "context_write"
)

type SideEffectRecord struct {
	NodeID    string         `json:"nodeId"`
	Kind      SideEffectKind `json:"kind"`
	Target    string         `json:"target,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration"`
	Timestamp time.Time      `json:"timestamp"`
}

type SideEffectJournal struct{
	records []SideEffectRecord
	mu      sync.RWMutex
}

func NewSideEffectJournal() *SideEffectJournal {
	return &SideEffectJournal{}
}

func (j *SideEffectJournal) Record(nodeID string, kind SideEffectKind, target string, input, output json.RawMessage, err string, duration time.Duration) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = append(j.records, SideEffectRecord{
		NodeID:    nodeID,
		Kind:      kind,
		Target:    target,
		Input:     redactForJournal(input),
		Output:    redactForJournal(output),
		Error:     err,
		Duration:  duration,
		Timestamp: time.Now().UTC(),
	})
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
