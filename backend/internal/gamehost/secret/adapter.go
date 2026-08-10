package secret

import (
	"context"
	"fmt"
	"sync"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
)

type KernelLeaseBroker interface {
	Issue(ctx context.Context, req kernelsecret.LeaseRequest) (kernelsecret.Lease, error)
	RevokeLease(leaseID kernelsecret.LeaseID) error
	GetLease(leaseID kernelsecret.LeaseID) (kernelsecret.Lease, bool)
	RevokeByRuntimeInstance(instanceID string) int
	RevokeAll() int
}

type RuntimeIdentityReader interface {
	ResolveRuntime(runtimeID string) (pluginID string, extensionID string, state string, err error)
	ResolveService(runtimeID, serviceID string) (pluginID string, extensionID string, state string, err error)
	ExtensionEnabled(extensionID string) bool
}

type PermissionGate interface {
	CanAcquireSecret(ctx context.Context, extensionID, pluginID, runtimeID, serviceID, ref string) bool
}

type SecretLeaseAdapter struct {
	broker    KernelLeaseBroker
	index     *LeaseBindingIndex
	reader    RuntimeIdentityReader
	gate      PermissionGate
	mu        sync.Mutex
	acquiring map[string]bool
}

func NewSecretLeaseAdapter(
	broker KernelLeaseBroker,
	reader RuntimeIdentityReader,
	gate PermissionGate,
) *SecretLeaseAdapter {
	return &SecretLeaseAdapter{
		broker:    broker,
		index:     NewLeaseBindingIndex(nil),
		reader:    reader,
		gate:      gate,
		acquiring: make(map[string]bool),
	}
}

func NewSecretLeaseAdapterWithClock(
	broker KernelLeaseBroker,
	reader RuntimeIdentityReader,
	gate PermissionGate,
	indexClock func() int64,
) *SecretLeaseAdapter {
	return &SecretLeaseAdapter{
		broker:    broker,
		index:     NewLeaseBindingIndex(indexClock),
		reader:    reader,
		gate:      gate,
		acquiring: make(map[string]bool),
	}
}

func (a *SecretLeaseAdapter) AcquireServiceLease(
	ctx context.Context,
	runtimeID, pluginID, serviceID string,
	ref kernelsecret.SecretRef,
	purpose Purpose,
	required bool,
	generation int64,
) (SecretAcquireResult, error) {
	result := SecretAcquireResult{Ref: ref, Purpose: purpose}

	if runtimeID == "" {
		result.Reason = "runtime id required"
		return result, ErrRuntimeInvalid
	}
	if pluginID == "" {
		result.Reason = "plugin id required"
		return result, ErrServiceInvalid
	}
	if serviceID == "" {
		result.Reason = "service id required"
		return result, ErrServiceInvalid
	}
	if !ref.Valid() {
		result.Reason = "invalid secret ref"
		return result, ErrSecretRefInvalid
	}

	pluginOK, extID, _, err := a.reader.ResolveRuntime(runtimeID)
	if err != nil {
		result.Reason = fmt.Sprintf("runtime not found: %v", err)
		return result, ErrRuntimeInvalid
	}
	if pluginOK != pluginID {
		result.Reason = "runtime/plugin mismatch"
		return result, ErrBindingInvalid
	}

	svcPluginID, svcExtID, _, err := a.reader.ResolveService(runtimeID, serviceID)
	if err != nil {
		result.Reason = fmt.Sprintf("service not found: %v", err)
		return result, ErrServiceInvalid
	}
	if svcPluginID != pluginID {
		result.Reason = "service/plugin mismatch"
		return result, ErrBindingInvalid
	}
	if svcExtID != extID {
		result.Reason = "service/runtime extension mismatch"
		return result, ErrBindingInvalid
	}

	if !a.reader.ExtensionEnabled(extID) {
		result.Reason = "extension disabled or uninstalled"
		return result, ErrExtensionDisabled
	}

	a.mu.Lock()
	stopped := a.acquiring["__stop_"+runtimeID+"/"+serviceID]
	shutdown := a.acquiring["__shutdown"]
	a.mu.Unlock()
	if shutdown {
		result.Reason = "host shutting down"
		return result, ErrHostShutdown
	}
	if stopped {
		result.Reason = "service stopped"
		return result, ErrServiceStopped
	}

	if a.gate != nil && !a.gate.CanAcquireSecret(ctx, extID, pluginID, runtimeID, serviceID, string(ref)) {
		result.Reason = "permission denied"
		return result, ErrPermissionDenied
	}

	kernelReq := kernelsecret.LeaseRequest{
		Ref:               ref,
		Purpose:           string(purpose),
		RuntimeInstanceID: runtimeID,
		ExtensionID:       extID,
		ModuleID:          serviceID,
		Generation:        generation,
		MaxUses:           1,
	}

	kernelLease, err := a.broker.Issue(ctx, kernelReq)
	if err != nil {
		result.Reason = fmt.Sprintf("broker error: %v", err)
		return result, ErrSecretStoreFailure
	}

	a.index.Record(kernelLease.ID, runtimeID, serviceID, string(ref), extID, generation, purpose)

	result.LeaseID = kernelLease.ID
	result.Granted = true
	result.ExpiresAt = kernelLease.ExpiresAt.UnixNano()
	result.Reason = "granted"
	return result, nil
}

func (a *SecretLeaseAdapter) RevokeServiceLeases(runtimeID, serviceID string, reason string) RevokeOutcome {
	leases := a.index.ActiveLeasesByService(runtimeID, serviceID)
	revoked := 0
	for _, l := range leases {
		if a.broker.RevokeLease(l) == nil {
			a.index.MarkRevoked(l)
			revoked++
		}
	}
	// Direct kernel bypass — kernel tracks by RuntimeInstanceID, so if a ServiceID does not
	// appear at the kernel level, we still ensure local index cleanliness.
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: runtimeID + "/" + serviceID, Reason: reason}
}

func (a *SecretLeaseAdapter) RevokeRuntimeLeases(runtimeID string, reason string) RevokeOutcome {
	if runtimeID == "" {
		return RevokeOutcome{RequestedBy: runtimeID, Reason: reason}
	}
	revoked := a.broker.RevokeByRuntimeInstance(runtimeID)
	leases := a.index.ActiveLeasesByRuntime(runtimeID)
	for _, l := range leases {
		a.index.MarkRevoked(l)
	}
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: runtimeID, Reason: reason}
}

func (a *SecretLeaseAdapter) RevokeExtensionLeases(extensionID string, reason string) RevokeOutcome {
	leases := a.index.ActiveLeasesByExtension(extensionID)
	revoked := 0
	for _, l := range leases {
		if a.broker.RevokeLease(l) == nil {
			a.index.MarkRevoked(l)
			revoked++
		}
	}
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: extensionID, Reason: reason}
}

func (a *SecretLeaseAdapter) ActiveServiceLeases(runtimeID, serviceID string) []kernelsecret.LeaseID {
	return a.index.ActiveLeasesByService(runtimeID, serviceID)
}

func (a *SecretLeaseAdapter) ActiveRuntimeLeases(runtimeID string) []kernelsecret.LeaseID {
	return a.index.ActiveLeasesByRuntime(runtimeID)
}

func (a *SecretLeaseAdapter) ActiveExtensionLeases(extensionID string) []kernelsecret.LeaseID {
	return a.index.ActiveLeasesByExtension(extensionID)
}

func (a *SecretLeaseAdapter) MarkServiceStopping(runtimeID, serviceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acquiring["__stop_"+runtimeID+"/"+serviceID] = true
}

func (a *SecretLeaseAdapter) ClearServiceStopping(runtimeID, serviceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acquiring["__stop_"+runtimeID+"/"+serviceID] = false
}

func (a *SecretLeaseAdapter) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acquiring["__shutdown"] = true
}

func (a *SecretLeaseAdapter) RevokeAll() RevokeOutcome {
	revoked := a.broker.RevokeAll()
	a.index.Clear()
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: "*", Reason: "shutdown"}
}

func (a *SecretLeaseAdapter) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.acquiring = make(map[string]bool)
	a.index.Clear()
}
