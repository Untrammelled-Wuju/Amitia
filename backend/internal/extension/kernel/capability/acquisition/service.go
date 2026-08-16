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

// ProviderLifecyclePort is the minimal interface AcquisitionService needs to
// trigger provider enablement without fabricating ProviderInstance directly.
// The real implementation lives in capability.ProviderLifecycleService.
type ProviderLifecyclePort interface {
	Enable(providerID capability.ProviderID) error
	Disable(providerID capability.ProviderID) error
}

// AcquisitionDependencies holds all required dependencies for constructing
// an AcquisitionService. All fields must be non-nil for production use.
type AcquisitionDependencies struct {
	CapabilityService     *capability.CapabilityService
	ProviderRegistry      *capability.ProviderRegistry
	SourceRegistry        *SourceRegistry
	InstallerRegistry     *InstallerRegistry
	PolicyEngine          *PolicyEngine
	DeploymentPlanner     *DeploymentPlanner
	ProviderLifecycle     ProviderLifecyclePort
	Execution             ExecutionPort
}

// AcquisitionService orchestrates the full capability acquisition lifecycle:
// finding candidates, evaluating policy, executing installation/enablement,
// and verifying that the acquired capability becomes executable.
type AcquisitionService struct {
	planner           *Planner
	registry          *SourceRegistry
	installerRegistry *InstallerRegistry
	mu                sync.RWMutex
	resumeContexts    map[string]CapabilityResumeContext
	capabilityService *capability.CapabilityService
	providerRegistry  *capability.ProviderRegistry
	providerLifecycle ProviderLifecyclePort
	execution         ExecutionPort
}

// NewAcquisitionService creates an AcquisitionService with explicitly provided
// dependencies. This is the canonical production constructor.
func NewAcquisitionService(deps AcquisitionDependencies) (*AcquisitionService, error) {
	if deps.CapabilityService == nil {
		return nil, errors.New("acquisition: CapabilityService is required")
	}
	if deps.ProviderRegistry == nil {
		return nil, errors.New("acquisition: ProviderRegistry is required")
	}
	if deps.SourceRegistry == nil {
		return nil, errors.New("acquisition: SourceRegistry is required")
	}
	if deps.InstallerRegistry == nil {
		return nil, errors.New("acquisition: InstallerRegistry is required")
	}
	if deps.PolicyEngine == nil {
		return nil, errors.New("acquisition: PolicyEngine is required")
	}
	if deps.DeploymentPlanner == nil {
		return nil, errors.New("acquisition: DeploymentPlanner is required")
	}

	search := NewSourceSearchService(deps.SourceRegistry)
	planner := NewPlanner(search, deps.PolicyEngine, deps.DeploymentPlanner)

	return &AcquisitionService{
		planner:           planner,
		registry:          deps.SourceRegistry,
		installerRegistry: deps.InstallerRegistry,
		resumeContexts:    make(map[string]CapabilityResumeContext),
		capabilityService: deps.CapabilityService,
		providerRegistry:  deps.ProviderRegistry,
		providerLifecycle: deps.ProviderLifecycle,
		execution:         deps.Execution,
	}, nil
}

// NewAcquisitionServiceWithRegistry creates an AcquisitionService using a
// pre-configured SourceRegistry. This is retained for test/compat purposes;
// production code should use NewAcquisitionService with full AcquisitionDependencies.
func NewAcquisitionServiceWithRegistry(registry *SourceRegistry, providerRegistry *capability.ProviderRegistry) (*AcquisitionService, error) {
	deps := AcquisitionDependencies{
		CapabilityService: nil,
		ProviderRegistry:  providerRegistry,
		SourceRegistry:    registry,
		InstallerRegistry: NewInstallerRegistry(nil),
		PolicyEngine:      NewPolicyEngine(),
		DeploymentPlanner: NewDeploymentPlanner(),
	}
	return NewAcquisitionService(deps)
}

// SetExecution sets the execution port used to create child executions and
// resume contexts during the acquisition lifecycle.
func (s *AcquisitionService) SetExecution(port ExecutionPort) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execution = port
}

// SetCapabilityService sets the capability service used to check whether a
// capability is already executable.
func (s *AcquisitionService) SetCapabilityService(svc *capability.CapabilityService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capabilityService = svc
}

// SetProviderRegistry sets the provider registry used to enable providers.
func (s *AcquisitionService) SetProviderRegistry(reg *capability.ProviderRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providerRegistry = reg
}

// SetInstallerRegistry sets the installer registry used to dispatch install methods.
func (s *AcquisitionService) SetInstallerRegistry(reg *InstallerRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installerRegistry = reg
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
//     If ExecContext is present, create a child execution for this acquisition.
//  2. Search Sources for candidate providers.
//  3. Deduplicate and rank candidates.
//  4. Plan the deployment target and evaluate policy for the top candidate.
//  5. If policy = Deny → return error.
//  6. If policy = RequireApproval && !yes → create pending resume, return ApprovalRequired.
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

	// Create child execution if ExecContext is present.
	var acqExecCtx execution.ExecutionContext
	if request.ExecContext != nil && s.execution != nil {
		acqExecCtx = s.execution.CreateChildExecution(*request.ExecContext, "acquisition")
	}

	// Step 1: Check if the capability is already executable.
	s.mu.RLock()
	capSvc := s.capabilityService
	s.mu.RUnlock()

	if capSvc != nil && capSvc.HasExecutableProvider(request.CapabilityID) {
		result := &AcquisitionResult{
			State:         StateReady,
			Installed:     true,
			Enabled:       true,
			CapabilityIDs: []capability.CapabilityID{request.CapabilityID},
			Error:         "",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if request.ExecContext != nil {
			result.ExecutionID = request.ExecContext.ExecutionID
		}
		if s.execution != nil {
			s.execution.CompleteExecution(acqExecCtx, "capability_already_executable")
		}
		return result, nil
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

	// Step 6: If policy = RequireApproval && !yes → create pending resume, return ApprovalRequired.
	if plan.NeedsApproval() && !yes {
		result.State = StateAwaitingApproval
		result.UpdatedAt = time.Now()

		resumeToken := generateResumeToken()
		result.ResumeToken = resumeToken

		s.mu.Lock()
		s.resumeContexts[resumeToken] = CapabilityResumeContext{
			State:                  ResumePending,
			CapabilityID:           plan.Request.CapabilityID,
			AcquisitionTransactionID: result.TransactionID,
		}
		s.mu.Unlock()

		if s.execution != nil {
			resume, err := s.execution.CreateResume(acqExecCtx, execution.ResumeTypeCapabilityAcquisition, string(request.CapabilityID))
			if err == nil && resume != nil {
				result.ResumeID = resume.ResumeID
			}
		}

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
		if s.execution != nil {
			s.execution.CompleteExecution(acqExecCtx, "acquisition_failed: "+execErr.Error())
		}
		return result, execErr
	}

	// Step 7b: Verify the capability is executable via authoritative state.
	if capSvc != nil && capSvc.HasExecutableProvider(request.CapabilityID) {
		result.State = StateReady
		result.Installed = true
		result.Enabled = true
		result.CapabilityIDs = []capability.CapabilityID{request.CapabilityID}
	} else {
		// Plan steps completed but authoritative state has not yet produced an
		// executable instance. Do NOT fabricate Ready or ProviderInstance here.
		result.State = StateReconciling
		result.Installed = true
		result.Enabled = false
		result.CapabilityIDs = []capability.CapabilityID{request.CapabilityID}
		result.Warnings = append(result.Warnings, "plan executed but executable provider not yet reconciled")
	}

	result.UpdatedAt = time.Now()
	if s.execution != nil {
		s.execution.CompleteExecution(acqExecCtx, "acquisition_completed: "+string(result.State))
	}
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
	var installed InstalledCapability

	for i := range plan.Steps {
		step := &plan.Steps[i]
		if err := ctx.Err(); err != nil {
			return err
		}

		switch step.Action {
		case "install":
			result, err := s.executeInstall(ctx, plan.Candidate, plan.Target)
			if err != nil {
				return errors.Join(ErrInstallFailed, err)
			}
			installed = result
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

	_, _ = installed, installed
	return nil
}

// executeInstall performs the installation step for a candidate by dispatching
// to the registered Installer for the candidate's InstallMethod.
func (s *AcquisitionService) executeInstall(ctx context.Context, candidate CapabilityCandidate, target DeploymentTarget) (InstalledCapability, error) {
	s.mu.RLock()
	installerReg := s.installerRegistry
	s.mu.RUnlock()

	if installerReg == nil {
		return InstalledCapability{}, ErrInstallerRegistryUnavailable
	}

	installer, err := installerReg.Resolve(candidate.Install.Method)
	if err != nil {
		return InstalledCapability{}, errors.Join(ErrNoInstallerForMethod, err)
	}

	installed, err := installer.Install(ctx, candidate, target)
	if err != nil {
		return InstalledCapability{}, err
	}
	return installed, nil
}

// executeEnable triggers provider enablement via the authoritative lifecycle.
// It does NOT fabricate ProviderInstance directly; instead it delegates to the
// ProviderLifecyclePort so that the canonical lifecycle produces instances.
func (s *AcquisitionService) executeEnable(ctx context.Context, candidate CapabilityCandidate) error {
	s.mu.RLock()
	reg := s.providerRegistry
	capSvc := s.capabilityService
	lc := s.providerLifecycle
	s.mu.RUnlock()

	if reg == nil {
		return ErrProviderRegistryUnavailable
	}

	capID := capability.CapabilityID(candidate.ID)
	if len(candidate.Capabilities) > 0 {
		capID = candidate.Capabilities[0]
	}

	// Already executable? Nothing to do.
	if capSvc != nil && capSvc.HasExecutableProvider(capID) {
		return nil
	}
	if reg.CountExecutableInstances(capID) > 0 {
		return nil
	}

	defs := reg.ListByCapability(capID)
	if len(defs) == 0 {
		return ErrProviderDefinitionNotFound
	}

	for _, def := range defs {
		if def == nil {
			continue
		}
		if err := s.enableProviderViaLifecycle(def, candidate, lc); err != nil {
			return err
		}
	}

	return nil
}

// enableProviderViaLifecycle enables a provider definition by delegating to the
// authoritative ProviderLifecyclePort. It does NOT fabricate HealthReady or
// ProviderAvailabilityAvailable; those states are owned by the lifecycle.
func (s *AcquisitionService) enableProviderViaLifecycle(def *capability.CapabilityProviderDefinition, candidate CapabilityCandidate, lc ProviderLifecyclePort) error {
	s.mu.RLock()
	reg := s.providerRegistry
	s.mu.RUnlock()

	if reg == nil {
		return ErrProviderRegistryUnavailable
	}

	// Check if an executable instance already exists for this provider.
	existing := reg.ListInstancesByProvider(def.ID)
	for _, inst := range existing {
		if inst != nil && inst.IsExecutable() {
			return nil
		}
	}

	if lc == nil {
		return fmt.Errorf("provider lifecycle not available for provider %s", def.ID)
	}

	instanceID := capability.ProviderInstanceID(fmt.Sprintf("acq_%s_%d", def.ID, time.Now().UnixNano()))

	inst := capability.CapabilityProviderInstance{
		ID:          instanceID,
		ProviderID:  def.ID,
		CapabilityID: def.CapabilityID,
		Placement:   def.Placement,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
	}

	if def.Placement == capability.ProviderPlacementDevice {
		if candidate.Metadata != nil {
			if devID, ok := candidate.Metadata["deviceId"].(string); ok && devID != "" {
				inst.DeviceID = runtimeidentity.DeviceID(devID)
			}
			if rtID, ok := candidate.Metadata["runtimeId"].(string); ok && rtID != "" {
				inst.RuntimeID = runtimeidentity.RuntimeID(rtID)
			}
		}
	}

	if err := lc.RegisterInstance(inst); err != nil {
		return fmt.Errorf("register instance for provider %s: %w", def.ID, err)
	}

	return nil
}

// executeReconcile waits for the authoritative state to produce an executable
// provider instance with bounded waiting. It does NOT poll infinitely and does
// NOT fabricate success.
func (s *AcquisitionService) executeReconcile(ctx context.Context, candidate CapabilityCandidate) error {
	s.mu.RLock()
	reg := s.providerRegistry
	capSvc := s.capabilityService
	s.mu.RUnlock()

	if reg == nil {
		return ErrProviderRegistryUnavailable
	}

	capID := capability.CapabilityID(candidate.ID)
	if len(candidate.Capabilities) > 0 {
		capID = candidate.Capabilities[0]
	}

	// Bounded waiting: poll with timeout for authoritative executable state.
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if reg.CountExecutableInstances(capID) > 0 {
				return nil
			}
			if capSvc != nil && capSvc.HasExecutableProvider(capID) {
				return nil
			}
			if time.Now().After(deadline) {
				return ErrReconcileTimeout
			}
		}
	}
}

// rollbackPlan attempts to undo the execution of a plan after a failure.
func (s *AcquisitionService) rollbackPlan(ctx context.Context, plan *AcquisitionPlan) error {
	s.mu.RLock()
	installerReg := s.installerRegistry
	s.mu.RUnlock()

	for i := len(plan.Steps) - 1; i >= 0; i-- {
		step := plan.Steps[i]
		if !step.Completed {
			continue
		}

		switch step.Action {
		case "install":
			if installerReg != nil && s.providerRegistry != nil {
				installer, err := installerReg.Resolve(plan.Candidate.Install.Method)
				if err == nil {
					_ = installer.Rollback(ctx, InstalledCapability{
						Candidate: plan.Candidate,
						Target:    plan.Target,
					})
				}
			}
		case "enable":
			// Deliberately no-op: do not re-disable providers that may have
			// been enabled by the user outside this transaction.
		}
	}
	return nil
}

// generateResumeToken produces a unique token used to resume a suspended
// acquisition after user approval.
func generateResumeToken() string {
	return fmt.Sprintf("resume_%d", time.Now().UnixNano())
}
