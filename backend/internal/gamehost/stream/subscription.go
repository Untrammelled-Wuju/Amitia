package stream

import (
	"sync"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type SubscriptionID string

func NewSubscriptionID() SubscriptionID {
	return SubscriptionID("sub_" + uuid.New().String())
}

type Subscription struct {
	ID          SubscriptionID
	RuntimeID   domain.RuntimeInstanceID
	ServiceID   domain.ServiceID
	ChannelID   domain.ChannelID
	Cursor      Cursor
	Generation  StreamGeneration
	Queue       *BoundedQueue
	mu          sync.Mutex
	closed      bool
	closedCh    chan struct{}
}

func NewSubscription(id SubscriptionID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID, generation StreamGeneration, queueCap int) *Subscription {
	return &Subscription{
		ID:         id,
		RuntimeID:  runtimeID,
		ServiceID:  serviceID,
		ChannelID:  channelID,
		Generation: generation,
		Queue:      NewBoundedQueue(queueCap),
		closedCh:   make(chan struct{}),
	}
}

func (s *Subscription) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Subscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.closedCh)
}

func (s *Subscription) ClosedCh() <-chan struct{} {
	return s.closedCh
}

func (s *Subscription) AdvanceCursor(seq Sequence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cursor.Sequence = seq
}

func (s *Subscription) Key() string {
	return string(s.RuntimeID) + "/" + string(s.ServiceID) + "/" + string(s.ChannelID)
}
