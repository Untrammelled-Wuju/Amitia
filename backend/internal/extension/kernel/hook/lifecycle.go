package hook

import (
	"context"
)

type HookLifecycleManager struct {
	PointRegistry HookPointRegistry
	ContribStore  ContributionStore
	Circuit       *CircuitBreaker
	RuntimeBridge RuntimeBridge
	Permission    PermissionChecker
	Scope         ScopeChecker
	Dependency    DependencyChecker
	PlanCache     *PlanCache
}

func (lm *HookLifecycleManager) permissionChecker() PermissionChecker {
	if lm.Permission != nil {
		return lm.Permission
	}
	return NopPermissionChecker{}
}

func (lm *HookLifecycleManager) scopeChecker() ScopeChecker {
	if lm.Scope != nil {
		return lm.Scope
	}
	return NopScopeChecker{}
}

func (lm *HookLifecycleManager) dependencyChecker() DependencyChecker {
	if lm.Dependency != nil {
		return lm.Dependency
	}
	return NopDependencyChecker{}
}

func (lm *HookLifecycleManager) validateRuntimeBinding(binding RuntimeBinding) error {
	if binding.RuntimeType == "" || binding.ModuleID == "" || binding.Entry == "" {
		return NewHookError(ErrCodeRuntimeNotReady, "runtime binding incomplete")
	}
	return nil
}

func (lm *HookLifecycleManager) validatePermissionRequirements(reqs []PermissionRequirement) error {
	for _, pr := range reqs {
		if pr.PermissionID == "" {
			return NewHookError(ErrCodeHookResultInvalid, "permission id required")
		}
	}
	return nil
}

func (lm *HookLifecycleManager) invalidatePlanFor(hookPointID string) {
	if lm.PlanCache != nil {
		lm.PlanCache.Invalidate(hookPointID)
	}
}

func (lm *HookLifecycleManager) InstallContribution(ctx context.Context, contrib HookContributionDefinition) error {
	point, err := lm.PointRegistry.GetPoint(ctx, contrib.HookPointID)
	if err != nil {
		return WrapHookError(ErrCodeHookPointNotFound, "hook point not found: "+contrib.HookPointID, err)
	}
	if err := contrib.Validate(point); err != nil {
		return err
	}
	if err := lm.validatePermissionRequirements(contrib.PermissionRequirements); err != nil {
		return err
	}
	if err := lm.validateRuntimeBinding(contrib.RuntimeBinding); err != nil {
		return err
	}
	contrib.Enabled = false
	if err := lm.ContribStore.Register(ctx, contrib); err != nil {
		return WrapHookError(ErrCodeHookResultInvalid, "register contribution", err)
	}
	lm.invalidatePlanFor(contrib.HookPointID)
	return nil
}

func (lm *HookLifecycleManager) activateContribution(ctx context.Context, contrib HookContributionDefinition) error {
	permOK, permReason := lm.permissionChecker().Check(ctx, contrib.ExtensionID, contrib.ToPermissionRequirements(), "")
	if !permOK {
		return NewHookError(ErrCodePermissionDenied, "permission denied: "+permReason)
	}
	scopeOK, scopeReason := lm.scopeChecker().Check(ctx, contrib.ToScopeEvaluationRequest())
	if !scopeOK {
		return NewHookError(ErrCodeScopeDenied, "scope denied: "+scopeReason)
	}
	depOK, depReason := lm.dependencyChecker().Check(ctx, contrib.DependencyRequirements)
	if !depOK {
		return NewHookError(ErrCodeDependencyUnavailable, "dependency unavailable: "+depReason)
	}
	if !lm.RuntimeBridge.IsReady(ctx, contrib) {
		return NewHookError(ErrCodeRuntimeNotReady, "runtime not ready")
	}
	if err := lm.ContribStore.SetEnabled(ctx, contrib.ContributionID, true); err != nil {
		return WrapHookError(ErrCodeHookRuntimeError, "activate contribution", err)
	}
	if lm.Circuit != nil {
		lm.Circuit.Reset(contrib.ContributionID)
	}
	lm.invalidatePlanFor(contrib.HookPointID)
	return nil
}

func (lm *HookLifecycleManager) EnableContribution(ctx context.Context, contributionID string) error {
	contrib, err := lm.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return WrapHookError(ErrCodeHookNotFound, "get contribution", err)
	}
	if _, err := lm.PointRegistry.GetPoint(ctx, contrib.HookPointID); err != nil {
		return WrapHookError(ErrCodeHookPointNotFound, "hook point not found for contribution", err)
	}
	return lm.activateContribution(ctx, contrib)
}

func (lm *HookLifecycleManager) DisableContribution(ctx context.Context, contributionID string) error {
	contrib, err := lm.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return WrapHookError(ErrCodeHookNotFound, "get contribution", err)
	}
	if err := lm.ContribStore.SetEnabled(ctx, contributionID, false); err != nil {
		return WrapHookError(ErrCodeHookRuntimeError, "deactivate contribution", err)
	}
	lm.invalidatePlanFor(contrib.HookPointID)
	return nil
}

func (lm *HookLifecycleManager) UpdateContribution(ctx context.Context, oldID string, newContrib HookContributionDefinition) error {
	old, err := lm.ContribStore.Get(ctx, oldID)
	if err != nil {
		return WrapHookError(ErrCodeHookNotFound, "get old contribution", err)
	}
	point, err := lm.PointRegistry.GetPoint(ctx, newContrib.HookPointID)
	if err != nil {
		return WrapHookError(ErrCodeHookPointNotFound, "hook point not found: "+newContrib.HookPointID, err)
	}
	if err := newContrib.Validate(point); err != nil {
		return err
	}
	if err := lm.validatePermissionRequirements(newContrib.PermissionRequirements); err != nil {
		return err
	}
	if err := lm.validateRuntimeBinding(newContrib.RuntimeBinding); err != nil {
		return err
	}
	wasEnabled := old.Enabled
	newContrib.Enabled = false
	if err := lm.ContribStore.Register(ctx, newContrib); err != nil {
		return WrapHookError(ErrCodeHookResultInvalid, "register new generation", err)
	}
	if wasEnabled {
		if err := lm.activateContribution(ctx, newContrib); err != nil {
			_ = lm.ContribStore.Unregister(ctx, newContrib.ContributionID)
			return err
		}
	}
	_ = lm.ContribStore.SetEnabled(ctx, oldID, false)
	if err := lm.ContribStore.Unregister(ctx, oldID); err != nil {
		return WrapHookError(ErrCodeHookRuntimeError, "unregister old generation", err)
	}
	if lm.Circuit != nil {
		lm.Circuit.Reset(oldID)
	}
	lm.invalidatePlanFor(old.HookPointID)
	if newContrib.HookPointID != old.HookPointID {
		lm.invalidatePlanFor(newContrib.HookPointID)
	}
	return nil
}

func (lm *HookLifecycleManager) UninstallContribution(ctx context.Context, contributionID string) error {
	contrib, err := lm.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return WrapHookError(ErrCodeHookNotFound, "get contribution", err)
	}
	_ = lm.ContribStore.SetEnabled(ctx, contributionID, false)
	if err := lm.ContribStore.Unregister(ctx, contributionID); err != nil {
		return WrapHookError(ErrCodeHookRuntimeError, "unregister contribution", err)
	}
	lm.invalidatePlanFor(contrib.HookPointID)
	return nil
}

func (lm *HookLifecycleManager) UninstallByExtension(ctx context.Context, extensionID string) error {
	contribs, err := lm.ContribStore.ListByExtension(ctx, extensionID)
	if err != nil {
		return WrapHookError(ErrCodeHookRuntimeError, "list contributions by extension", err)
	}
	for _, c := range contribs {
		if err := lm.UninstallContribution(ctx, c.ContributionID); err != nil {
			return err
		}
	}
	return nil
}
