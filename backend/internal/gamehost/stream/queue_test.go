package stream

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestBoundedQueue_BasicPushPop(t *testing.T) {
	q := NewBoundedQueue(3)
	q.Push(QueueEntry{Sequence: 1})
	q.Push(QueueEntry{Sequence: 2})
	q.Push(QueueEntry{Sequence: 3})

	if q.Len() != 3 {
		t.Errorf("expected len 3, got %d", q.Len())
	}
	if q.Cap() != 3 {
		t.Errorf("expected cap 3, got %d", q.Cap())
	}

	entry := q.PopOldest()
	if entry.Sequence != Sequence(1) {
		t.Errorf("expected seq 1, got %d", entry.Sequence)
	}
	if q.Len() != 2 {
		t.Errorf("expected len 2 after pop, got %d", q.Len())
	}
}

func TestBoundedQueue_IsFull(t *testing.T) {
	q := NewBoundedQueue(2)
	q.Push(QueueEntry{Sequence: 1})
	if q.IsFull() {
		t.Error("should not be full")
	}
	q.Push(QueueEntry{Sequence: 2})
	if !q.IsFull() {
		t.Error("should be full")
	}
}

func TestBoundedQueue_IsEmpty(t *testing.T) {
	q := NewBoundedQueue(2)
	if !q.IsEmpty() {
		t.Error("new queue should be empty")
	}
	q.Push(QueueEntry{Sequence: 1})
	if q.IsEmpty() {
		t.Error("queue should not be empty after push")
	}
}

func TestBoundedQueue_Coalesce(t *testing.T) {
	q := NewBoundedQueue(10)
	q.Push(QueueEntry{Sequence: 1})
	q.Coalesce(QueueEntry{Sequence: 2, Payload: []byte("v2")}, "key-a")
	q.Coalesce(QueueEntry{Sequence: 3, Payload: []byte("v3")}, "key-b")
	q.Coalesce(QueueEntry{Sequence: 4, Payload: []byte("v4")}, "key-a")

	if q.Len() != 3 {
		t.Errorf("expected 3 entries (coalesced key-a), got %d", q.Len())
	}
}

func TestBoundedQueue_CoalesceSameKey(t *testing.T) {
	q := NewBoundedQueue(10)
	q.Coalesce(QueueEntry{Sequence: 1, Payload: []byte("v1")}, "player-pos")
	q.Coalesce(QueueEntry{Sequence: 2, Payload: []byte("v2")}, "player-pos")
	q.Coalesce(QueueEntry{Sequence: 3, Payload: []byte("v3")}, "player-pos")

	if q.Len() != 1 {
		t.Errorf("expected 1 entry after coalesce, got %d", q.Len())
	}
	newest := q.PeekNewest()
	if newest.Sequence != Sequence(3) {
		t.Errorf("expected seq 3, got %d", newest.Sequence)
	}
}

func TestBoundedQueue_Drain(t *testing.T) {
	q := NewBoundedQueue(5)
	q.Push(QueueEntry{Sequence: 1})
	q.Push(QueueEntry{Sequence: 2})
	q.Push(QueueEntry{Sequence: 3})

	drained := q.Drain()
	if len(drained) != 3 {
		t.Errorf("expected 3 drained entries, got %d", len(drained))
	}
	if !q.IsEmpty() {
		t.Error("queue should be empty after drain")
	}
}

func TestBoundedQueue_ByteTracking(t *testing.T) {
	q := NewBoundedQueueWithBytes(5, 1024)
	q.Push(QueueEntry{Sequence: 1, Payload: make([]byte, 100)})

	bytes := q.Bytes()
	if bytes == 0 {
		t.Error("bytes should be > 0 after push")
	}

	q.PopOldest()
	if q.Bytes() != 0 {
		t.Errorf("bytes should be 0 after draining, got %d", q.Bytes())
	}
}

func TestStreamManager_EventOrdering(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-test")
	serviceID := domain.ServiceID("svc-test")
	channelID := domain.ChannelID("ch-test")

	for i := 0; i < 10; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	seq := sm.GetSequence(runtimeID, serviceID, channelID)
	if seq != Sequence(10) {
		t.Errorf("expected seq 10, got %d", seq)
	}

	replay := sm.GetReplayBuffer(runtimeID, serviceID, channelID)
	if replay == nil {
		t.Fatal("expected replay buffer")
	}
	if replay.LatestSeq() != Sequence(10) {
		t.Errorf("expected latest seq 10 in replay, got %d", replay.LatestSeq())
	}
}

func TestStreamManager_StateCoalesce(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindState}
	runtimeID := domain.RuntimeInstanceID("rt-state")
	serviceID := domain.ServiceID("svc-state")
	channelID := domain.ChannelID("ch-state")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("v1"))
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	err = sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("v2"))
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	err = sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("v3"))
	if err != nil {
		t.Fatalf("publish v3: %v", err)
	}

	seq := sm.GetSequence(runtimeID, serviceID, channelID)
	if seq != Sequence(3) {
		t.Errorf("expected seq 3, got %d", seq)
	}
}

func TestStreamManager_RejectOverflow(t *testing.T) {
	policy := RateLimitPolicy{MessagesPerSecond: 1000, Burst: 1000}
	resolver := NewPolicyResolverWithLimits(2, 0, 0, policy)
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-reject")
	serviceID := domain.ServiceID("svc-reject")
	channelID := domain.ChannelID("ch-reject")

	err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("a"))
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	err = sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte("b"))
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}

	stats := sm.StatsFor(runtimeID, serviceID, channelID)
	if stats.Published != 2 {
		t.Errorf("expected 2 published, got %d", stats.Published)
	}
}
