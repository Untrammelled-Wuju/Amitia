package lifecycle_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type DefaultStateLoader struct {
	defRepo  domain.DefinitionRepository
	instRepo domain.InstallationRepository
	rtRepo   domain.RuntimeRepository
}

func NewDefaultStateLoader(defRepo domain.DefinitionRepository, instRepo domain.InstallationRepository, rtRepo domain.RuntimeRepository) *DefaultStateLoader {
	return &DefaultStateLoader{defRepo: defRepo, instRepo: instRepo, rtRepo: rtRepo}
}

func (l *DefaultStateLoader) Load(ctx context.Context, extID domain.ExtensionID) (LifecycleStateSnapshot, error) {
	snap := LifecycleStateSnapshot{ExtensionID: extID}
	inst, err := l.instRepo.GetInstallation(ctx, extID)
	if err == nil {
		snap.Installation = &inst
		snap.Enablement = inst.EnablementState
		version := inst.InstalledVersion
		def, err := l.defRepo.GetExtension(ctx, extID, version)
		if err == nil {
			snap.Definition = &def
			snap.Modules = def.Modules
			snap.Contributions = def.AllContributions()
			var deps []domain.DependencyDefinition
			deps = append(deps, def.Dependencies...)
			for _, m := range def.Modules {
				deps = append(deps, m.Dependencies...)
			}
			snap.Dependencies = deps
		}
	}
	instances, _ := l.rtRepo.ListInstances(ctx, extID)
	snap.Runtime = instances
	return snap, nil
}

var _ StateLoader = (*DefaultStateLoader)(nil)

type DefaultPreflightChecker struct{}

func NewDefaultPreflightChecker() *DefaultPreflightChecker { return &DefaultPreflightChecker{} }

func (c *DefaultPreflightChecker) Check(_ context.Context, cmd LifecycleCommand, current, target LifecycleStateSnapshot) []string {
	var issues []string
	switch cmd.Kind {
	case CmdEnable:
		if current.Installation == nil {
			issues = append(issues, "extension not installed")
		}
	case CmdUpdate:
		if current.Installation == nil {
			issues = append(issues, "cannot update non-installed extension")
		}
		if cmd.TargetVersion.Major == 0 && cmd.TargetVersion.Minor == 0 && cmd.TargetVersion.Patch == 0 {
			issues = append(issues, "target version required")
		}
	case CmdRollback:
		if current.Installation == nil {
			issues = append(issues, "cannot rollback non-installed extension")
		}
		if cmd.SnapshotID == "" && len(current.Installation.RollbackPoints) == 0 {
			issues = append(issues, "no rollback points available")
		}
	case CmdUninstall:
		if current.Installation == nil {
			issues = append(issues, "extension not installed")
		}
	case CmdEnableModule, CmdDisableModule:
		if current.Definition == nil {
			issues = append(issues, "definition not loaded")
			break
		}
		found := false
		for _, m := range current.Definition.Modules {
			if m.ID == cmd.ModuleID {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, fmt.Sprintf("module %s not found", cmd.ModuleID))
		}
	}
	return issues
}

var _ PreflightChecker = (*DefaultPreflightChecker)(nil)

type DefaultExecutor struct {
	mu        sync.Mutex
	instRepo  domain.InstallationRepository
	defRepo   domain.DefinitionRepository
	rtRepo    domain.RuntimeRepository
}

func NewDefaultExecutor(instRepo domain.InstallationRepository, defRepo domain.DefinitionRepository, rtRepo domain.RuntimeRepository) *DefaultExecutor {
	return &DefaultExecutor{instRepo: instRepo, defRepo: defRepo, rtRepo: rtRepo}
}

func (e *DefaultExecutor) Execute(ctx context.Context, plan LifecyclePlan) (LifecycleResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := LifecycleResult{
		Command:     plan.Command,
		Status:      "running",
		StartedAt:   time.Now().UTC(),
		OperationID: plan.Command.RequestID,
	}
	for _, step := range plan.Steps {
		if err := e.executeStep(ctx, plan.Command, step); err != nil {
			result.Failed = append(result.Failed, step.ID)
			result.Status = "failed"
			result.Error = err.Error()
			completed := time.Now().UTC()
			result.CompletedAt = &completed
			e.runCompensations(ctx, plan, step)
			return result, err
		}
		result.Applied = append(result.Applied, step.ID)
	}
	result.Status = "completed"
	completed := time.Now().UTC()
	result.CompletedAt = &completed
	finalSnap, _ := e.loadFinal(ctx, plan.Command.ExtensionID)
	result.FinalState = finalSnap
	return result, nil
}

func (e *DefaultExecutor) executeStep(ctx context.Context, cmd LifecycleCommand, step LifecycleStep) error {
	switch step.Action {
	case "verify":
		if cmd.PackageID == "" && (cmd.Kind == CmdInstall) {
			return fmt.Errorf("package id required")
		}
	case "snapshot":
	case "install", "replace":
	case "write", "register":
		if cmd.Kind == CmdInstall || cmd.Kind == CmdUpdate {
			inst := domain.ExtensionInstallation{
				InstallationID:    fmt.Sprintf("inst_%s_%d", cmd.ExtensionID, time.Now().UnixNano()),
				ExtensionID:       cmd.ExtensionID,
				InstalledVersion:  cmd.TargetVersion,
				PackageID:         cmd.PackageID,
				InstallationState: domain.InstallationStateInstalled,
				EnablementState:   domain.EnablementDisabled,
				InstalledAt:       time.Now().UTC(),
				UpdatedAt:         time.Now().UTC(),
			}
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				return err
			}
		}
	case "start":
		if cmd.Kind == CmdEnable || cmd.Kind == CmdUpdate || cmd.Kind == CmdRollback {
			inst, err := e.instRepo.GetInstallation(ctx, cmd.ExtensionID)
			if err == nil {
				inst.EnablementState = domain.EnablementEnabled
				inst.UpdatedAt = time.Now().UTC()
				_ = e.instRepo.PutInstallation(ctx, inst)
			}
		}
	case "stop":
		if cmd.Kind == CmdDisable || cmd.Kind == CmdUninstall || cmd.Kind == CmdUpdate {
			inst, err := e.instRepo.GetInstallation(ctx, cmd.ExtensionID)
			if err == nil {
				if cmd.Kind == CmdDisable {
					inst.EnablementState = domain.EnablementDisabled
				}
				inst.UpdatedAt = time.Now().UTC()
				_ = e.instRepo.PutInstallation(ctx, inst)
			}
		}
	case "activate", "deactivate", "pause", "drain":
	case "release", "remove", "delete", "unregister":
		if cmd.Kind == CmdUninstall {
			_ = e.instRepo.DeleteInstallation(ctx, cmd.ExtensionID)
		}
	case "reconcile", "load", "restore":
	case "audit":
	}
	return nil
}

func (e *DefaultExecutor) runCompensations(_ context.Context, plan LifecyclePlan, failedStep LifecycleStep) {
	for _, c := range plan.Compensations {
		_ = c
	}
	_ = failedStep
}

func (e *DefaultExecutor) loadFinal(ctx context.Context, extID domain.ExtensionID) (LifecycleStateSnapshot, error) {
	snap := LifecycleStateSnapshot{ExtensionID: extID}
	inst, err := e.instRepo.GetInstallation(ctx, extID)
	if err == nil {
		snap.Installation = &inst
		snap.Enablement = inst.EnablementState
	}
	return snap, nil
}

var _ PlanExecutor = (*DefaultExecutor)(nil)

type InMemoryAuditWriter struct {
	mu      sync.Mutex
	entries []LifecycleAuditEvent
}

func NewInMemoryAuditWriter() *InMemoryAuditWriter {
	return &InMemoryAuditWriter{}
}

func (w *InMemoryAuditWriter) Record(_ context.Context, event LifecycleAuditEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, event)
}

func (w *InMemoryAuditWriter) Events() []LifecycleAuditEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]LifecycleAuditEvent{}, w.entries...)
}

var _ AuditWriter = (*InMemoryAuditWriter)(nil)
