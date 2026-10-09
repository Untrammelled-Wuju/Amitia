package nativebridge

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIOSGuestRealWebSocketRelayCarriesOriginalNativeRequest(t *testing.T) {
	bridge := NewIOSBridge()
	handler := NewRelayHandler()
	handler.RegisterBridge("ios", bridge)
	router := gin.New()
	router.GET("/relay", handler.HandleWebSocket)
	server := httptest.NewServer(router)
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/relay?platform=ios", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	var hello RelayEnvelope
	if err := connection.ReadJSON(&hello); err != nil || hello.Platform != "ios" || hello.Generation == 0 {
		t.Fatalf("invalid host handshake: %v", err)
	}
	type outcome struct {
		response Response
		err      error
	}
	done := make(chan outcome, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		response, err := bridge.Execute(ctx, Request{ProtocolVersion: 1, Platform: "ios", RequestId: "source-native-original", Operation: "calendar.list", Payload: map[string]any{"limit": 3}})
		done <- outcome{response, err}
	}()
	var envelope RelayEnvelope
	if err := connection.ReadJSON(&envelope); err != nil {
		t.Fatal(err)
	}
	var request Request
	if json.Unmarshal(envelope.Payload, &request) != nil || request.Platform != "ios" || request.RequestId != "source-native-original" || request.Operation != "calendar.list" || envelope.Generation != hello.Generation {
		t.Fatal("original native request changed in real relay")
	}
	payload, _ := json.Marshal(Response{ProtocolVersion: 1, RequestId: request.RequestId, Status: "success", Result: map[string]any{"events": []any{}}})
	if err := connection.WriteJSON(RelayEnvelope{Type: "native_bridge.response", Platform: "ios", Generation: hello.Generation, RequestId: request.RequestId, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.err != nil || result.response.Status != "success" || result.response.RequestId != request.RequestId {
		t.Fatalf("real native relay failed: %v", result.err)
	}
	connection.Close()
	deadline := time.Now().Add(2 * time.Second)
	for bridge.SessionAttached() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if bridge.SessionAttached() || bridge.Health(context.Background()) == HealthReady {
		t.Fatal("disconnected native relay remained ready")
	}
}
