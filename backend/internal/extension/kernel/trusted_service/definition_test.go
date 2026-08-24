package trusted_service

import (
	"testing"
)

func TestServiceState_IsTerminal(t *testing.T) {
	terminal := []ServiceState{ServiceStateStopped, ServiceStateFailed, ServiceStateQuarantined}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Fatalf("expected %s to be terminal", s)
		}
	}

	nonTerminal := []ServiceState{ServiceStateRegistered, ServiceStateStarting, ServiceStateReady, ServiceStateDegraded, ServiceStateStopping, ServiceStateCrashed}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Fatalf("expected %s to NOT be terminal", s)
		}
	}
}

func TestServiceState_IsHealthy(t *testing.T) {
	if !ServiceStateReady.IsHealthy() {
		t.Fatal("expected ready to be healthy")
	}
	if ServiceStateDegraded.IsHealthy() {
		t.Fatal("expected degraded to NOT be healthy")
	}
	if ServiceStateStopped.IsHealthy() {
		t.Fatal("expected stopped to NOT be healthy")
	}
}

func TestTrustLevel_AllowedForService(t *testing.T) {
	if !TrustLevelOfficial.AllowedForService() {
		t.Fatal("expected official to be allowed")
	}
	if !TrustLevelTrusted.AllowedForService() {
		t.Fatal("expected trusted to be allowed")
	}
	if !TrustLevelCommunity.AllowedForService() {
		t.Fatal("expected community to be allowed behind full sandbox gate")
	}
	if !TrustLevelCommunity.RequiresFullSandbox() {
		t.Fatal("expected community to require full sandbox")
	}
	if TrustLevelUnknown.AllowedForService() {
		t.Fatal("expected unknown to NOT be allowed")
	}
}

func TestCurrentPlatform(t *testing.T) {
	p := CurrentPlatform()
	if p == "" {
		t.Fatal("expected non-empty platform")
	}
}

func TestServiceInstance_StateManagement(t *testing.T) {
	inst := &ServiceInstance{
		InstanceID: "inst-1",
		ServiceID:  "svc-1",
		State:      ServiceStateRegistered,
	}

	inst.SetState(ServiceStateStarting)
	if inst.State_() != ServiceStateStarting {
		t.Fatalf("expected starting, got %s", inst.State_())
	}

	inst.MarkStarted()
	if inst.State_() != ServiceStateReady {
		t.Fatalf("expected ready after MarkStarted, got %s", inst.State_())
	}
	if inst.StartedAt == nil {
		t.Fatal("expected non-nil StartedAt")
	}

	inst.MarkStopped()
	if inst.State_() != ServiceStateStopped {
		t.Fatalf("expected stopped after MarkStopped, got %s", inst.State_())
	}
	if inst.StoppedAt == nil {
		t.Fatal("expected non-nil StoppedAt")
	}
}

func TestServiceInstance_HealthTracking(t *testing.T) {
	inst := &ServiceInstance{
		InstanceID: "inst-1",
		ServiceID:  "svc-1",
		State:      ServiceStateReady,
	}

	count := inst.RecordHealthFail()
	if count != 1 {
		t.Fatalf("expected 1 health fail, got %d", count)
	}
	count = inst.RecordHealthFail()
	if count != 2 {
		t.Fatalf("expected 2 health fails, got %d", count)
	}

	inst.RecordHealthSuccess()
	if inst.HealthFails != 0 {
		t.Fatalf("expected 0 health fails after success, got %d", inst.HealthFails)
	}
	if inst.LastHealthAt == nil {
		t.Fatal("expected non-nil LastHealthAt after success")
	}
}

func TestServiceInstance_RestartTracking(t *testing.T) {
	inst := &ServiceInstance{
		InstanceID: "inst-1",
		ServiceID:  "svc-1",
		State:      ServiceStateReady,
	}

	count := inst.IncrementRestart()
	if count != 1 {
		t.Fatalf("expected 1 restart, got %d", count)
	}
	count = inst.IncrementRestart()
	if count != 2 {
		t.Fatalf("expected 2 restarts, got %d", count)
	}
	if inst.RestartCount != 2 {
		t.Fatalf("expected RestartCount=2, got %d", inst.RestartCount)
	}
}

func TestServiceInstance_MarkCrashed(t *testing.T) {
	inst := &ServiceInstance{
		InstanceID: "inst-1",
		ServiceID:  "svc-1",
		State:      ServiceStateReady,
		stopCh:     make(chan struct{}),
	}

	inst.MarkCrashed()
	if inst.State_() != ServiceStateCrashed {
		t.Fatalf("expected crashed, got %s", inst.State_())
	}
}
