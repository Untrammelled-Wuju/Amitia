package interaction

import (
	"context"
	"errors"
	"fmt"
)

type PauseResumeService struct {
	tracker InteractionTracker
}

func NewPauseResumeService(tracker InteractionTracker) *PauseResumeService {
	return &PauseResumeService{tracker: tracker}
}

func (s *PauseResumeService) Pause(ctx context.Context, interactionID, reason string) error {
	record, ok, err := s.tracker.Get(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("pause: get: %w", err)
	}
	if !ok {
		return ErrInteractionNotFound
	}
	if isPausedStatus(record.Status) {
		return nil
	}
	if record.IsTerminal() {
		return fmt.Errorf("pause: interaction %s is terminal (%s)", interactionID, record.Status)
	}
	if !isActiveStatus(record.Status) && !isProcessingStatus(record.Status) {
		return fmt.Errorf("pause: interaction %s cannot pause from status %s", interactionID, record.Status)
	}

	updated, err := s.tracker.TransitionCAS(ctx, interactionID, record.StatusVersion, InteractionStatusPausing)
	if err != nil {
		return fmt.Errorf("pause transition: %w", err)
	}

	final, ferr := s.tracker.TransitionCAS(ctx, updated.ID, updated.StatusVersion, InteractionStatusPaused)
	if ferr != nil {
		if errors.Is(ferr, ErrVersionConflict) || errors.Is(ferr, ErrInvalidTransition) {
			return nil
		}
		return fmt.Errorf("pause finalize: %w", ferr)
	}
	_ = final
	return nil
}

func (s *PauseResumeService) Resume(ctx context.Context, interactionID string) error {
	record, ok, err := s.tracker.Get(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("resume: get: %w", err)
	}
	if !ok {
		return ErrInteractionNotFound
	}
	if record.IsActive() && !isPausedStatus(record.Status) {
		return nil
	}
	if record.Status != InteractionStatusPaused {
		return fmt.Errorf("resume: interaction %s not paused (status=%s)", interactionID, record.Status)
	}

	updated, err := s.tracker.TransitionCAS(ctx, interactionID, record.StatusVersion, InteractionStatusResuming)
	if err != nil {
		return fmt.Errorf("resume transition: %w", err)
	}

	final, ferr := s.tracker.TransitionCAS(ctx, updated.ID, updated.StatusVersion, InteractionStatusProcessing)
	if ferr != nil {
		if errors.Is(ferr, ErrVersionConflict) || errors.Is(ferr, ErrInvalidTransition) {
			return nil
		}
		return fmt.Errorf("resume finalize: %w", ferr)
	}
	_ = final
	return nil
}

func (s *PauseResumeService) ListControllable(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	records, err := s.tracker.ListActive(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := make([]*InteractionRecord, 0, len(records))
	for _, r := range records {
		if !isPausedStatus(r.Status) && !r.IsTerminal() {
			result = append(result, r)
		}
	}
	return result, nil
}

func isProcessingStatus(status InteractionStatus) bool {
	switch status {
	case InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided:
		return true
	default:
		return false
	}
}
