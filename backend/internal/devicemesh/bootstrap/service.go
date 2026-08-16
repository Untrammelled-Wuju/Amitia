package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CredExchangeFunc func(ctx context.Context, tx *sql.Tx, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, now time.Time, credHash string, expires time.Time) (string, string, error)

type DeviceTrustFunc func(ctx context.Context, tx *sql.Tx, deviceID runtimeidentity.DeviceID) error

type Service struct {
	repo       *Repository
	db         *sql.DB
	ticketTTL  time.Duration
	credTTL    int64
	exchangeFn CredExchangeFunc
	trustFn    DeviceTrustFunc
}

func NewService(repo *Repository, ttlSeconds int) *Service {
	return &Service{
		repo:      repo,
		ticketTTL: time.Duration(ttlSeconds) * time.Second,
	}
}

func NewServiceWithDependencies(repo *Repository, db *sql.DB, exchangeFn CredExchangeFunc, trustFn DeviceTrustFunc, ticketTTL int, credTTL int64) *Service {
	return &Service{
		repo:      repo,
		db:        db,
		exchangeFn: exchangeFn,
		trustFn:    trustFn,
		ticketTTL: time.Duration(ticketTTL) * time.Second,
		credTTL:   credTTL,
	}
}

func (s *Service) Issue(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID, platform runtimeidentity.Platform) (*BootstrapTicket, string, error) {
	raw, err := GenerateRawTicket()
	if err != nil {
		return nil, "", err
	}
	hash := HashRawTicket(raw)

	now := time.Now().UTC()
	ticket := &BootstrapTicket{
		TicketID:   uuid.New().String(),
		TicketHash: hash,
		UserID:     userID,
		DeviceID:   deviceID,
		RuntimeID:  runtimeID,
		Platform:   platform,
		Status:     TicketActive,
		ExpiresAt:  now.Add(s.ticketTTL),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, ticket); err != nil {
		return nil, "", fmt.Errorf("bootstrap: issue: %w", err)
	}

	return ticket, raw, nil
}

func (s *Service) Consume(ctx context.Context, rawTicket string) (*BootstrapTicket, error) {
	hash := HashRawTicket(rawTicket)
	ticket, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("bootstrap: ticket not found")
	}

	now := time.Now().UTC()
	ok, err := s.repo.Consume(ctx, hash, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		if ticket.Status == TicketConsumed {
			return nil, fmt.Errorf("bootstrap: ticket already consumed")
		}
		if ticket.Status == TicketExpired || now.After(ticket.ExpiresAt) {
			return nil, fmt.Errorf("bootstrap: ticket expired")
		}
		if ticket.Status == TicketRevoked {
			return nil, fmt.Errorf("bootstrap: ticket revoked")
		}
		return nil, fmt.Errorf("bootstrap: ticket invalid")
	}

	ticket.Status = TicketConsumed
	consumedAt := now
	ticket.ConsumedAt = &consumedAt
	ticket.UpdatedAt = now
	return ticket, nil
}

type TicketSnapshot struct {
	UserID    runtimeidentity.UserID
	DeviceID  runtimeidentity.DeviceID
	RuntimeID runtimeidentity.RuntimeID
	ExpiresAt time.Time
}

type TicketValidator interface {
	Validate(ctx context.Context, rawTicket string) (*TicketSnapshot, error)
}

func (s *Service) Validate(ctx context.Context, rawTicket string) (*TicketSnapshot, error) {
	hash := HashRawTicket(rawTicket)
	ticket, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("bootstrap: ticket not found")
	}

	now := time.Now().UTC()
	if ticket.Status == TicketConsumed {
		return nil, fmt.Errorf("bootstrap: ticket already consumed")
	}
	if ticket.Status == TicketExpired || now.After(ticket.ExpiresAt) {
		return nil, fmt.Errorf("bootstrap: ticket expired")
	}
	if ticket.Status == TicketRevoked {
		return nil, fmt.Errorf("bootstrap: ticket revoked")
	}
	return &TicketSnapshot{
		UserID:    ticket.UserID,
		DeviceID:  ticket.DeviceID,
		RuntimeID: ticket.RuntimeID,
		ExpiresAt: ticket.ExpiresAt,
	}, nil
}

func (s *Service) Exchange(ctx context.Context, rawTicket string, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (string, string, error) {
	if s.db == nil || s.exchangeFn == nil || s.trustFn == nil {
		return "", "", fmt.Errorf("bootstrap: exchange not fully configured")
	}

	hash := HashRawTicket(rawTicket)
	ticket, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return "", "", err
	}
	if ticket == nil {
		return "", "", fmt.Errorf("bootstrap: ticket not found")
	}

	now := time.Now().UTC()
	if ticket.Status != TicketActive {
		return "", "", fmt.Errorf("bootstrap: ticket not active")
	}
	if now.After(ticket.ExpiresAt) {
		return "", "", fmt.Errorf("bootstrap: ticket expired")
	}

	expires := now.Add(time.Duration(s.credTTL) * time.Second)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("bootstrap: begin tx: %w", err)
	}
	defer tx.Rollback()

	consumed, err := s.repo.ConsumeTx(ctx, tx, hash, now)
	if err != nil {
		return "", "", fmt.Errorf("bootstrap: consume: %w", err)
	}
	if !consumed {
		return "", "", fmt.Errorf("bootstrap: ticket already consumed")
	}

	credID, rawCred, err := s.exchangeFn(ctx, tx, ticket.UserID, ticket.DeviceID, ticket.RuntimeID, now, hash, expires)
	if err != nil {
		return "", "", fmt.Errorf("credential: exchange: %w", err)
	}

	if err := s.trustFn(ctx, tx, deviceID); err != nil {
		return "", "", fmt.Errorf("device: mark trusted: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("bootstrap: commit: %w", err)
	}

	ticket.Status = TicketConsumed
	consumedAt := now
	ticket.ConsumedAt = &consumedAt
	ticket.UpdatedAt = now

	return credID, rawCred, nil
}

func (s *Service) RevokeExpired(ctx context.Context) (int64, error) {
	return s.repo.RevokeExpired(ctx, time.Now().UTC())
}
