package browser

import (
	"context"
	"testing"
	"time"
)

func TestProductionRecoveryPolicy(t *testing.T) {
	runtime := &fakeRuntime{}
	sessions := &productionSessionManager{}
	tabs := &productionTabManager{}
	elements := newElementStore(1024)
	resources := &productionResourceTransfer{}
	policy := DefaultBrowserRecoveryPolicy()

	recovery := NewProductionRecovery(runtime, sessions, tabs, elements, resources, policy)

	if !recovery.CanRecover() {
		t.Fatal("Should be able to recover with default policy")
	}

	recovery.attempts = policy.MaxAttempts
	if recovery.CanRecover() {
		t.Fatal("Should not be able to recover after max attempts")
	}
}

func TestProductionRecoveryDisabled(t *testing.T) {
	runtime := &fakeRuntime{}
	sessions := &productionSessionManager{}
	tabs := &productionTabManager{}
	elements := newElementStore(1024)
	resources := &productionResourceTransfer{}
	policy := BrowserRecoveryPolicy{
		Enabled: false,
	}

	recovery := NewProductionRecovery(runtime, sessions, tabs, elements, resources, policy)

	_, err := recovery.AttemptRecovery(context.Background())
	if err == nil {
		t.Fatal("Recovery should fail when disabled")
	}
	if !IsRecoveryFailed(err) {
		t.Fatalf("Expected recovery_failed error, got: %v", err)
	}
}

func TestProductionRecoveryLimitReached(t *testing.T) {
	runtime := &fakeRuntime{}
	sessions := &productionSessionManager{}
	tabs := &productionTabManager{}
	elements := newElementStore(1024)
	resources := &productionResourceTransfer{}
	policy := BrowserRecoveryPolicy{
		Enabled:     true,
		MaxAttempts: 1,
	}

	recovery := NewProductionRecovery(runtime, sessions, tabs, elements, resources, policy)

	_, err := recovery.AttemptRecovery(context.Background())
	if err != nil {
		t.Fatalf("First recovery attempt should succeed: %v", err)
	}

	_, err = recovery.AttemptRecovery(context.Background())
	if err == nil {
		t.Fatal("Second recovery attempt should fail")
	}
	if err.Code != ErrCodeRecoveryLimitReached {
		t.Fatalf("Expected recovery_limit_reached error, got: %v", err)
	}
}

func TestProductionRecoveryResetAttempts(t *testing.T) {
	runtime := &fakeRuntime{}
	sessions := &productionSessionManager{}
	tabs := &productionTabManager{}
	elements := newElementStore(1024)
	resources := &productionResourceTransfer{}
	policy := BrowserRecoveryPolicy{
		Enabled:     true,
		MaxAttempts: 1,
	}

	recovery := NewProductionRecovery(runtime, sessions, tabs, elements, resources, policy)

	_, err := recovery.AttemptRecovery(context.Background())
	if err != nil {
		t.Fatalf("First recovery attempt should succeed: %v", err)
	}

	recovery.ResetAttempts()

	if !recovery.CanRecover() {
		t.Fatal("Should be able to recover after reset")
	}
}

func TestProductionRecoveryPolicyUpdate(t *testing.T) {
	runtime := &fakeRuntime{}
	sessions := &productionSessionManager{}
	tabs := &productionTabManager{}
	elements := newElementStore(1024)
	resources := &productionResourceTransfer{}
	policy := BrowserRecoveryPolicy{
		Enabled: false,
	}

	recovery := NewProductionRecovery(runtime, sessions, tabs, elements, resources, policy)

	newPolicy := BrowserRecoveryPolicy{
		Enabled:            true,
		AutoRestartRuntime: true,
		RestoreSessions:    true,
		RestoreTabs:        true,
		RestoreLastSafeURL: true,
		MaxAttempts:        3,
		Backoff:            2 * time.Second,
	}

	recovery.SetPolicy(newPolicy)

	updatedPolicy := recovery.Policy()
	if !updatedPolicy.Enabled {
		t.Fatal("Policy should be enabled after update")
	}
	if updatedPolicy.MaxAttempts != 3 {
		t.Fatalf("Expected MaxAttempts=3, got %d", updatedPolicy.MaxAttempts)
	}
}

func TestDefaultBrowserRecoveryPolicy(t *testing.T) {
	policy := DefaultBrowserRecoveryPolicy()

	if !policy.Enabled {
		t.Fatal("Default policy should be enabled")
	}
	if !policy.AutoRestartRuntime {
		t.Fatal("Default policy should auto restart runtime")
	}
	if !policy.RestoreSessions {
		t.Fatal("Default policy should restore sessions")
	}
	if !policy.RestoreTabs {
		t.Fatal("Default policy should restore tabs")
	}
	if !policy.RestoreLastSafeURL {
		t.Fatal("Default policy should restore last safe URL")
	}
	if policy.MaxAttempts != 2 {
		t.Fatalf("Expected MaxAttempts=2, got %d", policy.MaxAttempts)
	}
}
