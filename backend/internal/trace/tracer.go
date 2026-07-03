package trace

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TraceSpan struct {
	ID          string                 `json:"id"`
	ParentID    string                 `json:"parentId,omitempty"`
	Name        string                 `json:"name"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime,omitempty"`
	Status      string                 `json:"status"`
	Attributes  map[string]interface{} `json:"attributes"`
	Events      []TraceEvent           `json:"events"`
	Error       string                 `json:"error,omitempty"`
}

type TraceEvent struct {
	Name       string                 `json:"name"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}

type InteractionTrace struct {
	TraceID      string      `json:"traceId"`
	RequestID    string      `json:"requestId"`
	Scope        string      `json:"scope"`
	Spans        []TraceSpan `json:"spans"`
	ContextHash  string      `json:"contextHash"`
	CommitHash   string      `json:"commitHash"`
	StartTime    time.Time   `json:"startTime"`
	EndTime      time.Time   `json:"endTime,omitempty"`
}

type Tracer struct {
	activeSpans map[string]*TraceSpan
}

func NewTracer() *Tracer {
	return &Tracer{
		activeSpans: map[string]*TraceSpan{},
	}
}

func (t *Tracer) StartSpan(name string, parentID string) *TraceSpan {
	span := &TraceSpan{
		ID:        uuid.New().String(),
		ParentID:  parentID,
		Name:      name,
		StartTime: time.Now().UTC(),
		Status:    "running",
		Attributes: map[string]interface{}{},
		Events:    []TraceEvent{},
	}
	t.activeSpans[span.ID] = span
	return span
}

func (t *Tracer) EndSpan(span *TraceSpan, err error) {
	span.EndTime = time.Now().UTC()
	if err != nil {
		span.Status = "error"
		span.Error = err.Error()
	} else {
		span.Status = "ok"
	}
	delete(t.activeSpans, span.ID)
}

func (t *Tracer) AddEvent(span *TraceSpan, name string, attrs map[string]interface{}) {
	span.Events = append(span.Events, TraceEvent{
		Name:       name,
		Timestamp:  time.Now().UTC(),
		Attributes: attrs,
	})
}

func (t *Tracer) NewInteractionTrace(requestID, scope string) *InteractionTrace {
	return &InteractionTrace{
		TraceID:   uuid.New().String(),
		RequestID: requestID,
		Scope:     scope,
		Spans:     []TraceSpan{},
		StartTime: time.Now().UTC(),
	}
}

func (t *InteractionTrace) AddSpan(span TraceSpan) {
	t.Spans = append(t.Spans, span)
}

func (t *InteractionTrace) Complete() {
	t.EndTime = time.Now().UTC()
}

func ComputeContextHash(snapshot interface{}) string {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func ComputeCommitHash(commit interface{}) string {
	data, err := json.Marshal(commit)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
