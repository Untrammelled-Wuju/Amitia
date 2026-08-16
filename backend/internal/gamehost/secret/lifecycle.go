package secret

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	kernelsecret "github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type ServiceSecretManifest struct {
	Ref       kernelsecret.SecretRef
	Purpose   Purpose
	Required  bool
	ServiceID string
}

type RuntimeSecretLeaseSession = contracts.RuntimeSecretLeaseSession

func newSessionID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return "sess-" + hex.EncodeToString(buf)
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

func (o *LifecycleOrchestrator) UnregisterStartupManifest(runtimeID, serviceID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.startupManifests, runtimeStartupKey{runtimeID: runtimeID, serviceID: serviceID})
}

func (o *LifecycleOrchestrator) RemoveRuntimeStartupManifests(runtimeID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for key := range o.startupManifests {
		if key.runtimeID == runtimeID {
			delete(o.startupManifests, key)
		}
	}
}

func (o *LifecycleOrchestrator) PrepareServiceStart(ctx context.Context, execCtx runtime.ServiceExecutionContext) (*RuntimeSecretLeaseSession, error) {
	leases := o.adapter.ActiveServiceLeases(string(execCtx.RuntimeID), string(execCtx.ServiceID))
	for _, leaseID := range leases {
		lease, valid, err := o.adapter.QueryServiceLease(string(execCtx.RuntimeID), string(execCtx.ServiceID), execCtx.Generation, leaseID)
		if err != nil || !valid {
			continue
		}
		return o.sessionForExistingLease(string(execCtx.RuntimeID), string(execCtx.ServiceID), execCtx.Generation, lease.ID), nil
	}

	startup := o.getStartupManifest(string(execCtx.RuntimeID), string(execCtx.ServiceID))
	if len(startup) == 0 {
		return nil, nil
	}

	manifest := RuntimeSecretManifest{
		RuntimeID:   string(execCtx.RuntimeID),
		ExtensionID: execCtx.ExtensionID,
		PluginID:    string(execCtx.PluginID),
		ServiceID:   string(execCtx.ServiceID),
		Generation:  execCtx.Generation,
		Startup:     startup,
	}
	handle := o.AcquireRuntimeStartup(ctx, manifest)
	if handle.Failed {
		return nil, handle.LastError
	}
	return o.registerStartupSession(manifest, handle), nil
}

func (o *LifecycleOrchestrator) sessionForExistingLease(runtimeID, serviceID string, generation int64, leaseID kernelsecret.LeaseID) *RuntimeSecretLeaseSession {
	sess := &RuntimeSecretLeaseSession{
		SessionID:  newSessionID(),
		RuntimeID:  runtimeID,
		ServiceID:  serviceID,
		Generation: generation,
		LeaseIDs:   []kernelsecret.LeaseID{leaseID},
	}
	o.mu.Lock()
	if o.sessions == nil {
		o.sessions = make(map[string]*RuntimeSecretLeaseSession)
	}
	o.sessions[sess.SessionID] = sess
	key := runtimeStartupKey{runtimeID: runtimeID, serviceID: serviceID}
	o.serviceSessions[key] = append(o.serviceSessions[key], sess.SessionID)
	o.mu.Unlock()
	return sess
}

func (o *LifecycleOrchestrator) registerStartupSession(manifest RuntimeSecretManifest, handle StartupHandle) *RuntimeSecretLeaseSession {
	var leaseIDs []kernelsecret.LeaseID
	for _, r := range handle.Results {
		if r.Result.Granted && r.Result.LeaseID != "" {
			leaseIDs = append(leaseIDs, r.Result.LeaseID)
		}
	}
	if len(leaseIDs) == 0 {
		return nil
	}
	sess := &RuntimeSecretLeaseSession{
		SessionID:   newSessionID(),
		ExtensionID: manifest.ExtensionID,
		PluginID:    manifest.PluginID,
		RuntimeID:   manifest.RuntimeID,
		ServiceID:   manifest.ServiceID,
		Generation:  manifest.Generation,
		LeaseIDs:    leaseIDs,
	}
	o.mu.Lock()
	if o.sessions == nil {
		o.sessions = make(map[string]*RuntimeSecretLeaseSession)
	}
	o.sessions[sess.SessionID] = sess
	key := runtimeStartupKey{runtimeID: manifest.RuntimeID, serviceID: manifest.ServiceID}
	o.serviceSessions[key] = append(o.serviceSessions[key], sess.SessionID)
	o.mu.Unlock()
	return sess
}

func (o *LifecycleOrchestrator) SessionByID(sessionID string) (*RuntimeSecretLeaseSession, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	sess, ok := o.sessions[sessionID]
	return sess, ok
}

func (o *LifecycleOrchestrator) SessionsForService(runtimeID, serviceID string) []*RuntimeSecretLeaseSession {
	o.mu.RLock()
	defer o.mu.RUnlock()
	key := runtimeStartupKey{runtimeID: runtimeID, serviceID: serviceID}
	var result []*RuntimeSecretLeaseSession
	for _, sid := range o.serviceSessions[key] {
		if sess, ok := o.sessions[sid]; ok {
			result = append(result, sess)
		}
	}
	return result
}

func (o *LifecycleOrchestrator) RevokeSession(sessionID, reason string) RevokeOutcome {
	sess, ok := o.SessionByID(sessionID)
	if !ok {
		return RevokeOutcome{RequestedBy: sessionID, Reason: reason}
	}
	var revoked int
	for _, leaseID := range sess.LeaseIDs {
		if err := o.adapter.RevokeLease(leaseID, reason); err == nil {
			revoked++
		}
	}
	o.mu.Lock()
	delete(o.sessions, sessionID)
	key := runtimeStartupKey{runtimeID: sess.RuntimeID, serviceID: sess.ServiceID}
	filtered := make([]string, 0, len(o.serviceSessions[key]))
	for _, sid := range o.serviceSessions[key] {
		if sid != sessionID {
			filtered = append(filtered, sid)
		}
	}
	o.serviceSessions[key] = filtered
	o.mu.Unlock()
	return RevokeOutcome{RevokedCount: revoked, RequestedBy: sessionID, Reason: reason}
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
	RuntimeID   string
	ExtensionID string
	PluginID    string
	ServiceID   string
	Generation  int64
	Startup     []ServiceSecretManifest
	Runtime     []ServiceSecretManifest
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
	sessions         map[string]*RuntimeSecretLeaseSession
	serviceSessions  map[runtimeStartupKey][]string
}

func NewLifecycleOrchestrator(adapter *SecretLeaseAdapter) *LifecycleOrchestrator {
	return &LifecycleOrchestrator{
		adapter:         adapter,
		sessions:        make(map[string]*RuntimeSecretLeaseSession),
		serviceSessions: make(map[runtimeStartupKey][]string),
	}
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
