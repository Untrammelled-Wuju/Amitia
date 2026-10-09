package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

type iosGuestTransport struct{ send func([]byte) error }

func (t iosGuestTransport) Send(payload []byte) error { return t.send(payload) }

func TestIOSGuestBridgeOriginalRequestAndGeneration(t *testing.T) {
	bridge := NewIOSBridge()
	if bridge.Health(context.Background()) == HealthReady {
		t.Fatal("unattached host is ready")
	}
	var lastEnvelope RelayEnvelope
	transport := iosGuestTransport{send: func(payload []byte) error {
		if err := json.Unmarshal(payload, &lastEnvelope); err != nil {
			return err
		}
		var request Request
		if err := json.Unmarshal(lastEnvelope.Payload, &request); err != nil {
			return err
		}
		if request.Platform != "ios" || request.RequestId != "original-request" {
			return fmt.Errorf("request changed")
		}
		response, _ := json.Marshal(Response{ProtocolVersion: 1, RequestId: request.RequestId, Status: "success", Result: map[string]any{"confirmed": true}})
		envelope, _ := json.Marshal(RelayEnvelope{Type: "native_bridge.response", Platform: "ios", Generation: lastEnvelope.Generation, RequestId: request.RequestId, Payload: response})
		return bridge.HandleRelayEnvelope(envelope)
	}}
	first := bridge.AttachRelaySession(transport)
	response, err := bridge.Execute(context.Background(), Request{ProtocolVersion: 1, Platform: "ios", RequestId: "original-request", Operation: "calendar.list"})
	if err != nil || response.Status != "success" || lastEnvelope.Generation != first {
		t.Fatalf("original host call failed: %v", err)
	}
	second := bridge.AttachRelaySession(transport)
	bridge.DetachRelaySession(first)
	if !bridge.SessionAttached() || bridge.Generation() != second {
		t.Fatal("old detach removed current host")
	}
	old, _ := json.Marshal(RelayEnvelope{Type: "native_bridge.response", Generation: first, RequestId: "original-request", Payload: json.RawMessage(`{}`)})
	if err := bridge.HandleRelayEnvelope(old); err == nil {
		t.Fatal("old generation accepted")
	}
	bridge.DetachRelaySession(second)
	if _, err := bridge.Execute(context.Background(), Request{ProtocolVersion: 1, Platform: "ios", RequestId: "original-request", Operation: "calendar.list"}); err == nil {
		t.Fatal("detached host call accepted")
	}
	late, _ := json.Marshal(RelayEnvelope{Type: "native_bridge.event", Generation: second, Payload: json.RawMessage(`{}`)})
	if err := bridge.HandleRelayEnvelope(late); err == nil {
		t.Fatal("detached host event accepted")
	}
}

func TestIOSGuestBridgeRejectsMismatchedResponseAndPlatform(t *testing.T) {
	bridge := NewIOSBridge()
	sent := 0
	bridge.AttachRelaySession(iosGuestTransport{send: func(payload []byte) error {
		sent++
		var envelope RelayEnvelope
		json.Unmarshal(payload, &envelope)
		response, _ := json.Marshal(Response{ProtocolVersion: 1, RequestId: "wrong-request", Status: "success"})
		envelope.Type = "native_bridge.response"
		envelope.Payload = response
		encoded, _ := json.Marshal(envelope)
		return bridge.HandleRelayEnvelope(encoded)
	}})
	if _, err := bridge.Execute(context.Background(), Request{ProtocolVersion: 1, Platform: "android", RequestId: "original", Operation: "calendar.list"}); err == nil || sent != 0 {
		t.Fatal("wrong platform executed")
	}
	if _, err := bridge.Execute(context.Background(), Request{ProtocolVersion: 1, Platform: "ios", RequestId: "original", Operation: "calendar.list"}); err == nil || sent != 1 {
		t.Fatal("mismatched response accepted")
	}
}

func TestIOSGuestBridgeHealthDuringReconnect(t *testing.T) {
	bridge := NewIOSBridge()
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 100 {
			generation := bridge.AttachRelaySession(iosGuestTransport{send: func([]byte) error { return nil }})
			bridge.DetachRelaySession(generation)
		}
	}()
	go func() {
		defer workers.Done()
		for range 100 {
			bridge.Health(context.Background())
			bridge.SessionAttached()
			bridge.Generation()
		}
	}()
	workers.Wait()
}
