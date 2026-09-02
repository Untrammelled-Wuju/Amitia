package extension

import (
	"testing"
	"time"

	workflowmetrics "github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func TestWorkflowAndroidRuntimeHealthDefaultsToStale(t *testing.T) {
	runtime := &Runtime{}
	status := runtime.WorkflowAndroidRuntimeHealth()
	if !status.Stale {
		t.Fatal("zero Android runtime health must be stale until a device heartbeat arrives")
	}
}

func TestSetWorkflowAndroidRuntimeHealthNormalizesStateAndFreshness(t *testing.T) {
	runtime := &Runtime{}
	runtime.SetWorkflowAndroidRuntimeHealth(WorkflowAndroidRuntimeHealthStatus{
		DeviceID:          "  device-a  ",
		RuntimeReady:      true,
		NativeBridgeReady: true,
		InteractionState:  " waiting_unlock ",
	})

	status := runtime.WorkflowAndroidRuntimeHealth()
	if status.DeviceID != "device-a" {
		t.Fatalf("device id = %q, want device-a", status.DeviceID)
	}
	if status.InteractionState != "WAITING_UNLOCK" {
		t.Fatalf("interaction state = %q, want WAITING_UNLOCK", status.InteractionState)
	}
	if status.UpdatedAt.IsZero() || status.Stale {
		t.Fatalf("fresh device heartbeat must have updatedAt and not be stale: %+v", status)
	}
}

func TestWorkflowAndroidRuntimeHealthMarksExpiredHeartbeatStale(t *testing.T) {
	runtime := &Runtime{}
	runtime.workflowAndroidHealth = WorkflowAndroidRuntimeHealthStatus{
		RuntimeReady: true,
		UpdatedAt:    time.Now().UTC().Add(-workflowAndroidHealthStaleAfter - time.Second),
	}
	status := runtime.WorkflowAndroidRuntimeHealth()
	if !status.Stale {
		t.Fatal("expired Android runtime health heartbeat must be stale")
	}
}

func TestUpdateWorkflowWakeDeviceStatusPersistsKnownStates(t *testing.T) {
	runtime := &Runtime{}
	if err := runtime.updateWorkflowWakeDeviceStatus(" wake_suspended ", " realtime voice "); err != nil {
		t.Fatalf("update wake device status: %v", err)
	}
	state := runtime.workflowWakeState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.deviceState != "wake_suspended" || state.deviceReason != "realtime voice" {
		t.Fatalf("unexpected wake device state: %q / %q", state.deviceState, state.deviceReason)
	}
	if state.deviceUpdatedAt.IsZero() {
		t.Fatal("wake device state must record updatedAt")
	}
}

func TestUpdateWorkflowWakeDeviceStatusRejectsUnknownState(t *testing.T) {
	runtime := &Runtime{}
	if err := runtime.updateWorkflowWakeDeviceStatus("unlock_phone", ""); err == nil {
		t.Fatal("unknown wake device state must be rejected")
	}
}

func TestSetWorkflowAndroidRuntimeHealthEmitsRecoveryAndAccessibilityMetrics(t *testing.T) {
	previousDefault := workflowmetrics.DefaultWorkflowReliabilityMetrics
	metrics := workflowmetrics.NewWorkflowReliabilityMetrics()
	workflowmetrics.DefaultWorkflowReliabilityMetrics = metrics
	defer func() { workflowmetrics.DefaultWorkflowReliabilityMetrics = previousDefault }()

	runtime := &Runtime{}
	runtime.SetWorkflowAndroidRuntimeHealth(WorkflowAndroidRuntimeHealthStatus{
		RuntimeReady:           false,
		AccessibilityReady:     true,
		LastRuntimeFailureAtMS: 1000,
		RecoveryAttempt:        1,
		InteractionState:       "AVAILABLE",
	})
	runtime.SetWorkflowAndroidRuntimeHealth(WorkflowAndroidRuntimeHealthStatus{
		RuntimeReady:           false,
		AccessibilityReady:     false,
		LastRuntimeFailureAtMS: 2000,
		RecoveryAttempt:        2,
		RecoveryExhausted:      true,
		InteractionState:       "BLOCKED",
	})

	snapshot := metrics.Snapshot()
	if got := snapshot.Counters[workflowmetrics.MetricRuntimeCrashTotal]; got != 2 {
		t.Fatalf("runtime crash metric = %d, want 2", got)
	}
	if got := snapshot.Counters[workflowmetrics.MetricRuntimeRecoveryTotal]; got != 2 {
		t.Fatalf("runtime recovery metric = %d, want 2", got)
	}
	if got := snapshot.Counters[workflowmetrics.MetricRuntimeRecoveryExhaustedTotal]; got != 1 {
		t.Fatalf("runtime recovery exhausted metric = %d, want 1", got)
	}
	if got := snapshot.Counters[workflowmetrics.MetricAndroidAccessibilityDisconnect]; got != 1 {
		t.Fatalf("accessibility disconnect metric = %d, want 1", got)
	}
}
