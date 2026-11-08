package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestReadyCallbackCanAwaitSameConnectionResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	handler := NewHandler(nil, nil)
	resolved := make(chan struct{})
	handler.SetOnInvocationResult(func(result protocol.RuntimeResultPayload) {
		if result.InvocationID == "ready-role-query" {
			close(resolved)
		}
	})

	upgrader := websocket.Upgrader{}
	socketReady := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		socketReady <- socket
	}))
	defer server.Close()

	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	serverSocket := <-socketReady
	defer serverSocket.Close()

	mesh := NewMeshConnection(serverSocket, "ready-session", 3, "core", "device", "runtime")
	callbackDone := make(chan struct{})
	readyCallback := ReadyHandler(func(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) {
		defer close(callbackDone)
		if spaceID != "core" || deviceID != "device" {
			t.Errorf("unexpected ready identity: %s %s", spaceID, deviceID)
			return
		}
		if err := handler.sendEnvelope(mesh, protocol.MessageTypeRuntimeInvoke, protocol.RuntimeInvokePayload{InvocationID: "ready-role-query"}); err != nil {
			t.Errorf("send ready invocation: %v", err)
			return
		}
		select {
		case <-resolved:
		case <-ctx.Done():
			t.Errorf("ready callback waited for result: %v", ctx.Err())
		}
	})

	handler.dispatchReady(readyCallback, nil, "core", "device")

	loopDone := make(chan error, 1)
	go func() {
		loopDone <- handler.messageLoop(ctx, mesh, "ready-session", 3, 0)
	}()

	socket.SetReadDeadline(time.Now().Add(2 * time.Second))
	var invokeEnvelope protocol.Envelope
	if err := socket.ReadJSON(&invokeEnvelope); err != nil {
		t.Fatal(err)
	}
	if invokeEnvelope.MessageType != protocol.MessageTypeRuntimeInvoke {
		t.Fatalf("unexpected message type: %s", invokeEnvelope.MessageType)
	}

	resultPayload := protocol.RuntimeResultPayload{InvocationID: "ready-role-query", RuntimeSessionID: "ready-session", ConnectionGeneration: 3, DeviceID: "device", RuntimeID: "runtime"}
	resultBytes, err := json.Marshal(resultPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.WriteJSON(protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          protocol.MessageTypeRuntimeResult,
		SpaceID:              "core",
		DeviceID:             "device",
		RuntimeID:            "runtime",
		RuntimeSessionID:     "ready-session",
		ConnectionGeneration: 3,
		Sequence:             1,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(resultBytes),
		Payload:              resultBytes,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-callbackDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-loopDone:
	case <-time.After(time.Second):
		t.Fatal("message loop did not stop")
	}
}
