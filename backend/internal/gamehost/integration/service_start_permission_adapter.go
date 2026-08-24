package integration

import (
	"context"
	"fmt"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

// ServiceStartPermissionAdapter turns Kernel permission decisions into a hard
// runtime-start gate. Manifest declarations are not grants: starting executable
// plugin code requires an effective service.runtime.execute grant, and a service
// that requests outbound networking additionally requires service.network.request.
type ServiceStartPermissionAdapter struct {
	effective *ghpermission.EffectivePermissionAdapter
}

func NewServiceStartPermissionAdapter(effective *ghpermission.EffectivePermissionAdapter) (*ServiceStartPermissionAdapter, error) {
	if effective == nil {
		return nil, fmt.Errorf("service start permission adapter: effective permission is required")
	}
	return &ServiceStartPermissionAdapter{effective: effective}, nil
}

func (a *ServiceStartPermissionAdapter) AuthorizeServiceStart(
	ctx context.Context,
	execCtx runtime.ServiceExecutionContext,
	definition *trusted_service.ServiceRuntimeDefinition,
) error {
	if err := a.require(ctx, execCtx, kernelpermission.PermissionServiceRuntimeExecute); err != nil {
		return err
	}
	if definition != nil && definition.Network.AllowOutbound {
		if err := a.require(ctx, execCtx, kernelpermission.PermissionServiceNetworkRequest); err != nil {
			return err
		}
	}
	return nil
}

func (a *ServiceStartPermissionAdapter) require(ctx context.Context, execCtx runtime.ServiceExecutionContext, permissionID string) error {
	result := a.effective.CheckServicePermission(
		ctx,
		string(execCtx.RuntimeID),
		string(execCtx.PluginID),
		string(execCtx.ServiceID),
		permissionID,
	)
	if result.Allowed() {
		return nil
	}
	if result.Decision == ghpermission.DecisionRequireApproval {
		return fmt.Errorf("%s requires one-time user approval", permissionID)
	}
	if result.Detail != "" {
		return fmt.Errorf("%s denied: %s (%s)", permissionID, result.Reason, result.Detail)
	}
	return fmt.Errorf("%s denied: %s", permissionID, result.Reason)
}

var _ runtime.ServiceStartPermissionGuard = (*ServiceStartPermissionAdapter)(nil)
