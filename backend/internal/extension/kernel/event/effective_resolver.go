package event

import (
	"context"
)

type EventPermissionChecker interface {
	CheckSubscriptionPermission(ctx context.Context, def EventSubscriptionDefinition) (granted bool, reason string, err error)
}

type EventScopeChecker interface {
	CheckSubscriptionScope(ctx context.Context, def EventSubscriptionDefinition, envelope EventEnvelope) (valid bool, reason string, err error)
}

type EventDependencyChecker interface {
	CheckSubscriptionDependencies(ctx context.Context, def EventSubscriptionDefinition) (ready bool, reason string, err error)
}

type EventRuntimeChecker interface {
	CheckSubscriptionRuntime(ctx context.Context, def EventSubscriptionDefinition) (available bool, reason string, err error)
}

type EffectiveResolver interface {
	Resolve(ctx context.Context, def EventSubscriptionDefinition) SubscriptionEffectiveState
	ResolveForDelivery(ctx context.Context, def EventSubscriptionDefinition, envelope EventEnvelope) SubscriptionEffectiveState
}

type DefaultEffectiveResolver struct {
	permissionChecker EventPermissionChecker
	scopeChecker      EventScopeChecker
	dependencyChecker EventDependencyChecker
	runtimeChecker    EventRuntimeChecker
	circuitLookup     CircuitStateLookup
}

type CircuitStateLookup interface {
	LookupCircuitState(subscriptionID string) CircuitState
}

func NewDefaultEffectiveResolver(
	permissionChecker EventPermissionChecker,
	scopeChecker EventScopeChecker,
	dependencyChecker EventDependencyChecker,
	runtimeChecker EventRuntimeChecker,
	circuitLookup CircuitStateLookup,
) *DefaultEffectiveResolver {
	return &DefaultEffectiveResolver{
		permissionChecker: permissionChecker,
		scopeChecker:      scopeChecker,
		dependencyChecker: dependencyChecker,
		runtimeChecker:    runtimeChecker,
		circuitLookup:     circuitLookup,
	}
}

func (r *DefaultEffectiveResolver) Resolve(ctx context.Context, def EventSubscriptionDefinition) SubscriptionEffectiveState {
	state := SubscriptionEffectiveState{
		Enabled:          def.Enabled,
		Generation:       def.Generation,
		CircuitState:     CircuitClosed,
	}
	if !def.Enabled {
		state.Reason = "subscription_disabled"
		return state
	}
	if r.permissionChecker != nil {
		granted, reason, err := r.permissionChecker.CheckSubscriptionPermission(ctx, def)
		if err != nil {
			state.PermissionGranted = false
			state.Reason = "permission_check_error"
			return state
		}
		state.PermissionGranted = granted
		if !granted {
			state.Reason = reason
			if state.Reason == "" {
				state.Reason = "permission_denied"
			}
			return state
		}
	} else {
		state.PermissionGranted = false
		state.Reason = "permission_checker_missing"
		return state
	}
	state.ScopeValid = true
	if r.dependencyChecker != nil {
		ready, reason, err := r.dependencyChecker.CheckSubscriptionDependencies(ctx, def)
		if err != nil {
			state.DependenciesReady = false
			state.Reason = "dependency_check_error"
			return state
		}
		state.DependenciesReady = ready
		if !ready {
			state.Reason = reason
			if state.Reason == "" {
				state.Reason = "dependency_missing"
			}
			return state
		}
	} else {
		state.DependenciesReady = false
		state.Reason = "dependency_checker_missing"
		return state
	}
	if r.runtimeChecker != nil {
		available, reason, err := r.runtimeChecker.CheckSubscriptionRuntime(ctx, def)
		if err != nil {
			state.RuntimeAvailable = false
			state.Reason = "runtime_check_error"
			return state
		}
		state.RuntimeAvailable = available
		if !available {
			state.Reason = reason
			if state.Reason == "" {
				state.Reason = "blocked_runtime"
			}
			return state
		}
	} else {
		state.RuntimeAvailable = false
		state.Reason = "runtime_checker_missing"
		return state
	}
	if r.circuitLookup != nil {
		state.CircuitState = r.circuitLookup.LookupCircuitState(def.ContributionID)
		if state.CircuitState == CircuitOpen {
			state.Reason = "circuit_open"
			return state
		}
	} else {
		state.CircuitState = CircuitClosed
	}
	state.Reason = ""
	return state
}

func (r *DefaultEffectiveResolver) ResolveForDelivery(ctx context.Context, def EventSubscriptionDefinition, envelope EventEnvelope) SubscriptionEffectiveState {
	state := r.Resolve(ctx, def)
	if !state.IsActive() {
		return state
	}
	if r.scopeChecker != nil {
		valid, reason, err := r.scopeChecker.CheckSubscriptionScope(ctx, def, envelope)
		if err != nil {
			state.ScopeValid = false
			state.Reason = "scope_check_error"
			return state
		}
		state.ScopeValid = valid
		if !valid {
			state.Reason = reason
			if state.Reason == "" {
				state.Reason = "delivery_rejected_scope"
			}
			return state
		}
	} else {
		state.ScopeValid = false
		state.Reason = "scope_checker_missing"
		return state
	}
	return state
}

type NoopEffectiveResolver struct{}

func NewNoopEffectiveResolver() *NoopEffectiveResolver {
	return &NoopEffectiveResolver{}
}

func (r *NoopEffectiveResolver) Resolve(_ context.Context, def EventSubscriptionDefinition) SubscriptionEffectiveState {
	return SubscriptionEffectiveState{
		Enabled:           def.Enabled,
		Generation:        def.Generation,
		PermissionGranted: true,
		ScopeValid:        true,
		DependenciesReady: true,
		RuntimeAvailable:  true,
		CircuitState:      CircuitClosed,
	}
}

func (r *NoopEffectiveResolver) ResolveForDelivery(_ context.Context, def EventSubscriptionDefinition, _ EventEnvelope) SubscriptionEffectiveState {
	return r.Resolve(context.Background(), def)
}
