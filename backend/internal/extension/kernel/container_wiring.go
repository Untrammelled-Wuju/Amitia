package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type containerCandidateProvider struct {
	instRepo domain.InstallationRepository
	defRepo  domain.DefinitionRepository
}

func newContainerCandidateProvider(instRepo domain.InstallationRepository, defRepo domain.DefinitionRepository) *containerCandidateProvider {
	return &containerCandidateProvider{instRepo: instRepo, defRepo: defRepo}
}

func (p *containerCandidateProvider) FindCandidates(ctx context.Context, target string, targetType dependency.TargetType) ([]dependency.Candidate, error) {
	if targetType != dependency.TargetExtension {
		return nil, nil
	}
	inst, err := p.instRepo.GetInstallation(ctx, domain.ExtensionID(target))
	if err != nil {
		return nil, nil
	}
	if inst.InstallationState != domain.InstallationStateInstalled {
		return nil, nil
	}
	def, err := p.defRepo.GetExtension(ctx, inst.ExtensionID, inst.InstalledVersion)
	if err != nil {
		return nil, nil
	}
	return []dependency.Candidate{
		{
			TargetID:  target,
			Type:      targetType,
			Version:   def.Version,
			Origin:    "installed",
			Available: true,
			Trust:     string(def.Publisher.TrustLevel),
		},
	}, nil
}

type containerStateLoader struct {
	instRepo    domain.InstallationRepository
	defRepo     domain.DefinitionRepository
	moduleRepo  sqlite.ModuleRepository
	contribRepo sqlite.ContributionRepository
	runtimeRepo domain.RuntimeRepository
	enablement  enablement.StateStore
}

func newContainerStateLoader(
	instRepo domain.InstallationRepository,
	defRepo domain.DefinitionRepository,
	moduleRepo sqlite.ModuleRepository,
	contribRepo sqlite.ContributionRepository,
	runtimeRepo domain.RuntimeRepository,
	enablementStore enablement.StateStore,
) *containerStateLoader {
	return &containerStateLoader{
		instRepo:    instRepo,
		defRepo:     defRepo,
		moduleRepo:  moduleRepo,
		contribRepo: contribRepo,
		runtimeRepo: runtimeRepo,
		enablement:  enablementStore,
	}
}

func (l *containerStateLoader) Load(ctx context.Context, extID domain.ExtensionID) (lifecycle_manager.LifecycleStateSnapshot, error) {
	snap := lifecycle_manager.LifecycleStateSnapshot{ExtensionID: extID}

	inst, err := l.instRepo.GetInstallation(ctx, extID)
	if err == nil {
		snap.Installation = &inst
		snap.Enablement = inst.EnablementState
	}

	if snap.Installation != nil {
		def, err := l.defRepo.GetExtension(ctx, extID, snap.Installation.InstalledVersion)
		if err == nil {
			snap.Definition = &def
			snap.Dependencies = def.Dependencies
		}
	}

	if modules, err := l.moduleRepo.ListModules(ctx, extID); err == nil {
		snap.Modules = modules
	}

	if contribs, err := l.contribRepo.ListContributions(ctx, extID); err == nil {
		snap.Contributions = contribs
	}

	if instances, err := l.runtimeRepo.ListInstances(ctx, extID); err == nil {
		snap.Runtime = instances
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: string(extID)}
	if state, err := l.enablement.Get(ctx, extSubject); err == nil {
		if state.Enablement == enablement.EnablementEnabled {
			snap.Enablement = domain.EnablementEnabled
		} else {
			snap.Enablement = domain.EnablementDisabled
		}
	}

	return snap, nil
}

type containerPreflightChecker struct {
	depResolver dependency.Resolver
}

func newContainerPreflightChecker(depResolver dependency.Resolver) *containerPreflightChecker {
	return &containerPreflightChecker{depResolver: depResolver}
}

func (c *containerPreflightChecker) Check(ctx context.Context, cmd lifecycle_manager.LifecycleCommand, current lifecycle_manager.LifecycleStateSnapshot, target lifecycle_manager.LifecycleStateSnapshot) []string {
	var issues []string

	switch cmd.Kind {
	case lifecycle_manager.CmdEnable:
		if current.Installation == nil {
			issues = append(issues, "extension not installed")
		}
		if current.Definition != nil && len(current.Dependencies) > 0 {
			depReq := dependency.ResolveRequest{
				SourceID: string(cmd.ExtensionID),
				Phase:    dependency.PhaseEnable,
			}
			for _, dep := range current.Dependencies {
				depReq.Requests = append(depReq.Requests, dependency.Request{
					Target:   string(dep.ID),
					Type:     dependency.TargetType(dep.Type),
					Required: !dep.Optional,
				})
			}
			if len(depReq.Requests) > 0 {
				result := c.depResolver.Resolve(ctx, depReq)
				for _, c := range result.Conflicts {
					issues = append(issues, fmt.Sprintf("dependency conflict: %s - %s", c.Kind, c.Detail))
				}
			}
		}
	case lifecycle_manager.CmdUninstall:
		if current.Installation == nil {
			issues = append(issues, "extension not installed")
		}
	case lifecycle_manager.CmdUpdate:
		if current.Installation == nil {
			issues = append(issues, "extension not installed, cannot update")
		}
	}

	return issues
}

type containerPlanExecutor struct {
	instRepo    domain.InstallationRepository
	defRepo     domain.DefinitionRepository
	moduleRepo  sqlite.ModuleRepository
	contribRepo sqlite.ContributionRepository
	enablement  enablement.StateStore
	installer   *TypedContributionInstaller
}

func newContainerPlanExecutor(
	instRepo domain.InstallationRepository,
	defRepo domain.DefinitionRepository,
	moduleRepo sqlite.ModuleRepository,
	contribRepo sqlite.ContributionRepository,
	enablementStore enablement.StateStore,
	installer *TypedContributionInstaller,
) *containerPlanExecutor {
	return &containerPlanExecutor{
		instRepo:    instRepo,
		defRepo:      defRepo,
		moduleRepo:  moduleRepo,
		contribRepo: contribRepo,
		enablement:  enablementStore,
		installer:   installer,
	}
}

func (e *containerPlanExecutor) Execute(ctx context.Context, plan lifecycle_manager.LifecyclePlan) (lifecycle_manager.LifecycleResult, error) {
	result := lifecycle_manager.LifecycleResult{
		Command:   plan.Command,
		Status:    "succeeded",
		StartedAt: time.Now().UTC(),
	}
	extID := plan.Command.ExtensionID
	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: string(extID)}

	switch plan.Command.Kind {
	case lifecycle_manager.CmdEnable:
		if plan.CurrentState.Installation != nil {
			inst := *plan.CurrentState.Installation
			inst.EnablementState = domain.EnablementEnabled
			inst.UpdatedAt = time.Now().UTC()
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "update_installation")
		}
		_ = e.enablement.SetEnablement(ctx, extSubject, enablement.EnablementEnabled)
		result.Applied = append(result.Applied, "set_enablement")
		if e.installer != nil {
			e.installer.ActivateContributions(ctx, extID)
			result.Applied = append(result.Applied, "activate_contributions")
		}

	case lifecycle_manager.CmdDisable:
		if plan.CurrentState.Installation != nil {
			inst := *plan.CurrentState.Installation
			inst.EnablementState = domain.EnablementDisabled
			inst.UpdatedAt = time.Now().UTC()
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "update_installation")
		}
		_ = e.enablement.SetEnablement(ctx, extSubject, enablement.EnablementDisabled)
		result.Applied = append(result.Applied, "set_enablement")
		if e.installer != nil {
			e.installer.DeactivateContributions(ctx, extID)
			result.Applied = append(result.Applied, "deactivate_contributions")
		}

	case lifecycle_manager.CmdUninstall:
		if e.installer != nil {
			e.installer.StopRuntimeInstances(ctx, extID)
			e.installer.UninstallContributions(ctx, extID)
			result.Applied = append(result.Applied, "uninstall_contributions")
		}
		_ = e.enablement.SetEnablement(ctx, extSubject, enablement.EnablementDisabled)
		_ = e.contribRepo.DeleteContributions(ctx, extID)
		_ = e.moduleRepo.DeleteModules(ctx, extID)
		_ = e.instRepo.DeleteInstallation(ctx, extID)
		if plan.CurrentState.Definition != nil {
			_ = e.defRepo.DeleteExtension(ctx, extID, plan.CurrentState.Definition.Version)
		}
		result.Applied = append(result.Applied, "uninstall")

	case lifecycle_manager.CmdInstall:
		now := time.Now().UTC()
		inst := domain.ExtensionInstallation{
			ExtensionID:       extID,
			InstalledVersion:  plan.Command.TargetVersion,
			PackageID:         plan.Command.PackageID,
			InstallationState: domain.InstallationStateInstalled,
			EnablementState:   domain.EnablementDisabled,
			InstalledAt:       now,
			UpdatedAt:         now,
			Generation:        1,
		}
		if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}
		result.Applied = append(result.Applied, "create_installation")
		if e.installer != nil {
			e.installer.InstallContributions(ctx, plan.CurrentState.Contributions)
			result.Applied = append(result.Applied, "install_contributions")
		}

	case lifecycle_manager.CmdUpdate:
		if plan.CurrentState.Installation != nil {
			inst := *plan.CurrentState.Installation
			inst.InstalledVersion = plan.Command.TargetVersion
			inst.UpdatedAt = time.Now().UTC()
			inst.Generation = inst.Generation + 1
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "update_version")
		} else {
			result.Status = "failed"
			result.Error = "installation not found for update"
		}

	case lifecycle_manager.CmdRollback:
		if plan.CurrentState.Installation != nil {
			inst := *plan.CurrentState.Installation
			inst.InstalledVersion = plan.Command.TargetVersion
			inst.UpdatedAt = time.Now().UTC()
			if inst.Generation > 1 {
				inst.Generation = inst.Generation - 1
			}
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "rollback_version")
		} else {
			result.Status = "failed"
			result.Error = "installation not found for rollback"
		}

	case lifecycle_manager.CmdRepair:
		if plan.CurrentState.Installation != nil {
			inst := *plan.CurrentState.Installation
			inst.InstallationState = domain.InstallationStateInstalled
			inst.UpdatedAt = time.Now().UTC()
			if err := e.instRepo.PutInstallation(ctx, inst); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "repair_installation")
		}
		if e.installer != nil {
			e.installer.RepairContributions(ctx, extID)
			result.Applied = append(result.Applied, "repair_contributions")
		}

	case lifecycle_manager.CmdEnableModule:
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(plan.Command.ModuleID),
			ParentID: string(extID),
		}
		if err := e.enablement.SetEnablement(ctx, modSubject, enablement.EnablementEnabled); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}
		result.Applied = append(result.Applied, "enable_module")

	case lifecycle_manager.CmdDisableModule:
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(plan.Command.ModuleID),
			ParentID: string(extID),
		}
		if err := e.enablement.SetEnablement(ctx, modSubject, enablement.EnablementDisabled); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}
		result.Applied = append(result.Applied, "disable_module")

	case lifecycle_manager.CmdSetContributionOverride:
		if plan.Command.ContributionID != "" {
			contrib, err := e.contribRepo.GetContribution(ctx, extID, plan.Command.ContributionID)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			if override, ok := plan.Command.Metadata["enabled_override"].(bool); ok {
				if override {
					contrib.Definition["enabled_override"] = "enabled"
				} else {
					contrib.Definition["enabled_override"] = "disabled"
				}
			}
			if err := e.contribRepo.PutContribution(ctx, contrib); err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "set_contribution_override")
		}

	default:
		result.Status = "skipped"
		result.Skipped = append(result.Skipped, string(plan.Command.Kind))
	}

	now := time.Now().UTC()
	result.CompletedAt = &now
	result.OperationID = plan.Command.RequestID
	return result, nil
}

type containerAuditWriter struct {
	opRepo sqlite.OperationRepository
}

func newContainerAuditWriter(opRepo sqlite.OperationRepository) *containerAuditWriter {
	return &containerAuditWriter{opRepo: opRepo}
}

func (w *containerAuditWriter) Record(ctx context.Context, event lifecycle_manager.LifecycleAuditEvent) {
	log.Printf("[lifecycle-audit] op=%s cmd=%s phase=%s status=%s err=%s",
		event.OperationID, event.Command.Kind, event.Phase, event.Status, event.Error)

	if w.opRepo == nil {
		return
	}

	op := sqlite.Operation{
		OperationID:   event.OperationID,
		OperationType: string(event.Command.Kind),
		ExtensionID:   event.Command.ExtensionID,
		Status:        event.Status,
		StartedAt:     event.Timestamp,
	}

	if event.Error != "" {
		op.ErrorMessage = event.Error
	}

	if event.Phase == "execute" && (event.Status == "succeeded" || event.Status == "failed" || event.Status == "skipped") {
		now := time.Now().UTC()
		op.FinishedAt = &now
	}

	_ = w.opRepo.PutOperation(ctx, op)
}

type containerExtensionSummaryProvider struct {
	instRepo   domain.InstallationRepository
	moduleRepo sqlite.ModuleRepository
}

func newContainerExtensionSummaryProvider(instRepo domain.InstallationRepository, moduleRepo sqlite.ModuleRepository) *containerExtensionSummaryProvider {
	return &containerExtensionSummaryProvider{instRepo: instRepo, moduleRepo: moduleRepo}
}

func (p *containerExtensionSummaryProvider) List(ctx context.Context) ([]developer_console.ExtensionSummary, error) {
	insts, err := p.instRepo.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]developer_console.ExtensionSummary, 0, len(insts))
	for _, inst := range insts {
		modules, _ := p.moduleRepo.ListModules(ctx, inst.ExtensionID)
		summary := developer_console.ExtensionSummary{
			ExtensionID: string(inst.ExtensionID),
			Version:     inst.InstalledVersion.String(),
			Enabled:     inst.EnablementState == domain.EnablementEnabled,
			Status:      string(inst.InstallationState),
			ModuleCount: len(modules),
		}
		out = append(out, summary)
	}
	return out, nil
}

type containerInvocationSummaryProvider struct {
	repo *developer_console.DiagnosticRepository
}

func newContainerInvocationSummaryProvider(repo *developer_console.DiagnosticRepository) *containerInvocationSummaryProvider {
	return &containerInvocationSummaryProvider{repo: repo}
}

func (p *containerInvocationSummaryProvider) Active(ctx context.Context) (int, error) {
	recs, err := p.repo.ListInvocations(ctx, developer_console.ConsoleFilters{})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range recs {
		if r.Status == "running" || r.Status == "pending" {
			count++
		}
	}
	return count, nil
}

func (p *containerInvocationSummaryProvider) Recent(ctx context.Context, since time.Time) (int, error) {
	recs, err := p.repo.ListInvocations(ctx, developer_console.ConsoleFilters{StartTime: &since})
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

type containerEventSummaryProvider struct {
	repo *developer_console.DiagnosticRepository
}

func newContainerEventSummaryProvider(repo *developer_console.DiagnosticRepository) *containerEventSummaryProvider {
	return &containerEventSummaryProvider{repo: repo}
}

func (p *containerEventSummaryProvider) Recent(ctx context.Context, since time.Time) (int, error) {
	recs, err := p.repo.ListEvents(ctx, developer_console.ConsoleFilters{StartTime: &since})
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

type containerStorageSummaryProvider struct {
	db *sql.DB
}

func newContainerStorageSummaryProvider(db *sql.DB) *containerStorageSummaryProvider {
	return &containerStorageSummaryProvider{db: db}
}

func (p *containerStorageSummaryProvider) EntryCount(ctx context.Context) (int, error) {
	var count int
	err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_contributions WHERE enabled_override IS NOT NULL OR registered = 1`).Scan(&count)
	if err != nil {
		return 0, nil
	}
	return count, nil
}

type containerPermissionSummaryProvider struct {
	db *sql.DB
}

func newContainerPermissionSummaryProvider(db *sql.DB) *containerPermissionSummaryProvider {
	return &containerPermissionSummaryProvider{db: db}
}

func (p *containerPermissionSummaryProvider) Grants(ctx context.Context) (int, error) {
	var count int
	err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_permission_grants WHERE state = 'granted'`).Scan(&count)
	if err != nil {
		return 0, nil
	}
	return count, nil
}

type containerLifecycleSummaryProvider struct {
	repo *developer_console.DiagnosticRepository
}

func newContainerLifecycleSummaryProvider(repo *developer_console.DiagnosticRepository) *containerLifecycleSummaryProvider {
	return &containerLifecycleSummaryProvider{repo: repo}
}

func (p *containerLifecycleSummaryProvider) Events(ctx context.Context, since time.Time) (int, error) {
	recs, err := p.repo.ListLifecycle(ctx, developer_console.ConsoleFilters{StartTime: &since})
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}
