package interaction

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DeadLetterStatus string

const (
	DeadLetterStatusPending   DeadLetterStatus = "pending"
	DeadLetterStatusReplaying DeadLetterStatus = "replaying"
	DeadLetterStatusReplayed  DeadLetterStatus = "replayed"
	DeadLetterStatusArchived  DeadLetterStatus = "archived"
)

type DeadLetterRecord struct {
	ID          string           `json:"id"`
	OutboxID    string           `json:"outboxId"`
	EventType   string           `json:"eventType"`
	AggregateID string           `json:"aggregateId"`
	Payload     []byte           `json:"payload"`
	LastError   string           `json:"lastError"`
	RetryCount  int              `json:"retryCount"`
	Status      DeadLetterStatus `json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	ReplayedAt  time.Time        `json:"replayedAt,omitempty"`
}

type DeadLetterStore interface {
	Append(record *DeadLetterRecord) (string, error)
	Get(id string) (*DeadLetterRecord, error)
	ListPending() ([]DeadLetterRecord, error)
	MarkReplaying(id string) error
	MarkReplayed(id string) error
	MarkArchived(id string) error
}

type InMemoryDeadLetterStore struct {
	mu      sync.RWMutex
	records map[string]*DeadLetterRecord
}

func NewInMemoryDeadLetterStore() *InMemoryDeadLetterStore {
	return &InMemoryDeadLetterStore{
		records: make(map[string]*DeadLetterRecord),
	}
}

func (s *InMemoryDeadLetterStore) Append(record *DeadLetterRecord) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.Status == "" {
		record.Status = DeadLetterStatusPending
	}
	s.records[record.ID] = record
	return record.ID, nil
}

func (s *InMemoryDeadLetterStore) Get(id string) (*DeadLetterRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[id]
	if !ok {
		return nil, errors.New("dead_letter: record not found")
	}
	return rec, nil
}

func (s *InMemoryDeadLetterStore) ListPending() ([]DeadLetterRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []DeadLetterRecord
	for _, rec := range s.records {
		if rec.Status == DeadLetterStatusPending {
			result = append(result, *rec)
		}
	}
	return result, nil
}

func (s *InMemoryDeadLetterStore) MarkReplaying(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("dead_letter: record not found")
	}
	if rec.Status != DeadLetterStatusPending {
		return errors.New("dead_letter: record not in pending state")
	}
	rec.Status = DeadLetterStatusReplaying
	return nil
}

func (s *InMemoryDeadLetterStore) MarkReplayed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("dead_letter: record not found")
	}
	rec.Status = DeadLetterStatusReplayed
	rec.ReplayedAt = time.Now()
	return nil
}

func (s *InMemoryDeadLetterStore) MarkArchived(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("dead_letter: record not found")
	}
	rec.Status = DeadLetterStatusArchived
	return nil
}

var _ DeadLetterStore = (*InMemoryDeadLetterStore)(nil)
