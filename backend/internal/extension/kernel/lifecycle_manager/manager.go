package lifecycle_manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type CommandKind string

const (
	CmdInstall             CommandKind = "install"
	CmdEnable              CommandKind = "enable"
	CmdDisable             CommandKind = "disable"
	CmdUpdate              CommandKind = "update"
	CmdRollback            CommandKind = "rollback"
	CmdUninstall           CommandKind = "uninstall"
	CmdRepair              CommandKind = "repair"
	CmdEnableModule        CommandKind = "enable_module"
	CmdDisableModule       CommandKind = "disable_module"
	CmdSetContributionOverride CommandKind = "set_contribution_override"
)

type LifecycleCommand struct {
	Kind            CommandKind
	ExtensionID     domain.ExtensionID
	ModuleID        domain.ModuleID
	ContributionID  domain.ContributionID
	TargetVersion   domain.SemanticVersion
	PackageID       string
	SnapshotID      string
	Reason          string
	DryRun          bool
	Force           bool
	UserID          string
	RequestID       string
	Metadata        map[string]any
}

type LifecyclePlan struct {
	Command         LifecycleCommand
	CurrentState    LifecycleStateSnapshot
	TargetState     LifecycleStateSnapshot
	Steps           []LifecycleStep
	Risks           []string
	BlockingIssues  []string
	Compensations   []CompensationAction
	AuditSummary    string
	EstimatedDuration time.Duration
	GeneratedAt     time.Time
}

type LifecycleStateSnapshot struct {
	ExtensionID    domain.ExtensionID
	Installation   *domain.ExtensionInstallation
	Definition     *domain.ExtensionDefinition
	Modules        []domain.ModuleDefinition
	Contributions  []domain.ContributionDefinition
	Runtime        []domain.RuntimeInstance
	Enablement     domain.EnablementState
	Dependencies   []domain.DependencyDefinition
}

type LifecycleStep struct {
	ID            string
	Name          string
	Action        string
	Compensatable bool
	Compensation  string
}

type CompensationAction struct {
	StepID   string
	Action   string
	Reason   string
}

type LifecycleResult struct {
	Command       LifecycleCommand
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	Applied       []string
	Failed        []string
	Skipped       []string
	FinalState    LifecycleStateSnapshot
	Error         string
	OperationID   string
}

type CommandValidator interface {
	Validate(ctx context.Context, cmd LifecycleCommand, current LifecycleStateSnapshot) []string
}

type StateLoader interface {
	Load(ctx context.Context, extID domain.ExtensionID) (LifecycleStateSnapshot, error)
}

type PreflightChecker interface {
	Check(ctx context.Context, cmd LifecycleCommand, current LifecycleStateSnapshot, target LifecycleStateSnapshot) []string
}

type PlanExecutor interface {
	Execute(ctx context.Context, plan LifecyclePlan) (LifecycleResult, error)
}

type AuditWriter interface {
	Record(ctx context.Context, event LifecycleAuditEvent)
}

type LifecycleAuditEvent struct {
	OperationID string
	Command     LifecycleCommand
	Phase       string
	Status      string
	Error       string
	Timestamp   time.Time
	Metadata    map[string]any
}

type Manager struct {
	mu          sync.Mutex
	inProgress  map[domain.ExtensionID]string
	validators  []CommandValidator
	loader      StateLoader
	preflight   PreflightChecker
	executor    PlanExecutor
	audit       AuditWriter
}

func NewManager(loader StateLoader, preflight PreflightChecker, executor PlanExecutor, audit AuditWriter) *Manager {
	return &Manager{
		inProgress: make(map[domain.ExtensionID]string),
		loader:     loader,
		preflight:  preflight,
		executor:   executor,
		audit:      audit,
	}
}

func (m *Manager) RegisterValidator(v CommandValidator) {
	m.validators = append(m.validators, v)
}

var (
	ErrOperationInProgress = errors.New("lifecycle_manager: operation in progress for extension")
	ErrInvalidCommand      = errors.New("lifecycle_manager: invalid command")
	ErrBlockingIssue       = errors.New("lifecycle_manager: blocking issue detected")
)

func (m *Manager) Execute(ctx context.Context, cmd LifecycleCommand) (LifecycleResult, error) {
	if cmd.RequestID == "" {
		cmd.RequestID = newOperationID()
	}
	if err := m.acquireLock(cmd); err != nil {
		return LifecycleResult{}, err
	}
	defer m.releaseLock(cmd)

	current, err := m.loader.Load(ctx, cmd.ExtensionID)
	if err != nil {
		return LifecycleResult{}, err
	}

	for _, v := range m.validators {
		issues := v.Validate(ctx, cmd, current)
		if len(issues) > 0 {
			return LifecycleResult{Command: cmd, Status: "rejected", Error: issues[0]}, fmt.Errorf("%w: %v", ErrInvalidCommand, issues)
		}
	}

	plan, err := m.buildPlan(ctx, cmd, current)
	if err != nil {
		return LifecycleResult{}, err
	}

	if len(plan.BlockingIssues) > 0 && !cmd.Force {
		return LifecycleResult{Command: cmd, Status: "blocked", Error: plan.BlockingIssues[0]}, fmt.Errorf("%w: %v", ErrBlockingIssue, plan.BlockingIssues)
	}

	if cmd.DryRun {
		return LifecycleResult{
			Command:     cmd,
			Status:      "dry_run",
			StartedAt:   plan.GeneratedAt,
			CompletedAt: &plan.GeneratedAt,
			Applied:     nil,
			FinalState:  plan.TargetState,
			OperationID: cmd.RequestID,
		}, nil
	}

	if m.audit != nil {
		m.audit.Record(ctx, LifecycleAuditEvent{
			OperationID: cmd.RequestID,
			Command:     cmd,
			Phase:       "execute",
			Status:      "starting",
			Timestamp:   time.Now().UTC(),
		})
	}
	result, err := m.executor.Execute(ctx, plan)
	if err != nil {
		if m.audit != nil {
			m.audit.Record(ctx, LifecycleAuditEvent{
				OperationID: cmd.RequestID,
				Command:     cmd,
				Phase:       "execute",
				Status:      "failed",
				Error:       err.Error(),
				Timestamp:   time.Now().UTC(),
			})
		}
		return result, err
	}
	if m.audit != nil {
		m.audit.Record(ctx, LifecycleAuditEvent{
			OperationID: cmd.RequestID,
			Command:     cmd,
			Phase:       "execute",
			Status:      result.Status,
			Timestamp:   time.Now().UTC(),
		})
	}
	return result, nil
}

func (m *Manager) Plan(ctx context.Context, cmd LifecycleCommand) (LifecyclePlan, error) {
	current, err := m.loader.Load(ctx, cmd.ExtensionID)
	if err != nil {
		return LifecyclePlan{}, err
	}
	return m.buildPlan(ctx, cmd, current)
}

func (m *Manager) buildPlan(ctx context.Context, cmd LifecycleCommand, current LifecycleStateSnapshot) (LifecyclePlan, error) {
	target := computeTargetState(cmd, current)
	plan := LifecyclePlan{
		Command:      cmd,
		CurrentState: current,
		TargetState:  target,
		GeneratedAt:  time.Now().UTC(),
	}
	if m.preflight != nil {
		plan.BlockingIssues = append(plan.BlockingIssues, m.preflight.Check(ctx, cmd, current, target)...)
	}
	plan.Steps = buildStepsForCommand(cmd)
	plan.Compensations = buildCompensations(plan.Steps)
	plan.AuditSummary = fmt.Sprintf("%s %s@%s", cmd.Kind, cmd.ExtensionID, cmd.TargetVersion.String())
	plan.EstimatedDuration = time.Minute
	return plan, nil
}

func (m *Manager) acquireLock(cmd LifecycleCommand) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if opID, ok := m.inProgress[cmd.ExtensionID]; ok {
		return fmt.Errorf("%w: op=%s", ErrOperationInProgress, opID)
	}
	m.inProgress[cmd.ExtensionID] = cmd.RequestID
	return nil
}

func (m *Manager) releaseLock(cmd LifecycleCommand) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inProgress, cmd.ExtensionID)
}

func computeTargetState(cmd LifecycleCommand, current LifecycleStateSnapshot) LifecycleStateSnapshot {
	target := current
	switch cmd.Kind {
	case CmdInstall:
		target.Enablement = domain.EnablementDisabled
		if current.Installation == nil {
			target.Installation = &domain.ExtensionInstallation{
				ExtensionID:       cmd.ExtensionID,
				InstalledVersion:  cmd.TargetVersion,
				PackageID:         cmd.PackageID,
				InstallationState: domain.InstallationStateInstalled,
				EnablementState:   domain.EnablementDisabled,
				InstalledAt:       time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			}
		}
	case CmdEnable:
		target.Enablement = domain.EnablementEnabled
		if target.Installation != nil {
			target.Installation.EnablementState = domain.EnablementEnabled
		}
	case CmdDisable:
		target.Enablement = domain.EnablementDisabled
		if target.Installation != nil {
			target.Installation.EnablementState = domain.EnablementDisabled
		}
	case CmdUninstall:
		target.Enablement = domain.EnablementDisabled
		target.Installation = nil
	case CmdUpdate:
		if target.Installation != nil {
			target.Installation.InstalledVersion = cmd.TargetVersion
			target.Installation.UpdatedAt = time.Now().UTC()
		}
	case CmdRollback:
		if target.Installation != nil {
			target.Installation.InstalledVersion = cmd.TargetVersion
			target.Installation.UpdatedAt = time.Now().UTC()
		}
	}
	return target
}

func buildStepsForCommand(cmd LifecycleCommand) []LifecycleStep {
	switch cmd.Kind {
	case CmdInstall:
		return []LifecycleStep{
			{ID: "verify_package", Name: "Verify Package", Action: "verify", Compensatable: false},
			{ID: "create_snapshot", Name: "Create Snapshot", Action: "snapshot", Compensatable: true, Compensation: "delete_snapshot"},
			{ID: "install_files", Name: "Install Files", Action: "install", Compensatable: true, Compensation: "remove_files"},
			{ID: "write_definition", Name: "Write Definition", Action: "write", Compensatable: true, Compensation: "delete_definition"},
			{ID: "register_installation", Name: "Register Installation", Action: "register", Compensatable: true, Compensation: "unregister"},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdEnable:
		return []LifecycleStep{
			{ID: "validate_dependencies", Name: "Validate Dependencies", Action: "validate", Compensatable: false},
			{ID: "update_enablement", Name: "Update Enablement", Action: "write", Compensatable: true, Compensation: "restore_enablement"},
			{ID: "start_runtime", Name: "Start Runtime", Action: "start", Compensatable: true, Compensation: "stop_runtime"},
			{ID: "activate_contributions", Name: "Activate Contributions", Action: "activate", Compensatable: true, Compensation: "deactivate"},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdDisable:
		return []LifecycleStep{
			{ID: "pause_schedules", Name: "Pause Schedules", Action: "pause", Compensatable: true, Compensation: "resume_schedules"},
			{ID: "drain_invocations", Name: "Drain Invocations", Action: "drain", Compensatable: false},
			{ID: "deactivate_contributions", Name: "Deactivate Contributions", Action: "deactivate", Compensatable: true, Compensation: "activate"},
			{ID: "stop_runtime", Name: "Stop Runtime", Action: "stop", Compensatable: true, Compensation: "start_runtime"},
			{ID: "update_enablement", Name: "Update Enablement", Action: "write", Compensatable: true, Compensation: "restore_enablement"},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdUninstall:
		return []LifecycleStep{
			{ID: "disable_extension", Name: "Disable Extension", Action: "disable", Compensatable: false},
			{ID: "release_resources", Name: "Release Resources", Action: "release", Compensatable: false},
			{ID: "remove_files", Name: "Remove Files", Action: "remove", Compensatable: false},
			{ID: "delete_definition", Name: "Delete Definition", Action: "delete", Compensatable: false},
			{ID: "unregister_installation", Name: "Unregister Installation", Action: "unregister", Compensatable: false},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdUpdate:
		return []LifecycleStep{
			{ID: "create_snapshot", Name: "Create Snapshot", Action: "snapshot", Compensatable: true, Compensation: "delete_snapshot"},
			{ID: "drain_runtime", Name: "Drain Runtime", Action: "drain", Compensatable: false},
			{ID: "stop_runtime", Name: "Stop Runtime", Action: "stop", Compensatable: true, Compensation: "start_runtime"},
			{ID: "replace_files", Name: "Replace Files", Action: "replace", Compensatable: true, Compensation: "restore_files"},
			{ID: "update_definition", Name: "Update Definition", Action: "write", Compensatable: true, Compensation: "restore_definition"},
			{ID: "start_runtime", Name: "Start Runtime", Action: "start", Compensatable: true, Compensation: "stop_runtime"},
			{ID: "activate_contributions", Name: "Activate Contributions", Action: "activate", Compensatable: true, Compensation: "deactivate"},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdRollback:
		return []LifecycleStep{
			{ID: "load_snapshot", Name: "Load Snapshot", Action: "load", Compensatable: false},
			{ID: "stop_runtime", Name: "Stop Runtime", Action: "stop", Compensatable: false},
			{ID: "restore_files", Name: "Restore Files", Action: "restore", Compensatable: false},
			{ID: "restore_definition", Name: "Restore Definition", Action: "restore", Compensatable: false},
			{ID: "start_runtime", Name: "Start Runtime", Action: "start", Compensatable: true, Compensation: "stop_runtime"},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdRepair:
		return []LifecycleStep{
			{ID: "verify_integrity", Name: "Verify Integrity", Action: "verify", Compensatable: false},
			{ID: "rebuild_state", Name: "Rebuild State", Action: "rebuild", Compensatable: false},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	case CmdEnableModule, CmdDisableModule, CmdSetContributionOverride:
		return []LifecycleStep{
			{ID: "update_state", Name: "Update State", Action: "write", Compensatable: true, Compensation: "restore_state"},
			{ID: "reconcile_runtime", Name: "Reconcile Runtime", Action: "reconcile", Compensatable: false},
			{ID: "audit", Name: "Audit", Action: "audit", Compensatable: false},
		}
	}
	return nil
}

func buildCompensations(steps []LifecycleStep) []CompensationAction {
	var out []CompensationAction
	for _, s := range steps {
		if s.Compensatable && s.Compensation != "" {
			out = append(out, CompensationAction{StepID: s.ID, Action: s.Compensation, Reason: "rollback on failure"})
		}
	}
	return out
}

func newOperationID() string {
	return fmt.Sprintf("op_%d", time.Now().UnixNano())
}
