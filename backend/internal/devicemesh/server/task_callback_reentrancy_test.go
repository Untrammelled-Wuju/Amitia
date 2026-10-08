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
)

func TestTaskCallbackCanAwaitSameConnectionResultAndErrorsKeepSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	handler := NewHandler(nil, nil)
	resolved := make(chan struct{})
	connection := make(chan *MeshConnection, 1)
	handler.SetOnInvocationResult(func(protocol.RuntimeResultPayload) { close(resolved) })
	handler.SetOnTaskClaimPayload(func(protocol.TaskClaimPayload) bool {
		mesh := <-connection
		if handler.sendEnvelope(mesh, protocol.MessageTypeRuntimeInvoke, protocol.RuntimeInvokePayload{InvocationID: "role-query"}) != nil {
			return false
		}
		select {
		case <-resolved:
			return true
		case <-ctx.Done():
			return false
		}
	})
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		socket, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer socket.Close()
		mesh := NewMeshConnection(socket, "session", 7, "core", "device", "runtime")
		connection <- mesh
		_ = handler.messageLoop(ctx, mesh, "session", 7, 0)
	}))
	defer server.Close()
	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	socket.SetReadDeadline(time.Now().Add(5 * time.Second))
	sequence := int64(0)
	send := func(kind protocol.MessageType, payload any) {
		t.Helper()
		sequence++
		body, _ := json.Marshal(payload)
		err := socket.WriteJSON(protocol.Envelope{EnvelopeVersion: meshprotocol.EnvelopeVersion, Protocol: meshprotocol.ProtocolName, MessageType: kind, SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7, Sequence: sequence, PayloadSchemaVersion: 1, PayloadHash: protocol.ComputePayloadHash(body), Payload: body})
		if err != nil {
			t.Fatal(err)
		}
	}
	read := func(kind protocol.MessageType, expected int64) protocol.Envelope {
		t.Helper()
		var envelope protocol.Envelope
		if err := socket.ReadJSON(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.MessageType != kind || envelope.Sequence != expected || envelope.SpaceID != "core" || envelope.DeviceID != "device" || envelope.RuntimeID != "runtime" || envelope.RuntimeSessionID != "session" || envelope.ConnectionGeneration != 7 || !envelope.VerifyPayloadHash() {
			t.Fatalf("任务回调或错误响应改变了连接身份与顺序: %+v", envelope)
		}
		return envelope
	}
	send(protocol.MessageTypeTaskClaim, protocol.TaskClaimPayload{TaskRunID: "task", AttemptID: "attempt", LeaseID: "lease", WorkerID: "runtime", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7, LeaseDurationMs: 300000})
	read(protocol.MessageTypeRuntimeInvoke, 1)
	send(protocol.MessageTypeRuntimeResult, protocol.RuntimeResultPayload{InvocationID: "role-query"})
	lease := read(protocol.MessageTypeTaskLeaseAck, 2)
	var ack protocol.TaskLeaseAckPayload
	if json.Unmarshal(lease.Payload, &ack) != nil || !ack.Accepted {
		t.Fatal("任务回调等待同一连接响应时阻塞了租约确认")
	}
	send(protocol.MessageTypeError, protocol.ErrorPayload{Code: "unsupported"})
	read(protocol.MessageTypeError, 3)
}
