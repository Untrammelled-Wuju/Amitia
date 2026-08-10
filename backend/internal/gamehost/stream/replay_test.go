package stream

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestReplayBuffer_BasicAppend(t *testing.T) {
	rb := NewReplayBuffer(5)
	rb.Append(QueueEntry{Sequence: 1, Payload: []byte("a")})
	rb.Append(QueueEntry{Sequence: 2, Payload: []byte("b")})
	rb.Append(QueueEntry{Sequence: 3, Payload: []byte("c")})

	if rb.Len() != 3 {
		t.Errorf("expected len 3, got %d", rb.Len())
	}
	if rb.LatestSeq() != Sequence(3) {
		t.Errorf("expected latest seq 3, got %d", rb.LatestSeq())
	}
}

func TestReplayBuffer_Eviction(t *testing.T) {
	rb := NewReplayBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Append(QueueEntry{Sequence: Sequence(i), Payload: []byte{byte(i)}})
	}

	if rb.Len() != 3 {
		t.Errorf("expected len 3 after eviction, got %d", rb.Len())
	}
	if rb.LatestSeq() != Sequence(5) {
		t.Errorf("expected latest seq 5, got %d", rb.LatestSeq())
	}
}

func TestReplayBuffer_Replay(t *testing.T) {
	rb := NewReplayBuffer(10)
	for i := 1; i <= 5; i++ {
		rb.Append(QueueEntry{Sequence: Sequence(i), Payload: []byte{byte(i)}})
	}

	entries, err := rb.Replay(3)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Sequence != Sequence(4) {
		t.Errorf("first replayed entry should be seq 4, got %d", entries[0].Sequence)
	}
}

func TestReplayBuffer_ReplayAtLatest(t *testing.T) {
	rb := NewReplayBuffer(5)
	rb.Append(QueueEntry{Sequence: 1, Payload: []byte("a")})
	rb.Append(QueueEntry{Sequence: 2, Payload: []byte("b")})

	entries, err := rb.Replay(2)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries at latest, got %d", len(entries))
	}
}

func TestReplayBuffer_CursorStale(t *testing.T) {
	rb := NewReplayBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Append(QueueEntry{Sequence: Sequence(i), Payload: []byte{byte(i)}})
	}

	_, err := rb.Replay(1)
	if err != ErrCursorStale {
		t.Errorf("expected ErrCursorStale, got %v", err)
	}
}

func TestReplayBuffer_Clear(t *testing.T) {
	rb := NewReplayBuffer(5)
	rb.Append(QueueEntry{Sequence: 1, Payload: []byte("a")})
	rb.Append(QueueEntry{Sequence: 2, Payload: []byte("b")})

	rb.Clear()
	if rb.Len() != 0 {
		t.Errorf("expected len 0 after clear, got %d", rb.Len())
	}
	if rb.LatestSeq() != SequenceZero {
		t.Errorf("expected latest seq 0 after clear, got %d", rb.LatestSeq())
	}
}

func TestReplayBuffer_HasSequence(t *testing.T) {
	rb := NewReplayBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Append(QueueEntry{Sequence: Sequence(i), Payload: []byte{byte(i)}})
	}

	if rb.HasSequence(2) {
		t.Error("seq 2 should have been evicted")
	}
	if !rb.HasSequence(3) {
		t.Error("seq 3 should be present")
	}
	if !rb.HasSequence(5) {
		t.Error("seq 5 should be present")
	}
}

func TestReplayBuffer_DeepCopy(t *testing.T) {
	rb := NewReplayBuffer(5)
	original := []byte("original")
	rb.Append(QueueEntry{Sequence: 1, Payload: original})

	entries, err := rb.Replay(SequenceZero)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	original[0] = 'X'
	if entries[0].Payload[0] != 'o' {
		t.Error("replay entry should be deep copy, mutation affected it")
	}
}

func TestReplayBuffer_Range(t *testing.T) {
	rb := NewReplayBuffer(10)
	for i := 1; i <= 10; i++ {
		rb.Append(QueueEntry{Sequence: Sequence(i), Payload: []byte{byte(i)}})
	}

	rangeEntries := rb.ReplayRange(3, 6)
	if len(rangeEntries) != 4 {
		t.Errorf("expected 4 entries from range [3,6], got %d", len(rangeEntries))
	}
	if rangeEntries[0].Sequence != Sequence(3) {
		t.Errorf("first range entry should be seq 3, got %d", rangeEntries[0].Sequence)
	}
	if rangeEntries[3].Sequence != Sequence(6) {
		t.Errorf("last range entry should be seq 6, got %d", rangeEntries[3].Sequence)
	}
}

func TestStreamManager_ReplayResume(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	input := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-resume")
	serviceID := domain.ServiceID("svc-resume")
	channelID := domain.ChannelID("ch-resume")

	for i := 0; i < 10; i++ {
		err := sm.Publish(nil, input, runtimeID, serviceID, channelID, []byte{byte(i)})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	replay := sm.GetReplayBuffer(runtimeID, serviceID, channelID)
	entries, err := replay.Replay(7)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (seq 8,9,10), got %d", len(entries))
	}
	if entries[0].Sequence != Sequence(8) {
		t.Errorf("first should be seq 8, got %d", entries[0].Sequence)
	}
}

func TestStreamManager_MultiKeyCoalesce(t *testing.T) {
	resolver := NewPolicyResolver()
	sm := NewStreamManager(resolver)

	stateInput := PolicyInput{Kind: domain.ChannelKindState}
	eventInput := PolicyInput{Kind: domain.ChannelKindEvent}
	runtimeID := domain.RuntimeInstanceID("rt-multi")
	serviceID := domain.ServiceID("svc-multi")

	err := sm.Publish(nil, stateInput, runtimeID, serviceID, "state-ch", []byte("s1"))
	if err != nil {
		t.Fatalf("state publish: %v", err)
	}
	err = sm.Publish(nil, eventInput, runtimeID, serviceID, "event-ch", []byte("e1"))
	if err != nil {
		t.Fatalf("event publish: %v", err)
	}

	stateSeq := sm.GetSequence(runtimeID, serviceID, "state-ch")
	eventSeq := sm.GetSequence(runtimeID, serviceID, "event-ch")
	if stateSeq != Sequence(1) || eventSeq != Sequence(1) {
		t.Errorf("expected both seq 1, got state=%d event=%d", stateSeq, eventSeq)
	}
}
