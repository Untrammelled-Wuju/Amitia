package agent

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestCandidateGateAllowsOnlyRolePreflight(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	calls := 0
	handler := func(invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID}, nil
	}
	dispatcher.Register("coordination.data", handler)
	dispatcher.Register("browser.navigate", handler)
	gate := &bindingGate{dispatcher: dispatcher}
	for _, action := range []string{"roles", "apply", "interrupted", "snapshot"} {
		input, _ := json.Marshal(map[string]string{"operation": action})
		_, err := gate.Resolve("coordination.data")(protocol.RuntimeInvokePayload{Input: input})
		if (action == "roles") != (err == nil) {
			t.Fatalf("candidate gate %s: %v", action, err)
		}
	}
	if _, err := gate.Resolve("browser.navigate")(protocol.RuntimeInvokePayload{Input: json.RawMessage(`{"operation":"roles"}`)}); err == nil {
		t.Fatal("candidate executed device capability before cutover")
	}
	if calls != 1 {
		t.Fatalf("side effects before binding: %d", calls)
	}
	gate.active.Store(true)
	if _, err := gate.Resolve("browser.navigate")(protocol.RuntimeInvokePayload{}); err != nil {
		t.Fatal(err)
	}
}
