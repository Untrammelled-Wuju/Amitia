package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type Service struct {
	repo    *Repository
	ticketTTL time.Duration
}

func NewService(repo *Repository, ttlSeconds int) *Service {
	return &Service{
		repo:      repo,
		ticketTTL: time.Duration(ttlSeconds) * time.Second,
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

func (s *Service) RevokeExpired(ctx context.Context) (int64, error) {
	return s.repo.RevokeExpired(ctx, time.Now().UTC())
}
