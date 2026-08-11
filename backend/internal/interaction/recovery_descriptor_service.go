package interaction

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type RecoveryAssociator interface {
	Associate(ctx context.Context, input RecoveryDescriptorInput) (*RecoveryDescriptor, error)
}

type RecoveryDescriptorService struct {
	tracker   InteractionTracker
	builder   *RecoveryDescriptorBuilder
	validator *RecoveryDescriptorValidator
	mu        sync.Mutex
	now       func() time.Time
}

func NewRecoveryDescriptorService(
	tracker InteractionTracker,
	builder *RecoveryDescriptorBuilder,
	validator *RecoveryDescriptorValidator,
) *RecoveryDescriptorService {
	return &RecoveryDescriptorService{
		tracker:   tracker,
		builder:   builder,
		validator: validator,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *RecoveryDescriptorService) Associate(ctx context.Context, input RecoveryDescriptorInput) (*RecoveryDescriptor, error) {
	if input.Interaction == nil {
		return nil, errors.New("recovery: input interaction is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok, err := s.tracker.Get(ctx, input.Interaction.ID)
	if err != nil {
		return nil, fmt.Errorf("recovery: fetch interaction: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("recovery: interaction not found: %s", input.Interaction.ID)
	}
	if input.Require == "" {
		input.Require = RecoveryBestEffort
	}
	if record.Status == InteractionStatusCommitted {
		return nil, fmt.Errorf("recovery: interaction already committed: %s", record.ID)
	}
	if s.validator != nil {
		if existing := record.RecoveryDescriptor; existing != nil {
			if err := s.checkScopeMatch(existing, record); err != nil {
				return nil, err
			}
		}
	}
	desc, err := s.builder.Build(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("recovery: build: %w", err)
	}
	if existing := record.RecoveryDescriptor; existing != nil && existing.Fingerprint == desc.Fingerprint {
		existing.UpdatedAt = s.now()
		return existing, nil
	}
	desc.UpdatedAt = s.now()
	descJSON, err := desc.NormalizeOnSerialize()
	if err != nil {
		return nil, fmt.Errorf("recovery: serialize: %w", err)
	}
	expectedVersion := record.StatusVersion
	saved, err := s.persist(ctx, record, descJSON, expectedVersion)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *RecoveryDescriptorService) checkScopeMatch(existing *RecoveryDescriptor, record *InteractionRecord) error {
	if existing.Scope.UserID != record.Scope.UserID {
		return fmt.Errorf("recovery: scope_mismatch: userId")
	}
	if existing.Scope.CharacterID != "" && record.Scope.CharacterID != "" && existing.Scope.CharacterID != record.Scope.CharacterID {
		return fmt.Errorf("recovery: scope_mismatch: characterId")
	}
	if existing.Scope.ConversationID != "" && record.Scope.ConversationID != "" && existing.Scope.ConversationID != record.Scope.ConversationID {
		return fmt.Errorf("recovery: scope_mismatch: conversationId")
	}
	return nil
}

func (s *RecoveryDescriptorService) persist(ctx context.Context, record *InteractionRecord, descJSON []byte, expectedVersion int64) (*RecoveryDescriptor, error) {
	if expectedVersion < 0 {
		return nil, ErrInteractionCASConflict
	}
	if len(descJSON) == 0 {
		return nil, errors.New("recovery: empty descriptor JSON")
	}
	desc, err := DescriptorFromJSON(descJSON)
	if err != nil {
		return nil, fmt.Errorf("recovery: prepare descriptor: %w", err)
	}
	desc.Revision = 1
	if existing := record.RecoveryDescriptor; existing != nil {
		desc.Revision = existing.Revision + 1
	}
	if desc.Revision < 1 {
		desc.Revision = 1
	}
	desc.UpdatedAt = s.now()
	update := InteractionMetadataUpdate{
		RecoveryDescriptor: desc,
	}
	if expectedVersion > 0 {
		v := expectedVersion
		update.ExpectedStatusVersion = &v
	}
	if _, err = s.tracker.UpdateMetadata(ctx, record.ID, update); err != nil {
		if errors.Is(err, ErrInteractionCASConflict) {
			return nil, err
		}
		return nil, fmt.Errorf("recovery: persist descriptor: %w", err)
	}
	return desc, nil
}

func (s *RecoveryDescriptorService) Load(ctx context.Context, interactionID string) (*RecoveryDescriptor, bool, error) {
	record, ok, err := s.tracker.Get(ctx, interactionID)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	if record.RecoveryDescriptor == nil {
		return nil, false, nil
	}
	if data, err := record.RecoveryDescriptor.NormalizeOnSerialize(); err == nil && len(data) > 0 {
		return record.RecoveryDescriptor, true, nil
	}
	return record.RecoveryDescriptor, true, nil
}

func (s *RecoveryDescriptorService) Validate(ctx context.Context, d RecoveryDescriptor) (RecoveryValidationResult, error) {
	if s.validator == nil {
		return RecoveryValidationResult{
			Compatibility: RecoveryManual,
			RecoveryClass: AgentRecoveryManual,
		}, errors.New("recovery: validator not configured")
	}
	return s.validator.Validate(ctx, &d)
}
