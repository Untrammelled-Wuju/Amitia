package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ControlHostPolicyAdapter struct {
	manager *runtime.Manager
}

func NewControlHostPolicyAdapter(manager *runtime.Manager) *ControlHostPolicyAdapter {
	return &ControlHostPolicyAdapter{
		manager: manager,
	}
}

func (a *ControlHostPolicyAdapter) AllowPluginControl(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	targetMode domain.ControlMode,
) (control.PolicyCheckResult, error) {
	state, err := a.manager.GetRuntimeState(runtimeID)
	if err != nil {
		return control.PolicyCheckResult{Allowed: false, Reason: err.Error()}, err
	}

	lifecycleIntent, err := a.manager.GetLifecycleIntent(runtimeID)
	if err != nil {
		return control.PolicyCheckResult{Allowed: false, Reason: err.Error()}, err
	}

	if lifecycleIntent == "emergency" || lifecycleIntent == "emergency_stop" || lifecycleIntent == "disable" || lifecycleIntent == "uninstall" {
		return control.PolicyCheckResult{Allowed: false, Reason: "runtime lifecycle intent blocks control: " + lifecycleIntent}, nil
	}

	switch state {
	case domain.RuntimeStateStopping, domain.RuntimeStateStopped, domain.RuntimeStateSuspended:
		return control.PolicyCheckResult{Allowed: false, Reason: "runtime state blocks control: " + string(state)}, nil
	}

	return control.PolicyCheckResult{Allowed: true}, nil
}

var _ control.HostPolicyChecker = (*ControlHostPolicyAdapter)(nil)
