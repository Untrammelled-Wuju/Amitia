package execution

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestUnknownOutcomeReasonRequiresDispatchedSideEffect(t *testing.T) {
	tool := capability.ToolDefinition{HasSideEffects: true, SideEffect: capability.SideEffectExternal}
	timeout := capability.NewToolTimedOutResult("invocation", "mail.send")
	if reason := unknownOutcomeReason(tool, timeout, false); reason != "" {
		t.Fatalf("undispatched tool should not require reconciliation: %s", reason)
	}
	if reason := unknownOutcomeReason(tool, timeout, true); reason == "" {
		t.Fatal("dispatched side-effecting timeout must require reconciliation")
	}
	unknown := capability.NewToolFailureResult("invocation", "mail.send", &capability.ToolError{
		Code: "provider_outcome_unknown", Message: "provider outcome unknown",
	})
	if reason := unknownOutcomeReason(tool, unknown, true); reason == "" {
		t.Fatal("unknown provider outcome must require reconciliation")
	}
	readOnly := capability.ToolDefinition{SideEffect: capability.SideEffectReadOnly}
	if reason := unknownOutcomeReason(readOnly, timeout, true); reason != "" {
		t.Fatalf("read-only timeout should not require reconciliation: %s", reason)
	}
}
