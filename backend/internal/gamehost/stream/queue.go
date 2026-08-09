package stream

import (
	"container/list"
	"sync"
)

type QueueEntry struct {
	Sequence  Sequence
	Payload   []byte
	Size      int64
	CreatedAt int64
}

func (e QueueEntry) ApproxSize() int64 {
	if e.Size > 0 {
		return e.Size
	}
	return int64(len(e.Payload) + 64)
}

type BoundedQueue struct {
	mu       sync.Mutex
	capacity int
	bytes    int64
	maxBytes int64
	items    *list.List
	coalesce map[string]*list.Element
}

func NewBoundedQueue(capacity int) *BoundedQueue {
	return &BoundedQueue{
		capacity: capacity,
		items:    list.New(),
		coalesce: make(map[string]*list.Element),
	}
}

func NewBoundedQueueWithBytes(capacity int, maxBytes int64) *BoundedQueue {
	q := NewBoundedQueue(capacity)
	q.maxBytes = maxBytes
	return q
}

func (q *BoundedQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

func (q *BoundedQueue) Cap() int {
	return q.capacity
}

func (q *BoundedQueue) Bytes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.bytes
}

func (q *BoundedQueue) Push(entry QueueEntry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items.PushBack(entry)
	q.bytes += entry.ApproxSize()
}

func (q *BoundedQueue) PopOldest() QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	front := q.items.Front()
	if front == nil {
		return QueueEntry{}
	}
	entry := q.items.Remove(front).(QueueEntry)
	q.bytes -= entry.ApproxSize()
	if q.bytes < 0 {
		q.bytes = 0
	}
	return entry
}

func (q *BoundedQueue) PeekOldest() QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	front := q.items.Front()
	if front == nil {
		return QueueEntry{}
	}
	return front.Value.(QueueEntry)
}

func (q *BoundedQueue) PeekNewest() QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	back := q.items.Back()
	if back == nil {
		return QueueEntry{}
	}
	return back.Value.(QueueEntry)
}

func (q *BoundedQueue) IsFull() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len() >= q.capacity
}

func (q *BoundedQueue) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len() == 0
}

func (q *BoundedQueue) Drain() []QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]QueueEntry, 0, q.items.Len())
	for e := q.items.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(QueueEntry))
	}
	q.items.Init()
	q.bytes = 0
	for k := range q.coalesce {
		delete(q.coalesce, k)
	}
	return result
}

func (q *BoundedQueue) Coalesce(entry QueueEntry, key string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if elem, ok := q.coalesce[key]; ok {
		old := elem.Value.(QueueEntry)
		q.bytes -= old.ApproxSize()
		elem.Value = entry
		q.bytes += entry.ApproxSize()
		return true
	}
	elem := q.items.PushBack(entry)
	q.coalesce[key] = elem
	q.bytes += entry.ApproxSize()
	return false
}

func (q *BoundedQueue) CoalesceOrPush(entry QueueEntry, key string) (coalesced bool) {
	return q.Coalesce(entry, key)
}
