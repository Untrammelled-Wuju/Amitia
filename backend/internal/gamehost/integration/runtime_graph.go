package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/channel"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type InstalledGeneration struct {
	Path         string
	GenerationID string
	TreeHash     string
	Version      string
}

// InstalledGenerationResolver resolves the package generation selected by the
// extension package lifecycle. GameHost never reconstructs package paths when
// this authoritative resolver is available.
type InstalledGenerationResolver interface {
	ResolveInstalledGeneration(ctx context.Context, extensionID string) (InstalledGeneration, error)
}

type StartupSecretManifestRegistrar interface {
	RegisterStartupManifest(runtimeID, serviceID string, startup []gamehostsecret.ServiceSecretManifest)
	UnregisterStartupManifest(runtimeID, serviceID string)
	RemoveRuntimeStartupManifests(runtimeID string)
}

type RuntimeGraphProvisioner struct {
	source             KernelContributionSource
	mapper             GamePluginContributionMapper
	pluginRegistry     *registry.Registry
	runtimeManager     *ghruntime.Manager
	topologyStore      *ghruntime.TopologyStore
	supervisor         *trusted_service.ProcessSupervisor
	definitionMapper   *service_definition.DefinitionMapper
	secretRegistrar    StartupSecretManifestRegistrar
	extensionRoot      string
	nodeResolver       script_host.NodeEnvironmentResolver
	generationResolver InstalledGenerationResolver
	runtimeExecutor    ghruntime.RuntimeExecutor
	channelReconciler  *channel.Reconciler
}

type RuntimeGraphProvisionerOptions struct {
	Source             KernelContributionSource
	Mapper             GamePluginContributionMapper
	PluginRegistry     *registry.Registry
	RuntimeManager     *ghruntime.Manager
	TopologyStore      *ghruntime.TopologyStore
	Supervisor         *trusted_service.ProcessSupervisor
	DefinitionMapper   *service_definition.DefinitionMapper
	SecretRegistrar    StartupSecretManifestRegistrar
	ExtensionRoot      string
	NodeResolver       script_host.NodeEnvironmentResolver
	GenerationResolver InstalledGenerationResolver
	ChannelReconciler  *channel.Reconciler
}

func NewRuntimeGraphProvisioner(opts RuntimeGraphProvisionerOptions) (*RuntimeGraphProvisioner, error) {
	if opts.Source == nil {
		return nil, fmt.Errorf("runtime graph provisioner: source is required")
	}
	if opts.Mapper == nil {
		return nil, fmt.Errorf("runtime graph provisioner: mapper is required")
	}
	if opts.PluginRegistry == nil {
		return nil, fmt.Errorf("runtime graph provisioner: plugin registry is required")
	}
	if opts.RuntimeManager == nil {
		return nil, fmt.Errorf("runtime graph provisioner: runtime manager is required")
	}
	if opts.TopologyStore == nil {
		return nil, fmt.Errorf("runtime graph provisioner: topology store is required")
	}
	if opts.Supervisor == nil {
		return nil, fmt.Errorf("runtime graph provisioner: supervisor is required")
	}
	if opts.DefinitionMapper == nil {
		opts.DefinitionMapper = service_definition.NewDefinitionMapper()
	}
	return &RuntimeGraphProvisioner{
		source:             opts.Source,
		mapper:             opts.Mapper,
		pluginRegistry:     opts.PluginRegistry,
		runtimeManager:     opts.RuntimeManager,
		topologyStore:      opts.TopologyStore,
		supervisor:         opts.Supervisor,
		definitionMapper:   opts.DefinitionMapper,
		secretRegistrar:    opts.SecretRegistrar,
		extensionRoot:      opts.ExtensionRoot,
		nodeResolver:       opts.NodeResolver,
		generationResolver: opts.GenerationResolver,
		channelReconciler:  opts.ChannelReconciler,
	}, nil
}

func (p *RuntimeGraphProvisioner) SetRuntimeExecutor(executor ghruntime.RuntimeExecutor) {
	if p == nil {
		return
	}
	p.runtimeExecutor = executor
}

func (p *RuntimeGraphProvisioner) SetSecretRegistrar(registrar StartupSecretManifestRegistrar) {
	if p == nil {
		return
	}
	p.secretRegistrar = registrar
}

func (p *RuntimeGraphProvisioner) Reconcile(ctx context.Context) error {
	plugins, err := p.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return fmt.Errorf("list enabled game plugins: %w", err)
	}
	desired, reconcileErr := p.reconcilePluginSet(ctx, plugins)
	if reconcileErr != nil {
		// Reconcile healthy plugins but do not prune while the desired set is
		// incomplete. This isolates failures without deleting a plugin merely
		// because another plugin failed to map or provision.
		return reconcileErr
	}
	if err := p.pruneOrphanRuntimes(ctx, desired, ""); err != nil {
		return fmt.Errorf("prune orphan game runtimes: %w", err)
	}
	return nil
}

// ReconcileExtension converges only one extension's runtime graph. It never
// performs a global prune and therefore cannot stop/remove unrelated plugins.
func (p *RuntimeGraphProvisioner) ReconcileExtension(ctx context.Context, extensionID string) error {
	if strings.TrimSpace(extensionID) == "" {
		return fmt.Errorf("extension id is required")
	}
	plugins, err := p.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return fmt.Errorf("list enabled game plugins for extension %s: %w", extensionID, err)
	}
	filtered := make([]KernelGamePlugin, 0)
	for _, kp := range plugins {
		if string(kp.Extension.ID) == extensionID {
			filtered = append(filtered, kp)
		}
	}
	desired, reconcileErr := p.reconcilePluginSet(ctx, filtered)
	if reconcileErr != nil {
		return reconcileErr
	}
	if err := p.pruneOrphanRuntimes(ctx, desired, extensionID); err != nil {
		return fmt.Errorf("prune extension %s orphan runtimes: %w", extensionID, err)
	}
	return nil
}

func (p *RuntimeGraphProvisioner) reconcilePluginSet(ctx context.Context, plugins []KernelGamePlugin) (map[ghdomain.PluginID]struct{}, error) {
	desired := make(map[ghdomain.PluginID]struct{}, len(plugins))
	var errs []error
	for _, kp := range plugins {
		descriptor, mapErr := p.mapper.ToDescriptor(ctx, kp.Extension, kp.Contribution)
		if mapErr != nil {
			errs = append(errs, fmt.Errorf("map enabled game plugin %s/%s: %w", kp.Extension.ID, kp.Contribution.ID, mapErr))
			continue
		}
		desired[descriptor.ID] = struct{}{}
		if err := p.reconcilePlugin(ctx, kp); err != nil {
			errs = append(errs, fmt.Errorf("reconcile plugin %s/%s: %w", kp.Extension.ID, kp.Contribution.ID, err))
		}
	}
	return desired, errors.Join(errs...)
}

func (p *RuntimeGraphProvisioner) pruneOrphanRuntimes(ctx context.Context, desired map[ghdomain.PluginID]struct{}, extensionID string) error {
	var errs []error
	for _, runtimeRef := range p.runtimeManager.ListRuntimes() {
		if runtimeRef == nil {
			continue
		}
		if extensionID != "" {
			snapshot, snapErr := p.topologyStore.GetTopologySnapshot(runtimeRef.ID)
			if snapErr != nil {
				// Ownership is unknown: fail closed and leave the runtime untouched.
				errs = append(errs, fmt.Errorf("resolve runtime %s extension ownership: %w", runtimeRef.ID, snapErr))
				continue
			}
			if snapshot.ExtensionID != extensionID {
				continue
			}
		}
		if _, keep := desired[runtimeRef.PluginID]; keep {
			continue
		}
		if err := p.pruneRuntime(ctx, runtimeRef); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *RuntimeGraphProvisioner) pruneRuntime(ctx context.Context, runtimeRef *ghruntime.RuntimeInstanceRef) error {
	if runtimeRef == nil {
		return nil
	}
	if ghdomain.IsActiveRuntimeState(runtimeRef.State) {
		if p.runtimeExecutor == nil {
			return fmt.Errorf("runtime %s for removed plugin %s is active but runtime executor is unavailable", runtimeRef.ID, runtimeRef.PluginID)
		}
		if err := p.runtimeExecutor.StopRuntime(ctx, runtimeRef.ID); err != nil {
			return fmt.Errorf("stop orphan runtime %s: %w", runtimeRef.ID, err)
		}
	}

	definitionIDs := make([]string, 0)
	if snapshot, err := p.topologyStore.GetTopologySnapshot(runtimeRef.ID); err == nil {
		for _, service := range snapshot.Services {
			if definitionID, resolveErr := p.topologyStore.ResolveDefinitionID(runtimeRef.ID, service.ServiceID); resolveErr == nil && definitionID != "" {
				definitionIDs = append(definitionIDs, definitionID)
			}
		}
	}
	if p.runtimeExecutor != nil {
		if err := p.runtimeExecutor.CleanupRuntime(ctx, runtimeRef.ID); err != nil {
			return fmt.Errorf("cleanup orphan runtime %s: %w", runtimeRef.ID, err)
		}
	}
	if p.channelReconciler != nil {
		if _, err := p.channelReconciler.ReconcileRuntimeChannels(ctx, runtimeRef.ID, nil); err != nil {
			return fmt.Errorf("remove orphan runtime channels %s: %w", runtimeRef.ID, err)
		}
	}
	// Unregister service definitions before deleting topology/runtime ownership.
	// If unregister fails, keeping authoritative ownership makes the residue
	// retryable instead of orphaning a supervisor definition with no owner.
	var unregisterErrs []error
	for _, definitionID := range definitionIDs {
		if p.supervisor.HasDefinition(definitionID) {
			if err := p.supervisor.Unregister(definitionID); err != nil {
				unregisterErrs = append(unregisterErrs, fmt.Errorf("unregister orphan service definition %s: %w", definitionID, err))
			}
		}
	}
	if err := errors.Join(unregisterErrs...); err != nil {
		return err
	}
	if p.secretRegistrar != nil {
		p.secretRegistrar.RemoveRuntimeStartupManifests(string(runtimeRef.ID))
	}
	if err := p.topologyStore.RemoveRuntime(runtimeRef.ID); err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("remove orphan topology %s: %w", runtimeRef.ID, err)
	}
	if err := p.runtimeManager.RemoveRuntime(runtimeRef.ID); err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("remove orphan runtime %s: %w", runtimeRef.ID, err)
	}
	return nil
}

func (p *RuntimeGraphProvisioner) reconcilePlugin(ctx context.Context, kp KernelGamePlugin) error {
	bootServices, err := p.buildBootServices(ctx, kp)
	if err != nil {
		return fmt.Errorf("build plugin services: %w", err)
	}

	descriptor, err := p.mapper.ToDescriptor(ctx, kp.Extension, kp.Contribution)
	if err != nil {
		return fmt.Errorf("map to descriptor: %w", err)
	}

	runtime, _, err := p.runtimeManager.EnsurePrimaryRuntime(ctx, descriptor.ID)
	if err != nil {
		return fmt.Errorf("ensure primary runtime: %w", err)
	}

	definitionIDs := make(map[ghdomain.ServiceID]string, len(bootServices))
	for _, bootService := range bootServices {
		metadata := map[string]string{"protocol": bootService.Protocol, "logicalServiceId": string(bootService.ID)}
		svcView := service_definition.ServiceRuntimeView{
			ExtensionID:         string(kp.Extension.ID),
			ModuleID:            bootService.ModuleID,
			RuntimeType:         bootService.RuntimeType,
			Name:                bootService.Name,
			Description:         bootService.Name,
			PublisherID:         kp.Extension.Publisher.PublisherID,
			PublisherTrust:      kp.Extension.Publisher.TrustLevel,
			EntryPoint:          bootService.EntryPoint,
			ExecutablePath:      bootService.ExecutablePath,
			ExecutableSHA256:    bootService.ExecutableSHA256,
			Arguments:           bootService.Arguments,
			IntegrityValue:      bootService.IntegrityValue,
			Dependencies:        bootService.Dependencies,
			SandboxReadOnlyRoot: bootService.SandboxReadOnlyRoot,
			Env:                 bootService.Env,
			Metadata:            metadata,
			Network:             bootService.Network,
			Limits:              bootService.Limits,
			Enabled:             true,
		}

		definitionID := svcView.ToDefinitionID()
		def, mapErr := p.definitionMapper.MapToDefinition(svcView)
		if mapErr != nil {
			return fmt.Errorf("map service %s to definition: %w", bootService.ID, mapErr)
		}
		if p.supervisor.HasDefinition(definitionID) {
			existing, getErr := p.supervisor.GetDefinition(definitionID)
			if getErr != nil {
				return fmt.Errorf("read existing definition %s: %w", definitionID, getErr)
			}
			if existing.ManifestHash != def.ManifestHash {
				if err := p.supervisor.Unregister(definitionID); err != nil {
					return fmt.Errorf("replace stale definition %s: %w", definitionID, err)
				}
				if err := p.supervisor.Register(def); err != nil {
					return fmt.Errorf("register replacement definition %s: %w", definitionID, err)
				}
			}
		} else if err := p.supervisor.Register(def); err != nil {
			return fmt.Errorf("register definition %s: %w", definitionID, err)
		}
		definitionIDs[bootService.ID] = definitionID
	}

	if err := p.topologyStore.PutRuntimeGraph(runtime, descriptor, definitionIDs); err != nil {
		return fmt.Errorf("put runtime graph: %w", err)
	}
	for _, bootService := range bootServices {
		if err := p.topologyStore.BindModuleID(runtime.ID, bootService.ID, bootService.ModuleID); err != nil {
			return fmt.Errorf("bind runtime module for service %s: %w", bootService.ID, err)
		}
	}

	if p.channelReconciler != nil {
		inputs := buildChannelMappingInputs(descriptor, runtime.ID)
		if _, err := p.channelReconciler.ReconcileRuntimeChannels(ctx, runtime.ID, inputs); err != nil {
			return fmt.Errorf("reconcile runtime channels: %w", err)
		}
	}

	if p.secretRegistrar != nil {
		snapshot, err := p.topologyStore.GetTopologySnapshot(runtime.ID)
		if err != nil {
			return fmt.Errorf("get topology snapshot for binding validation: %w", err)
		}
		view := newTopologyServiceView(snapshot)
		grouped, errs := p.extractSecretManifestGrouped(kp, view)
		if len(errs) > 0 {
			return fmt.Errorf("secret binding validation failed: %v", errs[0])
		}
		for serviceID, manifest := range grouped {
			p.secretRegistrar.RegisterStartupManifest(string(runtime.ID), serviceID, manifest)
		}
	}
	return nil
}

func buildChannelMappingInputs(descriptor ghdomain.PluginDescriptor, runtimeID ghdomain.RuntimeInstanceID) []channel.ChannelMappingInput {
	byService := make(map[ghdomain.ServiceID][]gameprotocol.ChannelDescriptor)
	for _, declared := range descriptor.Channels {
		metadata := make(map[string]json.RawMessage, len(declared.Metadata))
		for key, value := range declared.Metadata {
			encoded, err := json.Marshal(value)
			if err != nil {
				continue
			}
			metadata[key] = encoded
		}
		direction := gameprotocol.ChannelDirection(declared.Direction)
		if direction == "" {
			direction = gameprotocol.ChannelDirectionPluginToHost
		}
		var frequency *gameprotocol.FrequencyHint
		if declared.Frequency != "" {
			hint := gameprotocol.FrequencyHint(declared.Frequency)
			frequency = &hint
		}
		byService[declared.ServiceID] = append(byService[declared.ServiceID], gameprotocol.ChannelDescriptor{
			ID:            gameprotocol.ChannelID(declared.ID),
			Kind:          gameprotocol.ChannelKind(declared.Kind),
			SchemaID:      declared.SchemaID,
			Direction:     direction,
			FrequencyHint: frequency,
			Metadata:      metadata,
		})
	}
	inputs := make([]channel.ChannelMappingInput, 0, len(byService))
	for serviceID, descriptors := range byService {
		inputs = append(inputs, channel.ChannelMappingInput{
			PluginID:    descriptor.ID,
			RuntimeID:   runtimeID,
			ServiceID:   serviceID,
			Descriptors: descriptors,
		})
	}
	return inputs
}

type topologyServiceView struct {
	snapshot ghruntime.RuntimeTopologySnapshot
}

func newTopologyServiceView(snapshot ghruntime.RuntimeTopologySnapshot) *topologyServiceView {
	return &topologyServiceView{snapshot: snapshot}
}
func (v *topologyServiceView) HasService(serviceID string) bool {
	for _, svc := range v.snapshot.Services {
		if string(svc.ServiceID) == serviceID {
			return true
		}
	}
	return false
}
func (v *topologyServiceView) ExecutableServiceCount() int {
	count := 0
	for _, svc := range v.snapshot.Services {
		if svc.ServiceKind == ghdomain.ServiceKindProcess {
			count++
		}
	}
	return count
}
func (v *topologyServiceView) DefaultExecutableService() (string, bool) {
	for _, svc := range v.snapshot.Services {
		if svc.ServiceKind == ghdomain.ServiceKindProcess {
			return string(svc.ServiceID), true
		}
	}
	return "", false
}

func (p *RuntimeGraphProvisioner) extractSecretManifestGrouped(kp KernelGamePlugin, view gamehostsecret.TopologyServiceView) (map[string][]gamehostsecret.ServiceSecretManifest, []error) {
	grouped := make(map[string][]gamehostsecret.ServiceSecretManifest)
	var errs []error
	if len(kp.Extension.SecretRefs) == 0 {
		return grouped, nil
	}
	for _, sr := range kp.Extension.SecretRefs {
		if sr.Ref == "" {
			continue
		}
		manifest := gamehostsecret.ServiceSecretManifest{Ref: kernelsecret.SecretRef(sr.Ref), Purpose: gamehostsecret.Purpose(sr.Purpose), Required: sr.Required, ServiceID: sr.ServiceID}
		binded, err := gamehostsecret.ValidateAndBindSecretRef(manifest, view)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		grouped[binded.ServiceID] = append(grouped[binded.ServiceID], *binded)
	}
	return grouped, errs
}

func buildPluginNetworkPolicy(spec *gameprotocol.PluginNetworkPolicy, permissions []string) (trusted_service.ServiceNetworkPolicy, error) {
	// Cross-platform process isolation capabilities differ. Never infer a network
	// policy: every game plugin must choose its intended boundary explicitly so
	// Windows/macOS do not accidentally inherit a Linux-only deny-all default.
	if spec == nil || strings.TrimSpace(spec.Mode) == "" {
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("explicit game plugin network.mode is required")
	}
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	policy := trusted_service.ServiceNetworkPolicy{Mode: mode, Enforce: true}
	switch mode {
	case "none":
	case "loopback":
		policy.AllowOutbound = true
		policy.LoopbackOnly = true
	case "unrestricted":
		if !containsString(permissions, "service.network.request") {
			return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("unrestricted outbound network requires service.network.request")
		}
		policy.AllowOutbound = true
	default:
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("unsupported mode %q", mode)
	}
	if err := trusted_service.ValidateNetworkPolicySupport(policy); err != nil {
		return trusted_service.ServiceNetworkPolicy{}, fmt.Errorf("game plugin network mode %q is unavailable on this host: %w", mode, err)
	}
	return policy, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

type bootServiceInfo struct {
	ID                  ghdomain.ServiceID
	ModuleID            string
	Name                string
	RuntimeType         string
	EntryPoint          string
	ExecutablePath      string
	ExecutableSHA256    string
	Arguments           []string
	IntegrityValue      string
	Dependencies        []trusted_service.LibraryDep
	SandboxReadOnlyRoot string
	Protocol            string
	Env                 map[string]string
	Network             trusted_service.ServiceNetworkPolicy
	Limits              trusted_service.ServiceResourceLimits
}

func (p *RuntimeGraphProvisioner) buildBootService(ctx context.Context, kp KernelGamePlugin) (bootServiceInfo, error) {
	services, err := p.buildBootServices(ctx, kp)
	if err != nil {
		return bootServiceInfo{}, err
	}
	if len(services) != 1 {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s declares %d services; use buildBootServices", kp.Extension.ID, kp.Contribution.ID, len(services))
	}
	return services[0], nil
}

func (p *RuntimeGraphProvisioner) buildBootServices(ctx context.Context, kp KernelGamePlugin) ([]bootServiceInfo, error) {
	spec, err := gameprotocol.ParsePluginHostSpec(kp.Contribution.Definition)
	if err != nil {
		return nil, fmt.Errorf("plugin %s/%s: parse game plugin spec: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("plugin %s/%s: validate game plugin spec: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}
	if len(spec.Services) == 0 {
		service, err := p.buildBootServiceFor(ctx, kp, spec, spec.RuntimeModuleID, "", "")
		if err != nil {
			return nil, err
		}
		return []bootServiceInfo{service}, nil
	}
	seenModules := make(map[string]struct{}, len(spec.Services))
	services := make([]bootServiceInfo, 0, len(spec.Services))
	for _, declared := range spec.Services {
		if strings.TrimSpace(declared.Kind) != "" && strings.TrimSpace(declared.Kind) != "process" {
			return nil, fmt.Errorf("plugin %s/%s: external service %q is not executable by the process runtime graph", kp.Extension.ID, kp.Contribution.ID, declared.ID)
		}
		moduleID := strings.TrimSpace(declared.ModuleID)
		if _, duplicate := seenModules[moduleID]; duplicate {
			return nil, fmt.Errorf("plugin %s/%s: process services must use distinct runtime modules; duplicate module %q", kp.Extension.ID, kp.Contribution.ID, moduleID)
		}
		seenModules[moduleID] = struct{}{}
		service, err := p.buildBootServiceFor(ctx, kp, spec, moduleID, strings.TrimSpace(declared.ID), strings.TrimSpace(declared.Name))
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

func (p *RuntimeGraphProvisioner) buildBootServiceFor(ctx context.Context, kp KernelGamePlugin, gameSpec gameprotocol.PluginHostSpec, runtimeModuleID, serviceIDOverride, nameOverride string) (bootServiceInfo, error) {
	runtimeModuleID = strings.TrimSpace(runtimeModuleID)
	if runtimeModuleID == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtime module id is required", kp.Extension.ID, kp.Contribution.ID)
	}
	module, found := kp.Extension.FindModule(kerneldomain.ModuleID(runtimeModuleID))
	if !found || module.Runtime == nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtime module %s is unavailable", kp.Extension.ID, kp.Contribution.ID, runtimeModuleID)
	}
	if strings.TrimSpace(module.Runtime.EntryPoint) == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point is required", kp.Extension.ID, kp.Contribution.ID)
	}

	name := strings.TrimSpace(nameOverride)
	if name == "" {
		name = module.Name.Default
	}
	if name == "" {
		name = kp.Contribution.Name.Default
	}
	if name == "" {
		name = string(kp.Contribution.ID)
	}
	serviceID := strings.TrimSpace(serviceIDOverride)
	if serviceID == "" {
		serviceID = strings.TrimSpace(module.Runtime.ServiceID)
	}
	if serviceID == "" {
		serviceID = string(module.ID)
	}
	protocolVersion := gameSpec.ProtocolVersion

	networkPolicy, err := buildPluginNetworkPolicy(gameSpec.Network, kp.Contribution.RequiredPermissions)
	if err != nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: network policy: %w", kp.Extension.ID, kp.Contribution.ID, err)
	}

	info := bootServiceInfo{
		ID:          ghdomain.ServiceID(serviceID),
		ModuleID:    runtimeModuleID,
		Name:        name,
		RuntimeType: string(module.Runtime.Type),
		EntryPoint:  module.Runtime.EntryPoint,
		Protocol:    protocolVersion,
		Env:         module.Runtime.Env,
		Network:     networkPolicy,
		Limits: trusted_service.ServiceResourceLimits{
			MaxMemoryMB:     bytesToMiBCeil(module.Runtime.Memory),
			MaxCPUPercent:   module.Runtime.CPUPercent,
			MaxSubprocesses: module.Runtime.MaxSubprocesses,
		},
	}
	if info.RuntimeType == "" {
		info.RuntimeType = service_definition.ServiceRuntimeType
	}
	if !service_definition.IsValidServiceRuntimeType(info.RuntimeType) {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: unsupported game runtime type %q", kp.Extension.ID, kp.Contribution.ID, info.RuntimeType)
	}

	entryPath := module.Runtime.EntryPoint
	if p.generationResolver != nil {
		generation, resolveErr := p.generationResolver.ResolveInstalledGeneration(ctx, string(kp.Extension.ID))
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: resolve authoritative package generation: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if generation.Version != "" && generation.Version != fmt.Sprintf("%v", kp.Extension.Version) {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: package generation version %q does not match enabled definition version %q", kp.Extension.ID, kp.Contribution.ID, generation.Version, kp.Extension.Version)
		}
		entryPath, resolveErr = resolveGamePluginEntry(generation.Path, runtimeModuleID, entryPath)
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if err := ensureRegularFile(entryPath); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		info.SandboxReadOnlyRoot = generation.Path
	} else if p.extensionRoot != "" {
		// Compatibility path for isolated tests/legacy embedders only. Production
		// wiring supplies generationResolver and therefore never guesses paths.
		definitionHash, hashErr := hashExtensionDefinition(kp.Extension)
		if hashErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash installed definition: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
		}
		bundlePath := resolveGamePluginBundlePath(p.extensionRoot, string(kp.Extension.ID), fmt.Sprintf("%v", kp.Extension.Version), definitionHash)
		if bundlePath == "" {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: installed bundle path not found", kp.Extension.ID, kp.Contribution.ID)
		}
		var resolveErr error
		entryPath, resolveErr = resolveGamePluginEntry(bundlePath, runtimeModuleID, entryPath)
		if resolveErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, resolveErr)
		}
		if err := ensureRegularFile(entryPath); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		info.SandboxReadOnlyRoot = bundlePath
	}

	switch info.RuntimeType {
	case service_definition.JavaScriptRuntimeType:
		if p.nodeResolver == nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: managed Node resolver is unavailable", kp.Extension.ID, kp.Contribution.ID)
		}
		nodeEnv, err := p.nodeResolver.Resolve(ctx)
		if err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: resolve managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		if err := ensureRegularFile(nodeEnv.NodeBinary); err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		sha, err := hashRuntimeFile(nodeEnv.NodeBinary)
		if err != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash managed Node: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
		info.ExecutablePath = nodeEnv.NodeBinary
		info.ExecutableSHA256 = sha
		info.IntegrityValue = "managed-node:sha256:" + sha
		entrySHA, hashErr := hashRuntimeFile(entryPath)
		if hashErr != nil {
			return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash JavaScript entry point: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
		}
		info.Dependencies = []trusted_service.LibraryDep{{
			Name:     "game-plugin-entrypoint",
			Path:     entryPath,
			Sha256:   entrySHA,
			Required: true,
		}}
		info.Arguments = []string{entryPath}
	default:
		info.ExecutablePath = entryPath
		if _, err := os.Stat(entryPath); err == nil {
			sha, hashErr := hashRuntimeFile(entryPath)
			if hashErr != nil {
				return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: hash runtime executable: %w", kp.Extension.ID, kp.Contribution.ID, hashErr)
			}
			info.ExecutableSHA256 = sha
			info.IntegrityValue = "package-runtime:sha256:" + sha
		}
	}
	return info, nil
}

func resolveGamePluginBundlePath(extRoot, extensionID, version, definitionHash string) string {
	if extRoot == "" || extensionID == "" {
		return ""
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installedRoot := filepath.Join(extRoot, "installed", safeID)

	if version != "" && definitionHash != "" {
		candidate := filepath.Join(installedRoot, version, definitionHash)
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return candidate
		}
	}

	if version != "" {
		return newestInstalledGeneration(filepath.Join(installedRoot, version))
	}

	versions, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	for i := len(versions) - 1; i >= 0; i-- {
		if !versions[i].IsDir() {
			continue
		}
		if candidate := newestInstalledGeneration(filepath.Join(installedRoot, versions[i].Name())); candidate != "" {
			return candidate
		}
	}
	return ""
}

func newestInstalledGeneration(versionDir string) string {
	artifacts, err := os.ReadDir(versionDir)
	if err != nil {
		return ""
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		if !artifacts[i].IsDir() {
			continue
		}
		candidate := filepath.Join(versionDir, artifacts[i].Name())
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return candidate
		}
	}
	return ""
}

func hashExtensionDefinition(def kerneldomain.ExtensionDefinition) (string, error) {
	data, err := json.Marshal(def)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func resolveGamePluginEntry(bundlePath, moduleID, entryPoint string) (string, error) {
	moduleID = strings.TrimSpace(moduleID)
	entryPoint = strings.TrimSpace(entryPoint)
	if moduleID == "" || strings.ContainsAny(moduleID, `/\\:`) || moduleID == "." || moduleID == ".." {
		return "", fmt.Errorf("invalid runtime module id %q", moduleID)
	}
	if entryPoint == "" || filepath.IsAbs(entryPoint) || strings.Contains(entryPoint, "\\") {
		return "", fmt.Errorf("entry point must be a relative module path")
	}
	cleanEntry := filepath.Clean(entryPoint)
	if cleanEntry == "." || cleanEntry == ".." || strings.HasPrefix(cleanEntry, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("entry point escapes runtime module")
	}

	bundleAbs, err := filepath.Abs(bundlePath)
	if err != nil {
		return "", err
	}
	moduleRoot := filepath.Join(bundleAbs, "modules", moduleID)
	moduleAbs, err := filepath.Abs(moduleRoot)
	if err != nil {
		return "", err
	}
	entryAbs, err := filepath.Abs(filepath.Join(moduleAbs, cleanEntry))
	if err != nil {
		return "", err
	}
	if entryAbs == moduleAbs || !strings.HasPrefix(entryAbs, moduleAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("entry point escapes runtime module")
	}
	if !strings.HasPrefix(moduleAbs, bundleAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("runtime module escapes installed bundle")
	}
	return entryAbs, nil
}

func bytesToMiBCeil(value int64) int64 {
	if value <= 0 {
		return 0
	}
	const mib = int64(1024 * 1024)
	if value > (int64(^uint64(0)>>1) - (mib - 1)) {
		return int64(^uint64(0)>>1) / mib
	}
	return (value + mib - 1) / mib
}

func ensureRegularFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

func hashRuntimeFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
