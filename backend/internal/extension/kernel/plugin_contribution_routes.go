package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

type installerContributionRegistration struct {
	id           string
	adapter      domain.ContributionAdapter
	contribution domain.ContributionDefinition
	operation    installOp
}

func (r *installerContributionRegistration) ID() string {
	return r.id
}

type installerContributionHandler struct {
	installer *TypedContributionInstaller
	adapter   domain.ContributionAdapter
}

func (h *installerContributionHandler) Build(ctx context.Context, module plugincontract.ModuleRef, descriptor plugincontract.ContributionDescriptor) (plugincontract.ContributionRegistration, error) {
	if h == nil || h.installer == nil {
		return nil, fmt.Errorf("plugin contribution installer is unavailable")
	}
	contribution := domain.ContributionDefinition{
		ID:                  domain.ContributionID(descriptor.ID),
		ExtensionID:         domain.ExtensionID(module.PluginID),
		ModuleID:            domain.ModuleID(module.ModuleID),
		Kind:                domain.ContributionKind(descriptor.Kind),
		Adapter:             h.adapter,
		ContractVersion:     descriptor.ContractVersion,
		Schema:              append([]byte(nil), descriptor.Schema...),
		Name:                domain.LocalizedText{Default: descriptor.Name.Default, Translations: cloneStringStringMap(descriptor.Name.Translations)},
		Description:         domain.LocalizedText{Default: descriptor.Description.Default, Translations: cloneStringStringMap(descriptor.Description.Translations)},
		Version:             descriptor.Version,
		Definition:          cloneAnyMap(descriptor.Spec),
		RequiredPermissions: append([]string(nil), descriptor.RequiredPermissions...),
		RequiredScope:       append([]string(nil), descriptor.RequiredScope...),
		Metadata:            cloneAnyMap(descriptor.Metadata),
	}
	if descriptor.Exposure != nil {
		contribution.Exposure = domain.Exposure{
			VisibleByDefault:    descriptor.Exposure.VisibleByDefault,
			HiddenFromDiscovery: descriptor.Exposure.HiddenFromDiscovery,
			RequiredRoles:       append([]string(nil), descriptor.Exposure.RequiredRoles...),
		}
	}
	if descriptor.RuntimeBinding != nil {
		contribution.RuntimeBinding = &domain.RuntimeBinding{
			RuntimeID:  domain.RuntimeID(descriptor.RuntimeBinding.RuntimeID),
			Generation: descriptor.RuntimeBinding.Generation,
		}
	}
	for _, dependency := range descriptor.Dependencies {
		contribution.Dependencies = append(contribution.Dependencies, domain.DependencyDefinition{
			Type:     domain.DependencyType(dependency.Type),
			ID:       dependency.ID,
			Version:  dependency.Version,
			Optional: dependency.Optional,
			Reason:   dependency.Reason,
		})
	}
	operation, err := h.installer.buildInstallOp(ctx, contribution, 0)
	if err != nil {
		return nil, err
	}
	return &installerContributionRegistration{
		id:           descriptor.ID,
		adapter:      h.adapter,
		contribution: contribution,
		operation:    operation,
	}, nil
}

func (h *installerContributionHandler) Activate(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	r, err := installerRegistration(registration)
	if err != nil {
		return err
	}
	if r.operation.doInstall == nil {
		return nil
	}
	return r.operation.doInstall(ctx)
}

func (h *installerContributionHandler) Deactivate(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	r, err := installerRegistration(registration)
	if err != nil {
		return err
	}
	handler, ok := h.installer.adapterHandler(r.adapter)
	if !ok {
		return fmt.Errorf("unsupported contribution adapter %s", r.adapter)
	}
	if handler.deactivate == nil {
		return nil
	}
	return handler.deactivate(ctx, r.contribution)
}

func (h *installerContributionHandler) Discard(ctx context.Context, registration plugincontract.ContributionRegistration) error {
	r, err := installerRegistration(registration)
	if err != nil {
		return err
	}
	if r.operation.doRollback == nil {
		return nil
	}
	r.operation.doRollback(ctx)
	return nil
}

func installerRegistration(value plugincontract.ContributionRegistration) (*installerContributionRegistration, error) {
	registration, ok := value.(*installerContributionRegistration)
	if !ok || registration == nil {
		return nil, fmt.Errorf("unexpected contribution registration type %T", value)
	}
	return registration, nil
}

func registerBuiltinContributionRoutes(container *Container, installer *TypedContributionInstaller) error {
	if container == nil || container.PluginAdapters == nil || container.PluginRoutes == nil || installer == nil {
		return fmt.Errorf("plugin contribution routing is not configured")
	}
	for _, descriptor := range container.PluginAdapters.Snapshot().Adapters {
		if descriptor.External {
			continue
		}
		err := container.PluginRoutes.Register(
			descriptor.ID,
			descriptor.ContractVersion,
			&installerContributionHandler{installer: installer, adapter: domain.ContributionAdapter(descriptor.ID)},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func cloneStringStringMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func cloneAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
