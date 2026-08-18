package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
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
	instRepo          domain.InstallationRepository
	defRepo           domain.DefinitionRepository
	moduleRepo        sqlite.ModuleRepository
	contribRepo       sqlite.ContributionRepository
	enablement        enablement.StateStore
	installer         *TypedContributionInstaller
	packageRepo       *PackageRepository
	packageArtifact   *PackageArtifactStore
	packageGeneration *PackageGenerationStore
	packageSecurity   *package_security.PackageSecurityService
	uiHostNotifier    *SSEUIHostNotifier
}

func newContainerPlanExecutor(
	instRepo domain.InstallationRepository,
	defRepo domain.DefinitionRepository,
	moduleRepo sqlite.ModuleRepository,
	contribRepo sqlite.ContributionRepository,
	enablementStore enablement.StateStore,
	installer *TypedContributionInstaller,
	packageRepo *PackageRepository,
	packageArtifact *PackageArtifactStore,
	packageGeneration *PackageGenerationStore,
	packageSecurity *package_security.PackageSecurityService,
	uiHostNotifier *SSEUIHostNotifier,
) *containerPlanExecutor {
	return &containerPlanExecutor{
		instRepo:          instRepo,
		defRepo:           defRepo,
		moduleRepo:        moduleRepo,
		contribRepo:       contribRepo,
		enablement:        enablementStore,
		installer:         installer,
		packageRepo:       packageRepo,
		packageArtifact:   packageArtifact,
		packageGeneration: packageGeneration,
		packageSecurity:   packageSecurity,
		uiHostNotifier:    uiHostNotifier,
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
			if err := e.installer.ActivateContributions(ctx, extID); err != nil {
				if plan.CurrentState.Installation != nil {
					recoveryInst := *plan.CurrentState.Installation
					recoveryInst.EnablementState = domain.EnablementRequiresRecovery
					recoveryInst.UpdatedAt = time.Now().UTC()
					_ = e.instRepo.PutInstallation(ctx, recoveryInst)
				}
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "activate_contributions")
		}
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_enabled", string(extID), nil)
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
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
			if err := e.installer.DeactivateContributions(ctx, extID); err != nil {
				if plan.CurrentState.Installation != nil {
					recoveryInst := *plan.CurrentState.Installation
					recoveryInst.EnablementState = domain.EnablementPartiallyDisabled
					recoveryInst.UpdatedAt = time.Now().UTC()
					_ = e.instRepo.PutInstallation(ctx, recoveryInst)
				}
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
			result.Applied = append(result.Applied, "deactivate_contributions")
		}
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_disabled", string(extID), nil)
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
		}

	case lifecycle_manager.CmdUninstall:
		if e.installer != nil {
			if stopErr := e.installer.StopRuntimeInstances(ctx, extID); stopErr != nil {
				if plan.CurrentState.Installation != nil {
					recoveryInst := *plan.CurrentState.Installation
					recoveryInst.InstallationState = domain.InstallationStateUninstallFailed
					recoveryInst.EnablementState = domain.EnablementRequiresRecovery
					recoveryInst.UpdatedAt = time.Now().UTC()
					_ = e.instRepo.PutInstallation(ctx, recoveryInst)
				}
				result.Status = "failed"
				result.Error = stopErr.Error()
				return result, stopErr
			}
			if err := e.installer.UninstallContributions(ctx, extID); err != nil {
				if plan.CurrentState.Installation != nil {
					recoveryInst := *plan.CurrentState.Installation
					recoveryInst.InstallationState = domain.InstallationStateUninstallFailed
					recoveryInst.EnablementState = domain.EnablementRequiresRecovery
					recoveryInst.UpdatedAt = time.Now().UTC()
					_ = e.instRepo.PutInstallation(ctx, recoveryInst)
				}
				result.Status = "failed"
				result.Error = err.Error()
				return result, err
			}
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
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_uninstalled", string(extID), nil)
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
		}

	case lifecycle_manager.CmdInstall:
		generation, err := e.executeDirectInstallSaga(ctx, plan, &result)
		if err != nil {
			return result, err
		}
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_installed", string(extID), nil)
			e.uiHostNotifier.BroadcastExtensionChange("extension_generation_changed", string(extID), map[string]interface{}{"generation": generation})
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
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
			if e.uiHostNotifier != nil {
				e.uiHostNotifier.BroadcastExtensionChange("extension_updated", string(extID), nil)
				e.uiHostNotifier.BroadcastExtensionChange("extension_generation_changed", string(extID), map[string]interface{}{"generation": inst.Generation})
				e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
			}
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
			if e.uiHostNotifier != nil {
				e.uiHostNotifier.BroadcastExtensionChange("extension_rolled_back", string(extID), map[string]interface{}{"generation": inst.Generation})
				e.uiHostNotifier.BroadcastExtensionChange("extension_generation_changed", string(extID), map[string]interface{}{"generation": inst.Generation})
			}
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
			if e.installer != nil {
				if err := e.installer.RepairContributions(ctx, extID, inst.Generation); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					return result, err
				}
				result.Applied = append(result.Applied, "repair_contributions")
			}
			if e.uiHostNotifier != nil {
				e.uiHostNotifier.BroadcastExtensionChange("extension_enabled", string(extID), nil)
				e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
			}
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
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
		}

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
		if e.uiHostNotifier != nil {
			e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
		}

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
			if e.uiHostNotifier != nil {
				e.uiHostNotifier.BroadcastExtensionChange("extension_contributions_changed", string(extID), nil)
			}
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

func (e *containerPlanExecutor) executeDirectInstallSaga(ctx context.Context, plan lifecycle_manager.LifecyclePlan, result *lifecycle_manager.LifecycleResult) (int64, error) {
	extID := plan.Command.ExtensionID
	packageID := plan.Command.PackageID
	packageURI := plan.Command.PackageURI
	expectedHash, _ := plan.Command.Metadata["hash"].(string)

	if e.packageRepo == nil || e.packageArtifact == nil || e.packageGeneration == nil || e.packageSecurity == nil {
		return 0, fmt.Errorf("package services unavailable for direct install")
	}

	// Resolve PackageURI to ArtifactID if needed
	resolvedPackageID := packageID
	if packageURI != "" && packageID == "" {
		artifactID, err := e.resolveRemoteArtifact(ctx, string(extID), plan.Command.TargetVersion.String(), packageURI, expectedHash)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("resolve remote artifact: %v", err)
			return 0, err
		}
		resolvedPackageID = artifactID
	}

	artifact, err := e.packageRepo.GetArtifact(ctx, resolvedPackageID)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("get artifact: %v", err)
		return 0, err
	}

	if expectedHash != "" && artifact.ArchiveHash != expectedHash {
		err := fmt.Errorf("artifact hash mismatch: expected %s, got %s", expectedHash, artifact.ArchiveHash)
		result.Status = "failed"
		result.Error = err.Error()
		return 0, err
	}

	if err := e.packageArtifact.VerifyArchive(artifact); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("verify archive: %v", err)
		return 0, err
	}

	securityReport, err := e.packageSecurity.InspectFile(ctx, artifact.ArchivePath, package_security.PackageSource{
		SourceType: package_security.SourceLocalFile,
		LocalPath:  artifact.ArchivePath,
	})
	if err != nil || securityReport == nil || !securityReport.Passed {
		if err == nil {
			err = fmt.Errorf("archive security rejected package")
		}
		result.Status = "failed"
		result.Error = err.Error()
		return 0, err
	}

	pkg, err := amitiax.OpenArchive(artifact.ArchivePath)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("open archive: %v", err)
		return 0, err
	}
	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("verify integrity: %v", err)
		return 0, err
	}

	definition, err := pkg.Manifest.ToExtensionDefinition()
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("build definition: %v", err)
		return 0, err
	}
	if definition.ID != extID {
		err := fmt.Errorf("extension id mismatch: command=%s manifest=%s", extID, definition.ID)
		result.Status = "failed"
		result.Error = err.Error()
		return 0, err
	}
	result.Applied = append(result.Applied, "build_candidate_definitions")

	staging, err := e.packageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, "direct-install")
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("extract to staging: %v", err)
		return 0, err
	}
	result.Applied = append(result.Applied, "extract_to_staging")

	installRoot := e.packageSecurity.GetInstallRoot()
	if installRoot == "" {
		_ = e.packageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
		result.Status = "failed"
		result.Error = "install root not configured"
		return 0, fmt.Errorf("install root not configured")
	}

	installDir := filepath.Join(installRoot, string(definition.ID))
	if err := os.MkdirAll(installDir, 0755); err != nil {
		_ = e.packageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
		result.Status = "failed"
		result.Error = fmt.Sprintf("create install dir: %v", err)
		return 0, err
	}

	if err := copyDir(staging.Path, installDir); err != nil {
		_ = e.packageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
		result.Status = "failed"
		result.Error = fmt.Sprintf("copy to install dir: %v", err)
		return 0, err
	}

	_ = e.packageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)

	targetPath := installDir
	generation := int64(1)
	now := time.Now().UTC()
	installID := fmt.Sprintf("inst_%s_%d", extID, now.UnixNano())

	if plan.CurrentState.Installation != nil {
		generation = plan.CurrentState.Installation.Generation + 1
		installID = plan.CurrentState.Installation.InstallationID
	}

	installation := domain.ExtensionInstallation{
		InstallationID:    installID,
		ExtensionID:       definition.ID,
		InstalledVersion:  definition.Version,
		PackageID:         artifact.ArtifactID,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementDisabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        generation,
	}

	for _, module := range definition.Modules {
		if err := e.moduleRepo.PutModule(ctx, module); err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("put module %s: %v", module.ID, err)
			return 0, err
		}
		for _, contribution := range module.Contributions {
			if err := e.contribRepo.PutContribution(ctx, contribution); err != nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf("put contribution %s: %v", contribution.ID, err)
				return 0, err
			}
		}
	}
	result.Applied = append(result.Applied, "commit_kernel_repositories")

	if err := e.defRepo.PutExtension(ctx, definition); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("put definition: %v", err)
		return 0, err
	}
	result.Applied = append(result.Applied, "commit_installed_tree")

	if err := e.instRepo.PutInstallation(ctx, installation); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("put installation: %v", err)
		return 0, err
	}
	result.Applied = append(result.Applied, "create_installation")

	artifact.InstalledPath = targetPath
	result.Applied = append(result.Applied, "mark_installation_disabled")

	return generation, nil
}

// resolveRemoteArtifact downloads a package from a remote URI into the managed
// Artifact Store and registers it in the PackageRepository with a canonical ArtifactID.
func (e *containerPlanExecutor) resolveRemoteArtifact(ctx context.Context, extID, version, packageURI, expectedHash string) (string, error) {
	if packageURI == "" {
		return "", fmt.Errorf("packageURI is empty")
	}
	if e.packageArtifact == nil || e.packageRepo == nil {
		return "", fmt.Errorf("package artifact services unavailable")
	}

	archivePath, err := e.packageArtifact.HasArtifactAtHash(expectedHash)
	if err == nil && archivePath != "" {
		if existing, getErr := e.packageRepo.GetArtifactByArchivePath(ctx, archivePath); getErr == nil && existing.ArtifactID != "" {
			return existing.ArtifactID, nil
		}
	}

	metadata := ArtifactMetadata{
		ExtensionID:  extID,
		Version:      version,
		SourceURI:    packageURI,
		ExpectedHash: expectedHash,
	}
	result, err := e.packageArtifact.PutArchiveFromURI(ctx, packageURI, metadata)
	if err != nil {
		return "", fmt.Errorf("store remote artifact: %w", err)
	}

	artifactID := result.ArtifactID
	if artifactID == "" {
		artifactID = e.packageArtifact.ArtifactIDFromHash(result.ArchiveHash)
	}

	if err := e.packageRepo.PutArtifact(ctx, PackageArtifact{
		ArtifactID:  artifactID,
		ExtensionID: extID,
		Version:     version,
		ArchiveHash: result.ArchiveHash,
		ArchivePath: result.ArchivePath,
	}); err != nil {
		return "", fmt.Errorf("register artifact: %w", err)
	}

	return artifactID, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source dir: %w", err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read file %s: %w", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("write file %s: %w", dstPath, err)
		}
	}
	return nil
}

type containerHostAPIAuditQuery struct {
	repo sqlite.HostAPIAuditRepository
}

func newContainerHostAPIAuditQuery(repo sqlite.HostAPIAuditRepository) *containerHostAPIAuditQuery {
	return &containerHostAPIAuditQuery{repo: repo}
}

func (q *containerHostAPIAuditQuery) ListAuditLogs(ctx context.Context, extensionID, method, result, traceID string, limit, offset int) ([]developer_console.HostAPIAuditEntry, error) {
	filter := sqlite.HostAPIAuditFilter{
		ExtensionID: extensionID,
		Method:      method,
		Result:      result,
		TraceID:     traceID,
		Limit:       limit,
		Offset:      offset,
	}
	logs, err := q.repo.ListAuditLogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	entries := make([]developer_console.HostAPIAuditEntry, 0, len(logs))
	for _, log := range logs {
		entries = append(entries, developer_console.HostAPIAuditEntry{
			CallID:               log.CallID,
			TraceID:              log.TraceID,
			OperationID:          log.OperationID,
			InvocationID:         log.InvocationID,
			ExtensionID:          log.ExtensionID,
			ModuleID:             log.ModuleID,
			Method:               log.Method,
			Generation:           log.Generation,
			PermissionSnapshotID: log.PermissionSnapshotID,
			ScopeSnapshotID:      log.ScopeSnapshotID,
			StartedAt:            log.StartedAt,
			FinishedAt:           log.FinishedAt,
			Result:               log.Result,
			ErrorCode:            log.ErrorCode,
			ErrorMessage:         log.ErrorMessage,
			SideEffect:           log.SideEffect,
			InputMasked:          log.InputMasked,
			Phase:                log.Phase,
		})
	}
	return entries, nil
}

func (q *containerHostAPIAuditQuery) CountAuditLogs(ctx context.Context, extensionID, method, result, traceID string) (int64, error) {
	filter := sqlite.HostAPIAuditFilter{
		ExtensionID: extensionID,
		Method:      method,
		Result:      result,
		TraceID:     traceID,
	}
	return q.repo.CountAuditLogs(ctx, filter)
}
