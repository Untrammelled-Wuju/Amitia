package integration

import (
	"context"
	"fmt"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
)

type StartupSecretManifestRegistrar interface {
	RegisterStartupManifest(runtimeID, serviceID string, startup []gamehostsecret.ServiceSecretManifest)
	UnregisterStartupManifest(runtimeID, serviceID string)
	RemoveRuntimeStartupManifests(runtimeID string)
}

type RuntimeGraphProvisioner struct {
	source           KernelContributionSource
	mapper           GamePluginContributionMapper
	pluginRegistry   *registry.Registry
	runtimeManager   *ghruntime.Manager
	topologyStore    *ghruntime.TopologyStore
	supervisor       *trusted_service.ProcessSupervisor
	definitionMapper *service_definition.DefinitionMapper
	secretRegistrar  StartupSecretManifestRegistrar
}

type RuntimeGraphProvisionerOptions struct {
	Source           KernelContributionSource
	Mapper           GamePluginContributionMapper
	PluginRegistry   *registry.Registry
	RuntimeManager   *ghruntime.Manager
	TopologyStore    *ghruntime.TopologyStore
	Supervisor       *trusted_service.ProcessSupervisor
	DefinitionMapper *service_definition.DefinitionMapper
	SecretRegistrar  StartupSecretManifestRegistrar
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
		source:           opts.Source,
		mapper:           opts.Mapper,
		pluginRegistry:   opts.PluginRegistry,
		runtimeManager:   opts.RuntimeManager,
		topologyStore:    opts.TopologyStore,
		supervisor:       opts.Supervisor,
		definitionMapper: opts.DefinitionMapper,
		secretRegistrar:  opts.SecretRegistrar,
	}, nil
}

func (p *RuntimeGraphProvisioner) Reconcile(ctx context.Context) error {
	plugins, err := p.source.ListEnabledGamePlugins(ctx)
	if err != nil {
		return fmt.Errorf("list enabled game plugins: %w", err)
	}

	for _, kp := range plugins {
		if err := p.reconcilePlugin(ctx, kp); err != nil {
			return fmt.Errorf("reconcile plugin %s/%s: %w", kp.Extension.ID, kp.Contribution.ID, err)
		}
	}

	return nil
}

func (p *RuntimeGraphProvisioner) reconcilePlugin(ctx context.Context, kp KernelGamePlugin) error {
	bootService, err := p.buildBootService(kp)
	if err != nil {
		return fmt.Errorf("build boot service: %w", err)
	}

	descriptor, err := p.mapper.ToDescriptor(ctx, kp.Extension, kp.Contribution)
	if err != nil {
		return fmt.Errorf("map to descriptor: %w", err)
	}

	runtime, _, err := p.runtimeManager.EnsurePrimaryRuntime(ctx, descriptor.ID)
	if err != nil {
		return fmt.Errorf("ensure primary runtime: %w", err)
	}

	bootServiceID := bootService.ID

	svcView := service_definition.ServiceRuntimeView{
		ExtensionID: string(kp.Extension.ID),
		ModuleID:    bootService.ModuleID,
		RuntimeType: "service",
		Name:        bootService.Name,
		Description: bootService.Name,
		EntryPoint:  bootService.EntryPoint,
		Env:         bootService.Env,
		Enabled:     true,
	}

	definitionID := svcView.ToDefinitionID()

	if !p.supervisor.HasDefinition(definitionID) {
		def, err := p.definitionMapper.MapToDefinition(svcView)
		if err != nil {
			return fmt.Errorf("map to definition: %w", err)
		}
		if err := p.supervisor.Register(def); err != nil {
			return fmt.Errorf("register definition: %w", err)
		}
	}

	descriptor.Services = append(descriptor.Services, ghdomain.ServiceDescriptor{
		ID:       bootServiceID,
		Name:     bootService.Name,
		Kind:     ghdomain.ServiceKindProcess,
		Required: true,
	})

	definitionIDs := map[ghdomain.ServiceID]string{
		bootServiceID: definitionID,
	}

	if err := p.topologyStore.PutRuntimeGraph(runtime, descriptor, definitionIDs); err != nil {
		return fmt.Errorf("put runtime graph: %w", err)
	}
	if err := p.topologyStore.BindModuleID(runtime.ID, bootServiceID, bootService.ModuleID); err != nil {
		return fmt.Errorf("bind runtime module: %w", err)
	}

	if p.secretRegistrar != nil {
		grouped := p.extractSecretManifestGrouped(kp)
		for serviceID, manifest := range grouped {
			p.secretRegistrar.RegisterStartupManifest(string(runtime.ID), serviceID, manifest)
		}
	}

	return nil
}

func (p *RuntimeGraphProvisioner) extractSecretManifestGrouped(kp KernelGamePlugin) map[string][]gamehostsecret.ServiceSecretManifest {
	grouped := make(map[string][]gamehostsecret.ServiceSecretManifest)
	if len(kp.Extension.SecretRefs) == 0 {
		return grouped
	}
	for _, sr := range kp.Extension.SecretRefs {
		if sr.Ref == "" {
			continue
		}
		serviceID := sr.ServiceID
		if serviceID == "" {
			serviceID = string(kp.Contribution.ID)
		}
		grouped[serviceID] = append(grouped[serviceID], gamehostsecret.ServiceSecretManifest{
			Ref:       kernelsecret.SecretRef(sr.Ref),
			Purpose:   gamehostsecret.Purpose(sr.Purpose),
			Required:  sr.Required,
			ServiceID: sr.ServiceID,
		})
	}
	return grouped
}

type bootServiceInfo struct {
	ID         ghdomain.ServiceID
	ModuleID   string
	Name       string
	EntryPoint string
	Env        map[string]string
}

func (p *RuntimeGraphProvisioner) buildBootService(kp KernelGamePlugin) (bootServiceInfo, error) {
	runtimeModuleID, ok := kp.Contribution.Definition["runtimeModuleId"].(string)
	if !ok || runtimeModuleID == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtimeModuleId is required", kp.Extension.ID, kp.Contribution.ID)
	}
	module, found := kp.Extension.FindModule(kerneldomain.ModuleID(runtimeModuleID))
	if !found || module.Runtime == nil {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: runtime module %s is unavailable", kp.Extension.ID, kp.Contribution.ID, runtimeModuleID)
	}
	if module.Runtime.EntryPoint == "" {
		return bootServiceInfo{}, fmt.Errorf("plugin %s/%s: entry point is required", kp.Extension.ID, kp.Contribution.ID)
	}

	name := kp.Contribution.Name.Default
	if name == "" {
		name = kp.Extension.Name.Default
	}
	if name == "" {
		name = string(kp.Contribution.ID)
	}

	return bootServiceInfo{
		ID:         ghdomain.ServiceID(kp.Contribution.ID),
		ModuleID:   runtimeModuleID,
		Name:       name,
		EntryPoint: module.Runtime.EntryPoint,
		Env:        module.Runtime.Env,
	}, nil
}
