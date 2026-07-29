package kernel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func (r *Runtime) Enable(ctx context.Context, extensionID string) error {
	if r.container == nil {
		return errors.New("kernel: container not attached")
	}
	extID := domain.ExtensionID(extensionID)

	lock := r.getEnableLock(extensionID)
	lock.Lock()
	defer lock.Unlock()

	inst, err := r.container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: get installation: %w", err)
	}
	prevInst := inst
	prevEnablement := enablement.EnablementDisabled
	prevDesiredRuntime := enablement.DesiredRuntimeStopped
	if inst.EnablementState == domain.EnablementEnabled {
		prevEnablement = enablement.EnablementEnabled
		prevDesiredRuntime = enablement.DesiredRuntimeStarted
	}

	modules, err := r.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: list modules: %w", err)
	}

	candidateGeneration := inst.Generation + 1
	operationID := fmt.Sprintf("enable-%s-%d", extensionID, time.Now().UnixNano())
	r.logEnableStep(operationID, extensionID, "acquire_lock", "succeeded", nil)

	var startedInstanceIDs []string
	if r.container.RuntimeSupervisor != nil {
		for _, mod := range modules {
			if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
				defID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
				spec := runtime_supervisor.InstanceSpec{
					DefinitionID: defID,
					ExtensionID:  extID,
					ModuleID:     mod.ID,
					RuntimeType:  mod.Runtime.Type,
					Generation:   candidateGeneration,
				}
				result := r.container.RuntimeSupervisor.Reconcile(ctx, runtime_supervisor.ReconcileRequest{
					DefinitionID: defID,
					Desired:      runtime_supervisor.DesiredRunning,
					Spec:         spec,
				})
				if result.Error != nil || result.Actual != runtime_supervisor.ActualReady {
					r.stopInstances(ctx, startedInstanceIDs)
					rtErr := runtime_supervisor.ClassifyReconcileError(result)
					if rtErr == nil {
						rtErr = runtime_supervisor.NewRuntimeError(
							runtime_supervisor.CodeRuntimeReconcileFailed,
							fmt.Sprintf("extension=%s module=%s actual=%s", extensionID, mod.ID, result.Actual),
							result.Error,
						)
					}
					r.recordRuntimeOperation(ctx, extID, "enable", rtErr)
					r.logEnableStep(operationID, extensionID, "start_runtime", "failed", rtErr)
					return fmt.Errorf("kernel: enable extension %s: runtime reconcile failed: %w", extensionID, rtErr)
				}
				if result.InstanceID != "" {
					startedInstanceIDs = append(startedInstanceIDs, result.InstanceID)
				}
			}
		}
	}
	r.logEnableStep(operationID, extensionID, "start_runtime", "succeeded", nil)

	var contributionsActivated bool
	if r.container.ContributionInstaller != nil {
		if err := r.container.ContributionInstaller.ActivateContributions(ctx, extID); err != nil {
			rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, false)
			if rbErr != nil {
				r.markRequiresRecovery(ctx, extID, prevInst)
			}
			r.logEnableStep(operationID, extensionID, "activate_contributions", "failed", err)
			return fmt.Errorf("kernel: activate contributions: %w", err)
		}
		contributionsActivated = true
	}
	r.logEnableStep(operationID, extensionID, "activate_contributions", "succeeded", nil)

	var uiRegistered bool
	if r.container.UIContributionRepo != nil {
		uiDefs, uiErr := r.container.UIContributionRepo.ListByExtension(ctx, extensionID)
		if uiErr != nil {
			rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
			if rbErr != nil {
				r.markRequiresRecovery(ctx, extID, prevInst)
			}
			r.logEnableStep(operationID, extensionID, "register_ui", "failed", uiErr)
			return fmt.Errorf("kernel: list ui contributions: %w", uiErr)
		}
		for _, uiDef := range uiDefs {
			if err := r.container.UIHost.RegisterContribution(uiDef); err != nil {
				rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
				if rbErr != nil {
					r.markRequiresRecovery(ctx, extID, prevInst)
				}
				r.logEnableStep(operationID, extensionID, "register_ui", "failed", err)
				return fmt.Errorf("kernel: register ui contribution %s: %w", uiDef.ContributionID, err)
			}
			uiRegistered = true
			if uiDef.Kind == ui_contribution.UIContributionWebPage || uiDef.Kind == ui_contribution.UIContributionSchemaPage {
				entryKind := extension_page_host.PageKindWeb
				if uiDef.Kind == ui_contribution.UIContributionSchemaPage {
					entryKind = extension_page_host.PageKindSchema
				}
				perms := make([]string, 0, len(uiDef.Permissions))
				for _, p := range uiDef.Permissions {
					perms = append(perms, p.Name)
				}
				pageDef := extension_page_host.NewExtensionPageDefinition(extension_page_host.PageRegistrationInput{
					PageID:          extension_page_host.PageID(uiDef.ContributionID),
					ExtensionID:    extension_page_host.ExtensionID(uiDef.ExtensionID),
					ModuleID:       string(uiDef.ModuleID),
					ContributionID: extension_page_host.ContributionID(uiDef.ContributionID),
					Generation:     candidateGeneration,
					ContractVersion: uiDef.ContractVersion,
					EntryKind:       entryKind,
					EntryPath:       uiDef.Entry.Path,
					SchemaPath:      uiDef.Entry.SchemaPath,
					Title: extension_page_host.LocalizedText{
						Default:      uiDef.Display.Title.Default,
						Translations: uiDef.Display.Title.I18n,
					},
					Description: extension_page_host.LocalizedText{
						Default:      uiDef.Display.Description.Default,
						Translations: uiDef.Display.Description.I18n,
					},
					Icon:        uiDef.Display.Icon,
					Permissions: perms,
				})
				if err := r.container.PageHost.RegisterPage(ctx, pageDef); err != nil {
					rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
					if rbErr != nil {
						r.markRequiresRecovery(ctx, extID, prevInst)
					}
					r.logEnableStep(operationID, extensionID, "register_page", "failed", err)
					return fmt.Errorf("kernel: register page %s: %w", uiDef.ContributionID, err)
				}
			}
		}
	}
	r.logEnableStep(operationID, extensionID, "register_ui", "succeeded", nil)

	if r.container.RuntimeSupervisor != nil {
		for _, sid := range startedInstanceIDs {
			snap, hcErr := r.container.RuntimeSupervisor.GetInstance(ctx, sid)
			if hcErr != nil {
				rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
				if rbErr != nil {
					r.markRequiresRecovery(ctx, extID, prevInst)
				}
				r.logEnableStep(operationID, extensionID, "health_check", "failed", hcErr)
				return fmt.Errorf("kernel: health check instance %s: %w", sid, hcErr)
			}
			if snap.Health != runtime_supervisor.HealthHealthy && snap.Health != runtime_supervisor.HealthDegraded {
				rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
				if rbErr != nil {
					r.markRequiresRecovery(ctx, extID, prevInst)
				}
				hErr := fmt.Errorf("kernel: instance %s unhealthy: %s", sid, snap.Health)
				r.logEnableStep(operationID, extensionID, "health_check", "failed", hErr)
				return hErr
			}
		}
	}
	r.logEnableStep(operationID, extensionID, "health_check", "succeeded", nil)

	inst.EnablementState = domain.EnablementEnabled
	inst.UpdatedAt = time.Now().UTC()
	inst.Generation = candidateGeneration
	if err := r.container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		rbErr := r.rollbackEnableCandidate(ctx, extID, startedInstanceIDs, contributionsActivated, uiRegistered)
		if rbErr != nil {
			r.markRequiresRecovery(ctx, extID, prevInst)
		}
		r.logEnableStep(operationID, extensionID, "commit_installation", "failed", err)
		return fmt.Errorf("kernel: commit installation enabled: %w", err)
	}
	r.logEnableStep(operationID, extensionID, "commit_installation", "succeeded", nil)

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementEnabled); err != nil {
		r.rollbackEnableAfterCommit(ctx, extID, prevInst, prevEnablement, prevDesiredRuntime, startedInstanceIDs, contributionsActivated, uiRegistered)
		r.logEnableStep(operationID, extensionID, "commit_enablement", "failed", err)
		return fmt.Errorf("kernel: set extension enablement: %w", err)
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStarted); err != nil {
		r.rollbackEnableAfterCommit(ctx, extID, prevInst, prevEnablement, prevDesiredRuntime, startedInstanceIDs, contributionsActivated, uiRegistered)
		r.logEnableStep(operationID, extensionID, "commit_enablement", "failed", err)
		return fmt.Errorf("kernel: set desired runtime: %w", err)
	}
	for _, mod := range modules {
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(mod.ID),
			ParentID: extensionID,
		}
		if err := r.container.EnablementStore.SetEnablement(ctx, modSubject, enablement.EnablementEnabled); err != nil {
			r.rollbackEnableAfterCommit(ctx, extID, prevInst, prevEnablement, prevDesiredRuntime, startedInstanceIDs, contributionsActivated, uiRegistered)
			r.logEnableStep(operationID, extensionID, "commit_enablement", "failed", err)
			return fmt.Errorf("kernel: set module %s enablement: %w", mod.ID, err)
		}
		if err := r.container.EnablementStore.SetDesiredRuntime(ctx, modSubject, enablement.DesiredRuntimeStarted); err != nil {
			r.rollbackEnableAfterCommit(ctx, extID, prevInst, prevEnablement, prevDesiredRuntime, startedInstanceIDs, contributionsActivated, uiRegistered)
			r.logEnableStep(operationID, extensionID, "commit_enablement", "failed", err)
			return fmt.Errorf("kernel: set module %s desired runtime: %w", mod.ID, err)
		}
	}
	r.logEnableStep(operationID, extensionID, "commit_enablement", "succeeded", nil)
	r.logEnableStep(operationID, extensionID, "promote_generation", "succeeded", nil)

	return nil
}

func (r *Runtime) rollbackEnableCandidate(ctx context.Context, extID domain.ExtensionID, instanceIDs []string, contributionsActivated, uiRegistered bool) error {
	var rollbackErrs []error

	if uiRegistered {
		if r.container.UIHost != nil {
			r.container.UIHost.DisableExtension(ui_contribution.ExtensionID(extID))
		}
		if r.container.PageHost != nil {
			r.container.PageHost.HandleExtensionDisabled(ctx, extension_page_host.ExtensionID(extID))
		}
	}

	if contributionsActivated && r.container.ContributionInstaller != nil {
		if err := r.container.ContributionInstaller.DeactivateContributions(ctx, extID); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("deactivate contributions: %w", err))
		}
	}

	r.stopInstances(ctx, instanceIDs)

	if len(rollbackErrs) > 0 {
		return fmt.Errorf("rollback failed with %d errors: %v", len(rollbackErrs), rollbackErrs)
	}
	return nil
}

func (r *Runtime) rollbackEnableAfterCommit(ctx context.Context, extID domain.ExtensionID, prevInst domain.ExtensionInstallation, prevEnablement enablement.EnablementState, prevDesiredRuntime enablement.DesiredRuntimeState, instanceIDs []string, contributionsActivated, uiRegistered bool) {
	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: string(extID)}
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, prevEnablement); err != nil {
		log.Printf("[enable-tx] failed to restore enablement for %s: %v", extID, err)
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, prevDesiredRuntime); err != nil {
		log.Printf("[enable-tx] failed to restore desired runtime for %s: %v", extID, err)
	}

	prevInst.UpdatedAt = time.Now().UTC()
	if err := r.container.InstallationRepository.PutInstallation(ctx, prevInst); err != nil {
		log.Printf("[enable-tx] failed to restore installation for %s: %v", extID, err)
	}

	rbErr := r.rollbackEnableCandidate(ctx, extID, instanceIDs, contributionsActivated, uiRegistered)
	if rbErr != nil {
		r.markRequiresRecovery(ctx, extID, prevInst)
	}
}

func (r *Runtime) markRequiresRecovery(ctx context.Context, extID domain.ExtensionID, inst domain.ExtensionInstallation) {
	inst.EnablementState = domain.EnablementRequiresRecovery
	inst.UpdatedAt = time.Now().UTC()
	if err := r.container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		log.Printf("[enable-tx] failed to mark requires_recovery for %s: %v", extID, err)
	}
}

func (r *Runtime) logEnableStep(operationID, extensionID, step, status string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	log.Printf("[enable-tx] operationID=%s extensionID=%s step=%s status=%s error=%s",
		operationID, extensionID, step, status, errStr)
}

func (r *Runtime) Disable(ctx context.Context, extensionID string) error {
	if r.container == nil {
		return errors.New("kernel: container not attached")
	}
	extID := domain.ExtensionID(extensionID)

	inst, err := r.container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: get installation: %w", err)
	}

	modules, err := r.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: list modules: %w", err)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	var disableErrs []error
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled); err != nil {
		disableErrs = append(disableErrs, fmt.Errorf("set extension enablement: %w", err))
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped); err != nil {
		disableErrs = append(disableErrs, fmt.Errorf("set extension desired runtime: %w", err))
	}

	for _, mod := range modules {
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(mod.ID),
			ParentID: extensionID,
		}
		if err := r.container.EnablementStore.SetEnablement(ctx, modSubject, enablement.EnablementDisabled); err != nil {
			disableErrs = append(disableErrs, fmt.Errorf("set module %s enablement: %w", mod.ID, err))
		}
		if err := r.container.EnablementStore.SetDesiredRuntime(ctx, modSubject, enablement.DesiredRuntimeStopped); err != nil {
			disableErrs = append(disableErrs, fmt.Errorf("set module %s desired runtime: %w", mod.ID, err))
		}
	}

	var stopErrs []error
	if r.container.RuntimeSupervisor != nil {
		for _, mod := range modules {
			if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
				defID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
				snap := r.container.RuntimeSupervisor.Snapshot(ctx, defID)
				for _, instance := range snap.Instances {
					if stopErr := r.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, runtime_supervisor.StopReasonDisable); stopErr != nil {
						r.recordRuntimeStopFailure(ctx, extID, "disable", instance.InstanceID, stopErr)
						stopErrs = append(stopErrs, stopErr)
					}
				}
			}
		}
	}

	r.container.UIHost.DisableExtension(ui_contribution.ExtensionID(extensionID))
	r.container.PageHost.HandleExtensionDisabled(ctx, extension_page_host.ExtensionID(extensionID))

	if r.container.ContributionInstaller != nil {
		if err := r.container.ContributionInstaller.DeactivateContributions(ctx, extID); err != nil {
			inst.EnablementState = domain.EnablementPartiallyDisabled
			inst.UpdatedAt = time.Now().UTC()
			if putErr := r.container.InstallationRepository.PutInstallation(ctx, inst); putErr != nil {
				log.Printf("[disable-tx] failed to persist partially_disabled state for %s: %v", extensionID, putErr)
			}
			return fmt.Errorf("kernel: deactivate contributions: %w", err)
		}
	}

	finalState := domain.EnablementDisabled
	if len(stopErrs) > 0 || len(disableErrs) > 0 {
		finalState = domain.EnablementPartiallyDisabled
	}
	inst.EnablementState = finalState
	inst.UpdatedAt = time.Now().UTC()
	inst.Generation++
	if err := r.container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		return fmt.Errorf("kernel: update installation: %w", err)
	}

	return nil
}

func (r *Runtime) Uninstall(ctx context.Context, extensionID string) error {
	if r.container == nil {
		return errors.New("kernel: container not attached")
	}
	extID := domain.ExtensionID(extensionID)

	inst, err := r.container.InstallationRepository.GetInstallation(ctx, extID)
	version := ""
	if err == nil {
		version = inst.InstalledVersion.String()
	}

	if r.container.RuntimeSupervisor != nil {
		uninstallModules, modErr := r.container.ModuleRepository.ListModules(ctx, extID)
		if modErr == nil {
			var stopErrs []error
			for _, mod := range uninstallModules {
				if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
					defID := runtime_supervisor.BuildRuntimeDefinitionID(extensionID, string(mod.ID), mod.Runtime.Type)
					snap := r.container.RuntimeSupervisor.Snapshot(ctx, defID)
					for _, instance := range snap.Instances {
						if stopErr := r.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, runtime_supervisor.StopReasonUninstall); stopErr != nil {
							r.recordRuntimeStopFailure(ctx, extID, "uninstall", instance.InstanceID, stopErr)
							stopErrs = append(stopErrs, fmt.Errorf("stop runtime instance %s: %w", instance.InstanceID, stopErr))
						}
					}
				}
			}
			if len(stopErrs) > 0 {
				if inst.InstallationID != "" {
					inst.InstallationState = domain.InstallationStateUninstallFailed
					inst.EnablementState = domain.EnablementRequiresRecovery
					inst.UpdatedAt = time.Now().UTC()
					if putErr := r.container.InstallationRepository.PutInstallation(ctx, inst); putErr != nil {
						log.Printf("[uninstall-tx] failed to persist uninstall_failed state for %s: %v", extensionID, putErr)
					}
				}
				return fmt.Errorf("kernel: uninstall %s blocked: runtime stop failed with %d error(s): %v", extensionID, len(stopErrs), stopErrs)
			}
		}
	}

	if r.container.ContributionInstaller != nil {
		if err := r.container.ContributionInstaller.UninstallContributions(ctx, extID); err != nil {
			if inst.InstallationID != "" {
				inst.InstallationState = domain.InstallationStateUninstallFailed
				inst.EnablementState = domain.EnablementRequiresRecovery
				inst.UpdatedAt = time.Now().UTC()
				if putErr := r.container.InstallationRepository.PutInstallation(ctx, inst); putErr != nil {
					log.Printf("[uninstall-tx] failed to persist uninstall_failed state for %s: %v", extensionID, putErr)
				}
			}
			return fmt.Errorf("kernel: uninstall contributions: %w", err)
		}
	}

	var cleanupErrs []error
	if r.container.HookService != nil && r.container.HookService.Lifecycle != nil {
		if err := r.container.HookService.Lifecycle.UninstallByExtension(ctx, extensionID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("uninstall hooks: %w", err))
		}
	}
	if r.container.EventService != nil {
		if err := r.container.EventService.RemoveSubscriptionsByExtension(ctx, extensionID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("remove event subscriptions: %w", err))
		}
	}
	if r.container.ScheduleService != nil {
		if err := r.container.ScheduleService.DeleteAllByExtension(ctx, extensionID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("delete schedules: %w", err))
		}
	}
	if r.container.TaskRuntimeService != nil {
		if err := r.container.TaskRuntimeService.DeleteByExtension(ctx, extensionID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("delete tasks: %w", err))
		}
	}

	if err := r.container.ContributionRepository.DeleteContributions(ctx, extID); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("delete contribution records: %w", err))
	}
	for _, uiDef := range r.container.UIHost.ListAll() {
		if string(uiDef.ExtensionID) == extensionID {
			if err := r.container.UIHost.UnregisterContribution(uiDef.ContributionID); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("unregister ui contribution %s: %w", uiDef.ContributionID, err))
			}
		}
	}
	if _, err := r.container.PageHost.HandleExtensionUninstalled(ctx, extension_page_host.ExtensionID(extensionID)); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("handle extension uninstalled pages: %w", err))
	}
	if r.container.UIContributionRepo != nil {
		if err := r.container.UIContributionRepo.DeleteByExtension(ctx, extensionID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("delete ui contribution records: %w", err))
		}
	}

	if len(cleanupErrs) > 0 {
		if inst.InstallationID != "" {
			inst.InstallationState = domain.InstallationStateUninstallFailed
			inst.EnablementState = domain.EnablementRequiresRecovery
			inst.UpdatedAt = time.Now().UTC()
			if putErr := r.container.InstallationRepository.PutInstallation(ctx, inst); putErr != nil {
				log.Printf("[uninstall-tx] failed to persist uninstall_failed state for %s: %v", extensionID, putErr)
			}
		}
		return fmt.Errorf("kernel: uninstall %s failed with %d errors: %v", extensionID, len(cleanupErrs), cleanupErrs)
	}

	if err := r.container.ModuleRepository.DeleteModules(ctx, extID); err != nil {
		return fmt.Errorf("kernel: delete modules: %w", err)
	}
	if err := r.container.InstallationRepository.DeleteInstallation(ctx, extID); err != nil {
		return fmt.Errorf("kernel: delete installation: %w", err)
	}
	if version != "" {
		if parsed, pErr := domain.ParseVersion(version); pErr == nil {
			_ = r.container.DefinitionRepository.DeleteExtension(ctx, extID, parsed)
		}
	}

	safeID := safeDirectoryName(extensionID)
	if version != "" {
		installDir := filepath.Join(r.root, "installed", safeID, version)
		if err := os.RemoveAll(installDir); err != nil {
			return fmt.Errorf("kernel: remove install directory %s: %w", installDir, err)
		}
	} else {
		installDir := filepath.Join(r.root, "installed", safeID)
		if err := os.RemoveAll(installDir); err != nil {
			return fmt.Errorf("kernel: remove install directory %s: %w", installDir, err)
		}
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled); err != nil {
		log.Printf("[uninstall-tx] failed to set extension enablement disabled for %s: %v", extensionID, err)
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped); err != nil {
		log.Printf("[uninstall-tx] failed to set desired runtime stopped for %s: %v", extensionID, err)
	}

	r.mu.Lock()
	delete(r.installed, extensionID)
	r.mu.Unlock()

	return nil
}

func (r *Runtime) ResumeUninstall(ctx context.Context, extensionID string) error {
	if r.container == nil {
		return errors.New("kernel: container not attached")
	}
	extID := domain.ExtensionID(extensionID)

	inst, err := r.container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: get installation for resume uninstall: %w", err)
	}
	if inst.InstallationState != domain.InstallationStateUninstallFailed {
		return fmt.Errorf("kernel: cannot resume uninstall: installation state is %s, expected %s", inst.InstallationState, domain.InstallationStateUninstallFailed)
	}
	return r.Uninstall(ctx, extensionID)
}

func (r *Runtime) stopInstances(ctx context.Context, instanceIDs []string) {
	if r.container == nil || r.container.RuntimeSupervisor == nil {
		return
	}
	for _, sid := range instanceIDs {
		_ = r.container.RuntimeSupervisor.Stop(ctx, sid, runtime_supervisor.StopReasonRollback)
	}
}
