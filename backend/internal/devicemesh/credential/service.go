package credential

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	repo      *Repository
	ttlSeconds int64
	clock     Clock
}

func NewService(repo *Repository, ttlSeconds int64) *Service {
	return &Service{repo: repo, ttlSeconds: ttlSeconds, clock: SystemClock{}}
}

func (s *Service) WithClock(clock Clock) *Service {
	s.clock = clock
	return s
}

func (s *Service) Exchange(ctx context.Context, ticket *ExchangeTicketView) (*DeviceRuntimeCredential, string, error) {
	now := s.clock.Now()

	if ticket.Status != "active" && ticket.Status != "consumed" {
		return nil, "", errors.New("credential: ticket not active")
	}
	if now.After(ticket.ExpiresAt) {
		return nil, "", errors.New("credential: ticket expired")
	}

	raw, err := GenerateRawCredential()
	if err != nil {
		return nil, "", err
	}
	hash := HashRawCredential(raw)

	expires := now.Add(time.Duration(s.ttlSeconds) * time.Second)
	cred := &DeviceRuntimeCredential{
		ID:             uuid.New().String(),
		UserID:         ticket.UserID,
		DeviceID:       ticket.DeviceID,
		RuntimeID:      ticket.RuntimeID,
		CredentialHash: hash,
		Status:         CredentialActive,
		CreatedAt:      now,
		ExpiresAt:      expires,
		LastUsedAt:     now,
		Revision:       1,
	}

	if err := s.repo.ExchangeAtomic(ctx, ticket.UserID, ticket.DeviceID, ticket.RuntimeID, now, cred); err != nil {
		return nil, "", fmt.Errorf("credential: exchange atomic: %w", err)
	}

	return cred, raw, nil
}

func (s *Service) Validate(ctx context.Context, rawCredential string) (*DeviceRuntimeCredential, error) {
	now := s.clock.Now()
	hash := HashRawCredential(rawCredential)
	cred, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, errors.New("credential: not found")
	}
	if cred.Status == CredentialRevoked {
		return nil, errors.New("credential: revoked")
	}
	if cred.Status == CredentialExpired || now.After(cred.ExpiresAt) {
		return nil, errors.New("credential: expired")
	}

	if err := s.repo.UpdateLastUsed(ctx, cred.ID, now); err != nil {
		return nil, fmt.Errorf("credential: update last used: %w", err)
	}
	cred.LastUsedAt = now
	return cred, nil
}

func (s *Service) Revoke(ctx context.Context, credentialID string) error {
	return s.repo.RevokeByID(ctx, credentialID, s.clock.Now())
}

func (s *Service) RevokeAllForDevice(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) error {
	return s.repo.RevokeAllForDevice(ctx, userID, deviceID, s.clock.Now())
}

func (s *Service) ListByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*DeviceRuntimeCredential, error) {
	return s.repo.ListByUser(ctx, userID)
}

type ExchangeTicketView struct {
	UserID    runtimeidentity.UserID
	DeviceID  runtimeidentity.DeviceID
	RuntimeID runtimeidentity.RuntimeID
	Status    string
	ExpiresAt time.Time
}
