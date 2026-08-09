package runtime

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type CleanupAdapter interface {
	StopWithCleanup(ctx context.Context, supervisor *trusted_service.ProcessSupervisor, instanceID string, force bool) error
}

type cleanupAdapter struct{}

func NewCleanupAdapter() CleanupAdapter {
	return &cleanupAdapter{}
}

func (a *cleanupAdapter) StopWithCleanup(ctx context.Context, supervisor *trusted_service.ProcessSupervisor, instanceID string, force bool) error {
	if supervisor == nil {
		return &ExecutionError{
			Code:    ErrCleanupFailed,
			Message: "supervisor is nil",
		}
	}

	if instanceID == "" {
		return &ExecutionError{
			Code:    ErrCleanupFailed,
			Message: "instance id is empty",
		}
	}

	_, err := supervisor.Stop(ctx, trusted_service.StopRequest{
		ServiceID: instanceID,
		Reason:    "gamehost_cleanup",
		Force:     force,
	})
	if err != nil {
		return &ExecutionError{
			Code:    ErrCleanupFailed,
			Message: fmt.Sprintf("stop with cleanup failed for %s: %v", instanceID, err),
			Cause:   err,
		}
	}
	return nil
}

type CleanupVerifier interface {
	VerifyCleanup(ctx context.Context, supervisor *trusted_service.ProcessSupervisor, instanceID string) (bool, error)
}

type cleanupVerifier struct{}

func NewCleanupVerifier() CleanupVerifier {
	return &cleanupVerifier{}
}

func (v *cleanupVerifier) VerifyCleanup(ctx context.Context, supervisor *trusted_service.ProcessSupervisor, instanceID string) (bool, error) {
	if supervisor == nil {
		return false, &ExecutionError{
			Code:    ErrCleanupFailed,
			Message: "supervisor is nil",
		}
	}

	_, err := supervisor.Get(instanceID)
	if err == nil {
		return false, nil
	}

	if err == trusted_service.ErrServiceNotFound {
		return true, nil
	}

	return false, nil
}

type SupervisorStateAdapter interface {
	IsSupervisorStopping() bool
	IsSupervisorShutdown() bool
}

type SupervisorStateObserver interface {
	RegisterHealthCallback(callback func(event SupervisorHealthEvent))
	RegisterRestartCallback(callback func(event SupervisorRestartEvent))
	RegisterQuarantineCallback(callback func(event SupervisorQuarantineEvent))
	RegisterProcessExitCallback(callback func(event ProcessExitEvent))
}

type SupervisorStateProjection struct {
	RuntimeID  domain.RuntimeInstanceID
	InstanceID string
	ServiceID  domain.ServiceID
	Health     domain.HealthStatus
	Quarantined bool
	Generation int64
}
