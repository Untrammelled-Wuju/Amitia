package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestLiveRevocationStopsClientWithoutReconnect(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_, raw, err := connection.ReadMessage()
		if err != nil {
			return
		}
		var hello protocol.Envelope
		if json.Unmarshal(raw, &hello) != nil {
			return
		}
		envelope := protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, SpaceID: hello.SpaceID, DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "session-a", ConnectionGeneration: 1, PayloadSchemaVersion: 1}
		ack, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "session-a", ResumeMode: protocol.ResumeModeFresh, ServerTime: time.Now()})
		envelope.MessageType, envelope.Sequence, envelope.Payload, envelope.PayloadHash = protocol.MessageTypeHelloAck, 1, ack, protocol.ComputePayloadHash(ack)
		if connection.WriteJSON(envelope) != nil {
			return
		}
		revoked, _ := json.Marshal(protocol.ErrorPayload{Code: "mesh.credential_revoked", Message: "权限已撤销"})
		envelope.MessageType, envelope.Sequence, envelope.Payload, envelope.PayloadHash = protocol.MessageTypeError, 2, revoked, protocol.ComputePayloadHash(revoked)
		if connection.WriteJSON(envelope) != nil {
			return
		}
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	client := NewMeshClient(MeshClientConfig{CloudBaseURL: server.URL, Credential: "secret", SpaceID: "core", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}})
	client.Start()
	defer client.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := client.WaitStopped(ctx); err != nil {
		t.Fatal("revoked client kept running", err)
	}
	if client.State() != StateRevoked || connections.Load() != 1 {
		t.Fatalf("revoked state=%s connections=%d", client.State(), connections.Load())
	}
}

func TestProvisionedClientReadsHandshakeAndStopsBlockedConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	connected := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != meshprotocol.WebSocketPath || request.Header.Get("Authorization") != "AmitiaDevice test-device-secret" {
			t.Error("设备通道请求不正确")
			return
		}
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_, data, err := connection.ReadMessage()
		if err != nil {
			return
		}
		var hello protocol.Envelope
		if err := json.Unmarshal(data, &hello); err != nil || hello.MessageType != protocol.MessageTypeHello || !hello.VerifyPayloadHash() {
			t.Error("未收到设备握手")
			return
		}
		payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "session-a", ResumeMode: protocol.ResumeModeFresh, ServerTime: time.Now()})
		ack := protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, MessageType: protocol.MessageTypeHelloAck, SpaceID: hello.SpaceID, DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "session-a", ConnectionGeneration: 1, Sequence: 1, PayloadSchemaVersion: 1, Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)}
		if err := connection.WriteJSON(ack); err != nil {
			return
		}
		connected <- struct{}{}
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	client := NewMeshClient(MeshClientConfig{CloudBaseURL: server.URL, Credential: "test-device-secret", SpaceID: "core-b", Identity: &LocalIdentity{DeviceID: "device-a", RuntimeID: "runtime-a"}})
	client.Start()
	defer client.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.WaitReady(ctx); err != nil {
		t.Fatalf("握手没有进入可用状态: %v", err)
	}
	select {
	case <-connected:
	default:
		t.Fatal("未完成真实握手")
	}
	client.Stop()
	client.Stop()
	if err := client.WaitStopped(ctx); err != nil {
		t.Fatal("读取阻塞的通道未及时停止")
	}
}

func TestClientRejectsHandshakeFromWrongProvider(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_, _, _ = connection.ReadMessage()
		payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "session-a", ResumeMode: protocol.ResumeModeFresh})
		_ = connection.WriteJSON(protocol.Envelope{MessageType: protocol.MessageTypeHelloAck, SpaceID: "wrong-core", DeviceID: "device-a", RuntimeID: "runtime-a", ConnectionGeneration: 1, Sequence: 1, Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)})
	}))
	defer server.Close()
	client := NewMeshClient(MeshClientConfig{CloudBaseURL: server.URL, Credential: "secret", SpaceID: "core-b", Identity: &LocalIdentity{DeviceID: "device-a", RuntimeID: "runtime-a"}})
	err := client.connectAndServe()
	if err == nil || client.State() == StateReady {
		t.Fatal("错误 Core 的握手被接受")
	}
}

func TestFullResumeResetsCursorAndCannotPassReadinessBarrier(t *testing.T) {
	client := NewMeshClient(MeshClientConfig{Credential: "secret", SpaceID: "core", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}, Cursor: &SessionCursor{RuntimeSessionID: "old", LastEventSequence: 50}})
	payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "new", ResumeMode: protocol.ResumeModeFull})
	client.handleHelloAck(&protocol.Envelope{Sequence: 1, ConnectionGeneration: 2, Payload: payload})
	if client.State() != StateDegraded || client.handshakeErr == nil || client.conf.Cursor.RuntimeSessionID != "" || client.conf.Cursor.LastEventSequence != 0 {
		t.Fatal("full resume became usable without resetting uncertain state")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := client.WaitReady(ctx); err == nil {
		t.Fatal("degraded connection passed provider cutover barrier")
	}
}
