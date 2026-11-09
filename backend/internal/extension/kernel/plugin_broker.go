package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

type dynamicContributionRecord struct {
	descriptor    plugincontract.ContributionDescriptor
	registration  plugincontract.ContributionRegistration
	unregistering bool
}

type PluginBroker struct {
	container *Container

	mu                sync.Mutex
	validateManifest  bool
	contributions     map[string]map[string]dynamicContributionRecord
	pending           map[string]map[string]plugincontract.ContributionDescriptor
	registrationOrder []string
	offers            map[string][]plugincontract.ServiceOffer
	requirements      map[string][]plugincontract.ServiceRequirement
	servicesLoaded    map[string]bool
	serviceMu         sync.Mutex
}

func NewPluginBroker(container *Container) *PluginBroker {
	return &PluginBroker{
		container:        container,
		validateManifest: true,
		contributions:    make(map[string]map[string]dynamicContributionRecord),
		pending:          make(map[string]map[string]plugincontract.ContributionDescriptor),
		offers:           make(map[string][]plugincontract.ServiceOffer),
		requirements:     make(map[string][]plugincontract.ServiceRequirement),
		servicesLoaded:   make(map[string]bool),
	}
}

func (b *PluginBroker) RegisterContribution(ctx context.Context, module plugincontract.ModuleRef, descriptor plugincontract.ContributionDescriptor) error {
	if b == nil || b.container == nil || b.container.PluginRoutes == nil {
		return fmt.Errorf("plugin contribution router is unavailable")
	}
	if b.validateManifest {
		if err := b.reserveDynamicContribution(module, descriptor); err != nil {
			return err
		}
	}
	key := module.Key()
	registration, err := b.container.PluginRoutes.BuildAndActivate(ctx, module, descriptor)
	if b.validateManifest {
		b.releaseContributionReservation(key, descriptor.ID)
	}
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.contributions[key] == nil {
		b.contributions[key] = make(map[string]dynamicContributionRecord)
	}
	b.contributions[key][descriptor.ID] = dynamicContributionRecord{descriptor: descriptor, registration: registration}
	b.registrationOrder = append(b.registrationOrder, dynamicOrderKey(key, descriptor.ID))
	b.mu.Unlock()
	return nil
}

func (b *PluginBroker) UnregisterContribution(ctx context.Context, module plugincontract.ModuleRef, id string) error {
	return b.releaseContribution(ctx, module, id, false)
}

func (b *PluginBroker) releaseContribution(ctx context.Context, module plugincontract.ModuleRef, id string, force bool) error {
	if b == nil || b.container == nil || b.container.PluginRoutes == nil {
		return fmt.Errorf("plugin contribution router is unavailable")
	}
	key := module.Key()
	b.mu.Lock()
	_, exists := b.contributions[key][id]
	b.mu.Unlock()
	if !exists {
		return nil
	}
	if !force && b.validateManifest {
		_, declared, err := b.moduleContract(module)
		if err != nil {
			return err
		}
		if !declared.Dynamic.AllowUnregister {
			return fmt.Errorf("module %s does not allow dynamic contribution unregister", module.Key())
		}
	}
	b.mu.Lock()
	record, exists := b.contributions[key][id]
	if !exists {
		b.mu.Unlock()
		return nil
	}
	if record.unregistering {
		b.mu.Unlock()
		return fmt.Errorf("plugin contribution %s is already being unregistered", id)
	}
	record.unregistering = true
	b.contributions[key][id] = record
	b.mu.Unlock()
	err := b.container.PluginRoutes.DeactivateAndDiscard(ctx, module, record.descriptor, record.registration)
	b.mu.Lock()
	if err != nil {
		if current, stillExists := b.contributions[key][id]; stillExists {
			current.unregistering = false
			b.contributions[key][id] = current
		}
		b.mu.Unlock()
		return err
	}
	delete(b.contributions[key], id)
	if len(b.contributions[key]) == 0 {
		delete(b.contributions, key)
	}
	b.removeRegistrationOrder(dynamicOrderKey(key, id))
	b.mu.Unlock()
	return nil
}

func dynamicOrderKey(moduleKey, contributionID string) string {
	return moduleKey + "\x00" + contributionID
}

func splitDynamicOrderKey(value string) (string, string, bool) {
	moduleKey, contributionID, ok := strings.Cut(value, "\x00")
	return moduleKey, contributionID, ok && moduleKey != "" && contributionID != ""
}

func (b *PluginBroker) removeRegistrationOrder(key string) {
	for index := len(b.registrationOrder) - 1; index >= 0; index-- {
		if b.registrationOrder[index] == key {
			b.registrationOrder = append(b.registrationOrder[:index], b.registrationOrder[index+1:]...)
			return
		}
	}
}

func (b *PluginBroker) ReleaseExtension(ctx context.Context, extensionID string) error {
	if b == nil || b.container == nil {
		return nil
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return fmt.Errorf("extension id is required")
	}
	defer func() {
		b.mu.Lock()
		for key := range b.offers {
			if strings.HasPrefix(key, extensionID+"/") {
				delete(b.offers, key)
			}
		}
		for key := range b.requirements {
			if strings.HasPrefix(key, extensionID+"/") {
				delete(b.requirements, key)
			}
		}
		for key := range b.servicesLoaded {
			if strings.HasPrefix(key, extensionID+"/") {
				delete(b.servicesLoaded, key)
			}
		}
		for key := range b.pending {
			if strings.HasPrefix(key, extensionID+"/") {
				delete(b.pending, key)
			}
		}
		b.mu.Unlock()
	}()
	b.mu.Lock()
	order := append([]string(nil), b.registrationOrder...)
	b.mu.Unlock()

	var releaseErrs []error
	for index := len(order) - 1; index >= 0; index-- {
		moduleKey, contributionID, ok := splitDynamicOrderKey(order[index])
		if !ok || !strings.HasPrefix(moduleKey, extensionID+"/") {
			continue
		}
		moduleID := strings.TrimPrefix(moduleKey, extensionID+"/")
		module := plugincontract.ModuleRef{PluginID: extensionID, ModuleID: moduleID}
		if err := b.releaseContribution(ctx, module, contributionID, true); err != nil {
			releaseErrs = append(releaseErrs, fmt.Errorf("release dynamic contribution %s: %w", contributionID, err))
		}
	}

	if b.container.PluginServices != nil {
		b.serviceMu.Lock()
		modules := make([]plugincontract.ModuleRef, 0)
		for _, module := range b.container.PluginServices.Snapshot() {
			if module.Module.PluginID == extensionID {
				modules = append(modules, module.Module)
			}
		}
		if len(modules) > 0 {
			staged := b.container.PluginServices.Clone()
			for _, module := range modules {
				staged.Unregister(module)
			}
			if resolution := staged.Resolve(); !resolution.Valid() {
				releaseErrs = append(releaseErrs, fmt.Errorf("release plugin services: %+v", resolution.Issues))
			} else {
				for _, module := range modules {
					b.container.PluginServices.Unregister(module)
				}
			}
		}
		b.serviceMu.Unlock()
	}
	return errors.Join(releaseErrs...)
}

func (b *PluginBroker) RuntimeCrashed(ctx context.Context, extensionID, moduleID string, generation int64, reason string) error {
	if err := b.ReleaseExtension(ctx, extensionID); err != nil {
		return fmt.Errorf("runtime crash cleanup for %s/%s generation %d: %w", extensionID, moduleID, generation, err)
	}
	return nil
}

func (b *PluginBroker) ProvideService(module plugincontract.ModuleRef, offer plugincontract.ServiceOffer) error {
	if b == nil || b.container == nil || b.container.PluginServices == nil {
		return fmt.Errorf("plugin service catalog is unavailable")
	}
	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	if b.validateManifest {
		if err := b.validateDynamicServiceOffer(module, offer); err != nil {
			return err
		}
	}
	key := module.Key()
	offers, requirements, err := b.serviceState(module)
	if err != nil {
		return err
	}
	replaced := false
	for index := range offers {
		if offers[index].ServiceID == offer.ServiceID {
			offers[index] = offer
			replaced = true
			break
		}
	}
	if !replaced {
		offers = append(offers, offer)
	}
	if err := b.commitServiceState(module, offers, requirements); err != nil {
		return err
	}
	b.mu.Lock()
	b.offers[key] = offers
	b.mu.Unlock()
	return nil
}

func (b *PluginBroker) commitServiceState(
	module plugincontract.ModuleRef,
	offers []plugincontract.ServiceOffer,
	requirements []plugincontract.ServiceRequirement,
) error {
	staged := b.container.PluginServices.Clone()
	if err := staged.Replace(plugincontract.CatalogModule{
		Module: module,
		Services: plugincontract.ServiceDeclaration{
			Provides: offers,
			Requires: requirements,
		},
		Enabled: true,
	}); err != nil {
		return err
	}
	if resolution := staged.Resolve(); !resolution.Valid() {
		return fmt.Errorf("plugin service graph is invalid: %+v", resolution.Issues)
	}
	return b.container.PluginServices.Replace(plugincontract.CatalogModule{
		Module: module,
		Services: plugincontract.ServiceDeclaration{
			Provides: offers,
			Requires: requirements,
		},
		Enabled: true,
	})
}

func (b *PluginBroker) UnprovideService(module plugincontract.ModuleRef, serviceID string) error {
	if b == nil || b.container == nil || b.container.PluginServices == nil {
		return fmt.Errorf("plugin service catalog is unavailable")
	}
	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	key := module.Key()
	offers, requirements, err := b.serviceState(module)
	if err != nil {
		return err
	}
	filtered := offers[:0]
	for _, offer := range offers {
		if offer.ServiceID != serviceID {
			filtered = append(filtered, offer)
		}
	}
	offers = append([]plugincontract.ServiceOffer(nil), filtered...)
	if err := b.commitServiceState(module, offers, requirements); err != nil {
		return err
	}
	b.mu.Lock()
	b.offers[key] = offers
	b.mu.Unlock()
	return nil
}

func (b *PluginBroker) RequireService(module plugincontract.ModuleRef, requirement plugincontract.ServiceRequirement) ([]string, error) {
	if b == nil || b.container == nil || b.container.PluginServices == nil {
		return nil, fmt.Errorf("plugin service catalog is unavailable")
	}
	if b.validateManifest {
		if err := b.validateDynamicServiceRequirement(module, requirement); err != nil {
			return nil, err
		}
	}
	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	key := module.Key()
	offers, requirements, err := b.serviceState(module)
	if err != nil {
		return nil, err
	}
	replaced := false
	for index := range requirements {
		if requirements[index].ServiceID == requirement.ServiceID {
			requirements[index] = requirement
			replaced = true
			break
		}
	}
	if !replaced {
		requirements = append(requirements, requirement)
	}
	if err := b.commitServiceState(module, offers, requirements); err != nil {
		return nil, err
	}
	resolution := b.container.PluginServices.Clone().Resolve()
	if !resolution.Valid() {
		return nil, fmt.Errorf("plugin service graph is invalid: %+v", resolution.Issues)
	}
	b.mu.Lock()
	b.requirements[key] = requirements
	b.mu.Unlock()
	for _, binding := range resolution.Graph.Bindings[key] {
		if binding.ServiceID != requirement.ServiceID {
			continue
		}
		providerIDs := make([]string, 0, len(binding.Providers))
		for _, provider := range binding.Providers {
			providerIDs = append(providerIDs, provider.ProviderID)
		}
		return providerIDs, nil
	}
	if requirement.Optional {
		return nil, nil
	}
	return nil, fmt.Errorf("required service %s is unavailable", requirement.ServiceID)
}

func (b *PluginBroker) moduleContract(module plugincontract.ModuleRef) (*plugincontract.Manifest, *plugincontract.Module, error) {
	if b == nil || b.container == nil || b.container.InstallationRepository == nil || b.container.PackageRepository == nil {
		return nil, nil, fmt.Errorf("plugin package repository is unavailable")
	}
	extensionID := domain.ExtensionID(module.PluginID)
	installation, err := b.container.InstallationRepository.GetInstallation(context.Background(), extensionID)
	if err != nil {
		return nil, nil, err
	}
	artifact, err := b.container.PackageRepository.GetArtifact(context.Background(), installation.PackageID)
	if err != nil {
		return nil, nil, err
	}
	pkg, err := amitiax.OpenArchive(artifact.ArchivePath)
	if err != nil {
		return nil, nil, err
	}
	if pkg.Manifest.Plugin.ID == "" {
		return nil, nil, fmt.Errorf("plugin %s has no unified manifest", module.PluginID)
	}
	for index := range pkg.Manifest.Modules {
		if pkg.Manifest.Modules[index].ID == module.ModuleID {
			return &pkg.Manifest, &pkg.Manifest.Modules[index], nil
		}
	}
	return nil, nil, fmt.Errorf("plugin module %s is not declared", module.Key())
}

func (b *PluginBroker) reserveDynamicContribution(module plugincontract.ModuleRef, descriptor plugincontract.ContributionDescriptor) error {
	_, declared, err := b.moduleContract(module)
	if err != nil {
		return err
	}
	if declared.Dynamic.MaxContributions == 0 {
		return fmt.Errorf("module %s does not allow dynamic contributions", module.Key())
	}
	if len(declared.Dynamic.AllowedAdapters) > 0 && !containsString(declared.Dynamic.AllowedAdapters, descriptor.Adapter) {
		return fmt.Errorf("module %s does not allow adapter %s", module.Key(), descriptor.Adapter)
	}
	if len(declared.Dynamic.AllowedKinds) > 0 && !containsString(declared.Dynamic.AllowedKinds, descriptor.Kind) {
		return fmt.Errorf("module %s does not allow kind %s", module.Key(), descriptor.Kind)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = make(map[string]map[string]plugincontract.ContributionDescriptor)
	}
	if b.pending[module.Key()] == nil {
		b.pending[module.Key()] = make(map[string]plugincontract.ContributionDescriptor)
	}
	if existing := b.contributions[module.Key()]; existing != nil {
		if _, exists := existing[descriptor.ID]; exists {
			return fmt.Errorf("plugin contribution %s is already registered", descriptor.ID)
		}
	}
	if _, exists := b.pending[module.Key()][descriptor.ID]; exists {
		return fmt.Errorf("plugin contribution %s is already being registered", descriptor.ID)
	}
	count := len(b.contributions[module.Key()]) + len(b.pending[module.Key()])
	toolCount := 0
	routeCount := 0
	for _, record := range b.contributions[module.Key()] {
		if record.descriptor.Adapter == "tool" {
			toolCount++
		}
		routeCount += dynamicRouteCount(record.descriptor)
	}
	for _, descriptor := range b.pending[module.Key()] {
		if descriptor.Adapter == "tool" {
			toolCount++
		}
		routeCount += dynamicRouteCount(descriptor)
	}
	if err := dynamicContributionLimitError(module.Key(), declared.Dynamic, count, toolCount, routeCount, descriptor); err != nil {
		return err
	}
	b.pending[module.Key()][descriptor.ID] = descriptor
	return nil
}

func dynamicContributionLimitError(moduleKey string, dynamic plugincontract.DynamicCapability, total, toolCount, routeCount int, descriptor plugincontract.ContributionDescriptor) error {
	if total >= dynamic.MaxContributions {
		return fmt.Errorf("module %s reached dynamic contribution limit", moduleKey)
	}
	if descriptor.Adapter == "tool" && toolCount >= dynamic.MaxTools {
		return fmt.Errorf("module %s reached dynamic tool limit", moduleKey)
	}
	if routeCount+dynamicRouteCount(descriptor) > dynamic.MaxRoutes {
		return fmt.Errorf("module %s reached dynamic route limit", moduleKey)
	}
	return nil
}

func dynamicRouteCount(descriptor plugincontract.ContributionDescriptor) int {
	if descriptor.Adapter != "ui_provider" && descriptor.Kind != "ui_provider" && descriptor.Kind != "route.registry" {
		return 0
	}
	candidates := []any{
		descriptor.Spec["routes"],
		descriptor.Metadata["routes"],
	}
	if metadata, ok := descriptor.Spec["metadata"].(map[string]any); ok {
		candidates = append(candidates, metadata["routes"])
	}
	maxRoutes := 0
	for _, candidate := range candidates {
		if routes, ok := candidate.([]any); ok && len(routes) > maxRoutes {
			maxRoutes = len(routes)
		}
	}
	return maxRoutes
}

func (b *PluginBroker) releaseContributionReservation(key, id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.pending[key], id)
	if len(b.pending[key]) == 0 {
		delete(b.pending, key)
	}
}

func (b *PluginBroker) validateDynamicServiceOffer(module plugincontract.ModuleRef, offer plugincontract.ServiceOffer) error {
	_, declared, err := b.moduleContract(module)
	if err != nil {
		return err
	}
	if declared.Dynamic.MaxServices == 0 {
		return fmt.Errorf("module %s does not allow dynamic services", module.Key())
	}
	for _, declaredOffer := range declared.Services.Provides {
		if declaredOffer.ServiceID != offer.ServiceID {
			continue
		}
		if declaredOffer.Version != offer.Version {
			return fmt.Errorf("service %s version does not match manifest", offer.ServiceID)
		}
		return nil
	}
	return fmt.Errorf("service %s is not declared by module %s", offer.ServiceID, module.Key())
}

func (b *PluginBroker) validateDynamicServiceRequirement(module plugincontract.ModuleRef, requirement plugincontract.ServiceRequirement) error {
	_, declared, err := b.moduleContract(module)
	if err != nil {
		return err
	}
	for _, declaredRequirement := range declared.Services.Requires {
		if declaredRequirement.ServiceID == requirement.ServiceID {
			return nil
		}
	}
	return fmt.Errorf("service dependency %s is not declared by module %s", requirement.ServiceID, module.Key())
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (b *PluginBroker) ReleaseService(module plugincontract.ModuleRef, serviceID string) error {
	if b == nil || b.container == nil || b.container.PluginServices == nil {
		return fmt.Errorf("plugin service catalog is unavailable")
	}
	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	key := module.Key()
	offers, requirements, err := b.serviceState(module)
	if err != nil {
		return err
	}
	filtered := requirements[:0]
	for _, requirement := range requirements {
		if requirement.ServiceID != serviceID {
			filtered = append(filtered, requirement)
		}
	}
	requirements = append([]plugincontract.ServiceRequirement(nil), filtered...)
	if err := b.commitServiceState(module, offers, requirements); err != nil {
		return err
	}
	b.mu.Lock()
	b.requirements[key] = requirements
	b.mu.Unlock()
	return nil
}

func (b *PluginBroker) InvokeAdapter(
	ctx context.Context,
	module plugincontract.ModuleRef,
	adapterID string,
	contractVersion int,
	request plugincontract.AdapterInvocationRequest,
) (plugincontract.AdapterInvocationResult, error) {
	if b == nil || b.container == nil || b.container.PluginRoutes == nil || b.container.PluginAdapters == nil {
		return plugincontract.AdapterInvocationResult{}, fmt.Errorf("plugin adapter broker is unavailable")
	}
	_, declared, err := b.moduleContract(module)
	if err != nil {
		return plugincontract.AdapterInvocationResult{}, err
	}
	if !containsString(declared.Dynamic.AllowedAdapters, adapterID) {
		return plugincontract.AdapterInvocationResult{}, fmt.Errorf("module %s does not allow adapter %s", module.Key(), adapterID)
	}
	descriptor, ok := b.container.PluginAdapters.Get(adapterID, contractVersion)
	if !ok {
		return plugincontract.AdapterInvocationResult{}, fmt.Errorf("adapter %s v%d is not registered", adapterID, contractVersion)
	}
	if err := b.validateAdapterPermissions(declared, descriptor); err != nil {
		return plugincontract.AdapterInvocationResult{}, err
	}
	if len(descriptor.Operations) > 0 && !containsString(descriptor.Operations, request.Operation) {
		return plugincontract.AdapterInvocationResult{}, fmt.Errorf("adapter %s does not declare operation %s", adapterID, request.Operation)
	}
	if request.ContributionID != "" {
		declaredContribution := false
		for _, contribution := range declared.Contributions {
			if contribution.ID == request.ContributionID {
				declaredContribution = true
				break
			}
		}
		b.mu.Lock()
		_, dynamicContribution := b.contributions[module.Key()][request.ContributionID]
		b.mu.Unlock()
		if !declaredContribution && !dynamicContribution {
			return plugincontract.AdapterInvocationResult{}, fmt.Errorf("contribution %s is not owned by module %s", request.ContributionID, module.Key())
		}
	}
	request.Module = module
	if request.InvocationID == "" {
		request.InvocationID = uuid.NewString()
	}
	return b.container.PluginRoutes.InvokeAdapter(ctx, adapterID, contractVersion, request)
}

func (b *PluginBroker) CancelAdapterInvocation(
	ctx context.Context,
	module plugincontract.ModuleRef,
	adapterID string,
	contractVersion int,
	request plugincontract.AdapterCancellationRequest,
) error {
	if b == nil || b.container == nil || b.container.PluginRoutes == nil || b.container.PluginAdapters == nil {
		return fmt.Errorf("plugin adapter broker is unavailable")
	}
	_, declared, err := b.moduleContract(module)
	if err != nil {
		return err
	}
	if !containsString(declared.Dynamic.AllowedAdapters, adapterID) {
		return fmt.Errorf("module %s does not allow adapter %s", module.Key(), adapterID)
	}
	descriptor, ok := b.container.PluginAdapters.Get(adapterID, contractVersion)
	if !ok {
		return fmt.Errorf("adapter %s v%d is not registered", adapterID, contractVersion)
	}
	if err := b.validateAdapterPermissions(declared, descriptor); err != nil {
		return err
	}
	return b.container.PluginRoutes.CancelAdapterInvocation(ctx, adapterID, contractVersion, request)
}

func (b *PluginBroker) validateAdapterPermissions(manifest *plugincontract.Module, descriptor plugincontract.AdapterDescriptor) error {
	if manifest == nil || len(descriptor.PermissionRequirements) == 0 {
		return nil
	}
	allowed := make(map[string]bool)
	for _, permission := range manifest.Permissions {
		allowed[permission.Name] = true
	}
	for _, requirement := range descriptor.PermissionRequirements {
		if !allowed[requirement] {
			return fmt.Errorf("adapter %s requires undeclared permission %s", descriptor.ID, requirement)
		}
	}
	return nil
}

func (b *PluginBroker) serviceState(module plugincontract.ModuleRef) ([]plugincontract.ServiceOffer, []plugincontract.ServiceRequirement, error) {
	key := module.Key()
	b.mu.Lock()
	if b.servicesLoaded == nil {
		b.servicesLoaded = make(map[string]bool)
	}
	if b.servicesLoaded[key] {
		offers := append([]plugincontract.ServiceOffer(nil), b.offers[key]...)
		requirements := append([]plugincontract.ServiceRequirement(nil), b.requirements[key]...)
		b.mu.Unlock()
		return offers, requirements, nil
	}
	b.mu.Unlock()
	var offers []plugincontract.ServiceOffer
	var requirements []plugincontract.ServiceRequirement
	if b.validateManifest || (b.container != nil && b.container.InstallationRepository != nil && b.container.PackageRepository != nil) {
		_, declared, err := b.moduleContract(module)
		if err != nil {
			return nil, nil, err
		}
		offers = append([]plugincontract.ServiceOffer(nil), declared.Services.Provides...)
		requirements = append([]plugincontract.ServiceRequirement(nil), declared.Services.Requires...)
	}
	b.mu.Lock()
	if b.servicesLoaded[key] {
		offers = append([]plugincontract.ServiceOffer(nil), b.offers[key]...)
		requirements = append([]plugincontract.ServiceRequirement(nil), b.requirements[key]...)
	} else {
		b.offers[key] = offers
		b.requirements[key] = requirements
		b.servicesLoaded[key] = true
	}
	b.mu.Unlock()
	return offers, requirements, nil
}
