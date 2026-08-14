package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
)

type ControlPermissionAdapter struct {
	effectivePermission *ghpermission.EffectivePermissionAdapter
}

func NewControlPermissionAdapter(effectivePermission *ghpermission.EffectivePermissionAdapter) *ControlPermissionAdapter {
	return &ControlPermissionAdapter{
		effectivePermission: effectivePermission,
	}
}

func (a *ControlPermissionAdapter) CheckControlOutput(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	pluginID domain.PluginID,
) (control.PermissionCheckResult, error) {
	if serviceID != "" {
		result := a.effectivePermission.CheckServicePermission(
			ctx,
			string(runtimeID),
			string(pluginID),
			string(serviceID),
			kernelpermission.PermissionGameHostControl,
		)
		return mapDecisionToControlResult(result)
	}
	result := a.effectivePermission.CheckRuntimePermission(
		ctx,
		string(runtimeID),
		string(pluginID),
		kernelpermission.PermissionGameHostControl,
	)
	return mapDecisionToControlResult(result)
}

func mapDecisionToControlResult(result ghpermission.DecisionResult) (control.PermissionCheckResult, error) {
	switch result.Decision {
	case ghpermission.DecisionAllowed:
		return control.PermissionCheckResult{Allowed: true}, nil
	case ghpermission.DecisionRequireApproval:
		return control.PermissionCheckResult{Allowed: false, Reason: "approval_required"}, nil
	default:
		reason := string(result.Reason)
		if reason == "" {
			reason = "permission_denied"
		}
		return control.PermissionCheckResult{Allowed: false, Reason: reason}, nil
	}
}

var _ control.EffectivePermissionChecker = (*ControlPermissionAdapter)(nil)
