package nativebridge

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	relayReadLimit  = 64 * 1024
	relayPingPeriod = 30 * time.Second
	relayWriteWait  = 10 * time.Second
	relayReadWait   = 60 * time.Second
)

type websocketTransport struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func newWebsocketTransport(conn *websocket.Conn) *websocketTransport {
	conn.SetReadLimit(relayReadLimit)
	conn.SetReadDeadline(time.Now().Add(relayReadWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(relayReadWait))
		return nil
	})
	return &websocketTransport{conn: conn}
}

func (t *websocketTransport) Send(payload []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conn.SetWriteDeadline(time.Now().Add(relayWriteWait))
	return t.conn.WriteMessage(websocket.TextMessage, payload)
}

func (t *websocketTransport) read() ([]byte, error) {
	_, data, err := t.conn.ReadMessage()
	return data, err
}

func (t *websocketTransport) close() error {
	return t.conn.Close()
}

func (t *websocketTransport) startPongLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(relayPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			t.conn.SetWriteDeadline(time.Now().Add(relayWriteWait))
			err := t.conn.WriteMessage(websocket.PingMessage, nil)
			t.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

type wsRelaySession struct {
	transport *websocketTransport
	relay     *productionRelaySession
}

func newWsRelaySession(conn *websocket.Conn) *wsRelaySession {
	t := newWebsocketTransport(conn)
	r := newRelaySession(t)
	return &wsRelaySession{
		transport: t,
		relay:     r,
	}
}

func (s *wsRelaySession) SendRequest(ctx context.Context, req Request) (Response, error) {
	return s.relay.SendRequest(ctx, req)
}

func (s *wsRelaySession) handleEnvelope(env RelayEnvelope) {
	s.relay.handleIncomingEnvelope(env)
}

func (s *wsRelaySession) generation() uint64 {
	return s.relay.generationValue()
}

func (s *wsRelaySession) close() {
	_ = s.transport.close()
}

type RelayConnection struct {
	Platform string
	Session  *wsRelaySession
	done     chan struct{}
}

func NewRelayConnection(platform string, conn *websocket.Conn) *RelayConnection {
	return &RelayConnection{
		Platform: platform,
		Session:  newWsRelaySession(conn),
		done:     make(chan struct{}),
	}
}

func (c *RelayConnection) Close() {
	close(c.done)
	c.Session.close()
}

func (c *RelayConnection) Done() <-chan struct{} {
	return c.done
}

func (c *RelayConnection) StartPongLoop() {
	go c.Session.transport.startPongLoop(c.done)
}

func (c *RelayConnection) ReadLoop(handle func([]byte) error) error {
	for {
		data, err := c.Session.transport.read()
		if err != nil {
			return err
		}
		if err := handle(data); err != nil {
			return err
		}
	}
}

func encodeRelayEnvelope(env RelayEnvelope) ([]byte, error) {
	return json.Marshal(env)
}

