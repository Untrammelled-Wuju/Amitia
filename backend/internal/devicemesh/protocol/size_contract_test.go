package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	runtimeprotocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestOwnedPayloadHasRoomForRuntimeEnvelope(t *testing.T) {
	input := json.RawMessage(`{"content":"` + strings.Repeat("x", (4<<20)-64) + `"}`)
	for _, payload := range []any{
		runtimeprotocol.RuntimeInvokePayload{InvocationID: "invocation", Input: input, Handler: "coordination.data"},
		runtimeprotocol.RuntimeResultPayload{InvocationID: "invocation", Result: input},
	} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := json.Marshal(runtimeprotocol.Envelope{Payload: encoded})
		if err != nil || len(envelope) > MaxMessageSizeBytes {
			t.Fatalf("owned data cannot fit transport envelope: %d %v", len(envelope), err)
		}
	}
}
