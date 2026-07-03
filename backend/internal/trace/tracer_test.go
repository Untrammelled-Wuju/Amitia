package trace

import (
	"errors"
	"testing"
)

func TestTracerStartSpan(t *testing.T) {
	tr := NewTracer()
	span := tr.StartSpan("compute", "")
	if span.ID == "" {
		t.Error("expected non-empty span ID")
	}
	if span.Name != "compute" {
		t.Errorf("expected compute, got %s", span.Name)
	}
	if span.Status != "running" {
		t.Errorf("expected running, got %s", span.Status)
	}
	if len(tr.activeSpans) != 1 {
		t.Errorf("expected 1 active span, got %d", len(tr.activeSpans))
	}
}

func TestTracerEndSpanOk(t *testing.T) {
	tr := NewTracer()
	span := tr.StartSpan("compute", "")
	tr.EndSpan(span, nil)
	if span.Status != "ok" {
		t.Errorf("expected ok, got %s", span.Status)
	}
	if span.EndTime.IsZero() {
		t.Error("expected non-zero EndTime")
	}
	if len(tr.activeSpans) != 0 {
		t.Errorf("expected 0 active spans, got %d", len(tr.activeSpans))
	}
}

func TestTracerEndSpanError(t *testing.T) {
	tr := NewTracer()
	span := tr.StartSpan("compute", "")
	tr.EndSpan(span, errors.New("timeout"))
	if span.Status != "error" {
		t.Errorf("expected error, got %s", span.Status)
	}
	if span.Error != "timeout" {
		t.Errorf("expected timeout, got %s", span.Error)
	}
}

func TestTracerAddEvent(t *testing.T) {
	tr := NewTracer()
	span := tr.StartSpan("compute", "")
	tr.AddEvent(span, "checkpoint", map[string]interface{}{"step": "appraisal"})
	if len(span.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "checkpoint" {
		t.Errorf("expected checkpoint, got %s", span.Events[0].Name)
	}
	attrStep, ok := span.Events[0].Attributes["step"].(string)
	if !ok || attrStep != "appraisal" {
		t.Errorf("expected step appraisal, got %v", span.Events[0].Attributes["step"])
	}
}

func TestTracerParentChildSpans(t *testing.T) {
	tr := NewTracer()
	parent := tr.StartSpan("interaction", "")
	child := tr.StartSpan("appraisal", parent.ID)
	if child.ParentID != parent.ID {
		t.Errorf("expected parentID %s, got %s", parent.ID, child.ParentID)
	}
}

func TestNewInteractionTrace(t *testing.T) {
	tr := NewTracer()
	it := tr.NewInteractionTrace("req-1", "chat")
	if it.TraceID == "" {
		t.Error("expected non-empty TraceID")
	}
	if it.RequestID != "req-1" {
		t.Errorf("expected req-1, got %s", it.RequestID)
	}
	if it.Scope != "chat" {
		t.Errorf("expected chat, got %s", it.Scope)
	}
	if it.StartTime.IsZero() {
		t.Error("expected non-zero StartTime")
	}
}

func TestInteractionTraceAddSpan(t *testing.T) {
	tr := NewTracer()
	it := tr.NewInteractionTrace("req-1", "chat")
	span := tr.StartSpan("compute", "")
	tr.EndSpan(span, nil)
	it.AddSpan(*span)
	if len(it.Spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(it.Spans))
	}
}

func TestInteractionTraceComplete(t *testing.T) {
	tr := NewTracer()
	it := tr.NewInteractionTrace("req-1", "chat")
	it.Complete()
	if it.EndTime.IsZero() {
		t.Error("expected non-zero EndTime after Complete")
	}
}

func TestComputeContextHash(t *testing.T) {
	hash1 := ComputeContextHash(map[string]string{"key": "value"})
	hash2 := ComputeContextHash(map[string]string{"key": "value"})
	if hash1 != hash2 {
		t.Error("expected same hash for same content")
	}
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}
}

func TestComputeContextHashDifferent(t *testing.T) {
	hash1 := ComputeContextHash(map[string]string{"key": "value1"})
	hash2 := ComputeContextHash(map[string]string{"key": "value2"})
	if hash1 == hash2 {
		t.Error("expected different hashes for different content")
	}
}

func TestComputeCommitHash(t *testing.T) {
	hash := ComputeCommitHash(map[string]int{"energy": 50})
	if hash == "" {
		t.Error("expected non-empty commit hash")
	}
}
