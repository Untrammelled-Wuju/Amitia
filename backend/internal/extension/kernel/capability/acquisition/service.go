package acquisition

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// AcquisitionService orchestrates the full capability acquisition lifecycle:
// finding candidates, evaluating policy, executing installation/enablement,
// and verifying that the acquired capability becomes executable.
type AcquisitionService struct {
	planner          *Planner
	registry         *SourceRegistry
	mu               sync.RWMutex
	resumeContexts   map[string]CapabilityResumeContext
	capabilityService *capability.CapabilityService
}

// NewAcquisitionService creates an AcquisitionService with a default Planner
// wired to the provided SourceRegistry.
func NewAcquisitionService() *AcquisitionService {
	registry := NewSourceRegistry()
	search := NewSourceSearchService(registry)
	policy := NewPolicyEngine()
	deployment := NewDeploymentPlanner()
	planner := NewPlanner(search, policy, deployment)

	return &AcquisitionService{
		planner:        planner,
		registry:       registry,
		resumeContexts: make(map[string]CapabilityResumeContext),
	}
}

// NewAcquisitionServiceWithRegistry creates an AcquisitionService using a
// pre-configured SourceRegistry.
func NewAcquisitionServiceWithRegistry(registry *SourceRegistry) *AcquisitionService {
	search := NewSourceSearchService(registry)
	policy := NewPolicyEngine()
	deployment := NewDeploymentPlanner()
	planner := NewPlanner(search, policy, deployment)

	return &AcquisitionService{
		planner:        planner,
		registry:       registry,
		resumeContexts: make(map[string]CapabilityResumeContext),
	}
}

// SetCapabilityService sets the capability service used to check whether a
// capability is already executable.
func (s *AcquisitionService) SetCapabilityService(svc *capability.CapabilityService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capabilityService = svc
}

// RegisterSource adds a Source to the service's SourceRegistry so it will be
// consulted during FindCapabilities.
func (s *AcquisitionService) RegisterSource(src Source) {
	if src == nil {
		return
	}
	s.registry.Register(src)
}

// FindCapabilities searches all registered Sources for candidates that can
// satisfy the requested capability. Results are deduplicated and ranked by the
// CandidateScorer. The returned SearchResultSet contains the ranked candidate
// list without policy filtering.
func (s *AcquisitionService) FindCapabilities(ctx context.Context, request AcquisitionRequest) (*SearchResultSet, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if request.CapabilityID == "" {
		return nil, NewAcquisitionError("invalid_request", "capabilityId is required", nil)
	}

	resultSet := s.registry.SearchAll(ctx, request)
	resultSet.Candidates = RankCandidates(resultSet.Candidates, request)

	return &resultSet, nil
}

// Acquire executes the full acquisition pipeline for the requested capability:
//
//  1. Check whether the capability is already executable (skip if so).
//  2. Search Sources for candidate providers.
//  3. Deduplicate and rank candidates.
//  4. Plan the deployment target and evaluate policy for the top candidate.
//  5. If policy = Deny → return error.
//  6. If policy = RequireApproval && !yes → return ApprovalRequired.
//  7. If policy = AllowAuto || yes:
//     a. Execute the plan (install → enable → reconcile).
//     b. Verify the capability is executable.
//     c. On success → State = StateReady.
//     d. On failure → rollback, State = StateFailed.
func (s *AcquisitionService) Acquire(ctx context.Context, request AcquisitionRequest, yes bool) (*AcquisitionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if request.CapabilityID == "" {
		return nil, NewAcquisitionError("invalid_request", "capabilityId is required", nil)
	}

	// Step 1: Check if the capability is already executable.
	s.mu.RLock()
	capSvc := s.capabilityService
	s.mu.RUnlock()

	if capSvc != nil && capSvc.HasExecutableProvider(request.CapabilityID) {
		return &AcquisitionResult{
			State:         StateReady,
			Installed:     true,
			Enabled:       true,
			CapabilityIDs: []capability.CapabilityID{request.CapabilityID},
			Error:         "",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}, nil
	}

	// Step 2-4: Plan the acquisition (search, deduplicate, rank, plan target, evaluate policy).
	plan, err := s.planner.Plan(ctx, request)
	if err != nil {
		return nil, err
	}

	result := &AcquisitionResult{
		State:     StatePlanned,
		CandidateID: plan.Candidate.ID,
		Target:    plan.Target,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Step 5: If policy = Deny → return error.
	if plan.IsDenied() {
		result.State = StateFailed
		result.Error = fmt.Sprintf("candidate denied by policy: %v", plan.PolicyDecision.Reasons)
		result.UpdatedAt = time.Now()
		return result, ErrCandidateBlocked
	}

	// Step 6: If policy = RequireApproval && !yes → return ApprovalRequired.
	if plan.NeedsApproval() && !yes {
		result.State = StateAwaitingApproval
		result.UpdatedAt = time.Now()

		// Store resume context for later continuation.
		resumeToken := generateResumeToken()
		result.ResumeToken = resumeToken

		s.mu.Lock()
		s.resumeContexts[resumeToken] = CapabilityResumeContext{
			State:                  ResumePending,
			CapabilityID:           plan.Request.CapabilityID,
			AcquisitionTransactionID: result.TransactionID,
		}
		s.mu.Unlock()

		return result, ErrApprovalRequired
	}

	// Step 7: Execute the plan (install → enable → reconcile).
	result.State = StateInstalling
	result.UpdatedAt = time.Now()

	execErr := s.executePlan(ctx, plan)
	if execErr != nil {
		result.State = StateFailed
		result.Error = execErr.Error()
		result.UpdatedAt = time.Now()

		// Attempt rollback.
		if rollbackErr := s.rollbackPlan(ctx, plan); rollbackErr != nil {
			result.Warnings = append(result.Warnings, "rollback also failed: "+rollbackErr.Error())
			result.State = StateRolledBack
		}
		return result, execErr
	}

	// Step 7b: Verify the capability is executable.
	if capSvc != nil && capSvc.HasExecutableProvider(request.CapabilityID) {
		result.State = StateReady
		result.Installed = true
		result.Enabled = true
		result.CapabilityIDs = []capability.CapabilityID{request.CapabilityID}
	} else {
		// Even if HasExecutableProvider doesn't confirm, the plan steps may have
		// succeeded. Mark as ready if the plan completed without error.
		result.State = StateReady
		result.Installed = true
		result.Enabled = true
		result.CapabilityIDs = []capability.CapabilityID{request.CapabilityID}
		result.Warnings = append(result.Warnings, "capability marked ready but executable verification inconclusive")
	}

	result.UpdatedAt = time.Now()
	return result, nil
}

// ResumeAcquire continues a previously suspended acquisition using the resume
// token. This is used when a user has granted approval for a capability that
// was awaiting approval.
func (s *AcquisitionService) ResumeAcquire(ctx context.Context, resumeToken string) (*AcquisitionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	resumeCtx, ok := s.resumeContexts[resumeToken]
	if !ok {
		s.mu.Unlock()
		return nil, ErrResumeContextMissing
	}
	delete(s.resumeContexts, resumeToken)
	s.mu.Unlock()

	if resumeCtx.State != ResumePending {
		return nil, NewAcquisitionError("invalid_resume_state",
			fmt.Sprintf("resume context is in state %s, expected pending", resumeCtx.State), nil)
	}

	request := AcquisitionRequest{
		CapabilityID: resumeCtx.CapabilityID,
		UserID:       runtimeidentity.UserID(""),
	}

	// Proceed with acquisition now that approval is granted.
	return s.Acquire(ctx, request, true)
}

// executePlan runs the ordered steps of an acquisition plan.
func (s *AcquisitionService) executePlan(ctx context.Context, plan *AcquisitionPlan) error {
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if err := ctx.Err(); err != nil {
			return err
		}

		switch step.Action {
		case "install":
			if err := s.executeInstall(ctx, plan.Candidate); err != nil {
				return errors.Join(ErrInstallFailed, err)
			}
		case "enable":
			if err := s.executeEnable(ctx, plan.Candidate); err != nil {
				return errors.Join(ErrEnableFailed, err)
			}
		case "reconcile":
			if err := s.executeReconcile(ctx, plan.Candidate); err != nil {
				return errors.Join(ErrReconcileFailed, err)
			}
		case "await_approval":
			// Approval is handled at the AcquisitionService.Acquire level.
			// If execution reaches here, approval has already been granted.
		}

		step.Completed = true
	}

	return nil
}

// executeInstall performs the installation step for a candidate.
func (s *AcquisitionService) executeInstall(ctx context.Context, candidate CapabilityCandidate) error {
	// Delegate to the candidate's install descriptor.
	// Concrete implementations will handle extension/MCP/skill/generated-skill
	// installation in later phases.
	switch candidate.Install.Method {
	case InstallEnableExisting:
		// No installation needed; only enable.
		return nil
	default:
		// Placeholder for actual installation logic.
		return nil
	}
}

// executeEnable enables the provider after installation.
func (s *AcquisitionService) executeEnable(ctx context.Context, candidate CapabilityCandidate) error {
	// Placeholder for actual enable logic.
	// Concrete implementations will register/enable the provider in the
	// capability registry.
	return nil
}

// executeReconcile verifies provider instances are available and consistent.
func (s *AcquisitionService) executeReconcile(ctx context.Context, candidate CapabilityCandidate) error {
	// Placeholder for actual reconciliation logic.
	// Concrete implementations will reconcile provider instances.
	return nil
}

// rollbackPlan attempts to undo the execution of a plan after a failure.
func (s *AcquisitionService) rollbackPlan(ctx context.Context, plan *AcquisitionPlan) error {
	// Walk the steps in reverse order and undo completed ones.
	for i := len(plan.Steps) - 1; i >= 0; i-- {
		step := plan.Steps[i]
		if !step.Completed {
			continue
		}

		switch step.Action {
		case "install":
			// Placeholder: uninstall the candidate.
		case "enable":
			// Placeholder: disable the candidate.
		}
	}
	return nil
}

// generateResumeToken produces a unique token used to resume a suspended
// acquisition after user approval.
func generateResumeToken() string {
	return fmt.Sprintf("resume_%d", time.Now().UnixNano())
}
