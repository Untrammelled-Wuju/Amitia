package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

type packageTrustMutationApplier struct {
	service *trust.TrustService
}

func (a packageTrustMutationApplier) Apply(ctx context.Context, mutation trust.PolicyMutation) (func() error, error) {
	switch mutation.Kind {
	case trust.PolicyMutationPublisherTrust:
		var record PackagePublisherKeyRecord
		if err := json.Unmarshal(mutation.NewValue, &record); err != nil {
			return nil, err
		}
		previous, previousErr := a.service.Store().Get(ctx, record.PublisherID)
		if previousErr != nil && !errors.Is(previousErr, trust.ErrPublisherNotFound) {
			return nil, previousErr
		}
		identity := publisherIdentityFromRecord(record)
		if err := a.service.Store().RegisterUserDecision(identity); err != nil {
			return nil, err
		}
		if previousErr == nil {
			return func() error { return a.service.Store().RegisterUserDecision(*previous) }, nil
		}
		return nil, nil
	case trust.PolicyMutationRevocation:
		var entry trust.RevocationEntry
		if err := json.Unmarshal(mutation.NewValue, &entry); err != nil {
			return nil, err
		}
		return nil, a.service.RevokePackage(ctx, entry)
	case trust.PolicyMutationBlocklist:
		var entry trust.PackageBlockEntry
		if err := json.Unmarshal(mutation.NewValue, &entry); err != nil {
			return nil, err
		}
		return nil, a.service.BlockPackage(ctx, entry)
	default:
		return nil, fmt.Errorf("kernel: unsupported trust mutation kind %s", mutation.Kind)
	}
}

type packageTrustMutationInvalidator struct {
	repository *PackageRepository
	sessions   *dev_mode.SessionManager
}

func (i packageTrustMutationInvalidator) Invalidate(ctx context.Context, mutation trust.PolicyMutation) error {
	if i.repository == nil || i.sessions == nil {
		return errors.New("kernel: trust invalidation unavailable")
	}
	if _, err := i.repository.InvalidateTrustPreviews(ctx, mutation.PublisherID, mutation.ArtifactID, mutation.PackageHash); err != nil {
		return err
	}
	i.sessions.RevokeAll()
	return nil
}

func newPackageTrustMutationCoordinator(repository *PackageTrustRepository, service *trust.TrustService, packages *PackageRepository, sessions *dev_mode.SessionManager) *trust.MutationCoordinator {
	return trust.NewMutationCoordinator(repository, packageTrustMutationApplier{service: service}, packageTrustMutationInvalidator{repository: packages, sessions: sessions})
}

func NewPackageTrustMutationCoordinator(container *Container) *trust.MutationCoordinator {
	if container == nil {
		return nil
	}
	return newPackageTrustMutationCoordinator(container.PackageTrustRepository, container.TrustService, container.PackageRepository, container.DevModeSessions)
}

func restorePackageTrustMutations(ctx context.Context, repository *PackageTrustRepository, service *trust.TrustService, packages *PackageRepository, sessions *dev_mode.SessionManager) error {
	applier := packageTrustMutationApplier{service: service}
	invalidation := packageTrustMutationInvalidator{repository: packages, sessions: sessions}
	active, err := repository.ActivePolicyMutations(ctx)
	if err != nil {
		return err
	}
	for _, mutation := range active {
		if mutation.Kind == trust.PolicyMutationPublisherTrust {
			continue
		}
		if _, err := applier.Apply(ctx, mutation); err != nil {
			return fmt.Errorf("kernel: restore active trust mutation %s: %w", mutation.MutationID, err)
		}
		if err := invalidation.Invalidate(ctx, mutation); err != nil {
			return fmt.Errorf("kernel: invalidate restored trust mutation %s: %w", mutation.MutationID, err)
		}
	}
	return newPackageTrustMutationCoordinator(repository, service, packages, sessions).ReplayPending(ctx)
}

func publisherIdentityFromRecord(record PackagePublisherKeyRecord) trust.PublisherIdentity {
	createdAt, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		createdAt = time.Now().UTC()
	}
	key := trust.PublisherKey{KeyID: record.KeyID, PublisherID: record.PublisherID,
		PublicKey: append([]byte(nil), record.PublicKey...), Algorithm: trust.AlgorithmEd25519,
		State: trust.KeyState(record.KeyState), CreatedAt: createdAt, RevokedReason: record.RevocationReason}
	if record.RevokedAt != "" {
		if revokedAt, parseErr := time.Parse(time.RFC3339Nano, record.RevokedAt); parseErr == nil {
			key.RevokedAt = &revokedAt
		}
	}
	return trust.PublisherIdentity{PublisherID: record.PublisherID, DisplayName: record.PublisherID,
		TrustLevel: trust.TrustLevel(record.TrustLevel), Source: trust.TrustSourceUserDecision, Keys: []trust.PublisherKey{key}}
}
