package control

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeEligibility string

const (
	RuntimeEligible    RuntimeEligibility = "eligible"
	RuntimeNotEligible RuntimeEligibility = "not_eligible"
	RuntimeStopping    RuntimeEligibility = "stopping"
	RuntimeStopped     RuntimeEligibility = "stopped"
	RuntimeStarting    RuntimeEligibility = "starting"
)

type PermissionCheckResult struct {
	Allowed bool
	Reason  string
}

type PolicyCheckResult struct {
	Allowed bool
	Reason  string
}

type ReleasePreflightInput struct {
	RuntimeID  domain.RuntimeInstanceID
	PluginID   domain.PluginID
	FromMode   domain.ControlMode
	TargetMode domain.ControlMode
}

type RuntimeReader interface {
	IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
	IsRuntimeStopping(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
	IsRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)
}

type PermissionChecker interface {
	CanPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, targetMode domain.ControlMode) (PermissionCheckResult, error)
}

type HostPolicyChecker interface {
	AllowPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, targetMode domain.ControlMode) (PolicyCheckResult, error)
}

type NoopPermissionChecker struct{}

func (NoopPermissionChecker) CanPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, targetMode domain.ControlMode) (PermissionCheckResult, error) {
	return PermissionCheckResult{Allowed: true}, nil
}

type NoopHostPolicyChecker struct{}

func (NoopHostPolicyChecker) AllowPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, targetMode domain.ControlMode) (PolicyCheckResult, error) {
	return PolicyCheckResult{Allowed: true}, nil
}

type AlwaysDenyHostPolicyChecker struct{}

func (AlwaysDenyHostPolicyChecker) AllowPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, targetMode domain.ControlMode) (PolicyCheckResult, error) {
	return PolicyCheckResult{Allowed: false, Reason: "host policy denied"}, nil
}

type RevokedPermissionChecker struct {
	revoked map[domain.RuntimeInstanceID]bool
}

func NewRevokedPermissionChecker() *RevokedPermissionChecker {
	return &RevokedPermissionChecker{revoked: make(map[domain.RuntimeInstanceID]bool)}
}

func (c *RevokedPermissionChecker) Revoke(runtimeID domain.RuntimeInstanceID) {
	c.revoked[runtimeID] = true
}

func (c *RevokedPermissionChecker) Restore(runtimeID domain.RuntimeInstanceID) {
	delete(c.revoked, runtimeID)
}

func (c *RevokedPermissionChecker) CanPluginControl(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, targetMode domain.ControlMode) (PermissionCheckResult, error) {
	if targetMode == domain.ControlModeObserveOnly {
		return PermissionCheckResult{Allowed: true}, nil
	}
	if c.revoked[runtimeID] {
		return PermissionCheckResult{Allowed: false, Reason: "plugin control permission revoked"}, nil
	}
	return PermissionCheckResult{Allowed: true}, nil
}

type FakeRuntimeReader struct {
	mu       sync.RWMutex
	active   map[domain.RuntimeInstanceID]bool
	stopping map[domain.RuntimeInstanceID]bool
	ready    map[domain.RuntimeInstanceID]bool
}

func NewFakeRuntimeReader() *FakeRuntimeReader {
	return &FakeRuntimeReader{
		active:   make(map[domain.RuntimeInstanceID]bool),
		stopping: make(map[domain.RuntimeInstanceID]bool),
		ready:    make(map[domain.RuntimeInstanceID]bool),
	}
}

func (r *FakeRuntimeReader) SetActive(runtimeID domain.RuntimeInstanceID, active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		r.active = make(map[domain.RuntimeInstanceID]bool)
	}
	r.active[runtimeID] = active
}

func (r *FakeRuntimeReader) SetStopping(runtimeID domain.RuntimeInstanceID, stopping bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopping == nil {
		r.stopping = make(map[domain.RuntimeInstanceID]bool)
	}
	r.stopping[runtimeID] = stopping
}

func (r *FakeRuntimeReader) SetReady(runtimeID domain.RuntimeInstanceID, ready bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ready == nil {
		r.ready = make(map[domain.RuntimeInstanceID]bool)
	}
	r.ready[runtimeID] = ready
}

func (r *FakeRuntimeReader) IsRuntimeActive(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active[runtimeID], nil
}

func (r *FakeRuntimeReader) IsRuntimeStopping(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stopping[runtimeID], nil
}

func (r *FakeRuntimeReader) IsRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ready[runtimeID], nil
}
