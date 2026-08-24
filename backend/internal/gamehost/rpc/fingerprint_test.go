package rpc

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestFingerprintDeterministic(t *testing.T) {
	fp1 := ComputeFingerprint("example.game.move", []byte(`{"x":1,"y":2}`))
	fp2 := ComputeFingerprint("example.game.move", []byte(`{"x":1,"y":2}`))
	if fp1 != fp2 {
		t.Errorf("fingerprint must be deterministic: %s != %s", fp1, fp2)
	}
}

func TestFingerprintDifferentPayloads(t *testing.T) {
	fp1 := ComputeFingerprint("example.game.move", []byte(`{"x":1}`))
	fp2 := ComputeFingerprint("example.game.move", []byte(`{"x":2}`))
	if fp1 == fp2 {
		t.Error("different payloads should produce different fingerprints")
	}
}

func TestFingerprintDifferentMethods(t *testing.T) {
	fp1 := ComputeFingerprint("example.game.move", []byte(`{"x":1}`))
	fp2 := ComputeFingerprint("example.game.action", []byte(`{"x":1}`))
	if fp1 == fp2 {
		t.Error("different methods should produce different fingerprints")
	}
}

func TestFingerprintNilPayload(t *testing.T) {
	fp1 := ComputeFingerprint("example.game.move", nil)
	fp2 := ComputeFingerprint("example.game.move", []byte{})
	if fp1 != fp2 {
		t.Error("nil and empty payload should produce same fingerprint")
	}
}

func TestComputeRequestFingerprint(t *testing.T) {
	env := protocol.Envelope{
		Method:  "test",
		Payload: json.RawMessage(`{"a":1}`),
	}
	fp := ComputeRequestFingerprint(env)
	if fp == "" {
		t.Error("fingerprint must not be empty")
	}
}

func TestFingerprintMetadataReservedExcluded(t *testing.T) {
	md1 := map[string]json.RawMessage{
		"rpc.timeout_ms": json.RawMessage(`5000`),
		"business.key":   json.RawMessage(`value1`),
	}
	md2 := map[string]json.RawMessage{
		"business.key": json.RawMessage(`value1`),
	}
	fp1 := FingerprintMetadata(md1)
	fp2 := FingerprintMetadata(md2)
	if fp1 != fp2 {
		t.Error("reserved keys should be excluded from fingerprint")
	}
}

func TestFingerprintMetadataStable(t *testing.T) {
	md := map[string]json.RawMessage{
		"zebra": json.RawMessage(`z`),
		"alpha": json.RawMessage(`a`),
		"mango": json.RawMessage(`m`),
	}
	fp1 := FingerprintMetadata(md)
	fp2 := FingerprintMetadata(md)
	if fp1 != fp2 {
		t.Error("fingerprint metadata must be stable across calls")
	}
}
