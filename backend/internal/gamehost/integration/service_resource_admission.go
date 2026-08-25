package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/resource"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
)

// ServiceResourceAdmissionAdapter wires the generic runtime launch path to the
// GameHost resource admission boundary without making the runtime package
// depend on the resource package.
type ServiceResourceAdmissionAdapter struct {
	admission *resource.ResourceAdmissionAdapter
}

func NewServiceResourceAdmissionAdapter(admission *resource.ResourceAdmissionAdapter) *ServiceResourceAdmissionAdapter {
	return &ServiceResourceAdmissionAdapter{admission: admission}
}

func (a *ServiceResourceAdmissionAdapter) PrepareServiceStart(
	ctx context.Context,
	execCtx ghruntime.ServiceExecutionContext,
	definition *trusted_service.ServiceRuntimeDefinition,
) (func(started bool), error) {
	if a == nil || a.admission == nil {
		return nil, fmt.Errorf("resource admission unavailable")
	}
	if definition == nil {
		return nil, fmt.Errorf("resource admission requires service definition")
	}

	profile := &resource.RuntimeResourceProfile{
		MaxMemoryMB:        definition.Limits.MaxMemoryMB,
		MaxCPUPercent:      definition.Limits.MaxCPUPercent,
		MaxFileDescriptors: definition.Limits.MaxFileDescriptors,
		MaxDiskMB:          definition.Limits.MaxDiskMB,
		MaxSubprocesses:    definition.Limits.MaxSubprocesses,
	}
	subject := resource.RuntimeIdentitySubject{
		PluginID:   string(execCtx.PluginID),
		RuntimeID:  string(execCtx.RuntimeID),
		ServiceID:  string(execCtx.ServiceID),
		Generation: execCtx.Generation,
	}
	rollback, err := a.admission.AcquireRuntimeStartup(ctx, subject, profile)
	if err != nil {
		return nil, err
	}
	return func(started bool) {
		if started {
			a.admission.CommitRuntimeStartup(subject.RuntimeID, subject.ServiceID)
			return
		}
		if rollback != nil {
			rollback()
		}
	}, nil
}

func (a *ServiceResourceAdmissionAdapter) ReleaseService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	if a == nil || a.admission == nil {
		return
	}
	a.admission.ReleaseService(string(runtimeID), string(serviceID))
}

var _ ghruntime.ServiceResourceAdmission = (*ServiceResourceAdmissionAdapter)(nil)
