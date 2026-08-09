package runtime

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TopologyBuilder struct{}

func NewTopologyBuilder() *TopologyBuilder {
	return &TopologyBuilder{}
}

func (b *TopologyBuilder) Build(
	runtime *domain.RuntimeInstance,
	descriptor domain.PluginDescriptor,
	now time.Time,
) (*RuntimeTopology, error) {
	if runtime == nil {
		return nil, NewTopologyError(ErrInvalidArgument, "runtime instance must not be nil")
	}
	if descriptor.ID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "plugin descriptor id must not be empty")
	}
	if runtime.PluginID != descriptor.ID {
		return nil, NewTopologyErrorWithCause(ErrPluginMismatch,
			"runtime plugin id does not match descriptor id",
			NewTopologyError(ErrPluginMismatch, string(runtime.PluginID)+" != "+string(descriptor.ID)))
	}
	if runtime.ID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "runtime instance id must not be empty")
	}

	if runtime.State != domain.RuntimeStateCreated {
		return nil, NewTopologyErrorWithCause(ErrInvalidState,
			"topology can only be built for runtime in 'created' state",
			NewTopologyError(ErrInvalidState, string(runtime.State)))
	}

	topology := NewRuntimeTopology(runtime.ID, runtime.PluginID, now)

	seenServiceIDs := make(map[domain.ServiceID]domain.ServiceDescriptor)
	for _, svc := range descriptor.Services {
		if _, exists := seenServiceIDs[svc.ID]; exists {
			return nil, NewTopologyErrorWithCause(ErrDuplicateService,
				"duplicate service id in descriptor",
				NewTopologyError(ErrDuplicateService, string(svc.ID)))
		}
		seenServiceIDs[svc.ID] = svc
	}

	for _, svc := range descriptor.Services {
		for _, depID := range svc.DependsOn {
			if _, exists := seenServiceIDs[depID]; !exists {
				return nil, NewTopologyErrorWithCause(ErrDependencyNotFound,
					"service depends on unknown service",
					NewTopologyError(ErrDependencyNotFound, string(svc.ID)+" -> "+string(depID)))
			}
		}
	}

	for _, svc := range descriptor.Services {
		serviceInstanceID := BuildServiceInstanceID(runtime.ID, svc.ID)

		instance, err := NewServiceInstance(
			serviceInstanceID,
			runtime.ID,
			runtime.PluginID,
			svc.ID,
			svc.Required,
			svc.Kind,
			svc.DependsOn,
			now,
		)
		if err != nil {
			return nil, err
		}

		if svc.Metadata != nil {
			for k, v := range svc.Metadata {
				if err := instance.SetMetadata(k, v, now); err != nil {
					return nil, err
				}
			}
		}

		if err := topology.AddService(instance); err != nil {
			return nil, err
		}
	}

	return topology, nil
}
