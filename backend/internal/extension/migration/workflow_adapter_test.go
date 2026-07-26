package migration

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension"
)

func TestWorkflowToDefinition(t *testing.T) {
	wf := extension.WorkflowDefinition{
		SchemaVersion: "1.0",
		Steps: []extension.WorkflowStep{
			{
				ID:   "step-1",
				Type: "skill",
				Input: json.RawMessage(`{"skillId":"test"}`),
				OnError: extension.WorkflowErrorPolicy{
					Mode:    "retry",
					Default: json.RawMessage(`{"fallback":"ok"}`),
				},
			},
		},
		Output: json.RawMessage(`{"result":"done"}`),
		Limits: extension.WorkflowLimits{
			MaxSteps:       10,
			MaxSkillCalls:  5,
			MaxInputBytes:  1048576,
			MaxOutputBytes: 1048576,
		},
	}

	result := WorkflowToDefinition(wf, "wf-001", "test-wf", "A test workflow", "ext-1", "")

	if result.ID != "wf-001" {
		t.Fatalf("expected ID wf-001, got %s", result.ID)
	}
	if result.Name != "test-wf" {
		t.Fatalf("expected name test-wf, got %s", result.Name)
	}
	if result.CallableByAgent {
		t.Fatal("expected CallableByAgent false for regular conversion")
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result.Nodes))
	}
	if result.Nodes[0].ID != "step-1" {
		t.Fatalf("expected node ID step-1, got %s", result.Nodes[0].ID)
	}
	if result.Nodes[0].Step.OnError.Mode != "retry" {
		t.Fatalf("expected error mode retry, got %s", result.Nodes[0].Step.OnError.Mode)
	}
	if result.Limits.MaxSteps != 10 {
		t.Fatalf("expected MaxSteps 10, got %d", result.Limits.MaxSteps)
	}
	if !result.Enabled {
		t.Fatal("expected enabled true")
	}
}

func TestWorkflowToCallable(t *testing.T) {
	wf := extension.WorkflowDefinition{
		SchemaVersion: "1.0",
		Steps:         []extension.WorkflowStep{},
		Output:        json.RawMessage(`{}`),
		Limits:        extension.WorkflowLimits{MaxSteps: 5},
	}

	result := WorkflowToCallable(wf, "wf-002", "callable-wf", "A callable workflow", "ext-2", "mod-1")

	if !result.CallableByAgent {
		t.Fatal("expected CallableByAgent true")
	}
}

func TestWorkflowToDefinitionEmptyOutput(t *testing.T) {
	wf := extension.WorkflowDefinition{
		SchemaVersion: "1.0",
		Steps:         []extension.WorkflowStep{},
		Output:        nil,
		Limits:        extension.WorkflowLimits{MaxSteps: 5},
	}

	result := WorkflowToDefinition(wf, "wf-003", "empty-output", "test", "", "")

	if len(result.OutputSchema) == 0 {
		t.Fatal("expected non-empty OutputSchema")
	}
}
