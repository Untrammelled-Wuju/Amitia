package main

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestWorkerToolAllowedByRole(t *testing.T) {
	read := capability.ToolDefinition{ModelName: "workspace_read", Name: "Read workspace", SideEffect: capability.SideEffectReadOnly}
	write := capability.ToolDefinition{ModelName: "workspace_write", Name: "Write workspace", SideEffect: capability.SideEffectWrite}
	device := capability.ToolDefinition{ModelName: "android_interaction_tap", Name: "Tap device", SideEffect: capability.SideEffectExternal}
	financial := capability.ToolDefinition{ModelName: "billing_transfer", Name: "Transfer funds", SideEffect: capability.SideEffectFinancial}
	cases := []struct {
		role string
		tool capability.ToolDefinition
		want bool
	}{
		{"research_worker", read, true},
		{"research_worker", write, false},
		{"review_worker", read, true},
		{"review_worker", write, false},
		{"operator_worker", device, true},
		{"operator_worker", write, false},
		{"coding_worker", write, true},
		{"coding_worker", device, false},
		{"coding_worker", financial, false},
	}
	for _, tc := range cases {
		if got := workerToolAllowed(tc.role, tc.tool); got != tc.want {
			t.Errorf("workerToolAllowed(%q, %q)=%v want %v", tc.role, tc.tool.ModelName, got, tc.want)
		}
	}
}

func TestValidWorkerRoleRejectsUnknownRole(t *testing.T) {
	for _, role := range []string{"research_worker", "coding_worker", "review_worker", "operator_worker"} {
		if !validWorkerRole(role) {
			t.Fatalf("role %q should be valid", role)
		}
	}
	if validWorkerRole("admin_worker") {
		t.Fatal("unknown role should be rejected")
	}
}
