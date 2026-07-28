package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type EventPermissionCheckerAdapter struct {
	Broker permission.PermissionBroker
}

func NewEventPermissionCheckerAdapter(broker permission.PermissionBroker) *EventPermissionCheckerAdapter {
	return &EventPermissionCheckerAdapter{Broker: broker}
}

func (a *EventPermissionCheckerAdapter) CheckSubscriptionPermission(ctx context.Context, def event.EventSubscriptionDefinition) (bool, string, error) {
	if a.Broker == nil {
		return false, "permission_broker_missing", nil
	}
	if len(def.PermissionRequirements) == 0 {
		return true, "", nil
	}
	reqs := make([]permission.PermissionRequirement, 0, len(def.PermissionRequirements))
	for _, r := range def.PermissionRequirements {
		reqs = append(reqs, permission.PermissionRequirement{
			PermissionID: r.Permission,
			Scope:        permission.ScopeForExtension(def.ExtensionID),
			Optional:     false,
		})
	}
	result := a.Broker.Evaluate(ctx, permission.PermissionEvaluationRequest{
		Subject:      permission.SubjectForExtension(def.ExtensionID),
		Requirements: reqs,
	})
	if result.Decision == permission.DecisionAllow {
		return true, "", nil
	}
	if result.Decision == permission.DecisionRequireApproval {
		return false, "permission_requires_approval", nil
	}
	return false, "permission_denied", nil
}

type EventScopeCheckerAdapter struct {
	Manager scope.ScopeManager
}

func NewEventScopeCheckerAdapter(manager scope.ScopeManager) *EventScopeCheckerAdapter {
	return &EventScopeCheckerAdapter{Manager: manager}
}

func (a *EventScopeCheckerAdapter) CheckSubscriptionScope(ctx context.Context, def event.EventSubscriptionDefinition, envelope event.EventEnvelope) (bool, string, error) {
	if a.Manager == nil {
		return false, "scope_manager_missing", nil
	}
	if envelope.IsFromHost() {
		if def.ScopeRule.RequiredScope == "" && !def.ScopeRule.CharacterBinding && !def.ScopeRule.ConversationBinding {
			return true, "", nil
		}
		req := scope.ScopeEvaluationRequest{
			SubjectType: scope.SubjectExtension,
			SubjectID:   def.ExtensionID,
			ExtensionID: def.ExtensionID,
			ModuleID:    def.ModuleID,
		}
		decision := a.Manager.Evaluate(ctx, req)
		if decision.Allowed {
			return true, "", nil
		}
		reason := "delivery_rejected_scope"
		if len(decision.Reasons) > 0 {
			reason = decision.Reasons[0].Code
		}
		return false, reason, nil
	}
	if envelope.ProducerID == def.ExtensionID {
		if def.ScopeRule.RequiredScope == "" && !def.ScopeRule.CharacterBinding && !def.ScopeRule.ConversationBinding {
			return true, "", nil
		}
		req := scope.ScopeEvaluationRequest{
			SubjectType: scope.SubjectExtension,
			SubjectID:   def.ExtensionID,
			ExtensionID: def.ExtensionID,
			ModuleID:    def.ModuleID,
		}
		decision := a.Manager.Evaluate(ctx, req)
		if decision.Allowed {
			return true, "", nil
		}
		reason := "delivery_rejected_scope"
		if len(decision.Reasons) > 0 {
			reason = decision.Reasons[0].Code
		}
		return false, reason, nil
	}
	req := scope.ScopeEvaluationRequest{
		SubjectType:  scope.SubjectExtension,
		SubjectID:    def.ExtensionID,
		ExtensionID:  def.ExtensionID,
		ModuleID:     def.ModuleID,
		ResourceType: "cross_extension_event",
		ResourceID:   envelope.ProducerID,
	}
	decision := a.Manager.Evaluate(ctx, req)
	if decision.Allowed {
		return true, "", nil
	}
	reason := "cross_extension_denied"
	if len(decision.Reasons) > 0 {
		reason = decision.Reasons[0].Code
	}
	return false, reason, nil
}

type EventDependencyCheckerAdapter struct {
	Resolver dependency.Resolver
}

func NewEventDependencyCheckerAdapter(resolver dependency.Resolver) *EventDependencyCheckerAdapter {
	return &EventDependencyCheckerAdapter{Resolver: resolver}
}

func (a *EventDependencyCheckerAdapter) CheckSubscriptionDependencies(ctx context.Context, def event.EventSubscriptionDefinition) (bool, string, error) {
	if len(def.DependencyRequirements) == 0 {
		return true, "", nil
	}
	if a.Resolver == nil {
		return true, "", nil
	}
	hasMissing := false
	for _, dep := range def.DependencyRequirements {
		if dep.Optional {
			continue
		}
		result := a.Resolver.Resolve(ctx, dependency.ResolveRequest{
			SourceID: def.ExtensionID,
			Requests: []dependency.Request{{
				Target:       dep.DependencyID,
				VersionRange: dep.VersionRange,
				Required:     !dep.Optional,
				Type:         dependency.TargetExtension,
			}},
		})
		for _, res := range result.Resolutions {
			if res.Status != dependency.StatusResolved && !dep.Optional {
				hasMissing = true
				break
			}
		}
	}
	if hasMissing {
		return false, "dependency_missing", nil
	}
	return true, "", nil
}

type EventRuntimeCheckerAdapter struct {
	Supervisor          runtime_supervisor.Supervisor
	EnablementResolver  enablement.EffectiveStateResolver
}

func NewEventRuntimeCheckerAdapter(supervisor runtime_supervisor.Supervisor, enablementResolver enablement.EffectiveStateResolver) *EventRuntimeCheckerAdapter {
	return &EventRuntimeCheckerAdapter{Supervisor: supervisor, EnablementResolver: enablementResolver}
}

func (a *EventRuntimeCheckerAdapter) CheckSubscriptionRuntime(ctx context.Context, def event.EventSubscriptionDefinition) (bool, string, error) {
	if a.EnablementResolver != nil {
		subject := enablement.StateSubject{
			Kind: enablement.SubjectExtension,
			ID:   def.ExtensionID,
		}
		eff := a.EnablementResolver.Resolve(ctx, subject, enablement.StateRuntimeContext{})
		if !eff.Enabled {
			return false, "extension_disabled", nil
		}
		if !eff.RuntimeReady {
			return false, "runtime_not_ready", nil
		}
	}
	if def.RuntimeBinding.Entry == "" {
		return true, "", nil
	}
	if a.Supervisor == nil {
		return true, "", nil
	}
	defID := runtime_supervisor.DefinitionID(fmt.Sprintf("%s/%s", def.ExtensionID, def.ModuleID))
	snap := a.Supervisor.Snapshot(ctx, defID)
	if len(snap.Instances) == 0 {
		return true, "", nil
	}
	for _, inst := range snap.Instances {
		if inst.Actual == runtime_supervisor.ActualReady || inst.Actual == runtime_supervisor.ActualStarting {
			if inst.Health != runtime_supervisor.HealthUnhealthy {
				return true, "", nil
			}
		}
		if inst.Actual == runtime_supervisor.ActualCrashed || inst.Actual == runtime_supervisor.ActualFailed {
			return false, "blocked_runtime", nil
		}
	}
	return true, "", nil
}

type EventCircuitStateLookupAdapter struct {
	Dispatcher *event.Dispatcher
}

func NewEventCircuitStateLookupAdapter(dispatcher *event.Dispatcher) *EventCircuitStateLookupAdapter {
	return &EventCircuitStateLookupAdapter{Dispatcher: dispatcher}
}

func (a *EventCircuitStateLookupAdapter) LookupCircuitState(subscriptionID string) event.CircuitState {
	if a.Dispatcher == nil {
		return event.CircuitClosed
	}
	return a.Dispatcher.LookupCircuitState(subscriptionID)
}

func BuildEventEffectiveResolver(
	broker permission.PermissionBroker,
	scopeManager scope.ScopeManager,
	depResolver dependency.Resolver,
	supervisor runtime_supervisor.Supervisor,
	dispatcher *event.Dispatcher,
	enablementResolver enablement.EffectiveStateResolver,
) event.EffectiveResolver {
	return event.NewDefaultEffectiveResolver(
		NewEventPermissionCheckerAdapter(broker),
		NewEventScopeCheckerAdapter(scopeManager),
		NewEventDependencyCheckerAdapter(depResolver),
		NewEventRuntimeCheckerAdapter(supervisor, enablementResolver),
		NewEventCircuitStateLookupAdapter(dispatcher),
	)
}
