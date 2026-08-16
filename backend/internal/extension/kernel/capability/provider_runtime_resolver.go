package capability

import (
	"context"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderExecutionLookup interface {
	GetProvider(
		ctx context.Context,
		id ProviderID,
	) (CapabilityProviderDefinition, bool)

	GetInstance(
		ctx context.Context,
		id ProviderInstanceID,
	) (CapabilityProviderInstance, bool)
}

type DeviceSessionResolver interface {
	ResolveActiveSession(
		ctx context.Context,
		userID runtimeidentity.UserID,
		deviceID runtimeidentity.DeviceID,
		runtimeID runtimeidentity.RuntimeID,
	) (runtimeidentity.RuntimeSessionID, bool)
}

type ProviderRegistryExecutionLookup struct {
	Registry *ProviderRegistry
}

func (l *ProviderRegistryExecutionLookup) GetProvider(
	ctx context.Context,
	id ProviderID,
) (CapabilityProviderDefinition, bool) {
	def, ok := l.Registry.GetByID(id)
	if !ok || def == nil {
		return CapabilityProviderDefinition{}, false
	}
	return *def, true
}

func (l *ProviderRegistryExecutionLookup) GetInstance(
	ctx context.Context,
	id ProviderInstanceID,
) (CapabilityProviderInstance, bool) {
	inst, ok := l.Registry.GetInstanceByID(id)
	if !ok || inst == nil {
		return CapabilityProviderInstance{}, false
	}
	return *inst, true
}

type LegacyRuntimeExecutionResolver struct{}

func (r *LegacyRuntimeExecutionResolver) ResolveRuntimeExecution(
	ctx context.Context,
	tool ToolDefinition,
	invocation ToolInvocationContext,
) (RuntimeExecutionRoute, error) {
	return RuntimeExecutionRoute{
		Binding:      tool.Runtime,
		Placement:    ProviderPlacementCore,
		RemoteDevice: false,
	}, nil
}

type ProviderRuntimeExecutionResolver struct {
	lookup          ProviderExecutionLookup
	sessionResolver DeviceSessionResolver
}

func NewProviderRuntimeExecutionResolver(lookup ProviderExecutionLookup) *ProviderRuntimeExecutionResolver {
	return &ProviderRuntimeExecutionResolver{lookup: lookup}
}

func NewProviderRuntimeExecutionResolverWithSessions(lookup ProviderExecutionLookup, sessionResolver DeviceSessionResolver) *ProviderRuntimeExecutionResolver {
	return &ProviderRuntimeExecutionResolver{lookup: lookup, sessionResolver: sessionResolver}
}

func (r *ProviderRuntimeExecutionResolver) SetSessionResolver(resolver DeviceSessionResolver) {
	r.sessionResolver = resolver
}

func (r *ProviderRuntimeExecutionResolver) ResolveRuntimeExecution(
	ctx context.Context,
	tool ToolDefinition,
	invocation ToolInvocationContext,
) (RuntimeExecutionRoute, error) {
	target := invocation.ExecutionTarget
	if target.IsZero() || target.ProviderID == "" || target.ProviderInstanceID == "" {
		return (&LegacyRuntimeExecutionResolver{}).ResolveRuntimeExecution(ctx, tool, invocation)
	}

	providerID := ParseProviderID(target.ProviderID)
	instanceID := ParseProviderInstanceID(target.ProviderInstanceID)

	if providerID.IsEmpty() || instanceID.IsEmpty() {
		return RuntimeExecutionRoute{}, ErrProviderExecutionTargetInvalid
	}

	definition, found := r.lookup.GetProvider(ctx, providerID)
	if !found {
		return RuntimeExecutionRoute{}, ErrProviderExecutionProviderNotFound
	}

	instance, found := r.lookup.GetInstance(ctx, instanceID)
	if !found {
		return RuntimeExecutionRoute{}, ErrProviderExecutionInstanceNotFound
	}

	if instance.ProviderID != providerID {
		return RuntimeExecutionRoute{}, ErrProviderExecutionBindingMismatch
	}

	if definition.CapabilityID != tool.CapabilityID {
		return RuntimeExecutionRoute{}, ErrProviderExecutionCapabilityMismatch
	}

	if instance.CapabilityID != tool.CapabilityID {
		return RuntimeExecutionRoute{}, ErrProviderExecutionCapabilityMismatch
	}

	if definition.Placement != instance.Placement {
		return RuntimeExecutionRoute{}, ErrProviderExecutionPlacementMismatch
	}

	placement := target.Placement
	if placement == "core" || placement == "local" {
		if definition.Placement != ProviderPlacementCore {
			return RuntimeExecutionRoute{}, ErrProviderExecutionPlacementMismatch
		}
		if instance.Placement != ProviderPlacementCore {
			return RuntimeExecutionRoute{}, ErrProviderExecutionPlacementMismatch
		}
	} else if placement == "device" {
		if definition.Placement != ProviderPlacementDevice {
			return RuntimeExecutionRoute{}, ErrProviderExecutionPlacementMismatch
		}
		if instance.Placement != ProviderPlacementDevice {
			return RuntimeExecutionRoute{}, ErrProviderExecutionPlacementMismatch
		}
	}

	if !instance.IsExecutable() {
		return RuntimeExecutionRoute{}, ErrProviderExecutionUnavailable
	}

	if definition.Runtime.RuntimeType == "" {
		return RuntimeExecutionRoute{}, ErrProviderRuntimeBindingInvalid
	}

	route := RuntimeExecutionRoute{
		Binding:            definition.Runtime,
		Placement:          definition.Placement,
		ProviderID:         providerID,
		ProviderInstanceID: instanceID,
		RemoteDevice:       definition.Placement == ProviderPlacementDevice,
	}

	if instance.RuntimeInstanceID != "" {
		route.ProviderRuntimeInstanceID = instance.RuntimeInstanceID
	}

	if instance.UserID != "" {
		route.UserID = instance.UserID
	} else {
		route.UserID = target.UserID
	}
	if instance.DeviceID != "" {
		route.DeviceID = instance.DeviceID
	} else {
		route.DeviceID = target.DeviceID
	}
	if instance.RuntimeID != "" {
		route.RuntimeID = instance.RuntimeID
	} else {
		route.RuntimeID = target.RuntimeID
	}

	if definition.Placement == ProviderPlacementDevice {
		sessionID, ok := r.resolveDeviceSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
		if !ok || sessionID == "" {
			return RuntimeExecutionRoute{}, ErrProviderExecutionTargetInvalid
		}
		route.RuntimeSessionID = sessionID
	} else if target.RuntimeSessionID != "" {
		route.RuntimeSessionID = target.RuntimeSessionID
	}

	return route, nil
}

func (r *ProviderRuntimeExecutionResolver) resolveDeviceSession(
	ctx context.Context,
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) (runtimeidentity.RuntimeSessionID, bool) {
	if r.sessionResolver == nil {
		return "", false
	}
	return r.sessionResolver.ResolveActiveSession(ctx, userID, deviceID, runtimeID)
}
