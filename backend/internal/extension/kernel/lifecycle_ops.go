package kernel

import (
	"context"
	"errors"
	"fmt"
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

	inst, err := r.container.InstallationRepository.GetInstallation(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: get installation: %w", err)
	}

	inst.EnablementState = domain.EnablementEnabled
	inst.UpdatedAt = time.Now().UTC()
	inst.Generation++
	if err := r.container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		return fmt.Errorf("kernel: update installation: %w", err)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementEnabled); err != nil {
		return fmt.Errorf("kernel: set extension enablement: %w", err)
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStarted); err != nil {
		return fmt.Errorf("kernel: set desired runtime: %w", err)
	}

	modules, err := r.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: list modules: %w", err)
	}
	for _, mod := range modules {
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(mod.ID),
			ParentID: extensionID,
		}
		_ = r.container.EnablementStore.SetEnablement(ctx, modSubject, enablement.EnablementEnabled)
		_ = r.container.EnablementStore.SetDesiredRuntime(ctx, modSubject, enablement.DesiredRuntimeStarted)
	}

	if r.container.UIContributionRepo != nil {
		uiDefs, uiErr := r.container.UIContributionRepo.ListByExtension(ctx, extensionID)
		if uiErr == nil {
			for _, uiDef := range uiDefs {
				_ = r.container.UIHost.RegisterContribution(uiDef)
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
						ExtensionID:     extension_page_host.ExtensionID(uiDef.ExtensionID),
						ModuleID:        string(uiDef.ModuleID),
						ContributionID:  extension_page_host.ContributionID(uiDef.ContributionID),
						Generation:      inst.Generation,
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
					_ = r.container.PageHost.RegisterPage(ctx, pageDef)
				}
			}
		}
	}

	if r.container.ContributionInstaller != nil {
		r.container.ContributionInstaller.ActivateContributions(ctx, extID)
	}

	return nil
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

	inst.EnablementState = domain.EnablementDisabled
	inst.UpdatedAt = time.Now().UTC()
	inst.Generation++
	if err := r.container.InstallationRepository.PutInstallation(ctx, inst); err != nil {
		return fmt.Errorf("kernel: update installation: %w", err)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	if err := r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled); err != nil {
		return fmt.Errorf("kernel: set extension enablement: %w", err)
	}
	if err := r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped); err != nil {
		return fmt.Errorf("kernel: set desired runtime: %w", err)
	}

	modules, err := r.container.ModuleRepository.ListModules(ctx, extID)
	if err != nil {
		return fmt.Errorf("kernel: list modules: %w", err)
	}
	for _, mod := range modules {
		modSubject := enablement.StateSubject{
			Kind:     enablement.SubjectModule,
			ID:       string(mod.ID),
			ParentID: extensionID,
		}
		_ = r.container.EnablementStore.SetEnablement(ctx, modSubject, enablement.EnablementDisabled)
		_ = r.container.EnablementStore.SetDesiredRuntime(ctx, modSubject, enablement.DesiredRuntimeStopped)
	}

	r.container.UIHost.DisableExtension(ui_contribution.ExtensionID(extensionID))
	r.container.PageHost.HandleExtensionDisabled(ctx, extension_page_host.ExtensionID(extensionID))

	if r.container.ContributionInstaller != nil {
		r.container.ContributionInstaller.DeactivateContributions(ctx, extID)
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

	snap := r.container.RuntimeSupervisor.Snapshot(ctx, runtime_supervisor.DefinitionID(extensionID))
	for _, instance := range snap.Instances {
		_ = r.container.RuntimeSupervisor.Stop(ctx, instance.InstanceID, runtime_supervisor.StopReasonUninstall)
	}

	contribs := r.container.ContributionRegistry.ListByExtension(extensionID)
	for _, c := range contribs {
		_ = r.container.ContributionRegistry.Unregister(c.ContributionID())
	}

	if r.container.ContributionInstaller != nil {
		r.container.ContributionInstaller.UninstallContributions(ctx, extID)
	}

	_ = r.container.ContributionRepository.DeleteContributions(ctx, extID)
	for _, uiDef := range r.container.UIHost.ListAll() {
		if string(uiDef.ExtensionID) == extensionID {
			_ = r.container.UIHost.UnregisterContribution(uiDef.ContributionID)
		}
	}
	_, _ = r.container.PageHost.HandleExtensionUninstalled(ctx, extension_page_host.ExtensionID(extensionID))
	if r.container.UIContributionRepo != nil {
		_ = r.container.UIContributionRepo.DeleteByExtension(ctx, extensionID)
	}
	_ = r.container.ModuleRepository.DeleteModules(ctx, extID)
	_ = r.container.InstallationRepository.DeleteInstallation(ctx, extID)
	if version != "" {
		if parsed, pErr := domain.ParseVersion(version); pErr == nil {
			_ = r.container.DefinitionRepository.DeleteExtension(ctx, extID, parsed)
		}
	}

	safeID := safeDirectoryName(extensionID)
	if version != "" {
		installDir := filepath.Join(r.root, "installed", safeID, version)
		_ = os.RemoveAll(installDir)
	} else {
		installDir := filepath.Join(r.root, "installed", safeID)
		_ = os.RemoveAll(installDir)
	}

	extSubject := enablement.StateSubject{Kind: enablement.SubjectExtension, ID: extensionID}
	_ = r.container.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled)
	_ = r.container.EnablementStore.SetDesiredRuntime(ctx, extSubject, enablement.DesiredRuntimeStopped)

	r.mu.Lock()
	delete(r.installed, extensionID)
	r.mu.Unlock()

	return nil
}
