package kernel

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

type installerContributionRegistration struct {
	id           string
	adapter      domain.ContributionAdapter
	contribution domain.ContributionDefinition
	operation    installOp
	providerID   capability.ProviderID
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
	if r.operation.doInstall != nil {
		if err := r.operation.doInstall(ctx); err != nil {
			return err
		}
	}
	if r.adapter == domain.ContributionAdapterTool {
		if err := h.installer.activateTool(ctx, r.contribution); err != nil {
			return err
		}
		return h.registerToolProvider(ctx, r)
	}
	return nil
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
	var providerErr error
	if r.providerID != "" {
		c := h.installer.container
		if c.ProviderLifecycle != nil {
			_, providerErr = c.ProviderLifecycle.UnregisterProvider(r.providerID)
		} else {
			_, providerErr = c.CapabilityProviders.DeregisterDefinitionCascade(r.providerID)
		}
	}
	if r.operation.doRollback != nil {
		r.operation.doRollback(ctx)
	}
	return providerErr
}

func (h *installerContributionHandler) registerToolProvider(ctx context.Context, r *installerContributionRegistration) error {
	c := h.installer.container
	if c.CapabilityProviders == nil {
		return nil
	}
	toolID, _ := r.contribution.Definition["toolId"].(string)
	if toolID == "" {
		toolID = r.id
	}
	tool, ok := c.ToolRegistry.Get(ctx, toolID)
	if !ok || (tool.Runtime.RuntimeType != capability.RuntimeTypeJavaScript && tool.Runtime.RuntimeType != capability.RuntimeTypePluginJS) {
		return nil
	}
	placement := capability.ProviderPlacementCore
	modulePlacement, _ := tool.Runtime.Metadata["modulePlacement"].(string)
	if modulePlacement == string(domain.ModulePlacementDevice) || (modulePlacement == "" && tool.Runtime.RuntimeType == capability.RuntimeTypePluginJS) {
		placement = capability.ProviderPlacementDevice
	}
	id := capability.BuildManifestProviderID(r.contribution.ExtensionID, r.contribution.ModuleID, &domain.ProviderMetadata{ID: "dynamic:" + r.id}, tool.CapabilityID)
	def := capability.CapabilityProviderDefinition{
		ID: id, CapabilityID: tool.CapabilityID, Kind: capability.ProviderKindExtension, Placement: placement,
		ExtensionID: tool.ExtensionID, ModuleID: tool.ModuleID, Runtime: tool.Runtime,
	}
	instance := capability.CapabilityProviderInstance{
		ID: capability.ProviderInstanceID(id), ProviderID: id, CapabilityID: tool.CapabilityID, Placement: placement,
		ExtensionID: tool.ExtensionID, ModuleID: tool.ModuleID, Health: capability.HealthReady, Availability: capability.ProviderAvailabilityAvailable,
	}
	if placement == capability.ProviderPlacementDevice {
		instance.SpaceID, instance.DeviceID, instance.RuntimeID = c.RuntimeIdentity.SpaceID, c.RuntimeIdentity.DeviceID, c.RuntimeIdentity.RuntimeID
	}
	if err := instance.Validate(); err != nil {
		return err
	}
	if c.ProviderLifecycle != nil {
		if err := c.ProviderLifecycle.RegisterProvider(def); err != nil {
			return err
		}
		r.providerID = id
		if err := c.ProviderLifecycle.RegisterInstance(instance); err != nil {
			_, cleanupErr := c.ProviderLifecycle.UnregisterProvider(id)
			return errors.Join(err, cleanupErr)
		}
	} else {
		if err := c.CapabilityProviders.RegisterDefinition(def); err != nil {
			return err
		}
		r.providerID = id
		if err := c.CapabilityProviders.RegisterInstance(instance); err != nil {
			_, cleanupErr := c.CapabilityProviders.DeregisterDefinitionCascade(id)
			return errors.Join(err, cleanupErr)
		}
	}
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
