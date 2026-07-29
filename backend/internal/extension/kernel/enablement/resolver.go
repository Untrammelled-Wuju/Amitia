package enablement

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type InMemoryStateStore struct {
	mu     sync.RWMutex
	states map[string]SubjectState
}

func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{states: make(map[string]SubjectState)}
}

func subjectKey(s StateSubject) string {
	return string(s.Kind) + "/" + s.ID
}

func (s *InMemoryStateStore) Get(_ context.Context, subject StateSubject) (SubjectState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[subjectKey(subject)]
	if !ok {
		return SubjectState{}, ErrSubjectNotFound
	}
	return state, nil
}

func (s *InMemoryStateStore) SetEnablement(ctx context.Context, subject StateSubject, state EnablementState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := subjectKey(subject)
	cur := s.states[key]
	cur.Subject = subject
	cur.Enablement = state
	cur.UpdatedAt = time.Now().UTC()
	s.states[key] = cur
	return nil
}

func (s *InMemoryStateStore) SetDesiredRuntime(ctx context.Context, subject StateSubject, state DesiredRuntimeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := subjectKey(subject)
	cur := s.states[key]
	cur.Subject = subject
	cur.DesiredRuntime = state
	cur.UpdatedAt = time.Now().UTC()
	s.states[key] = cur
	return nil
}

func (s *InMemoryStateStore) SetState(ctx context.Context, state SubjectState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := subjectKey(state.Subject)
	state.UpdatedAt = time.Now().UTC()
	s.states[key] = state
	return nil
}

func (s *InMemoryStateStore) List(_ context.Context, kind StateSubjectKind) ([]SubjectState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SubjectState
	for _, st := range s.states {
		if st.Subject.Kind == kind {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *InMemoryStateStore) Delete(ctx context.Context, subject StateSubject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, subjectKey(subject))
	return nil
}

var _ StateStore = (*InMemoryStateStore)(nil)

type DefaultResolver struct {
	store StateStore
}

func NewDefaultResolver(store StateStore) *DefaultResolver {
	return &DefaultResolver{store: store}
}

func (r *DefaultResolver) Resolve(ctx context.Context, subject StateSubject, runtimeCtx StateRuntimeContext) EffectiveState {
	now := time.Now().UTC()
	if runtimeCtx.Now.IsZero() {
		runtimeCtx.Now = now
	}
	state, err := r.store.Get(ctx, subject)
	eff := EffectiveState{
		Subject:      subject,
		EvaluatedAt:  now,
		Availability: AvailabilityUnavailable,
		Exposure:     ExposureHidden,
		Execution:    ExecutionInexecutable,
	}
	if err != nil {
		eff.Reasons = append(eff.Reasons, reason("not_installed", "installation", "subject not registered", "blocking"))
		return eff
	}

	if !r.checkInstallation(&eff, state) {
		return eff
	}
	if !r.checkDefinition(&eff, state) {
		return eff
	}
	if !r.checkExtensionEnablement(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkModuleEnablement(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkContributionOverride(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkScope(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkPermission(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkDependency(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkDesiredRuntime(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkActualRuntime(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkHealth(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkCircuit(&eff, state, runtimeCtx) {
		return eff
	}
	if !r.checkExposure(&eff, state, runtimeCtx) {
		return eff
	}
	eff.Availability = AvailabilityAvailable
	eff.Exposure = ExposureVisible
	eff.Execution = ExecutionExecutable
	eff.Visible = true
	eff.Executable = true
	return eff
}

func (r *DefaultResolver) checkInstallation(eff *EffectiveState, state SubjectState) bool {
	switch state.Installation {
	case InstallationStateInstalled:
		eff.Installed = true
		return true
	case InstallationStateNotInstalled, "":
		eff.Reasons = append(eff.Reasons, reason("not_installed", "installation", "not installed", "blocking"))
		return false
	case InstallationStateFailed:
		eff.Reasons = append(eff.Reasons, reason("installation_failed", "installation", "installation failed", "blocking"))
		return false
	case InstallationStateInstalling, InstallationStateUpdating, InstallationStateRollingBack, InstallationStateUninstalling:
		eff.Reasons = append(eff.Reasons, reason("installation_in_progress", "installation", "installation in progress", "blocking"))
		return false
	default:
		eff.Installed = true
		return true
	}
}

func (r *DefaultResolver) checkDefinition(eff *EffectiveState, state SubjectState) bool {
	switch state.Definition {
	case DefinitionStateValid, "":
		eff.DefinitionValid = true
		return true
	case DefinitionStateInvalid:
		eff.Reasons = append(eff.Reasons, reason("definition_invalid", "definition", "definition invalid", "blocking"))
	case DefinitionStateIncompatible:
		eff.Reasons = append(eff.Reasons, reason("incompatible", "definition", "incompatible", "blocking"))
	case DefinitionStateMigrationRequired:
		eff.Reasons = append(eff.Reasons, reason("migration_required", "definition", "migration required", "blocking"))
	case DefinitionStateMissingDependency:
		eff.Reasons = append(eff.Reasons, reason("dependency_missing", "definition", "missing dependency", "blocking"))
	case DefinitionStateCorrupted:
		eff.Reasons = append(eff.Reasons, reason("definition_corrupted", "definition", "corrupted", "blocking"))
	}
	return false
}

func (r *DefaultResolver) checkExtensionEnablement(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	if state.Enablement == EnablementDisabled {
		eff.Reasons = append(eff.Reasons, reason(subjectDisableReason(state.Subject.Kind), "enablement", "explicitly disabled", "blocking"))
		return false
	}
	eff.Enabled = true
	return true
}

func (r *DefaultResolver) checkModuleEnablement(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	if state.ParentEnabled != nil && !*state.ParentEnabled {
		eff.Reasons = append(eff.Reasons, reason("module_disabled", "enablement", "parent module disabled", "blocking"))
		return false
	}
	return true
}

func (r *DefaultResolver) checkContributionOverride(eff *EffectiveState, _ SubjectState, _ StateRuntimeContext) bool {
	return true
}

func (r *DefaultResolver) checkScope(eff *EffectiveState, state SubjectState, runtimeCtx StateRuntimeContext) bool {
	switch state.Scope {
	case ScopeStateActive, "":
		eff.ScopeAllowed = true
		return true
	case ScopeStateInactive:
		eff.Reasons = append(eff.Reasons, reason("scope_denied", "scope", "scope inactive", "blocking"))
		return false
	case ScopeStateExpired:
		eff.Reasons = append(eff.Reasons, reason("scope_denied", "scope", "scope expired", "blocking"))
		return false
	case ScopeStateRevoked:
		eff.Reasons = append(eff.Reasons, reason("scope_denied", "scope", "scope revoked", "blocking"))
		return false
	default:
		eff.ScopeAllowed = true
		return true
	}
}

func (r *DefaultResolver) checkPermission(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	switch state.Permission {
	case PermissionAllowed, "":
		eff.PermissionAllowed = true
		return true
	case PermissionDenied:
		eff.Reasons = append(eff.Reasons, reason("permission_denied", "permission", "permission denied", "blocking"))
		return false
	case PermissionExpired:
		eff.Reasons = append(eff.Reasons, reason("permission_denied", "permission", "permission expired", "blocking"))
		return false
	case PermissionRevoked:
		eff.Reasons = append(eff.Reasons, reason("permission_denied", "permission", "permission revoked", "blocking"))
		return false
	case PermissionApproval:
		eff.Reasons = append(eff.Reasons, reason("approval_required", "permission", "approval required", "blocking"))
		return false
	default:
		eff.PermissionAllowed = true
		return true
	}
}

func (r *DefaultResolver) checkDependency(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	if !state.DependencyReady {
		eff.Reasons = append(eff.Reasons, reason("dependency_missing", "dependency", "dependency not ready", "blocking"))
		return false
	}
	eff.DependencyReady = true
	return true
}

func (r *DefaultResolver) checkDesiredRuntime(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	switch state.DesiredRuntime {
	case DesiredRuntimeStarted, "":
		eff.DesiredReady = true
		return true
	case DesiredRuntimeStopped:
		eff.Reasons = append(eff.Reasons, reason("runtime_stopped", "desired_runtime", "desired stopped", "blocking"))
		return false
	case DesiredRuntimePaused:
		eff.Reasons = append(eff.Reasons, reason("runtime_stopped", "desired_runtime", "desired paused", "blocking"))
		return false
	case DesiredRuntimeQuarantine:
		eff.Reasons = append(eff.Reasons, reason("runtime_quarantined", "desired_runtime", "desired quarantine", "blocking"))
		return false
	default:
		eff.DesiredReady = true
		return true
	}
}

func (r *DefaultResolver) checkActualRuntime(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	switch state.ActualRuntime {
	case ActualRuntimeReady, ActualRuntimeRunning:
		eff.RuntimeReady = true
		return true
	case ActualRuntimeStarting, ActualRuntimeDegraded:
		eff.RuntimeReady = false
		eff.Reasons = append(eff.Reasons, reason("runtime_not_ready", "actual_runtime", "runtime not ready", "blocking"))
		return false
	case ActualRuntimeStopped, "":
		eff.Reasons = append(eff.Reasons, reason("runtime_stopped", "actual_runtime", "runtime stopped", "blocking"))
		return false
	case ActualRuntimeCrashed:
		eff.Reasons = append(eff.Reasons, reason("runtime_crashed", "actual_runtime", "runtime crashed", "blocking"))
		return false
	case ActualRuntimeQuarantined:
		eff.Reasons = append(eff.Reasons, reason("runtime_quarantined", "actual_runtime", "runtime quarantined", "blocking"))
		return false
	default:
		eff.RuntimeReady = true
		return true
	}
}

func (r *DefaultResolver) checkHealth(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	switch state.Health {
	case HealthHealthy, "":
		eff.Healthy = true
		return true
	case HealthDegraded:
		eff.Healthy = false
		eff.Reasons = append(eff.Reasons, reason("health_degraded", "health", "degraded", "warning"))
		return true
	case HealthUnhealthy:
		eff.Reasons = append(eff.Reasons, reason("health_unhealthy", "health", "unhealthy", "blocking"))
		return false
	case HealthUnknown:
		eff.Healthy = false
		eff.Reasons = append(eff.Reasons, reason("health_unknown", "health", "unknown", "warning"))
		return true
	default:
		eff.Healthy = true
		return true
	}
}

func (r *DefaultResolver) checkCircuit(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	switch state.Circuit {
	case CircuitClosed, "":
		eff.CircuitAllows = true
		return true
	case CircuitOpen:
		eff.Reasons = append(eff.Reasons, reason("circuit_open", "circuit", "circuit open", "blocking"))
		return false
	case CircuitHalfOpen:
		eff.CircuitAllows = true
		return true
	default:
		eff.CircuitAllows = true
		return true
	}
}

func (r *DefaultResolver) checkExposure(eff *EffectiveState, state SubjectState, _ StateRuntimeContext) bool {
	if !state.PlatformSupported {
		eff.Reasons = append(eff.Reasons, reason("platform_unsupported", "platform", "platform unsupported", "blocking"))
		return false
	}
	return true
}

func subjectDisableReason(kind StateSubjectKind) string {
	switch kind {
	case SubjectExtension:
		return "extension_disabled"
	case SubjectModule:
		return "module_disabled"
	case SubjectTool:
		return "tool_disabled"
	case SubjectAgentSkill:
		return "agent_skill_disabled"
	case SubjectWorkflow:
		return "workflow_disabled"
	case SubjectMCPServer:
		return "mcp_server_disabled"
	case SubjectMCPTool:
		return "mcp_tool_disabled"
	case SubjectSchedule:
		return "schedule_disabled"
	case SubjectUIContribution:
		return "contribution_disabled"
	case SubjectHook:
		return "hook_disabled"
	case SubjectEventSubscription:
		return "event_subscription_disabled"
	case SubjectBackgroundTask:
		return "background_task_disabled"
	case SubjectProvider:
		return "provider_disabled"
	}
	return "subject_disabled"
}

var _ EffectiveStateResolver = (*DefaultResolver)(nil)

type EnablementService struct {
	store    StateStore
	resolver EffectiveStateResolver
}

func NewEnablementService(store StateStore, resolver EffectiveStateResolver) *EnablementService {
	return &EnablementService{store: store, resolver: resolver}
}

func (s *EnablementService) Enable(ctx context.Context, subject StateSubject) error {
	if subject.ID == "" {
		return fmt.Errorf("%w: empty subject id", ErrInvalidSubject)
	}
	return s.store.SetEnablement(ctx, subject, EnablementEnabled)
}

func (s *EnablementService) Disable(ctx context.Context, subject StateSubject) error {
	if subject.ID == "" {
		return fmt.Errorf("%w: empty subject id", ErrInvalidSubject)
	}
	return s.store.SetEnablement(ctx, subject, EnablementDisabled)
}

func (s *EnablementService) SetDesiredRuntime(ctx context.Context, subject StateSubject, state DesiredRuntimeState) error {
	return s.store.SetDesiredRuntime(ctx, subject, state)
}

func (s *EnablementService) Resolve(ctx context.Context, subject StateSubject, runtimeCtx StateRuntimeContext) EffectiveState {
	return s.resolver.Resolve(ctx, subject, runtimeCtx)
}

func (s *EnablementService) Get(ctx context.Context, subject StateSubject) (SubjectState, error) {
	return s.store.Get(ctx, subject)
}
