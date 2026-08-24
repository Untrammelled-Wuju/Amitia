package integration

import (
	"context"
	"fmt"
	"strings"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	gamehostdomain "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

// GamePluginContributionMapper converts a Kernel game_plugin contribution into
// the generic GameHost descriptor. The mapper only understands host/runtime
// constructs; concrete-game metadata remains opaque to GameHost.
type GamePluginContributionMapper interface {
	ToDescriptor(
		ctx context.Context,
		extension kerneldomain.ExtensionDefinition,
		contribution kerneldomain.ContributionDefinition,
	) (gamehostdomain.PluginDescriptor, error)
}

type DefaultGamePluginContributionMapper struct{}

func NewDefaultGamePluginContributionMapper() *DefaultGamePluginContributionMapper {
	return &DefaultGamePluginContributionMapper{}
}

func (m *DefaultGamePluginContributionMapper) ToDescriptor(
	ctx context.Context,
	extension kerneldomain.ExtensionDefinition,
	contribution kerneldomain.ContributionDefinition,
) (gamehostdomain.PluginDescriptor, error) {
	_ = ctx
	pluginID := gamehostdomain.PluginID(fmt.Sprintf("%s/%s", extension.ID, contribution.ID))
	name := contribution.Name.Default
	if name == "" {
		name = extension.Name.Default
	}

	spec, err := protocol.ParseGamePluginSpec(contribution.Definition)
	if err != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("parse game plugin spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("validate game plugin spec: %w", err)
	}

	hostFeatures := make([]gamehostdomain.Capability, 0, len(spec.HostFeatures))
	for _, feature := range spec.HostFeatures {
		capability := gamehostdomain.Capability(feature)
		if err := gamehostdomain.ValidateCapability(capability); err != nil {
			return gamehostdomain.PluginDescriptor{}, fmt.Errorf("invalid host feature %q: %w", feature, err)
		}
		hostFeatures = append(hostFeatures, capability)
	}

	services, err := mapDeclaredServices(extension, spec)
	if err != nil {
		return gamehostdomain.PluginDescriptor{}, err
	}
	channels := make([]gamehostdomain.ChannelDescriptor, 0, len(spec.Channels))
	for _, ch := range spec.Channels {
		serviceID := strings.TrimSpace(ch.ServiceID)
		if serviceID == "" && len(services) == 1 {
			serviceID = string(services[0].ID)
		}
		channels = append(channels, gamehostdomain.ChannelDescriptor{
			ID:        gamehostdomain.ChannelID(strings.TrimSpace(ch.ID)),
			ServiceID: gamehostdomain.ServiceID(serviceID),
			Kind:      gamehostdomain.ChannelKind(strings.TrimSpace(ch.Kind)),
			SchemaID:  strings.TrimSpace(ch.SchemaID),
			Metadata:  cloneStringMap(ch.Metadata),
		})
	}
	controlSinks := make([]gamehostdomain.ControlSinkDeclaration, 0, len(spec.ControlEffectSinks))
	for _, sink := range spec.ControlEffectSinks {
		controlSinks = append(controlSinks, gamehostdomain.ControlSinkDeclaration{
			ID:          strings.TrimSpace(sink.ID),
			ServiceID:   gamehostdomain.ServiceID(strings.TrimSpace(sink.ServiceID)),
			Kind:        "effect",
			Description: strings.TrimSpace(sink.Description),
		})
	}

	metadata := make(map[string]string)
	if contribution.Metadata != nil {
		for k, v := range contribution.Metadata {
			if isKernelPrivateMetadataKey(k) {
				continue
			}
			metadata[k] = fmt.Sprintf("%v", v)
		}
	}
	metadata["runtimeModuleId"] = spec.RuntimeModuleID

	descriptor := gamehostdomain.PluginDescriptor{
		ID:              pluginID,
		ExtensionID:     string(extension.ID),
		Name:            name,
		Version:         fmt.Sprintf("%v", extension.Version),
		ProtocolVersion: protocol.ProtocolVersion,
		Capabilities:    hostFeatures,
		Services:        services,
		Channels:        channels,
		ControlSinks:    controlSinks,
		Metadata:        metadata,
	}
	if err := descriptor.Validate(); err != nil {
		return gamehostdomain.PluginDescriptor{}, fmt.Errorf("mapped plugin descriptor validation failed: %w", err)
	}
	return descriptor, nil
}

func mapDeclaredServices(extension kerneldomain.ExtensionDefinition, spec protocol.PluginHostSpec) ([]gamehostdomain.ServiceDescriptor, error) {
	declared := spec.Services
	if len(declared) == 0 {
		module, ok := extension.FindModule(kerneldomain.ModuleID(spec.RuntimeModuleID))
		if !ok || module.Runtime == nil {
			return nil, fmt.Errorf("runtime module %q is unavailable", spec.RuntimeModuleID)
		}
		serviceID := strings.TrimSpace(module.Runtime.ServiceID)
		if serviceID == "" {
			serviceID = string(module.ID)
		}
		return []gamehostdomain.ServiceDescriptor{{
			ID:       gamehostdomain.ServiceID(serviceID),
			Name:     firstNonEmptyString(module.Name.Default, serviceID),
			Kind:     gamehostdomain.ServiceKindProcess,
			Required: true,
			Metadata: map[string]string{"moduleId": string(module.ID)},
		}}, nil
	}

	result := make([]gamehostdomain.ServiceDescriptor, 0, len(declared))
	for _, service := range declared {
		moduleID := strings.TrimSpace(service.ModuleID)
		module, ok := extension.FindModule(kerneldomain.ModuleID(moduleID))
		if !ok {
			return nil, fmt.Errorf("declared service %q references unavailable module %q", service.ID, moduleID)
		}
		kind := gamehostdomain.ServiceKind(strings.TrimSpace(service.Kind))
		if kind == "" {
			kind = gamehostdomain.ServiceKindProcess
		}
		if kind == gamehostdomain.ServiceKindProcess && module.Runtime == nil {
			return nil, fmt.Errorf("declared process service %q module %q has no runtime", service.ID, moduleID)
		}
		dependsOn := make([]gamehostdomain.ServiceID, 0, len(service.DependsOn))
		for _, dep := range service.DependsOn {
			dependsOn = append(dependsOn, gamehostdomain.ServiceID(strings.TrimSpace(dep)))
		}
		metadata := cloneStringMap(service.Metadata)
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["moduleId"] = moduleID
		name := firstNonEmptyString(service.Name, module.Name.Default, service.ID)
		result = append(result, gamehostdomain.ServiceDescriptor{
			ID:        gamehostdomain.ServiceID(strings.TrimSpace(service.ID)),
			Name:      name,
			Kind:      kind,
			Required:  service.Required,
			DependsOn: dependsOn,
			Metadata:  metadata,
		})
	}
	return result, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isKernelPrivateMetadataKey(key string) bool {
	privateKeys := map[string]struct{}{
		"signature":      {},
		"secret":         {},
		"token":          {},
		"install_path":   {},
		"db_record_id":   {},
		"internal_state": {},
	}
	_, ok := privateKeys[key]
	return ok
}
