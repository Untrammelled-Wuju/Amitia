package interaction

import (
	"errors"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/outbox"
)

type testOutboxPublisher struct {
	mu    sync.Mutex
	events []outbox.OutboxRecord
}

func (p *testOutboxPublisher) Publish(event outbox.OutboxRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

func (p *testOutboxPublisher) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events)
}

type recordingReflectionMemory struct {
	requests []ReflectionMemoryCreateRequest
	err      error
}

func (m *recordingReflectionMemory) CreateReflectionMemory(req ReflectionMemoryCreateRequest) error {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return m.err
	}
	return nil
}

func TestReflectionMemoryPublisherCreatesMemoryFromAbstractionEvent(t *testing.T) {
	mem := &recordingReflectionMemory{}
	next := &testOutboxPublisher{}
	publisher := NewReflectionMemoryPublisher(mem, next)

	err := publisher.Publish(outbox.OutboxRecord{
		ID:          "outbox-1",
		EventType:   ReflectionMemoryAbstractionEventType,
		AggregateID: "ref-1",
		Payload: []byte(`{
			"candidateId":"ref-1",
			"characterId":"char-1",
			"conversationId":"conv-1",
			"requestId":"req-1",
			"confidence":0.82,
			"abstraction":{"topic":"偏好","abstract":"用户连续表达了喜欢安静环境的偏好","sourceIds":["mem-1","mem-2"]}
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mem.requests) != 1 {
		t.Fatalf("expected one memory create, got %d", len(mem.requests))
	}
	req := mem.requests[0]
	if req.CharacterID != "char-1" || req.MemoryType != "reflection" || req.Key != "偏好" || req.Value != "用户连续表达了喜欢安静环境的偏好" {
		t.Fatalf("unexpected memory request: %#v", req)
	}
	if req.Source != "reflection" || req.SourceMsgID != "ref-1" || req.SourceConvID != "conv-1" || req.Confidence != 82 || req.Importance != 8 {
		t.Fatalf("unexpected memory metadata: %#v", req)
	}
	if next.Count() != 1 {
		t.Fatalf("expected next publisher to receive event, got %d", next.Count())
	}
}

func TestReflectionMemoryPublisherCreatesMemoryFromApprovedCandidate(t *testing.T) {
	mem := &recordingReflectionMemory{}
	publisher := NewReflectionMemoryPublisher(mem, nil)

	err := publisher.Publish(outbox.OutboxRecord{
		ID:          "outbox-1",
		EventType:   ReflectionCandidateApprovedEventType,
		AggregateID: "ref-2",
		Payload: []byte(`{
			"conversationId":"conv-2",
			"candidate":{
				"id":"ref-2",
				"characterId":"char-2",
				"confidence":0.6,
				"memoryAbstractions":[
					{"topic":"作息","abstract":"用户常在夜间安排学习","sourceIds":["mem-3"]},
					{"topic":"情绪","abstract":"用户在考试前更容易紧张","sourceIds":["mem-4"]}
				]
			}
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mem.requests) != 2 {
		t.Fatalf("expected two memory creates, got %d", len(mem.requests))
	}
	if mem.requests[0].CharacterID != "char-2" || mem.requests[0].Confidence != 60 || mem.requests[0].Importance != 6 {
		t.Fatalf("unexpected first request: %#v", mem.requests[0])
	}
	if mem.requests[1].Key != "情绪" || mem.requests[1].Value != "用户在考试前更容易紧张" {
		t.Fatalf("unexpected second request: %#v", mem.requests[1])
	}
}

func TestReflectionMemoryPublisherPassesThroughNonReflectionEvents(t *testing.T) {
	mem := &recordingReflectionMemory{}
	next := &testOutboxPublisher{}
	publisher := NewReflectionMemoryPublisher(mem, next)

	err := publisher.Publish(outbox.OutboxRecord{
		ID:        "outbox-1",
		EventType: "interaction.completed",
		Payload:   []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mem.requests) != 0 {
		t.Fatalf("expected no memory create, got %d", len(mem.requests))
	}
	if next.Count() != 1 {
		t.Fatalf("expected next publisher to receive event, got %d", next.Count())
	}
}

func TestReflectionMemoryPublisherReturnsCreateError(t *testing.T) {
	mem := &recordingReflectionMemory{err: errors.New("create failed")}
	publisher := NewReflectionMemoryPublisher(mem, nil)

	err := publisher.Publish(outbox.OutboxRecord{
		ID:          "outbox-1",
		EventType:   ReflectionMemoryAbstractionEventType,
		AggregateID: "ref-1",
		Payload:     []byte(`{"characterId":"char-1","topic":"偏好","abstract":"用户喜欢安静环境"}`),
	})
	if err == nil {
		t.Fatal("expected create error")
	}
}
