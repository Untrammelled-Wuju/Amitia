package nativebridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  16 * 1024,
	WriteBufferSize: 16 * 1024,
}

type RelayHandler struct {
	mu       sync.RWMutex
	bridges  map[string]RelayBridge
	sessions map[string]*RelayConnection
}

type RelayBridge interface {
	Bridge
	AttachRelaySession(transport RelayTransport)
	DetachSession()
	Generation() uint64
	SessionAttached() bool
}

func NewRelayHandler() *RelayHandler {
	return &RelayHandler{
		bridges:  make(map[string]RelayBridge),
		sessions: make(map[string]*RelayConnection),
	}
}

func (h *RelayHandler) RegisterBridge(platform string, bridge RelayBridge) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.bridges[platform] = bridge
}

func (h *RelayHandler) HandleWebSocket(c *gin.Context) {
	platform := c.Query("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform parameter is required"})
		return
	}

	h.mu.RLock()
	bridge, ok := h.bridges[platform]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform: " + platform})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	relayConn := NewRelayConnection(platform, conn)

	h.mu.Lock()
	if old, exists := h.sessions[platform]; exists {
		old.Close()
	}
	h.sessions[platform] = relayConn
	h.mu.Unlock()

	transport := newRelayTransport(conn)
	bridge.AttachRelaySession(transport)

	relayConn.StartPongLoop()

	go func() {
		defer func() {
			bridge.DetachSession()
			h.mu.Lock()
			if h.sessions[platform] == relayConn {
				delete(h.sessions, platform)
			}
			h.mu.Unlock()
			relayConn.Close()
		}()

		err := relayConn.ReadLoop(func(data []byte) error {
			var env RelayEnvelope
			if err := json.Unmarshal(data, &env); err != nil {
				return nil
			}
			return h.handleEnvelope(platform, bridge, env)
		})
		if err != nil {
			return
		}
	}()
}

func (h *RelayHandler) handleEnvelope(platform string, bridge RelayBridge, env RelayEnvelope) error {
	switch env.Type {
	case "native_bridge.response":
		return h.handleResponse(platform, env)
	case "native_bridge.event":
		return h.handleEvent(platform, env)
	case "native_bridge.health":
		return h.handleHealthUpdate(platform, bridge, env)
	default:
		return fmt.Errorf("unknown relay envelope type: %s", env.Type)
	}
}

func (h *RelayHandler) handleResponse(platform string, env RelayEnvelope) error {
	h.mu.RLock()
	conn, ok := h.sessions[platform]
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	conn.Session.handleEnvelope(env)
	return nil
}

func (h *RelayHandler) handleEvent(platform string, env RelayEnvelope) error {
	h.mu.RLock()
	conn, ok := h.sessions[platform]
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	_ = conn
	return nil
}

func (h *RelayHandler) handleHealthUpdate(platform string, bridge RelayBridge, env RelayEnvelope) error {
	h.mu.RLock()
	conn, ok := h.sessions[platform]
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	_ = conn
	_ = bridge
	return nil
}

func (h *RelayHandler) GetSession(platform string) (*RelayConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.sessions[platform]
	return conn, ok
}

func (h *RelayHandler) GetBridge(platform string) (RelayBridge, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	bridge, ok := h.bridges[platform]
	return bridge, ok
}

type relayTransportFromConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func newRelayTransport(conn *websocket.Conn) RelayTransport {
	return &relayTransportFromConn{conn: conn}
}

func (t *relayTransportFromConn) Send(payload []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, payload)
}
