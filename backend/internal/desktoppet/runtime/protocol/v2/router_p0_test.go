package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type oneTimeTicketConsumer struct {
	mu   sync.Mutex
	used map[string]bool
}

func (c *oneTimeTicketConsumer) consume(_ context.Context, ticket string, runtimeID runtimeidentity.RuntimeID, deviceID runtimeidentity.DeviceID) (runtimeidentity.UserID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ticket == "" || runtimeID != "runtime-ws" || deviceID != "device-ws" || c.used[ticket] {
		return "", errors.New("ticket rejected")
	}
	c.used[ticket] = true
	return runtimeidentity.ParseUserID("user-ws"), nil
}

func newRuntimeWSTestServer(t *testing.T) (*httptest.Server, *RuntimeFacade) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&RuntimeSession{}, &RuntimeCommand{}, &EventRecord{}, &RuntimeActualState{}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultFacadeConfig()
	cfg.Path = "/runtime/ws"
	cfg.LoopbackOnly = false
	facade := NewRuntimeFacade(db, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := facade.Start(ctx); err != nil {
		t.Fatal(err)
	}
	consumer := &oneTimeTicketConsumer{used: map[string]bool{}}
	router := gin.New()
	RegisterInternalRoutes(router, facade, nil, consumer.consume)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, facade
}

func TestParseRuntimeBootstrapSubprotocolRejectsAmbiguousOrMissingCredentials(t *testing.T) {
	tests := []struct {
		name       string
		protocols  []string
		wantTicket string
		wantOK     bool
	}{
		{name: "valid", protocols: []string{runtimeV2WebSocketSubprotocol, runtimeV2BootstrapProtocolPrefix + "ticket-1"}, wantTicket: "ticket-1", wantOK: true},
		{name: "missing runtime protocol", protocols: []string{runtimeV2BootstrapProtocolPrefix + "ticket-1"}, wantOK: false},
		{name: "missing bootstrap protocol", protocols: []string{runtimeV2WebSocketSubprotocol}, wantOK: false},
		{name: "ambiguous bootstrap protocols", protocols: []string{runtimeV2WebSocketSubprotocol, runtimeV2BootstrapProtocolPrefix + "ticket-1", runtimeV2BootstrapProtocolPrefix + "ticket-2"}, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/runtime/ws", nil)
			if len(tc.protocols) > 0 {
				req.Header.Set("Sec-WebSocket-Protocol", strings.Join(tc.protocols, ", "))
			}
			selected, ticket, ok := parseRuntimeBootstrapSubprotocol(req)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want=%v selected=%q ticket=%q", ok, tc.wantOK, selected, ticket)
			}
			if tc.wantOK {
				if selected != runtimeV2WebSocketSubprotocol || ticket != tc.wantTicket {
					t.Fatalf("selected=%q ticket=%q", selected, ticket)
				}
			}
		})
	}
}

func TestRuntimeWebSocketTicketHelloAndIdentityHardBinding(t *testing.T) {
	server, _ := newRuntimeWSTestServer(t)

	resp, err := http.Get(server.URL + "/runtime/ws?deviceId=device-ws&runtimeId=runtime-ws")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing ticket: expected 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	plainReq, err := http.NewRequest(http.MethodGet, server.URL+"/runtime/ws?deviceId=device-ws&runtimeId=runtime-ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	plainReq.Header.Set("Sec-WebSocket-Protocol", runtimeV2WebSocketSubprotocol+", "+runtimeV2BootstrapProtocolPrefix+"ticket-1")
	plainResp, err := http.DefaultClient.Do(plainReq)
	if err != nil {
		t.Fatal(err)
	}
	if plainResp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("non-websocket request must not consume bootstrap ticket: expected 426, got %d", plainResp.StatusCode)
	}
	_ = plainResp.Body.Close()

	legacyQueryURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/runtime/ws?ticket=ticket-1&deviceId=device-ws&runtimeId=runtime-ws"
	legacyConn, legacyResp, legacyErr := websocket.DefaultDialer.Dial(legacyQueryURL, nil)
	if legacyConn != nil {
		_ = legacyConn.Close()
	}
	if legacyErr == nil || legacyResp == nil || legacyResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("query-string bootstrap ticket must be rejected, err=%v status=%v", legacyErr, func() any {
			if legacyResp == nil {
				return nil
			}
			return legacyResp.StatusCode
		}())
	}
	_ = legacyResp.Body.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/runtime/ws?deviceId=device-ws&runtimeId=runtime-ws"
	dialer := websocket.Dialer{Subprotocols: []string{
		runtimeV2WebSocketSubprotocol,
		runtimeV2BootstrapProtocolPrefix + "ticket-1",
	}}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.Subprotocol() != runtimeV2WebSocketSubprotocol {
		t.Fatalf("unexpected negotiated subprotocol: %q", conn.Subprotocol())
	}

	helloPayload := HelloPayload{
		RuntimeVersion:         "1.0.0",
		RuntimeContractVersion: CurrentSchemaVersion,
		DeviceID:               "device-ws",
		RuntimeID:              "runtime-ws",
		Capabilities:           []string{"runtime.command-ack"},
	}
	helloBytes, err := json.Marshal(helloPayload)
	if err != nil {
		t.Fatal(err)
	}
	hello := &Envelope{
		EnvelopeVersion:      EnvelopeVersion,
		Protocol:             ProtocolName,
		MessageType:          MessageTypeHello,
		MessageName:          "hello",
		MessageID:            "hello-1",
		UserID:               "user-ws",
		DeviceID:             "device-ws",
		RuntimeID:            "runtime-ws",
		ConnectionGeneration: 1,
		Sequence:             1,
		PayloadSchemaVersion: 1,
		PayloadHash:          ComputePayloadHash(helloBytes),
		SentAt:               time.Now().UTC(),
		Payload:              helloBytes,
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatal(err)
	}
	_, rawAck, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var ackEnv Envelope
	if err := json.Unmarshal(rawAck, &ackEnv); err != nil {
		t.Fatal(err)
	}
	if ackEnv.MessageType != MessageTypeHelloAck || ackEnv.RuntimeSessionID == "" {
		t.Fatalf("unexpected hello ack: type=%s session=%s", ackEnv.MessageType, ackEnv.RuntimeSessionID)
	}
	if ackEnv.UserID != "user-ws" || ackEnv.DeviceID != "device-ws" || ackEnv.RuntimeID != "runtime-ws" {
		t.Fatalf("hello ack identity mismatch: %+v", ackEnv)
	}

	// A consumed ticket must not create a second websocket session.
	second, resp2, err := dialer.Dial(wsURL, nil)
	if second != nil {
		_ = second.Close()
	}
	if err == nil || resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("consumed ticket must return 401, err=%v status=%v", err, func() any {
			if resp2 == nil {
				return nil
			}
			return resp2.StatusCode
		}())
	}
	_ = resp2.Body.Close()

	// Identity is hard-bound to the consumed ticket/connection; envelope identity
	// cannot replace it after the websocket has been established.
	forgedPayload := []byte(`{"time":"now"}`)
	forged := &Envelope{
		EnvelopeVersion:      EnvelopeVersion,
		Protocol:             ProtocolName,
		MessageType:          MessageTypePing,
		MessageName:          "ping",
		MessageID:            "ping-forged",
		UserID:               "evil-user",
		DeviceID:             "device-ws",
		RuntimeID:            "runtime-ws",
		RuntimeSessionID:     ackEnv.RuntimeSessionID,
		ConnectionGeneration: ackEnv.ConnectionGeneration,
		Sequence:             2,
		PayloadSchemaVersion: 1,
		PayloadHash:          ComputePayloadHash(forgedPayload),
		SentAt:               time.Now().UTC(),
		Payload:              forgedPayload,
	}
	if err := conn.WriteJSON(forged); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("forged envelope identity must close the websocket")
	}
}
