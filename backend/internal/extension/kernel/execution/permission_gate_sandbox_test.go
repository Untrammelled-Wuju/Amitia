package execution

import (
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestSandboxAutoApprovalOnlyInsideSandbox(t *testing.T) {
	tool := capability.ToolDefinition{ID: "builtin.execute_host_command"}
	sandboxed := capability.ToolInvocationContext{ExecContext: &execution.ExecutionContext{Sandbox: &execution.SandboxConstraint{Root: "/tmp/jail", AllowNetwork: true}}}
	if !sandboxAutoApproval(tool, sandboxed) {
		t.Fatal("host command must be auto approved inside a channel sandbox")
	}
	if sandboxAutoApproval(tool, capability.ToolInvocationContext{}) {
		t.Fatal("host command must keep manual approval outside a sandbox")
	}
	if sandboxAutoApproval(tool, capability.ToolInvocationContext{ExecContext: &execution.ExecutionContext{}}) {
		t.Fatal("empty execution context must keep manual approval")
	}
	other := capability.ToolDefinition{ID: "builtin.press_key"}
	if sandboxAutoApproval(other, sandboxed) {
		t.Fatal("tools outside the sandbox-scoped list must keep manual approval")
	}
	if sandboxAutoApproval(tool, capability.ToolInvocationContext{ExecContext: &execution.ExecutionContext{Sandbox: &execution.SandboxConstraint{Root: "  "}}}) {
		t.Fatal("blank sandbox root must keep manual approval")
	}
}
