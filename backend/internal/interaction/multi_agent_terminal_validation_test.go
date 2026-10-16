package interaction

import "testing"

func TestTerminalResultMatchesDurableChildAndAssignment(t *testing.T) {
	target := &AgentAssignment{ID: "assignment-1", ChildInteractionID: "child-1", Status: AssignmentRunning}
	valid := AgentAssignmentResult{AssignmentID: "assignment-1", ChildInteractionID: "child-1", Status: AssignmentSucceeded}
	if err := validateTerminalAssignmentResult(target, target.ID, valid); err != nil {
		t.Fatalf("valid terminal callback rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*AgentAssignmentResult)
	}{
		{"wrong assignment", func(r *AgentAssignmentResult) { r.AssignmentID = "assignment-2" }},
		{"wrong child", func(r *AgentAssignmentResult) { r.ChildInteractionID = "child-2" }},
		{"missing child", func(r *AgentAssignmentResult) { r.ChildInteractionID = "" }},
		{"not terminal", func(r *AgentAssignmentResult) { r.Status = AssignmentRunning }},
		{"empty status", func(r *AgentAssignmentResult) { r.Status = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			tc.mutate(&candidate)
			if err := validateTerminalAssignmentResult(target, target.ID, candidate); err == nil {
				t.Fatal("mismatched or nonterminal callback accepted")
			}
		})
	}
	completed := *target
	completed.Status = AssignmentSucceeded
	if err := validateTerminalAssignmentResult(&completed, target.ID, valid); err != nil {
		t.Fatalf("idempotent callback rejected: %v", err)
	}
	valid.Status = AssignmentFailed
	if err := validateTerminalAssignmentResult(&completed, target.ID, valid); err == nil {
		t.Fatal("conflicting terminal replay accepted")
	}
}
