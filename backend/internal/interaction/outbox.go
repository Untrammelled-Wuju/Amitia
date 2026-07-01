package interaction

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusPublished OutboxStatus = "published"
	OutboxStatusFailed    OutboxStatus = "failed"
	OutboxStatusDead      OutboxStatus = "dead"
)

const DefaultMaxRetries = 5
const DefaultRetryBackoff = 2 * time.Second

var (
	ErrOutboxAlreadyPublished = errors.New("outbox: record already published")
	ErrOutboxAlreadyDead      = errors.New("outbox: record is dead")
)

type OutboxRecord struct {
	ID          string       `json:"id"`
	AggregateID string       `json:"aggregateId"`
	EventType   string       `json:"eventType"`
	Payload     []byte       `json:"payload"`
	Status      OutboxStatus `json:"status"`
	RetryCount  int          `json:"retryCount"`
	MaxRetries  int          `json:"maxRetries"`
	NextRetryAt time.Time    `json:"nextRetryAt"`
	LastError   string       `json:"lastError,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
}

type OutboxPublisher interface {
	Publish(record OutboxRecord) error
}

type OutboxStore interface {
	Append(record *OutboxRecord) (string, error)
	MarkPublished(id string) error
	MarkFailed(id string, errMsg string) error
	MarkDead(id string) error
	ListPending() ([]OutboxRecord, error)
	Get(id string) (*OutboxRecord, error)
}

type InMemoryOutboxStore struct {
	mu      sync.RWMutex
	records map[string]*OutboxRecord
	order   []string
}

func NewInMemoryOutboxStore() *InMemoryOutboxStore {
	return &InMemoryOutboxStore{
		records: make(map[string]*OutboxRecord),
		order:   make([]string, 0),
	}
}

func (s *InMemoryOutboxStore) Append(record *OutboxRecord) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.Status == "" {
		record.Status = OutboxStatusPending
	}
	if record.MaxRetries == 0 {
		record.MaxRetries = DefaultMaxRetries
	}
	if record.NextRetryAt.IsZero() {
		record.NextRetryAt = record.CreatedAt
	}
	s.records[record.ID] = record
	s.order = append(s.order, record.ID)
	return record.ID, nil
}

func (s *InMemoryOutboxStore) MarkPublished(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("outbox: record not found")
	}
	if rec.Status == OutboxStatusPublished {
		return ErrOutboxAlreadyPublished
	}
	rec.Status = OutboxStatusPublished
	return nil
}

func (s *InMemoryOutboxStore) MarkFailed(id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("outbox: record not found")
	}
	rec.Status = OutboxStatusFailed
	rec.LastError = errMsg
	rec.RetryCount++
	rec.NextRetryAt = time.Now().Add(DefaultRetryBackoff * time.Duration(rec.RetryCount))
	return nil
}

func (s *InMemoryOutboxStore) MarkDead(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("outbox: record not found")
	}
	if rec.Status == OutboxStatusDead {
		return ErrOutboxAlreadyDead
	}
	rec.Status = OutboxStatusDead
	return nil
}

func (s *InMemoryOutboxStore) ListPending() ([]OutboxRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var result []OutboxRecord
	for _, rec := range s.records {
		if rec.Status == OutboxStatusPending || (rec.Status == OutboxStatusFailed && now.After(rec.NextRetryAt) && rec.RetryCount < rec.MaxRetries) {
			result = append(result, *rec)
		}
	}
	return result, nil
}

func (s *InMemoryOutboxStore) Get(id string) (*OutboxRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[id]
	if !ok {
		return nil, errors.New("outbox: record not found")
	}
	return rec, nil
}

type OutboxResult struct {
	SuccessCount int      `json:"successCount"`
	FailCount    int      `json:"failCount"`
	DeadCount    int      `json:"deadCount"`
	Errors       []string `json:"errors,omitempty"`
}

func (r OutboxResult) MarshalJSON() ([]byte, error) {
	type Alias OutboxResult
	return json.Marshal(Alias(r))
}

var _ OutboxStore = (*InMemoryOutboxStore)(nil)
