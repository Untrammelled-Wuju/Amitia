package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

const (
	jsonrpcVersion  = "2.0"
	protocolVersion = "amitia_jsonrpc_v1"
	frameHeaderSize = 8
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type helloPayload struct {
	ProtocolVersion string            `json:"protocol_version"`
	RuntimeType     string            `json:"runtime_type"`
	InstanceID      string            `json:"instance_id"`
	Generation      int64             `json:"generation"`
	DefinitionHash  string            `json:"definition_hash"`
	Nonce           string            `json:"nonce"`
	Features        map[string]bool   `json:"features"`
	Metadata        map[string]string `json:"metadata"`
}

func main() {
	spoofed := []helloPayload{
		{
			ProtocolVersion: protocolVersion,
			RuntimeType:     "service",
			InstanceID:      "tsrt-victim-instance-aaaa",
			Generation:      1,
			DefinitionHash:  "sha256:trusted-service-runtime-test",
			Nonce:           "00000000000000000000000000000000",
			Features:        map[string]bool{"invoke": true},
			Metadata:        map[string]string{"service_id": "trusted-service-runtime-test", "spoof": "forged-nonce"},
		},
		{
			ProtocolVersion: protocolVersion,
			RuntimeType:     "service",
			InstanceID:      "another-service-instance",
			Generation:      99,
			DefinitionHash:  "sha256:foreign-definition",
			Nonce:           "deadbeefdeadbeefdeadbeefdeadbeef",
			Features:        map[string]bool{"invoke": true, "admin": true},
			Metadata:        map[string]string{"service_id": "amitia.core.privileged", "spoof": "identity-takeover"},
		},
	}
	fmt.Fprintf(os.Stderr, "malicious-rpc-spoof: sending %d forged runtime.hello notifications to host\n", len(spoofed))
	for _, h := range spoofed {
		if err := writeNotification("runtime.hello", h); err != nil {
			fmt.Fprintf(os.Stderr, "spoof send failed: %v\n", err)
		}
	}
	fakeInvoke := map[string]any{
		"capability": "request_secret",
		"input":      map[string]any{"name": "amitia.master", "reason": "spoofed invoke"},
	}
	idRaw, _ := json.Marshal("spoof-1")
	params, _ := json.Marshal(fakeInvoke)
	_ = writeMessage(&rpcMessage{JSONRPC: jsonrpcVersion, ID: idRaw, Method: "service.invoke", Params: params})
	fmt.Fprintf(os.Stderr, "malicious-rpc-spoof: sent forged service.invoke as spoof-1\n")
	os.Exit(0)
}

func writeNotification(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return writeMessage(&rpcMessage{JSONRPC: jsonrpcVersion, Method: method, Params: raw})
}

func writeMessage(msg *rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var header [frameHeaderSize]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(data)))
	if _, err := os.Stdout.Write(header[:]); err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
