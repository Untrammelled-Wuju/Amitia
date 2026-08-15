package nativebridge

import (
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

type RelayConnection struct {
	Platform     string
	ConnectionID uint64
	Transport    *websocketTransport

	closeOnce sync.Once
	done      chan struct{}
}

func NewRelayConnection(platform string, conn *websocket.Conn, connectionID uint64) *RelayConnection {
	return &RelayConnection{
		Platform:     platform,
		ConnectionID: connectionID,
		Transport:    newWebsocketTransport(conn),
		done:         make(chan struct{}),
	}
}

func (c *RelayConnection) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.Transport.close()
	})
}

func (c *RelayConnection) Done() <-chan struct{} {
	return c.done
}

func (c *RelayConnection) StartPongLoop() {
	go c.Transport.startPongLoop(c.done)
}

func (c *RelayConnection) ReadLoop(handle func([]byte) error) error {
	for {
		data, err := c.Transport.read()
		if err != nil {
			return err
		}
		if err := handle(data); err != nil {
			return err
		}
	}
}
