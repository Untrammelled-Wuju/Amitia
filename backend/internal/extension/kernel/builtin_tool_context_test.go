package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBuiltinToolExecutionContextCarriesExecutionIdentity(t *testing.T) {
	ctx := context.Background()
	execution := builtinToolExecutionContext(ctx, capability.ToolInvocationContext{
		OperationID:    "request-1",
		InvocationID:   "tool-call-1",
		ConversationID: "conversation-1",
		CharacterID:    "role-1",
		SpaceID:        "space-1",
		UserMessage:    "hello",
		CorrelationID:  "correlation-1",
		CausationID:    "causation-1",
		IdempotencyKey: "idempotency-1",
	})
	if execution.Context != ctx || execution.RequestID != "request-1" || execution.ToolCallID != "tool-call-1" || execution.ConversationID != "conversation-1" || execution.CharacterID != "role-1" || execution.SpaceID != "space-1" || execution.UserMessage != "hello" || execution.CorrelationID != "correlation-1" || execution.CausationID != "causation-1" || execution.IdempotencyKey != "idempotency-1" {
		t.Fatalf("builtin execution context lost invocation identity: %+v", execution)
	}
}
