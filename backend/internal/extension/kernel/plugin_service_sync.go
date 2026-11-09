package kernel

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func contractCatalogModules(contract *plugincontract.Manifest, enabled bool) []plugincontract.CatalogModule {
	if contract == nil {
		return nil
	}
	modules := make([]plugincontract.CatalogModule, 0, len(contract.Modules))
	for _, module := range contract.Modules {
		modules = append(modules, plugincontract.CatalogModule{
			Module:   plugincontract.ModuleRef{PluginID: contract.Plugin.ID, ModuleID: module.ID},
			Services: module.Services,
			Enabled:  enabled,
		})
	}
	return modules
}

func setPluginContractServices(catalog *plugincontract.ServiceCatalog, contract *plugincontract.Manifest, enabled bool) error {
	if catalog == nil || contract == nil {
		return nil
	}
	modules := contractCatalogModules(contract, enabled)
	staged := catalog.Clone()
	for _, module := range modules {
		if err := staged.Replace(module); err != nil {
			return err
		}
	}
	if resolution := staged.Resolve(); !resolution.Valid() {
		return fmt.Errorf("plugin service graph is invalid: %+v", resolution.Issues)
	}
	for _, module := range modules {
		if err := catalog.Replace(module); err != nil {
			return err
		}
	}
	return nil
}

func removePluginContractServices(catalog *plugincontract.ServiceCatalog, contract *plugincontract.Manifest) error {
	if catalog == nil || contract == nil {
		return nil
	}
	modules := contractCatalogModules(contract, false)
	staged := catalog.Clone()
	for _, module := range modules {
		staged.Unregister(module.Module)
	}
	if resolution := staged.Resolve(); !resolution.Valid() {
		return fmt.Errorf("plugin service graph is invalid: %+v", resolution.Issues)
	}
	for _, module := range modules {
		catalog.Unregister(module.Module)
	}
	return nil
}

func (r *Runtime) validateInstalledPluginServiceChange(extensionID domain.ExtensionID, enabled bool) error {
	if r == nil || r.container == nil || r.container.PluginServices == nil {
		return nil
	}
	contract, err := r.installedPluginContract(extensionID)
	if err != nil {
		if !r.hasLoadedPluginContract(extensionID) {
			return nil
		}
		return fmt.Errorf("load plugin services for %s: %w", extensionID, err)
	}
	return setPluginContractServices(r.container.PluginServices.Clone(), contract, enabled)
}

func (r *Runtime) validateInstalledPluginServiceRemoval(extensionID domain.ExtensionID) error {
	if r == nil || r.container == nil || r.container.PluginServices == nil {
		return nil
	}
	contract, err := r.installedPluginContract(extensionID)
	if err != nil {
		if !r.hasLoadedPluginContract(extensionID) {
			return nil
		}
		return fmt.Errorf("load plugin services for %s: %w", extensionID, err)
	}
	return removePluginContractServices(r.container.PluginServices.Clone(), contract)
}

func (r *Runtime) hasLoadedPluginContract(extensionID domain.ExtensionID) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.installed[string(extensionID)]
	return exists
}

func appendPackageServiceGraphIssues(catalog *plugincontract.ServiceCatalog, pkg *amitiax.Package, preview *InstallPreview) {
	if catalog == nil || pkg == nil || pkg.Manifest.Plugin.ID == "" || preview == nil {
		return
	}
	staged := plugincontract.NewServiceCatalog()
	for _, module := range catalog.Snapshot() {
		if err := staged.Replace(module); err != nil {
			preview.Issues = append(preview.Issues, PreviewIssue{
				Category: PreviewNotInstallable,
				Code:     "service_catalog_invalid",
				Message:  err.Error(),
			})
			return
		}
	}
	for _, module := range contractCatalogModules(&pkg.Manifest, true) {
		if err := staged.Replace(module); err != nil {
			preview.Issues = append(preview.Issues, PreviewIssue{
				Category: PreviewNotInstallable,
				Code:     "service_provider_conflict",
				Message:  err.Error(),
			})
			return
		}
	}
	resolution := staged.Resolve()
	for _, issue := range resolution.Issues {
		category := PreviewNotInstallable
		if issue.Code == "required_service_unavailable" {
			category = PreviewMissingDependency
		}
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: category,
			Code:     issue.Code,
			Message:  fmt.Sprintf("%s %s: %s", issue.Module, issue.Service, issue.Detail),
		})
	}
}

func (r *Runtime) installedPluginContract(extensionID domain.ExtensionID) (*plugincontract.Manifest, error) {
	if r == nil {
		return nil, fmt.Errorf("runtime is unavailable")
	}
	r.mu.RLock()
	item, exists := r.installed[string(extensionID)]
	r.mu.RUnlock()
	if exists && strings.TrimSpace(item.Path) != "" {
		raw, err := os.ReadFile(filepath.Join(item.Path, amitiax.ManifestFile))
		if err == nil {
			contract, parseErr := plugincontract.Parse(raw)
			if parseErr == nil {
				report := contract.Validate()
				if report.HasErrors() {
					return nil, fmt.Errorf("installed extension %s has invalid manifest: %+v", extensionID, report.Issues)
				}
				return contract, nil
			}
		}
	}
	if r.container == nil || r.container.InstallationRepository == nil || r.container.PackageRepository == nil {
		return nil, fmt.Errorf("installed extension %s is unavailable", extensionID)
	}
	installation, err := r.container.InstallationRepository.GetInstallation(context.Background(), extensionID)
	if err != nil {
		return nil, err
	}
	artifact, err := r.container.PackageRepository.GetArtifact(context.Background(), installation.PackageID)
	if err != nil {
		return nil, err
	}
	pkg, err := amitiax.OpenArchive(artifact.ArchivePath)
	if err != nil {
		return nil, err
	}
	if pkg.Manifest.Plugin.ID == "" {
		return nil, fmt.Errorf("installed extension %s has no unified manifest", extensionID)
	}
	return &pkg.Manifest, nil
}

func (r *Runtime) setInstalledPluginServices(extensionID domain.ExtensionID, enabled bool) error {
	if r == nil || r.container == nil || r.container.PluginServices == nil {
		return nil
	}
	contract, err := r.installedPluginContract(extensionID)
	if err != nil {
		if !r.hasLoadedPluginContract(extensionID) {
			return nil
		}
		return fmt.Errorf("load plugin services for %s: %w", extensionID, err)
	}
	return setPluginContractServices(r.container.PluginServices, contract, enabled)
}

func (r *Runtime) removeInstalledPluginServices(extensionID domain.ExtensionID) error {
	if r == nil || r.container == nil || r.container.PluginServices == nil {
		return nil
	}
	contract, err := r.installedPluginContract(extensionID)
	if err != nil {
		if !r.hasLoadedPluginContract(extensionID) {
			return nil
		}
		return fmt.Errorf("load plugin services for %s: %w", extensionID, err)
	}
	return removePluginContractServices(r.container.PluginServices, contract)
}

func (r *Runtime) removeInstalledAdapterProviders(extensionID domain.ExtensionID) error {
	if r == nil || r.container == nil || r.container.AdapterProviderHandler == nil {
		return nil
	}
	contract, err := r.installedPluginContract(extensionID)
	if err != nil {
		if !r.hasLoadedPluginContract(extensionID) {
			return nil
		}
		return fmt.Errorf("load adapter providers for %s: %w", extensionID, err)
	}
	unregisterPackageAdapterProviders(r.container.AdapterProviderHandler, contract)
	return nil
}

func (r *Runtime) rebuildPluginServiceCatalog() error {
	if r == nil || r.container == nil || r.container.PluginServices == nil {
		return nil
	}
	r.mu.RLock()
	items := make([]InstalledExtension, 0, len(r.installed))
	for _, item := range r.installed {
		items = append(items, item)
	}
	r.mu.RUnlock()
	for _, item := range items {
		enabled := false
		if r.container.InstallationRepository != nil {
			installation, err := r.container.InstallationRepository.GetInstallation(context.Background(), domain.ExtensionID(item.ID))
			if err == nil {
				enabled = installation.EnablementState == domain.EnablementEnabled
			}
		}
		if err := r.setInstalledPluginServices(domain.ExtensionID(item.ID), enabled); err != nil {
			log.Printf("[plugin-services] restore %s: %v", item.ID, err)
		}
		contract, err := r.installedPluginContract(domain.ExtensionID(item.ID))
		if err != nil {
			continue
		}
		trust := plugincontract.TrustUserTrusted
		if r.container.InstallationRepository != nil && r.container.PackageRepository != nil {
			installation, installationErr := r.container.InstallationRepository.GetInstallation(context.Background(), domain.ExtensionID(item.ID))
			if installationErr == nil {
				artifact, artifactErr := r.container.PackageRepository.GetArtifact(context.Background(), installation.PackageID)
				if artifactErr == nil && artifact.SignatureStatus == "valid" {
					trust = plugincontract.TrustVerified
				}
			}
		}
		if err := registerPackageAdapterProviders(r.container.AdapterProviderHandler, contract, trust); err != nil {
			log.Printf("[adapter-providers] restore %s: %v", item.ID, err)
		}
	}
	return nil
}
