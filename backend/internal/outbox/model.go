package outbox

import (
	"time"
)

type DeadLetterStatus string

const (
	DeadLetterStatusPending     DeadLetterStatus = "pending"
	DeadLetterStatusRetrying    DeadLetterStatus = "retrying"
	DeadLetterStatusArchived    DeadLetterStatus = "archived"
	DeadLetterStatusReprocessed DeadLetterStatus = "reprocessed"
)

type DeadLetterRecord struct {
	ID          string           `json:"id"`
	OutboxID    string           `json:"outboxId"`
	EventType   string           `json:"eventType"`
	Payload     []byte           `json:"payload"`
	Status      DeadLetterStatus `json:"status"`
	RetryCount  int              `json:"retryCount"`
	MaxRetries  int              `json:"maxRetries"`
	NextRetryAt time.Time        `json:"nextRetryAt"`
	LastError   string           `json:"lastError"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusLeased    OutboxStatus = "leased"
	OutboxStatusPublished OutboxStatus = "published"
	OutboxStatusFailed    OutboxStatus = "failed"
	OutboxStatusRetry     OutboxStatus = "retry"
	OutboxStatusDead      OutboxStatus = "dead"
)

type OutboxRecord struct {
	ID             string       `json:"id"`
	AggregateID    string       `json:"aggregateId"`
	EventType      string       `json:"eventType"`
	Payload        []byte       `json:"payload"`
	PayloadVersion string       `json:"payloadVersion"`
	Status         OutboxStatus `json:"status"`
	LeaseOwner     string       `json:"leaseOwner"`
	LeaseToken     string       `json:"leaseToken"`
	LeasedUntil    time.Time    `json:"leasedUntil"`
	AvailableAt    time.Time    `json:"availableAt"`
	PublishedAt    *time.Time   `json:"publishedAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	RetryCount     int          `json:"retryCount"`
	MaxRetries     int          `json:"maxRetries"`
	NextRetryAt    time.Time    `json:"nextRetryAt"`
	LastError      string       `json:"lastError"`
	IdempotencyKey string       `json:"idempotencyKey"`
	CreatedAt      time.Time    `json:"createdAt"`
}

const (
	DefaultMaxRetries  = 10
	DefaultLeaseTTL    = 60 * time.Second
	DefaultRenewWindow = 15 * time.Second
	DefaultBatchSize   = 20
)

func ValidTransitions() map[OutboxStatus][]OutboxStatus {



	return map[OutboxStatus][]OutboxStatus{
		OutboxStatusPending:   {OutboxStatusLeased},
		OutboxStatusLeased:    {OutboxStatusPublished, OutboxStatusFailed, OutboxStatusRetry},
		OutboxStatusFailed:    {OutboxStatusLeased, OutboxStatusDead},
		OutboxStatusRetry:     {OutboxStatusLeased, OutboxStatusDead},
		OutboxStatusDead:      {},
		OutboxStatusPublished: {},
	}
}

func ValidateTransition(from, to OutboxStatus) bool {
	allowed, ok := ValidTransitions()[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}
