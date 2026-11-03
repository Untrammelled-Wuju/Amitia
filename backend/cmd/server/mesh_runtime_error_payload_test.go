package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedWrongOwnerErrorPreservesContextAndCause(t *testing.T) {
	original := fmt.Errorf("resource owner was device-b: %w", coordination.ErrWrongOwner)
	err := contextualizeOwnedRuntimeError(original, business.Request{
		RequestID: "request-1", RoleID: "role-1", ConversationID: "conversation-1",
		DeviceID: "device-a", TargetDeviceID: "device-b", CoreID: "core-a",
	})
	if !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("wrong-owner identity was lost: %v", err)
	}
	for _, expected := range []string{"resource owner was device-b", "request-1", "role-1", "conversation-1", "device-a", "device-b", "core-a"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("context %q missing from %q", expected, err.Error())
		}
	}
	runtimeError := business.DescribeRuntimeError(err)
	payload := meshRuntimeErrorPayload("mesh.wrong_owner", runtimeError, map[string]any{"saved": false})
	for _, key := range []string{"code", "errorCode", "message", "userMessage", "internalMessage", "error", "detail", "cause", "rawError", "stage", "retryable", "data"} {
		if _, exists := payload[key]; !exists {
			t.Fatalf("runtime error payload missing %q: %#v", key, payload)
		}
	}
	if payload["rawError"] != err.Error() || !strings.Contains(fmt.Sprint(payload["cause"]), err.Error()) {
		t.Fatalf("raw error fields do not preserve the wrapped error: %#v", payload)
	}
}
