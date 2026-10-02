package nativebridge

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"testing"
)

type operationTimeoutTransport struct{ send func([]byte) error }

func (t operationTimeoutTransport) Send(data []byte) error { return t.send(data) }

func TestOperationTimeoutNativePropagation(t *testing.T) {
	settings := timeoutpolicy.Settings{Disabled: true, Seconds: 450}
	restore := timeoutpolicy.Configure(settings)
	defer restore()
	var session *productionRelaySession
	transport := operationTimeoutTransport{send: func(data []byte) error {
		var envelope RelayEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		var request Request
		if err := json.Unmarshal(envelope.Payload, &request); err != nil {
			return err
		}
		if request.TimeoutPolicy == nil || *request.TimeoutPolicy != settings {
			t.Fatal("native policy was not forwarded")
		}
		response, _ := json.Marshal(Response{RequestId: request.RequestId, Status: "success"})
		session.handleIncomingEnvelope(RelayEnvelope{Type: "native_bridge.response", RequestId: request.RequestId, Payload: response})
		return nil
	}}
	session = newRelaySession(transport, 1)
	result, err := session.SendRequest(context.Background(), Request{RequestId: "test", ProtocolVersion: 1, Platform: "android", Operation: "root.execute"})
	if err != nil || result.Status != "success" {
		t.Fatalf("native execution failed: %+v %v", result, err)
	}
}
