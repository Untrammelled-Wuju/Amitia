package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestBuildServiceInstanceID(t *testing.T) {
	id := BuildServiceInstanceID("runtime-001", "bridge")
	expected := ServiceInstanceID("runtime-001/bridge")
	if id != expected {
		t.Errorf("expected %s, got %s", expected, id)
	}
}

func TestParseServiceInstanceID(t *testing.T) {
	runtimeID, serviceID, err := ParseServiceInstanceID("runtime-001/bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtimeID != "runtime-001" {
		t.Errorf("expected runtime-001, got %s", runtimeID)
	}
	if serviceID != "bridge" {
		t.Errorf("expected bridge, got %s", serviceID)
	}
}

func TestParseServiceInstanceID_InvalidFormat(t *testing.T) {
	_, _, err := ParseServiceInstanceID("invalid")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestDifferentRuntimesDontShareServiceInstanceID(t *testing.T) {
	idA := BuildServiceInstanceID("runtime-a", "bridge")
	idB := BuildServiceInstanceID("runtime-b", "bridge")
	if idA == idB {
		t.Error("different runtimes should produce different service instance ids")
	}
}

func TestIsValidServiceRuntimeState(t *testing.T) {
	validStates := []ServiceRuntimeState{
		ServiceStateCreated,
		ServiceStateStarting,
		ServiceStateRunning,
		ServiceStateStopping,
		ServiceStateStopped,
		ServiceStateFailed,
	}
	for _, s := range validStates {
		if !IsValidServiceRuntimeState(s) {
			t.Errorf("expected %s to be valid", s)
		}
	}
	invalidStates := []ServiceRuntimeState{
		ServiceRuntimeState("invalid"),
		ServiceRuntimeState(""),
		ServiceRuntimeState("restarting"),
	}
	for _, s := range invalidStates {
		if IsValidServiceRuntimeState(s) {
			t.Errorf("expected %s to be invalid", s)
		}
	}
}

func TestNewServiceInstance(t *testing.T) {
	now := time.Now()
	inst, err := NewServiceInstance(
		"runtime-001/bridge",
		"runtime-001",
		"minecraft-plugin",
		"bridge",
		true,
		domain.ServiceKindProcess,
		[]domain.ServiceID{},
		now,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.ID != "runtime-001/bridge" {
		t.Errorf("unexpected ID: %s", inst.ID)
	}
	if inst.State != ServiceStateCreated {
		t.Errorf("expected state created, got %s", inst.State)
	}
	if !inst.Required {
		t.Error("expected required to be true")
	}
	if inst.StartedAt != nil {
		t.Error("expected StartedAt to be nil")
	}
	if inst.StoppedAt != nil {
		t.Error("expected StoppedAt to be nil")
	}
	if inst.FailedAt != nil {
		t.Error("expected FailedAt to be nil")
	}
}

func TestNewServiceInstanceRejectsEmptyID(t *testing.T) {
	now := time.Now()
	_, err := NewServiceInstance("", "runtime-001", "plugin", "bridge", true, domain.ServiceKindProcess, nil, now)
	if err == nil {
		t.Fatal("expected error for empty instance id")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestNewServiceInstanceRejectsEmptyRuntimeID(t *testing.T) {
	now := time.Now()
	_, err := NewServiceInstance("rt/svc", "", "plugin", "bridge", true, domain.ServiceKindProcess, nil, now)
	if err == nil {
		t.Fatal("expected error for empty runtime id")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestNewServiceInstanceRejectsEmptyServiceID(t *testing.T) {
	now := time.Now()
	_, err := NewServiceInstance("rt/svc", "rt", "plugin", "", true, domain.ServiceKindProcess, nil, now)
	if err == nil {
		t.Fatal("expected error for empty service id")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestNewServiceInstanceRejectsInvalidKind(t *testing.T) {
	now := time.Now()
	_, err := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKind("invalid"), nil, now)
	if err == nil {
		t.Fatal("expected error for invalid service kind")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestServiceTransition_ValidPath(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	path := []ServiceRuntimeState{
		ServiceStateStarting,
		ServiceStateRunning,
		ServiceStateStopping,
		ServiceStateStopped,
	}

	for i, target := range path {
		later := now.Add(time.Duration(i+1) * time.Second)
		err := inst.Transition(target, later)
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if inst.State != target {
			t.Errorf("step %d: expected state %s, got %s", i, target, inst.State)
		}
	}
}

func TestServiceTransition_FailurePaths(t *testing.T) {
	cases := []struct {
		name string
		path []ServiceRuntimeState
	}{
		{
			name: "starting_to_failed",
			path: []ServiceRuntimeState{ServiceStateStarting, ServiceStateFailed},
		},
		{
			name: "running_to_failed",
			path: []ServiceRuntimeState{ServiceStateStarting, ServiceStateRunning, ServiceStateFailed},
		},
		{
			name: "stopping_to_failed",
			path: []ServiceRuntimeState{ServiceStateStarting, ServiceStateRunning, ServiceStateStopping, ServiceStateFailed},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)
			for i, target := range tc.path {
				later := now.Add(time.Duration(i+1) * time.Second)
				err := inst.Transition(target, later)
				if err != nil {
					t.Fatalf("step %d: unexpected error: %v", i, err)
				}
			}
		})
	}
}

func TestServiceTransition_InvalidTransitions(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	invalidTargets := []ServiceRuntimeState{
		ServiceStateRunning,
		ServiceStateStopped,
		ServiceStateFailed,
	}

	for _, target := range invalidTargets {
		err := inst.Transition(target, now.Add(time.Second))
		if err == nil {
			t.Errorf("expected error when transitioning from created to %s", target)
		}
	}
}

func TestServiceTransition_StoppedCantTransition(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	inst.Transition(ServiceStateStarting, now.Add(time.Second))
	inst.Transition(ServiceStateRunning, now.Add(2*time.Second))
	inst.Transition(ServiceStateStopping, now.Add(3*time.Second))
	inst.Transition(ServiceStateStopped, now.Add(4*time.Second))

	err := inst.Transition(ServiceStateRunning, now.Add(5*time.Second))
	if err == nil {
		t.Fatal("expected error when transitioning from stopped")
	}
	if !IsTopologyError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state, got %v", err)
	}
}

func TestServiceTransition_FailedCantTransition(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	inst.Transition(ServiceStateStarting, now.Add(time.Second))
	inst.Transition(ServiceStateFailed, now.Add(2*time.Second))

	err := inst.Transition(ServiceStateRunning, now.Add(3*time.Second))
	if err == nil {
		t.Fatal("expected error when transitioning from failed")
	}
	if !IsTopologyError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state, got %v", err)
	}
}

func TestServiceTransition_SameStateRejected(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	err := inst.Transition(ServiceStateCreated, now.Add(time.Second))
	if err == nil {
		t.Fatal("expected error when transitioning to same state")
	}
	if !IsTopologyError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state, got %v", err)
	}
}

func TestServiceTransitionFailureDoesNotMutate(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	originalState := inst.State
	originalUpdatedAt := inst.UpdatedAt
	originalStartedAt := inst.StartedAt

	err := inst.Transition(ServiceStateFailed, now.Add(time.Second))
	if err == nil {
		t.Fatal("expected transition to fail")
	}

	if inst.State != originalState {
		t.Errorf("State changed: got %s, want %s", inst.State, originalState)
	}
	if !inst.UpdatedAt.Equal(originalUpdatedAt) {
		t.Error("UpdatedAt changed")
	}
	if inst.StartedAt != originalStartedAt {
		t.Error("StartedAt changed")
	}
}

func TestServiceStartedAtSetOnce(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	t1 := now.Add(time.Second)
	inst.Transition(ServiceStateStarting, t1)
	inst.Transition(ServiceStateRunning, t1)

	if inst.StartedAt == nil {
		t.Fatal("StartedAt should be set after first running")
	}
	firstStartedAt := *inst.StartedAt

	t2 := now.Add(2 * time.Second)
	inst.Transition(ServiceStateStopping, t2)
	inst.Transition(ServiceStateStopped, t2)
	inst.Transition(ServiceStateStarting, t2)
	inst.Transition(ServiceStateRunning, t2)

	if inst.StartedAt == nil {
		t.Fatal("StartedAt should still be set")
	}
	if *inst.StartedAt != firstStartedAt {
		t.Errorf("StartedAt should not be overwritten: got %v, want %v", *inst.StartedAt, firstStartedAt)
	}
}

func TestServiceSnapshot(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance(
		"rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess,
		[]domain.ServiceID{"dep1", "dep2"}, now,
	)
	inst.SetMetadata("key", "value", now)

	snap := inst.Snapshot()

	if snap.ID != inst.ID {
		t.Errorf("ID mismatch")
	}
	if snap.RuntimeID != inst.RuntimeID {
		t.Errorf("RuntimeID mismatch")
	}
	if snap.PluginID != inst.PluginID {
		t.Errorf("PluginID mismatch")
	}
	if snap.ServiceID != inst.ServiceID {
		t.Errorf("ServiceID mismatch")
	}
	if snap.State != inst.State {
		t.Error("State mismatch")
	}
	if snap.Required != inst.Required {
		t.Error("Required mismatch")
	}
	if snap.ServiceKind != inst.ServiceKind {
		t.Error("ServiceKind mismatch")
	}
	if len(snap.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(snap.Dependencies))
	}
	if snap.Metadata["key"] != "value" {
		t.Errorf("Metadata mismatch")
	}
}

func TestServiceSnapshot_DeepCopy(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance(
		"rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess,
		[]domain.ServiceID{"dep1"}, now,
	)
	inst.SetMetadata("key", "original", now)

	snap := inst.Snapshot()

	snap.Dependencies[0] = "modified"
	snap.Metadata["key"] = "modified"

	if inst.Dependencies[0] != "dep1" {
		t.Error("modifying snapshot affected original Dependencies")
	}
	if inst.Metadata["key"] != "original" {
		t.Error("modifying snapshot affected original Metadata")
	}
}

func TestServiceSetMetadataRejectsEmptyKey(t *testing.T) {
	now := time.Now()
	inst, _ := NewServiceInstance("rt/svc", "rt", "plugin", "svc", true, domain.ServiceKindProcess, nil, now)

	err := inst.SetMetadata("", "value", now)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestIsTerminalServiceState(t *testing.T) {
	if !IsTerminalServiceState(ServiceStateStopped) {
		t.Error("expected stopped to be terminal")
	}
	if !IsTerminalServiceState(ServiceStateFailed) {
		t.Error("expected failed to be terminal")
	}
	if IsTerminalServiceState(ServiceStateRunning) {
		t.Error("running should not be terminal")
	}
	if IsTerminalServiceState(ServiceStateCreated) {
		t.Error("created should not be terminal")
	}
}
