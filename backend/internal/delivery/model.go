package delivery

import (
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusUnknown   DeliveryStatus = "unknown"
)

type DeliveryIntent struct {
	ID            string         `json:"id"`
	InteractionID string         `json:"interactionId"`
	Channel       string         `json:"channel"`
	PeerID        string         `json:"peerId"`
	ContentType   string         `json:"contentType"`
	Payload       []byte         `json:"payload"`
	Status        DeliveryStatus `json:"status"`
	CreatedAt     time.Time      `json:"createdAt"`
	DeliveredAt   *time.Time     `json:"deliveredAt"`
	RetryCount    int            `json:"retryCount"`
	MaxRetries    int            `json:"maxRetries"`
	LastError     string         `json:"lastError"`
}

type OutputLease struct {
	ID            string     `json:"id"`
	InteractionID string     `json:"interactionId"`
	CharacterID   string     `json:"characterId"`
	UserID        string     `json:"userId"`
	Channel       string     `json:"channel"`
	Status        string     `json:"status"`
	AcquiredAt    time.Time  `json:"acquiredAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	ReleasedAt    *time.Time `json:"releasedAt"`
	PreemptedBy   string     `json:"preemptedBy"`
}

type ChannelAdapter interface {
	Deliver(intent DeliveryIntent) error
	Name() string
}

type IntentStore interface {
	CreateIntent(intent DeliveryIntent) error
	GetIntent(id string) (*DeliveryIntent, error)
	UpdateStatus(id string, status DeliveryStatus, errMsg string) error
	ListPending(limit int) ([]DeliveryIntent, error)
}

func NewDeliveryIntent(interactionID, channel, peerID, contentType string, payload []byte) DeliveryIntent {
	return DeliveryIntent{
		ID:            uuid.New().String(),
		InteractionID: interactionID,
		Channel:       channel,
		PeerID:        peerID,
		ContentType:   contentType,
		Payload:       payload,
		Status:        DeliveryStatusPending,
		MaxRetries:    5,
		CreatedAt:     time.Now().UTC(),
	}
}

func NewOutputLease(interactionID, characterID, userID, channel string) OutputLease {
	return OutputLease{
		ID:            uuid.New().String(),
		InteractionID: interactionID,
		CharacterID:   characterID,
		UserID:        userID,
		Channel:       channel,
		Status:        "active",
		AcquiredAt:    time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(30 * time.Second),
	}
}

func (l *OutputLease) IsExpired() bool {
	return time.Now().UTC().After(l.ExpiresAt)
}

func (l *OutputLease) Preempt(byID string) {
	l.Status = "preempted"
	l.PreemptedBy = byID
	now := time.Now().UTC()
	l.ReleasedAt = &now
}

func (l *OutputLease) Release() {
	l.Status = "released"
	now := time.Now().UTC()
	l.ReleasedAt = &now
}
