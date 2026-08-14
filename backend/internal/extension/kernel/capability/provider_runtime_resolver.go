package capability

import (
	"context"
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
	lookup ProviderExecutionLookup
}

func NewProviderRuntimeExecutionResolver(lookup ProviderExecutionLookup) *ProviderRuntimeExecutionResolver {
	return &ProviderRuntimeExecutionResolver{lookup: lookup}
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

	if definition.CapabilityID != CapabilityID(tool.ID) {
		return RuntimeExecutionRoute{}, ErrProviderExecutionCapabilityMismatch
	}

	if instance.CapabilityID != CapabilityID(tool.ID) {
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

	route.RuntimeSessionID = target.RuntimeSessionID

	if definition.Placement == ProviderPlacementDevice {
		if route.RuntimeSessionID == "" {
			return RuntimeExecutionRoute{}, ErrProviderExecutionTargetInvalid
		}
	}

	return route, nil
}
