package server

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/credential"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestFreshHelloRequiresAuthenticatedIdentityAndValidHashBeforeSessionExists(t *testing.T) {
	handler := NewHandler(nil, nil)
	principal := credential.DeviceRuntimePrincipal{SpaceID: "core", DeviceID: "device", RuntimeID: "runtime"}
	body, _ := json.Marshal(protocol.HelloPayload{DeviceID: "device", RuntimeID: "runtime", RuntimeContractVersion: meshprotocol.RuntimeContractVersion})
	envelope := protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, MessageType: protocol.MessageTypeHello, MessageID: "hello", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", ConnectionGeneration: 1, Sequence: 1, PayloadSchemaVersion: 1, Payload: body, PayloadHash: protocol.ComputePayloadHash(body)}
	encoded, _ := json.Marshal(envelope)
	if _, _, err := handler.parseHello(encoded, principal); err != nil {
		t.Fatalf("fresh authenticated hello requires an existing session: %v", err)
	}
	envelope.PayloadHash = protocol.ComputePayloadHash(json.RawMessage(`{}`))
	encoded, _ = json.Marshal(envelope)
	if _, _, err := handler.parseHello(encoded, principal); err == nil {
		t.Fatal("modified hello body accepted")
	}
	body, _ = json.Marshal(protocol.HelloPayload{DeviceID: "another-device", RuntimeID: "runtime", RuntimeContractVersion: meshprotocol.RuntimeContractVersion})
	envelope.Payload, envelope.PayloadHash = body, protocol.ComputePayloadHash(body)
	encoded, _ = json.Marshal(envelope)
	if _, _, err := handler.parseHello(encoded, principal); err == nil {
		t.Fatal("hello payload identity mismatch accepted")
	}
	envelope.MessageType = protocol.MessageTypeRuntimeResult
	if err := envelope.ValidateEstablishedSession(meshprotocol.RuntimeProtocolDescriptor); err == nil {
		t.Fatal("established traffic without session accepted")
	}
}
