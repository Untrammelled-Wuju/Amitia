package secret

import (
	"context"
	"fmt"
	"sync"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ServiceSecretManifest struct {
	Ref      kernelsecret.SecretRef
	Purpose  Purpose
	Required bool
}

type runtimeStartupKey struct {
	runtimeID string
	serviceID string
}

func (o *LifecycleOrchestrator) RegisterStartupManifest(runtimeID, serviceID string, startup []ServiceSecretManifest) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.startupManifests == nil {
		o.startupManifests = make(map[runtimeStartupKey][]ServiceSecretManifest)
	}
	o.startupManifests[runtimeStartupKey{runtimeID: runtimeID, serviceID: serviceID}] = startup
}

func (o *LifecycleOrchestrator) PrepareServiceStart(ctx context.Context, execCtx runtime.ServiceExecutionContext) (string, error) {
	leases := o.adapter.ActiveServiceLeases(string(execCtx.RuntimeID), string(execCtx.ServiceID))
	for _, leaseID := range leases {
		lease, valid, err := o.adapter.QueryServiceLease(string(execCtx.RuntimeID), string(execCtx.ServiceID), execCtx.Generation, leaseID)
		if err != nil || !valid {
			continue
		}
		return string(lease.ID), nil
	}

	startup := o.getStartupManifest(string(execCtx.RuntimeID), string(execCtx.ServiceID))
	if len(startup) == 0 {
		return "", nil
	}

	manifest := RuntimeSecretManifest{
		RuntimeID:  string(execCtx.RuntimeID),
		PluginID:   string(execCtx.PluginID),
		ServiceID:  string(execCtx.ServiceID),
		Generation: execCtx.Generation,
		Startup:    startup,
	}
	handle := o.AcquireRuntimeStartup(ctx, manifest)
	if handle.Failed {
		return "", handle.LastError
	}
	for _, r := range handle.Results {
		if r.Result.Granted && r.Result.LeaseID != "" {
			return string(r.Result.LeaseID), nil
		}
	}
	return "", nil
}

func (o *LifecycleOrchestrator) getStartupManifest(runtimeID, serviceID string) []ServiceSecretManifest {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.startupManifests[runtimeStartupKey{runtimeID: runtimeID, serviceID: serviceID}]
}

func (o *LifecycleOrchestrator) RevokeServiceLeases(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, generation int64, reason string) {
	o.adapter.RevokeServiceLeases(string(runtimeID), string(serviceID), reason)
}

func (o *LifecycleOrchestrator) RevokeRuntimeGenerationLeases(runtimeID string, generation int64, reason string) {
	o.adapter.RevokeRuntimeGenerationLeases(runtimeID, generation, reason)
}

type RuntimeSecretManifest struct {
	RuntimeID  string
	PluginID   string
	ServiceID  string
	Generation int64
	Startup    []ServiceSecretManifest
	Runtime    []ServiceSecretManifest
}

type StartupHandle struct {
	Results   []AcquireOutcome
	Revoked   []kernelsecret.LeaseID
	Failed    bool
	LastError error
}

type LifecycleOrchestrator struct {
	adapter          *SecretLeaseAdapter
	mu               sync.RWMutex
	startupManifests map[runtimeStartupKey][]ServiceSecretManifest
}

func NewLifecycleOrchestrator(adapter *SecretLeaseAdapter) *LifecycleOrchestrator {
	return &LifecycleOrchestrator{adapter: adapter}
}

func (o *LifecycleOrchestrator) AcquireRuntimeStartup(
	ctx context.Context,
	manifest RuntimeSecretManifest,
) StartupHandle {
	handle := StartupHandle{}

	if len(manifest.Startup) == 0 {
		return handle
	}

	allRefs := append([]ServiceSecretManifest{}, manifest.Startup...)
	for _, m := range manifest.Runtime {
		allRefs = append(allRefs, m)
	}

	for _, sec := range allRefs {
		result, err := o.adapter.AcquireServiceLease(
			ctx,
			manifest.RuntimeID,
			manifest.PluginID,
			manifest.ServiceID,
			sec.Ref,
			sec.Purpose,
			sec.Required,
			manifest.Generation,
		)
		outcome := AcquireOutcome{
			Requested: SecretAcquireRequest{
				RuntimeID:  manifest.RuntimeID,
				PluginID:   manifest.PluginID,
				ServiceID:  manifest.ServiceID,
				Ref:        sec.Ref,
				Purpose:    sec.Purpose,
				Required:   sec.Required,
				Generation: manifest.Generation,
			},
			Result:     result,
			LeaseError: err,
		}
		handle.Results = append(handle.Results, outcome)

		if err != nil {
			if sec.Required {
				o.revokeAcquired(handle, manifest, sec.Ref)
				handle.Failed = true
				handle.LastError = fmt.Errorf("%w: ref=%s err=%v", ErrPartialAcquisition, sec.Ref, err)
				return handle
			}
			continue
		}
	}

	return handle
}

func (o *LifecycleOrchestrator) revokeAcquired(handle StartupHandle, manifest RuntimeSecretManifest, excludeRef kernelsecret.SecretRef) {
	for _, r := range handle.Results {
		if r.Result.Granted && r.Result.LeaseID != "" && r.Result.Ref != excludeRef {
			if err := o.adapter.RevokeLease(r.Result.LeaseID, "startup rollback"); err == nil {
				handle.Revoked = append(handle.Revoked, r.Result.LeaseID)
			}
		}
	}
}

func (o *LifecycleOrchestrator) RevokeServiceOnStop(runtimeID, serviceID, reason string) RevokeOutcome {
	return o.adapter.RevokeServiceLeases(runtimeID, serviceID, reason)
}

func (o *LifecycleOrchestrator) RevokeRuntimeOnStop(runtimeID, reason string) RevokeOutcome {
	return o.adapter.RevokeRuntimeLeases(runtimeID, reason)
}

func (o *LifecycleOrchestrator) RevokeRuntimeGeneration(runtimeID string, generation int64, reason string) RevokeOutcome {
	return o.adapter.RevokeRuntimeGenerationLeases(runtimeID, generation, reason)
}

func (o *LifecycleOrchestrator) RevokeOnDisable(extensionID, reason string) RevokeOutcome {
	return o.adapter.RevokeExtensionLeases(extensionID, reason)
}

func (o *LifecycleOrchestrator) RevokeOnUninstall(extensionID, reason string) RevokeOutcome {
	return o.adapter.RevokeExtensionLeases(extensionID, reason)
}
