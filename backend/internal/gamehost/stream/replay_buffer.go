package stream

import (
	"sync"
)

type ReplayBuffer struct {
	mu          sync.RWMutex
	capacity    int
	maxBytes    int64
	bytes       int64
	head        int
	tail        int
	count       int
	entries     []replayEntry
	firstSeq    Sequence
	latestSeq   Sequence
}

type replayEntry struct {
	Sequence Sequence
	Payload  []byte
	Size     int64
}

func NewReplayBuffer(capacity int) *ReplayBuffer {
	return &ReplayBuffer{
		capacity: capacity,
		entries:  make([]replayEntry, capacity),
	}
}

func NewReplayBufferWithBytes(capacity int, maxBytes int64) *ReplayBuffer {
	rb := NewReplayBuffer(capacity)
	rb.maxBytes = maxBytes
	return rb
}

func (rb *ReplayBuffer) Cap() int {
	return rb.capacity
}

func (rb *ReplayBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

func (rb *ReplayBuffer) LatestSeq() Sequence {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.latestSeq
}

func (rb *ReplayBuffer) Append(entry QueueEntry) {
	if rb.capacity == 0 {
		return
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()

	re := replayEntry{
		Sequence: entry.Sequence,
		Payload:  bytesClone(entry.Payload),
		Size:     entry.ApproxSize(),
	}

	if rb.count == 0 {
		rb.head = 0
		rb.tail = 0
		rb.entries[0] = re
		rb.count = 1
		rb.firstSeq = entry.Sequence
		rb.latestSeq = entry.Sequence
		rb.bytes = re.Size
		return
	}

	rb.tail = (rb.tail + 1) % rb.capacity
	rb.entries[rb.tail] = re
	if rb.count < rb.capacity {
		rb.count++
	} else {
		old := rb.entries[rb.head]
		rb.bytes -= old.Size
		rb.head = (rb.head + 1) % rb.capacity
		rb.firstSeq = rb.entries[rb.head].Sequence
	}
	rb.bytes += re.Size
	rb.latestSeq = entry.Sequence
}

func (rb *ReplayBuffer) Replay(fromSeq Sequence) ([]QueueEntry, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil, nil
	}
	if fromSeq >= rb.latestSeq {
		return nil, nil
	}
	if fromSeq < rb.firstSeq {
		return nil, ErrCursorStale
	}

	startIdx := rb.head
	if fromSeq > rb.firstSeq {
		offset := int(fromSeq - rb.firstSeq)
		startIdx = (rb.head + offset) % rb.capacity
	}

	result := make([]QueueEntry, 0)
	idx := (startIdx + 1) % rb.capacity
	for i := 0; i < rb.count; i++ {
		if idx == (rb.tail+1)%rb.capacity && i > 0 {
			break
		}
		entry := rb.entries[idx]
		if entry.Sequence == SequenceZero {
			break
		}
		if entry.Sequence > fromSeq {
			result = append(result, QueueEntry{
				Sequence: entry.Sequence,
				Payload:  bytesClone(entry.Payload),
				Size:     entry.Size,
			})
		}
		idx = (idx + 1) % rb.capacity
		if idx == (rb.tail+1)%rb.capacity {
			break
		}
	}
	return result, nil
}

func (rb *ReplayBuffer) HasSequence(seq Sequence) bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.count == 0 {
		return false
	}
	return seq >= rb.firstSeq && seq <= rb.latestSeq
}

func (rb *ReplayBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.count = 0
	rb.head = 0
	rb.tail = 0
	rb.bytes = 0
	rb.firstSeq = SequenceZero
	rb.latestSeq = SequenceZero
	for i := range rb.entries {
		rb.entries[i] = replayEntry{}
	}
}

func (rb *ReplayBuffer) ReplayRange(from, to Sequence) []QueueEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.count == 0 || from >= to {
		return nil
	}
	result := make([]QueueEntry, 0)
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.capacity
		entry := rb.entries[idx]
		if entry.Sequence == SequenceZero {
			break
		}
		if entry.Sequence >= from && entry.Sequence <= to {
			result = append(result, QueueEntry{
				Sequence: entry.Sequence,
				Payload:  bytesClone(entry.Payload),
				Size:     entry.Size,
			})
		}
	}
	return result
}

func bytesClone(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
