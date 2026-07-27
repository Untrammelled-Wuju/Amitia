package update

import (
	"context"
	"strings"
	"testing"
)

func setupTestRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		journal:    NewJournalManager(nil),
		executor:   nil,
		planner:    nil,
		repo:       nil,
		migrations: nil,
	}
}

func TestRecoveryManager_DecideStrategy(t *testing.T) {
	m := setupTestRecoveryManager()
	cases := []struct {
		name     string
		action   string
		expected string
	}{
		{"resume", "resume", "resume"},
		{"compensate", "compensate", "compensate"},
		{"rollback", "rollback", "rollback"},
		{"manual_intervention", "manual_intervention", "manual_intervention"},
		{"unknown_defaults_to_manual", "unknown", "manual_intervention"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			item := RecoveryItem{Action: c.action}
			got := m.decideStrategy(item)
			if got != c.expected {
				t.Errorf("decideStrategy(Action=%q) = %q, want %q", c.action, got, c.expected)
			}
		})
	}
}

func TestRecoveryManager_ExecuteRecovery_ManualIntervention(t *testing.T) {
	m := setupTestRecoveryManager()
	action := RecoveryAction{
		OperationID: "op-manual",
		Strategy:    "manual_intervention",
		Detail:      "stuck in canary stage",
	}
	err := m.ExecuteRecovery(context.Background(), action)
	if err == nil {
		t.Fatalf("expected error for manual_intervention strategy, got nil")
	}
	if !strings.Contains(err.Error(), "requires manual intervention") {
		t.Errorf("expected error to contain 'requires manual intervention', got: %v", err)
	}
}

func TestRecoveryManager_ExecuteRecovery_UnknownStrategy(t *testing.T) {
	m := setupTestRecoveryManager()
	action := RecoveryAction{
		OperationID: "op-unknown",
		Strategy:    "unknown",
	}
	err := m.ExecuteRecovery(context.Background(), action)
	if err == nil {
		t.Fatalf("expected error for unknown strategy, got nil")
	}
	if !strings.Contains(err.Error(), "unknown recovery strategy") {
		t.Errorf("expected error to contain 'unknown recovery strategy', got: %v", err)
	}
}

func TestJournalManager_ClassifyOperationState(t *testing.T) {
	cases := []struct {
		name     string
		stepType JournalStepType
		expected string
	}{
		{"migration_plan", JournalStepMigrationPlan, "migration_running"},
		{"canary_start", JournalStepCanaryStart, "canary_running"},
		{"generation_switch", JournalStepGenerationSwitch, "activating"},
		{"rollback_commit", JournalStepRollbackCommit, "committing"},
		{"rollback_execute", JournalStepRollbackExecute, "rollback_running"},
		{"snapshot_create", JournalStepSnapshotCreate, "snapshot_restoring"},
		{"unknown_step", JournalStepType("unknown"), "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyOperationState(c.stepType)
			if got != c.expected {
				t.Errorf("classifyOperationState(%q) = %q, want %q", c.stepType, got, c.expected)
			}
		})
	}
}

func TestJournalManager_DecideRecoveryAction(t *testing.T) {
	cases := []struct {
		state    string
		expected string
	}{
		{"migration_running", "compensate"},
		{"canary_running", "rollback"},
		{"activating", "rollback"},
		{"committing", "rollback"},
		{"rollback_running", "resume"},
		{"snapshot_restoring", "resume"},
		{"unknown", "manual_intervention"},
	}
	for _, c := range cases {
		t.Run(c.state, func(t *testing.T) {
			got := decideRecoveryAction(c.state)
			if got != c.expected {
				t.Errorf("decideRecoveryAction(%q) = %q, want %q", c.state, got, c.expected)
			}
		})
	}
}

func TestRecoveryManager_ApplyCompensation_NilDefinition(t *testing.T) {
	m := setupTestRecoveryManager()
	err := m.applyCompensation(context.Background(), "op-1", nil)
	if err == nil {
		t.Fatalf("expected error for nil compensation definition, got nil")
	}
	if !strings.Contains(err.Error(), "compensation definition is nil") {
		t.Errorf("expected error to contain 'compensation definition is nil', got: %v", err)
	}
}

func TestRecoveryManager_ApplyCompensation_UnsupportedAction(t *testing.T) {
	m := setupTestRecoveryManager()
	comp := &CompensationDefinition{Action: "unsupported"}
	err := m.applyCompensation(context.Background(), "op-1", comp)
	if err == nil {
		t.Fatalf("expected error for unsupported compensation action, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported compensation type") {
		t.Errorf("expected error to contain 'unsupported compensation type', got: %v", err)
	}
}

func TestRecoveryManager_ApplyCompensation_RestoreSnapshot_NoTarget(t *testing.T) {
	m := &RecoveryManager{
		journal:    NewJournalManager(nil),
		migrations: NewMigrationExecutor(),
	}
	comp := &CompensationDefinition{Action: "restore_snapshot", Target: ""}
	err := m.applyCompensation(context.Background(), "op-1", comp)
	if err == nil {
		t.Fatalf("expected error for restore_snapshot with empty target, got nil")
	}
	if !strings.Contains(err.Error(), "snapshot ID not specified") {
		t.Errorf("expected error to contain 'snapshot ID not specified', got: %v", err)
	}
}

func TestRecoveryManager_ApplyCompensation_CallEndpoint_NoTarget(t *testing.T) {
	m := setupTestRecoveryManager()
	comp := &CompensationDefinition{Action: "call_compensation_endpoint", Target: ""}
	err := m.applyCompensation(context.Background(), "op-1", comp)
	if err == nil {
		t.Fatalf("expected error for call_compensation_endpoint with empty target, got nil")
	}
	if !strings.Contains(err.Error(), "endpoint not specified") {
		t.Errorf("expected error to contain 'endpoint not specified', got: %v", err)
	}
}

func TestRecoveryManager_IsRecoveryRequired_NoRepo(t *testing.T) {
	m := setupTestRecoveryManager()
	required, err := m.IsRecoveryRequired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error when repo is nil: %v", err)
	}
	if required {
		t.Errorf("expected recovery not required when journal repo is nil, got required=true")
	}
}
