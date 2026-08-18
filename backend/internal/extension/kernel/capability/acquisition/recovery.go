package acquisition

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// RecoveryService orchestrates automatic recovery from missing-capability
// conditions. It uses a budget to prevent infinite recovery loops and delegates
// the actual acquisition to an AcquisitionService.
type RecoveryService struct {
	acquisitionService *AcquisitionService
	detector           MissingCapabilityDetector
	budgetMu           sync.Mutex
	budgets            map[string]*CapabilityRecoveryBudget
	resumeRepo         execution.ResumeRepository
}

// NewRecoveryService creates a RecoveryService backed by the given
// AcquisitionService. It uses the default MissingCapabilityDetector and a fresh
// recovery budget.
func NewRecoveryService(acquisitionService *AcquisitionService) *RecoveryService {
	return &RecoveryService{
		acquisitionService: acquisitionService,
		detector:           NewMissingCapabilityDetector(),
		budgets:            make(map[string]*CapabilityRecoveryBudget),
	}
}

// NewRecoveryServiceWithRepository creates a RecoveryService with a resume repository for persistence.
func NewRecoveryServiceWithRepository(acquisitionService *AcquisitionService, resumeRepo execution.ResumeRepository) *RecoveryService {
	return &RecoveryService{
		acquisitionService: acquisitionService,
		detector:           NewMissingCapabilityDetector(),
		budgets:            make(map[string]*CapabilityRecoveryBudget),
		resumeRepo:         resumeRepo,
	}
}

// newCapabilityRecoveryBudgetPtr returns a pointer to a default
// CapabilityRecoveryBudget.
func newCapabilityRecoveryBudgetPtr() *CapabilityRecoveryBudget {
	b := NewCapabilityRecoveryBudget()
	return &b
}

// Recover attempts to automatically recover from a missing-capability condition
// described by the supplied CapabilityResumeContext.
//
// The recovery procedure:
//  1. Verify the recovery budget has not been exhausted.
//  2. Reconstruct an AcquisitionRequest from the resume context.
//  3. If AcquisitionTransactionID is empty OR the state indicates a previous
//     rollback → start a fresh acquisition by calling Acquire.
//  4. Return the acquisition result.
func (s *RecoveryService) Recover(
	ctx context.Context,
	resumeCtx CapabilityResumeContext,
) (*AcquisitionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if resumeCtx.CapabilityID == "" {
		return nil, errors.New("recovery: capabilityId is required in resume context")
	}

	acqID := string(resumeCtx.CapabilityID)

	// Step 1: Check the budget scoped to the current RootExecution. A recovery
	// attempt in one user task must never consume another task's budget.
	budget := s.budgetFor(resumeCtx)
	if !s.canAttempt(budget, acqID) {
		return nil, fmt.Errorf("recovery: budget exhausted for capability %s", acqID)
	}

	// Step 2: Reconstruct the AcquisitionRequest from the resume context.
	request := s.reconstructRequest(resumeCtx)

	// Step 3: Determine whether to start fresh or resume.
	needsFresh := resumeCtx.AcquisitionTransactionID == "" ||
		resumeCtx.State == ResumePending

	var result *AcquisitionResult
	var err error

	if needsFresh {
		s.recordAttempt(budget, acqID)
		result, err = s.acquisitionService.Acquire(ctx, request, resumeCtx.Approved)
	} else {
		// An existing acquisition transaction is available and not in a rolled-back
		// state. Attempt to resume it via the stored transaction ID. If the
		// resume token is not tracked in the AcquisitionService, fall back to
		// a fresh acquire.
		s.recordAttempt(budget, acqID)
		result, err = s.attemptResume(ctx, resumeCtx, request)
	}

	if err != nil {
		return result, err
	}

	return result, nil
}

// RecoverFromError is a convenience entry point that detects a missing-capability
// condition from an execution error and, when detected, attempts automatic
// recovery in a single step.
func (s *RecoveryService) RecoverFromError(
	ctx context.Context,
	err error,
	invocation capability.ToolInvocationContext,
) (*AcquisitionResult, error) {
	resumeCtx, detectErr := s.detector.DetectFromError(ctx, err, invocation)
	if detectErr != nil {
		return nil, detectErr
	}
	if resumeCtx == nil {
		// Error is not a missing-capability condition; nothing to recover.
		return nil, nil
	}
	return s.Recover(ctx, *resumeCtx)
}

// RecoverFromResolution is a convenience entry point that detects a
// missing-capability condition from a resolution failure and, when detected,
// attempts automatic recovery in a single step.
func (s *RecoveryService) RecoverFromResolution(
	ctx context.Context,
	failure capability.ResolutionFailure,
	invocation capability.ToolInvocationContext,
) (*AcquisitionResult, error) {
	resumeCtx, detectErr := s.detector.DetectFromResolution(ctx, failure, invocation)
	if detectErr != nil {
		return nil, detectErr
	}
	if resumeCtx == nil {
		// Resolution did not fail or failure is not actionable.
		return nil, nil
	}
	return s.Recover(ctx, *resumeCtx)
}

// reconstructRequest rebuilds an AcquisitionRequest from a CapabilityResumeContext.
// Fields that are not persisted in the resume context (e.g. UserID) are left at
// their zero value; the AcquisitionService handles that gracefully.
func (s *RecoveryService) reconstructRequest(resumeCtx CapabilityResumeContext) AcquisitionRequest {
	return AcquisitionRequest{
		CapabilityID:       resumeCtx.CapabilityID,
		UserID:             runtimeidentity.UserID(resumeCtx.UserID),
		ExecContext:        resumeCtx.ExecContext,
		AutoInstallAllowed: true,
	}
}

// attemptResume tries to reuse an existing acquisition. If the resume token is
// not known to the underlying AcquisitionService (i.e. the transaction ID does
// not map to a tracked resume context), it starts a new Acquire call to ensure
// progress.
func (s *RecoveryService) attemptResume(
	ctx context.Context,
	resumeCtx CapabilityResumeContext,
	request AcquisitionRequest,
) (*AcquisitionResult, error) {
	// The AcquisitionService.ResumeAcquire uses resume tokens stored in its
	// internal map. When the resume context's AcquisitionTransactionID is not a
	// recognized resume token, ResumeAcquire returns ErrResumeContextMissing.
	// In that case we fall back to a fresh Acquire to make progress.
	result, err := s.acquisitionService.ResumeAcquire(ctx, resumeCtx.AcquisitionTransactionID)
	if err != nil {
		if errors.Is(err, ErrResumeContextMissing) {
			return s.acquisitionService.Acquire(ctx, request, resumeCtx.Approved)
		}
		return result, err
	}
	return result, nil
}

// Budget returns a read-only copy of the current recovery budget for observability.
func (s *RecoveryService) Budget() CapabilityRecoveryBudget {
	s.budgetMu.Lock()
	defer s.budgetMu.Unlock()
	aggregate := NewCapabilityRecoveryBudget()
	aggregate.CurrentAcquisitions = 0
	for _, budget := range s.budgets {
		aggregate.CurrentAcquisitions += budget.CurrentAcquisitions
		for capabilityID, attempts := range budget.AttemptedCapabilities {
			aggregate.AttemptedCapabilities[capabilityID] += attempts
		}
	}
	return aggregate
}

// ResetBudget resets the recovery budget. Useful when a new task begins.
func (s *RecoveryService) ResetBudget() {
	s.budgetMu.Lock()
	s.budgets = make(map[string]*CapabilityRecoveryBudget)
	s.budgetMu.Unlock()
}

func (s *RecoveryService) budgetFor(resumeCtx CapabilityResumeContext) *CapabilityRecoveryBudget {
	key := ""
	if resumeCtx.ExecContext != nil {
		key = resumeCtx.ExecContext.RootExecutionID
		if key == "" {
			key = resumeCtx.ExecContext.ExecutionID
		}
	}
	if key == "" {
		key = resumeCtx.ConversationID
	}
	if key == "" {
		key = "unscoped:" + resumeCtx.UserID
	}
	s.budgetMu.Lock()
	defer s.budgetMu.Unlock()
	budget := s.budgets[key]
	if budget == nil {
		budget = newCapabilityRecoveryBudgetPtr()
		s.budgets[key] = budget
	}
	return budget
}

func (s *RecoveryService) canAttempt(budget *CapabilityRecoveryBudget, capabilityID string) bool {
	s.budgetMu.Lock()
	defer s.budgetMu.Unlock()
	return budget.CanAttempt(capabilityID)
}

func (s *RecoveryService) recordAttempt(budget *CapabilityRecoveryBudget, capabilityID string) {
	s.budgetMu.Lock()
	budget.RecordAttempt(capabilityID)
	s.budgetMu.Unlock()
}

// _ is a compile-time guard ensuring RecoveryService methods return the expected types.
var _ = (*AcquisitionResult)(nil)
var _ = time.Now
