package interaction

import (
	"errors"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/outbox"
)

type testOutboxPublisher struct {
	mu     sync.Mutex
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
	requests []ReflectionCandidateSubmitRequest
	err      error
}

func (m *recordingReflectionMemory) SubmitReflectionCandidate(req ReflectionCandidateSubmitRequest) error {
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
		t.Fatalf("expected one candidate submit, got %d", len(mem.requests))
	}
	req := mem.requests[0]
	if req.CharacterID != "char-1" || req.Topic != "偏好" || req.Abstract != "用户连续表达了喜欢安静环境的偏好" {
		t.Fatalf("unexpected submit request: %#v", req)
	}
	if req.Importance != 8 {
		t.Fatalf("unexpected importance: %d", req.Importance)
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
		t.Fatalf("expected two candidate submits, got %d", len(mem.requests))
	}
	if mem.requests[0].CharacterID != "char-2" || mem.requests[0].Importance != 6 {
		t.Fatalf("unexpected first request: %#v", mem.requests[0])
	}
	if mem.requests[0].Topic != "作息" || mem.requests[0].Abstract != "用户常在夜间安排学习" {
		t.Fatalf("unexpected first request content: %#v", mem.requests[0])
	}
	if mem.requests[1].Topic != "情绪" || mem.requests[1].Abstract != "用户在考试前更容易紧张" {
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
		t.Fatalf("expected no candidate submit, got %d", len(mem.requests))
	}
	if next.Count() != 1 {
		t.Fatalf("expected next publisher to receive event, got %d", next.Count())
	}
}

func TestReflectionMemoryPublisherReturnsSubmitError(t *testing.T) {
	mem := &recordingReflectionMemory{err: errors.New("submit failed")}
	publisher := NewReflectionMemoryPublisher(mem, nil)

	err := publisher.Publish(outbox.OutboxRecord{
		ID:          "outbox-1",
		EventType:   ReflectionMemoryAbstractionEventType,
		AggregateID: "ref-1",
		Payload:     []byte(`{"characterId":"char-1","candidateId":"ref-1","topic":"偏好","abstract":"用户喜欢安静环境"}`),
	})
	if err == nil {
		t.Fatal("expected submit error")
	}
}
