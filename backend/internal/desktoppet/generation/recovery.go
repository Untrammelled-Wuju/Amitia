package generation

import (
	"fmt"

	"github.com/u-ai/backend/internal/imageprovider"
)

type RecoveryPhase string

const (
	RecoveryPhasePendingSubmit     RecoveryPhase = "pending_submit"
	RecoveryPhaseSubmittedAsync    RecoveryPhase = "submitted_async"
	RecoveryPhaseResultReceived    RecoveryPhase = "result_received"
	RecoveryPhaseArtifactPersisted RecoveryPhase = "artifact_persisted"
	RecoveryPhaseCompleted         RecoveryPhase = "completed"
	RecoveryPhaseFailed            RecoveryPhase = "failed"
)

type RecoveryDecision struct {
	Phase           RecoveryPhase
	ShouldResume    bool
	ShouldReSubmit  bool
	ShouldReQuery   bool
	ShouldRePersist bool
	ShouldSkip      bool
	Reason          string
	Error           *GenerationError
}

type RecoveryEvaluator struct {
	attemptRepo  AttemptRepository
	artifactRepo ArtifactRepository
}

func NewRecoveryEvaluator(attemptRepo AttemptRepository, artifactRepo ArtifactRepository) *RecoveryEvaluator {
	return &RecoveryEvaluator{
		attemptRepo:  attemptRepo,
		artifactRepo: artifactRepo,
	}
}

func (e *RecoveryEvaluator) Evaluate(attempt *ActionGenerationAttempt, caps imageprovider.ProviderCapabilities) (*RecoveryDecision, error) {
	if attempt == nil {
		return nil, NewGenerationError(ErrCodeRecoveryParentMissing, "attempt is nil", nil)
	}

	if attempt.Status == string(AttemptStatusSucceeded) {
		return &RecoveryDecision{
			Phase:      RecoveryPhaseCompleted,
			ShouldSkip: true,
			Reason:     "attempt already succeeded",
		}, nil
	}

	if attempt.Status == string(AttemptStatusFailed) {
		return &RecoveryDecision{
			Phase:      RecoveryPhaseFailed,
			ShouldSkip: true,
			Reason:     "attempt already failed",
		}, nil
	}

	hasPrimary, err := e.artifactRepo.HasPrimaryArtifact(attempt.ID)
	if err != nil {
		return nil, NewGenerationError(ErrCodeRecoveryArtifactPersisted, "failed to check primary artifact", err)
	}
	if hasPrimary {
		return &RecoveryDecision{
			Phase:      RecoveryPhaseArtifactPersisted,
			ShouldSkip: true,
			Reason:     "primary artifact already persisted, skip re-generation",
		}, nil
	}

	status := AttemptStatus(attempt.Status)

	switch status {
	case AttemptStatusPending,
		AttemptStatusPreparingReference,
		AttemptStatusBuildingPrompt,
		AttemptStatusWaitingRateLimit:
		return &RecoveryDecision{
			Phase:        RecoveryPhasePendingSubmit,
			ShouldResume: true,
			Reason:       "resume from pre-submit phase",
		}, nil

	case AttemptStatusSubmitting:
		if !caps.SupportsIdempotencyKey {
			return &RecoveryDecision{
				Phase:  RecoveryPhasePendingSubmit,
				Error:  NewGenerationError(ErrCodeRecoveryIdempotentNotSafe, "provider does not support idempotency key", nil),
				Reason: "cannot safely re-submit without idempotency support",
			}, nil
		}
		return &RecoveryDecision{
			Phase:          RecoveryPhasePendingSubmit,
			ShouldReSubmit: true,
			Reason:         "re-submit with idempotency key",
		}, nil

	case AttemptStatusUnknownSubmission:
		if !caps.SupportsIdempotencyKey {
			return &RecoveryDecision{
				Phase:  RecoveryPhaseSubmittedAsync,
				Error:  NewGenerationError(ErrCodeRecoveryIdempotentNotSafe, "submission status unknown and provider does not support idempotency", nil),
				Reason: "cannot safely re-submit without idempotency support",
			}, nil
		}
		if caps.SupportsAsyncOperation && attempt.ProviderOperationID != "" {
			return &RecoveryDecision{
				Phase:         RecoveryPhaseSubmittedAsync,
				ShouldReQuery: true,
				Reason:        "query async operation to resolve unknown submission status",
			}, nil
		}
		return &RecoveryDecision{
			Phase:          RecoveryPhasePendingSubmit,
			ShouldReSubmit: true,
			Reason:         "re-submit with idempotency key after unknown submission",
		}, nil

	case AttemptStatusPolling:
		if attempt.ProviderOperationID != "" {
			return &RecoveryDecision{
				Phase:         RecoveryPhaseSubmittedAsync,
				ShouldReQuery: true,
				Reason:        "resume polling for async operation",
			}, nil
		}
		if caps.SupportsIdempotencyKey {
			return &RecoveryDecision{
				Phase:          RecoveryPhasePendingSubmit,
				ShouldReSubmit: true,
				Reason:         "re-submit: no operation ID to query but idempotency supported",
			}, nil
		}
		return &RecoveryDecision{
			Phase:  RecoveryPhaseSubmittedAsync,
			Error:  NewGenerationError(ErrCodeRecoveryIdempotentNotSafe, "no operation ID and provider does not support idempotency", nil),
			Reason: "cannot safely re-submit without operation ID or idempotency support",
		}, nil

	case AttemptStatusResultReceived, AttemptStatusPersisting:
		return &RecoveryDecision{
			Phase:           RecoveryPhaseResultReceived,
			ShouldRePersist: true,
			Reason:          "resume artifact persistence",
		}, nil

	default:
		return &RecoveryDecision{
			Phase:  RecoveryPhaseFailed,
			Error:  NewGenerationError(ErrCodeRecoveryStatusUnknown, fmt.Sprintf("unknown attempt status: %s", attempt.Status), nil),
			Reason: "cannot determine recovery action for unknown status",
		}, nil
	}
}

func IdentifyRecoveryPhase(attempt *ActionGenerationAttempt) RecoveryPhase {
	if attempt == nil {
		return RecoveryPhaseFailed
	}
	if attempt.Status == string(AttemptStatusSucceeded) {
		return RecoveryPhaseCompleted
	}
	if attempt.Status == string(AttemptStatusFailed) {
		return RecoveryPhaseFailed
	}

	status := AttemptStatus(attempt.Status)
	switch status {
	case AttemptStatusPending,
		AttemptStatusPreparingReference,
		AttemptStatusBuildingPrompt,
		AttemptStatusWaitingRateLimit:
		return RecoveryPhasePendingSubmit
	case AttemptStatusSubmitting,
		AttemptStatusUnknownSubmission,
		AttemptStatusPolling:
		return RecoveryPhaseSubmittedAsync
	case AttemptStatusResultReceived,
		AttemptStatusPersisting:
		return RecoveryPhaseResultReceived
	default:
		return RecoveryPhaseFailed
	}
}

func CanSafelyReSubmit(caps imageprovider.ProviderCapabilities) bool {
	return caps.SupportsIdempotencyKey
}

func ShouldStartNewAttempt(decision *RecoveryDecision) bool {
	if decision == nil {
		return false
	}
	if decision.ShouldSkip {
		return false
	}
	if decision.Error != nil {
		return false
	}
	return decision.ShouldResume || decision.ShouldReSubmit || decision.ShouldReQuery || decision.ShouldRePersist
}
