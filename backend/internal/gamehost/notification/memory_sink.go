package notification

import (
	"context"
	"sync"
)

type MemorySink struct {
	mu            sync.Mutex
	notifications []Notification
}

func NewMemorySink() *MemorySink {
	return &MemorySink{
		notifications: make([]Notification, 0),
	}
}

func (s *MemorySink) Publish(ctx context.Context, n Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = append(s.notifications, n)
	return nil
}

func (s *MemorySink) Snapshot() []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Notification, len(s.notifications))
	copy(out, s.notifications)
	return out
}

func (s *MemorySink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.notifications)
}

func (s *MemorySink) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = s.notifications[:0]
}
