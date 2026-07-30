package trust

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type PolicyMutationKind string

const (
	PolicyMutationPublisherTrust PolicyMutationKind = "publisher_trust"
	PolicyMutationRevocation     PolicyMutationKind = "revocation"
	PolicyMutationBlocklist      PolicyMutationKind = "blocklist"
)

type PolicyMutationState string

const (
	PolicyMutationPending PolicyMutationState = "pending"
	PolicyMutationActive  PolicyMutationState = "active"
)

type PolicyMutation struct {
	MutationID  string              `json:"mutationId"`
	Version     uint64              `json:"version"`
	Kind        PolicyMutationKind  `json:"kind"`
	State       PolicyMutationState `json:"state"`
	Actor       string              `json:"actor"`
	Reason      string              `json:"reason"`
	PublisherID string              `json:"publisherId,omitempty"`
	KeyID       string              `json:"keyId,omitempty"`
	ArtifactID  string              `json:"artifactId,omitempty"`
	PackageHash string              `json:"packageHash,omitempty"`
	OldValue    []byte              `json:"oldValue,omitempty"`
	NewValue    []byte              `json:"newValue,omitempty"`
	Restrictive bool                `json:"restrictive"`
	CreatedAt   time.Time           `json:"createdAt"`
	ActivatedAt *time.Time          `json:"activatedAt,omitempty"`
}

type PolicyMutationJournal interface {
	ReservePending(context.Context, PolicyMutation) (PolicyMutation, error)
	MarkActive(context.Context, PolicyMutation) error
	Pending(context.Context) ([]PolicyMutation, error)
}

type PolicyMutationApplier interface {
	Apply(context.Context, PolicyMutation) (func() error, error)
}

type PolicyMutationInvalidator interface {
	Invalidate(context.Context, PolicyMutation) error
}

type MutationCoordinator struct {
	journal      PolicyMutationJournal
	applier      PolicyMutationApplier
	invalidation PolicyMutationInvalidator
}

func NewMutationCoordinator(journal PolicyMutationJournal, applier PolicyMutationApplier, invalidation PolicyMutationInvalidator) *MutationCoordinator {
	return &MutationCoordinator{journal: journal, applier: applier, invalidation: invalidation}
}

func (c *MutationCoordinator) Execute(ctx context.Context, mutation PolicyMutation) (PolicyMutation, error) {
	if c == nil || c.journal == nil || c.applier == nil || c.invalidation == nil {
		return PolicyMutation{}, errors.New("trust: mutation coordinator unavailable")
	}
	if mutation.Actor == "" || mutation.Reason == "" || mutation.Kind == "" {
		return PolicyMutation{}, errors.New("trust: mutation actor, reason and kind required")
	}
	mutation.State = PolicyMutationPending
	if mutation.CreatedAt.IsZero() {
		mutation.CreatedAt = time.Now().UTC()
	}
	pending, err := c.journal.ReservePending(ctx, mutation)
	if err != nil {
		return PolicyMutation{}, fmt.Errorf("trust: persist pending mutation: %w", err)
	}
	rollback, err := c.applier.Apply(ctx, pending)
	if err != nil {
		return pending, fmt.Errorf("trust: apply pending mutation: %w", err)
	}
	if err := c.invalidation.Invalidate(ctx, pending); err != nil {
		c.rollbackPermissive(pending, rollback)
		return pending, fmt.Errorf("trust: invalidate stale authority: %w", err)
	}
	active := pending
	active.State = PolicyMutationActive
	now := time.Now().UTC()
	active.ActivatedAt = &now
	if err := c.journal.MarkActive(ctx, active); err != nil {
		c.rollbackPermissive(pending, rollback)
		return pending, fmt.Errorf("trust: activate mutation: %w", err)
	}
	return active, nil
}

func (c *MutationCoordinator) ReplayPending(ctx context.Context) error {
	if c == nil || c.journal == nil || c.applier == nil || c.invalidation == nil {
		return errors.New("trust: mutation coordinator unavailable")
	}
	pending, err := c.journal.Pending(ctx)
	if err != nil {
		return fmt.Errorf("trust: read pending mutations: %w", err)
	}
	for _, mutation := range pending {
		rollback, applyErr := c.applier.Apply(ctx, mutation)
		if applyErr != nil {
			return fmt.Errorf("trust: replay mutation %s: %w", mutation.MutationID, applyErr)
		}
		if invalidateErr := c.invalidation.Invalidate(ctx, mutation); invalidateErr != nil {
			c.rollbackPermissive(mutation, rollback)
			return fmt.Errorf("trust: replay invalidation %s: %w", mutation.MutationID, invalidateErr)
		}
		active := mutation
		active.State = PolicyMutationActive
		now := time.Now().UTC()
		active.ActivatedAt = &now
		if activeErr := c.journal.MarkActive(ctx, active); activeErr != nil {
			c.rollbackPermissive(mutation, rollback)
			return fmt.Errorf("trust: replay activation %s: %w", mutation.MutationID, activeErr)
		}
	}
	return nil
}

func (c *MutationCoordinator) rollbackPermissive(mutation PolicyMutation, rollback func() error) {
	if mutation.Restrictive || rollback == nil {
		return
	}
	_ = rollback()
}

type PolicyMutationApplierFunc func(context.Context, PolicyMutation) (func() error, error)

func (f PolicyMutationApplierFunc) Apply(ctx context.Context, mutation PolicyMutation) (func() error, error) {
	return f(ctx, mutation)
}

type PolicyMutationInvalidatorFunc func(context.Context, PolicyMutation) error

func (f PolicyMutationInvalidatorFunc) Invalidate(ctx context.Context, mutation PolicyMutation) error {
	return f(ctx, mutation)
}
