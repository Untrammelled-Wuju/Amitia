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
	ResolveRuntime(
		ctx context.Context,
		runtimeID string,
	) (
		pluginID string,
		extensionID string,
		state string,
		generation int64,
		err error,
	)

	ResolveService(
		ctx context.Context,
		runtimeID string,
		serviceID string,
	) (
		pluginID string,
		extensionID string,
		state string,
		err error,
	)

	ExtensionEnabled(
		ctx context.Context,
		extensionID string,
	) (bool, error)
}

type SecretPermissionDecision struct {
	Allowed              bool
	Reason               string
	PermissionSnapshotID string
	ScopeSnapshotID      string
}

type PermissionGate interface {
	CheckSecretUse(
		ctx context.Context,
		extensionID string,
		pluginID string,
		runtimeID string,
		serviceID string,
		ref string,
	) (SecretPermissionDecision, error)
}

type SecretLeaseAdapter struct {
	broker       KernelLeaseBroker
	index        *LeaseBindingIndex
	reader       RuntimeIdentityReader
	gate         PermissionGate
	mu           sync.RWMutex
	stopping     map[serviceBinding]bool
	shutdown     bool
	sessionLeases map[string][]kernelsecret.LeaseID
}

type serviceBinding struct {
	runtimeID string
	serviceID string
}

func NewSecretLeaseAdapter(
	broker KernelLeaseBroker,
	reader RuntimeIdentityReader,
	gate PermissionGate,
) (*SecretLeaseAdapter, error) {
	if broker == nil {
		return nil, fmt.Errorf("broker is required")
	}
	if reader == nil {
		return nil, fmt.Errorf("identity reader is required")
	}
	if gate == nil {
		return nil, fmt.Errorf("permission gate is required")
	}
	return &SecretLeaseAdapter{
		broker:   broker,
		index:    NewLeaseBindingIndex(nil),
		reader:   reader,
		gate:     gate,
		stopping: make(map[serviceBinding]bool),
	}, nil
}

func NewSecretLeaseAdapterWithClock(
	broker KernelLeaseBroker,
	reader RuntimeIdentityReader,
	gate PermissionGate,
	indexClock func() int64,
) (*SecretLeaseAdapter, error) {
	if broker == nil {
		return nil, fmt.Errorf("broker is required")
	}
	if reader == nil {
		return nil, fmt.Errorf("identity reader is required")
	}
	if gate == nil {
		return nil, fmt.Errorf("permission gate is required")
	}
	return &SecretLeaseAdapter{
		broker:   broker,
		index:    NewLeaseBindingIndex(indexClock),
		reader:   reader,
		gate:     gate,
		stopping: make(map[serviceBinding]bool),
	}, nil
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
	if generation <= 0 {
		result.Reason = "generation must be positive"
		return result, ErrGenerationMismatch
	}

	pluginOK, extID, _, currentGen, err := a.reader.ResolveRuntime(ctx, runtimeID)
	if err != nil {
		result.Reason = fmt.Sprintf("runtime not found: %v", err)
		return result, ErrRuntimeInvalid
	}
	if pluginOK != pluginID {
		result.Reason = "runtime/plugin mismatch"
		return result, ErrBindingInvalid
	}
	if generation != currentGen {
		result.Reason = fmt.Sprintf("generation mismatch: expected %d, current %d", generation, currentGen)
		return result, ErrGenerationMismatch
	}

	svcPluginID, svcExtID, _, err := a.reader.ResolveService(ctx, runtimeID, serviceID)
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

	extEnabled, err := a.reader.ExtensionEnabled(ctx, extID)
	if err != nil {
		result.Reason = fmt.Sprintf("extension state check failed: %v", err)
		return result, ErrExtensionDisabled
	}
	if !extEnabled {
		result.Reason = "extension disabled or uninstalled"
		return result, ErrExtensionDisabled
	}

	a.mu.RLock()
	stopped := a.stopping[serviceBinding{runtimeID: runtimeID, serviceID: serviceID}]
	shutdown := a.shutdown
	a.mu.RUnlock()
	if shutdown {
		result.Reason = "host shutting down"
		return result, ErrHostShutdown
	}
	if stopped {
		result.Reason = "service stopped"
		return result, ErrServiceStopped
	}

	decision, err := a.gate.CheckSecretUse(ctx, extID, pluginID, runtimeID, serviceID, string(ref))
	if err != nil {
		result.Reason = fmt.Sprintf("permission check error: %v", err)
		return result, ErrPermissionDenied
	}
	if !decision.Allowed {
		result.Reason = fmt.Sprintf("permission denied: %s", decision.Reason)
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
	if decision.PermissionSnapshotID != "" {
		kernelReq.PermissionSnapshotID = decision.PermissionSnapshotID
	}
	if decision.ScopeSnapshotID != "" {
		kernelReq.ScopeSnapshotID = decision.ScopeSnapshotID
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
		if err := a.RevokeLease(l, reason); err == nil {
			revoked++
		}
	}
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
		if err := a.RevokeLease(l, reason); err == nil {
			revoked++
		}
	}
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: extensionID, Reason: reason}
}

func (a *SecretLeaseAdapter) RevokeLease(leaseID kernelsecret.LeaseID, reason string) error {
	if err := a.broker.RevokeLease(leaseID); err != nil {
		return err
	}
	a.index.MarkRevoked(leaseID)
	return nil
}

func (a *SecretLeaseAdapter) ReleaseServiceLease(runtimeID, serviceID string, generation int64, leaseID kernelsecret.LeaseID, reason string) error {
	if runtimeID == "" || serviceID == "" || generation <= 0 || leaseID == "" {
		return ErrBindingInvalid
	}
	entry, ok := a.index.LookupByLease(leaseID)
	if !ok || entry.RuntimeID != runtimeID || entry.ServiceID != serviceID || entry.Generation != generation {
		return ErrBindingInvalid
	}
	return a.RevokeLease(leaseID, reason)
}

func (a *SecretLeaseAdapter) QueryServiceLease(runtimeID, serviceID string, generation int64, leaseID kernelsecret.LeaseID) (kernelsecret.Lease, bool, error) {
	if runtimeID == "" || serviceID == "" || generation <= 0 || leaseID == "" {
		return kernelsecret.Lease{}, false, ErrBindingInvalid
	}
	entry, ok := a.index.LookupByLease(leaseID)
	if !ok || entry.RuntimeID != runtimeID || entry.ServiceID != serviceID || entry.Generation != generation {
		return kernelsecret.Lease{}, false, ErrBindingInvalid
	}
	lease, found := a.broker.GetLease(leaseID)
	if !found {
		return kernelsecret.Lease{}, false, nil
	}
	return lease, lease.CanUse(), nil
}

func (a *SecretLeaseAdapter) RevokeRuntimeGenerationLeases(runtimeID string, generation int64, reason string) RevokeOutcome {
	leases := a.index.ActiveLeasesByGeneration(runtimeID, generation)
	revoked := 0
	for _, l := range leases {
		if err := a.RevokeLease(l, reason); err == nil {
			revoked++
		}
	}
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: fmt.Sprintf("%s/gen%d", runtimeID, generation), Reason: reason}
}

func (a *SecretLeaseAdapter) ActiveServiceLeases(runtimeID, serviceID string) []kernelsecret.LeaseID {
	return a.index.ActiveLeasesByService(runtimeID, serviceID)
}

func (a *SecretLeaseAdapter) AcquireServiceLeaseSession(
	ctx context.Context,
	session *RuntimeSecretLeaseSession,
	pluginID string,
	purpose Purpose,
	required bool,
) error {
	if session == nil || len(session.LeaseIDs) == 0 {
		return ErrBindingInvalid
	}
	a.mu.Lock()
	if a.sessionLeases == nil {
		a.sessionLeases = make(map[string][]kernelsecret.LeaseID)
	}
	a.sessionLeases[session.SessionID] = append([]kernelsecret.LeaseID{}, session.LeaseIDs...)
	a.mu.Unlock()

	for _, leaseID := range session.LeaseIDs {
		if _, ok := a.index.LookupByLease(leaseID); !ok {
			_, err := a.AcquireServiceLease(ctx, session.RuntimeID, pluginID, session.ServiceID, kernelsecret.SecretRef(leaseID), purpose, required, session.Generation)
			if err != nil {
				a.RevokeServiceSession(session.SessionID)
				return err
			}
		}
	}
	return nil
}

func (a *SecretLeaseAdapter) RevokeServiceSession(sessionID string) RevokeOutcome {
	a.mu.Lock()
	leases, ok := a.sessionLeases[sessionID]
	if ok {
		delete(a.sessionLeases, sessionID)
	}
	a.mu.Unlock()
	if !ok {
		return RevokeOutcome{RequestedBy: sessionID, Reason: "session not found"}
	}
	revoked := 0
	for _, leaseID := range leases {
		if err := a.RevokeLease(leaseID, "session revoked"); err == nil {
			revoked++
		}
	}
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: sessionID, Reason: "session revoked"}
}

func (a *SecretLeaseAdapter) LeasesForSession(sessionID string) []kernelsecret.LeaseID {
	a.mu.RLock()
	defer a.mu.RUnlock()
	leases := a.sessionLeases[sessionID]
	if leases == nil {
		return nil
	}
	out := make([]kernelsecret.LeaseID, len(leases))
	copy(out, leases)
	return out
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
	a.stopping[serviceBinding{runtimeID: runtimeID, serviceID: serviceID}] = true
}

func (a *SecretLeaseAdapter) ClearServiceStopping(runtimeID, serviceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopping[serviceBinding{runtimeID: runtimeID, serviceID: serviceID}] = false
}

func (a *SecretLeaseAdapter) Shutdown() {
	a.mu.Lock()
	a.shutdown = true
	a.mu.Unlock()
	a.RevokeAll()
}

func (a *SecretLeaseAdapter) RevokeAll() RevokeOutcome {
	revoked := a.broker.RevokeAll()
	a.index.Clear()
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: "*", Reason: "shutdown"}
}

func (a *SecretLeaseAdapter) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopping = make(map[serviceBinding]bool)
	a.shutdown = false
	a.index.Clear()
}
