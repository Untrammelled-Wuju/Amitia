package execution

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type testStreamSink struct {
	mu     sync.Mutex
	events []capability.ToolStreamEvent
	err    error
}

func (s *testStreamSink) Emit(ctx context.Context, event capability.ToolStreamEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, event)
	return nil
}

func (s *testStreamSink) Events() []capability.ToolStreamEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]capability.ToolStreamEvent, len(s.events))
	copy(result, s.events)
	return result
}

func textOfLength(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func TestB20StreamSessionContentEvent(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 64 * 1024, MaxEvents: 4096}
	session := newToolStreamSession("inv-1", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type: capability.ToolStreamEventContent,
		Content: &capability.ToolContent{
			Type: capability.ToolContentText,
			Text: "hello world",
		},
	})
	if err != nil {
		t.Fatalf("emit content: %v", err)
	}

	if !session.HasVisibleOutput() {
		t.Fatal("expected visible=true after emit")
	}
	if session.StreamEventCount() != 1 {
		t.Fatalf("expected 1 event, got %d", session.StreamEventCount())
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event in sink, got %d", len(events))
	}
	if events[0].Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", events[0].Sequence)
	}
	if events[0].Type != capability.ToolStreamEventContent {
		t.Fatalf("expected content type, got %s", events[0].Type)
	}
}

func TestB20StreamSessionProgressEvent(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-2", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type: capability.ToolStreamEventProgress,
		Progress: &capability.ToolStreamProgress{
			Fraction:      0.5,
			Indeterminate: false,
		},
	})
	if err != nil {
		t.Fatalf("emit progress: %v", err)
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Progress == nil {
		t.Fatal("expected progress to be set")
	}
	if events[0].Progress.Fraction != 0.5 {
		t.Fatalf("expected fraction 0.5, got %f", events[0].Progress.Fraction)
	}
}

func TestB20StreamSessionRejectsTerminalFromRuntime(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-3", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type: capability.ToolStreamEventTerminal,
	})
	if err == nil {
		t.Fatal("expected error when runtime emits terminal")
	}
	toolErr, ok := err.(*capability.ToolError)
	if !ok {
		t.Fatalf("expected ToolError, got %T", err)
	}
	if toolErr.Code != capability.ErrorCodeStreamProtocol {
		t.Fatalf("expected stream_protocol_error, got %s", toolErr.Code)
	}
}

func TestB20StreamSessionSequenceStrictlyIncreasing(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 1 << 20}
	session := newToolStreamSession("inv-4", sink, policy, nil, nil)

	for i := 0; i < 100; i++ {
		err := session.Emit(context.Background(), capability.ToolStreamEmission{
			Type: capability.ToolStreamEventContent,
			Content: &capability.ToolContent{
				Type: capability.ToolContentText,
				Text: "chunk",
			},
		})
		if err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}

	events := sink.Events()
	for i, event := range events {
		if event.Sequence != uint64(i+1) {
			t.Fatalf("event %d: expected sequence %d, got %d", i, i+1, event.Sequence)
		}
	}
}

func TestB20StreamSessionFinishSendsTerminal(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-5", sink, policy, nil, nil)

	result := capability.NewToolSuccessResult("inv-5", "tool-1")
	err := session.Finish(context.Background(), result)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 terminal event, got %d", len(events))
	}
	if events[0].Type != capability.ToolStreamEventTerminal {
		t.Fatalf("expected terminal event, got %s", events[0].Type)
	}
	if events[0].Result == nil {
		t.Fatal("expected result in terminal event")
	}
	if events[0].Result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success status, got %s", events[0].Result.Status)
	}
}

func TestB20StreamSessionFinishIdempotent(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-6", sink, policy, nil, nil)

	result := capability.NewToolSuccessResult("inv-6", "tool-1")
	_ = session.Finish(context.Background(), result)
	err := session.Finish(context.Background(), result)
	if err != nil {
		t.Fatalf("second finish should return nil, got: %v", err)
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 terminal event, got %d", len(events))
	}
}

func TestB20StreamSessionRejectsEmitAfterFinish(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-7", sink, policy, nil, nil)

	_ = session.Finish(context.Background(), capability.NewToolSuccessResult("inv-7", "tool-1"))

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "late"},
	})
	if err == nil {
		t.Fatal("expected error when emitting after finish")
	}
}

func TestB20StreamSessionEventSizeLimit(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 100}
	session := newToolStreamSession("inv-8", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "this text is definitely longer than 100 bytes total event size"},
	})
	if err == nil {
		t.Fatal("expected error for oversized event")
	}
}

func TestB20StreamSessionEventCountLimit(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 1 << 20, MaxEvents: 3}
	session := newToolStreamSession("inv-9", sink, policy, nil, nil)

	for i := 0; i < 3; i++ {
		err := session.Emit(context.Background(), capability.ToolStreamEmission{
			Type:    capability.ToolStreamEventContent,
			Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "ok"},
		})
		if err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "overflow"},
	})
	if err == nil {
		t.Fatal("expected error when exceeding max events")
	}
	toolErr, ok := err.(*capability.ToolError)
	if !ok || toolErr.Code != capability.ErrorCodeStreamLimitExceeded {
		t.Fatalf("expected stream_limit_exceeded, got %v", err)
	}
}

func TestB20StreamSessionContentWithoutContent(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-10", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type: capability.ToolStreamEventContent,
	})
	if err == nil {
		t.Fatal("expected error for content event without content")
	}
}

func TestB20StreamSessionContentWithInvalidStructured(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-11", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type: capability.ToolStreamEventContent,
		Content: &capability.ToolContent{
			Type: capability.ToolContentStructured,
			Data: json.RawMessage(`{invalid`),
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestB20StreamSessionSinkError(t *testing.T) {
	sink := &testStreamSink{err: context.DeadlineExceeded}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-12", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "hello"},
	})
	if err == nil {
		t.Fatal("expected sink error to propagate")
	}
	if session.Err() == nil {
		t.Fatal("expected Err() to be set after sink error")
	}

	emitErr := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "second"},
	})
	if emitErr == nil || emitErr.Error() != err.Error() {
		t.Fatal("expected subsequent emits to return same error")
	}
}

func TestB20StreamSessionConcurrentEmit(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 1 << 20}
	session := newToolStreamSession("inv-13", sink, policy, nil, nil)

	var wg sync.WaitGroup
	var errCount int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := session.Emit(context.Background(), capability.ToolStreamEmission{
				Type:    capability.ToolStreamEventContent,
				Content: &capability.ToolContent{Type: capability.ToolContentText, Text: "c"},
			})
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	wg.Wait()

	if errCount > 0 {
		t.Fatalf("expected no errors in concurrent emit, got %d", errCount)
	}

	events := sink.Events()
	if len(events) != 50 {
		t.Fatalf("expected 50 events, got %d", len(events))
	}

	seen := make(map[uint64]bool)
	for _, event := range events {
		if seen[event.Sequence] {
			t.Fatalf("duplicate sequence %d", event.Sequence)
		}
		seen[event.Sequence] = true
	}
}

func TestB20StreamSessionTotalBytesLimit(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true, MaxEventBytes: 1 << 20, MaxTotalBytes: 500}
	session := newToolStreamSession("inv-14", sink, policy, nil, nil)

	var emitted int
	for i := 0; i < 10; i++ {
		err := session.Emit(context.Background(), capability.ToolStreamEmission{
			Type:    capability.ToolStreamEventContent,
			Content: &capability.ToolContent{Type: capability.ToolContentText, Text: textOfLength(50)},
		})
		if err == nil {
			emitted++
		}
	}
	if emitted >= 10 {
		t.Fatal("expected some emits to fail due to total bytes limit")
	}
}

func TestB20StreamSessionContentWithStreamContentType(t *testing.T) {
	sink := &testStreamSink{}
	policy := capability.ToolStreamingPolicy{Enabled: true}
	session := newToolStreamSession("inv-15", sink, policy, nil, nil)

	err := session.Emit(context.Background(), capability.ToolStreamEmission{
		Type:    capability.ToolStreamEventContent,
		Content: &capability.ToolContent{Type: capability.ToolContentStream, URI: "amitia://stream/1"},
	})
	if err == nil {
		t.Fatal("expected error for ToolContentStream type")
	}
}
