package nativebridge

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

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
	mu         sync.RWMutex
	bridges    map[string]RelayBridge
	sessions   map[string]*RelayConnection
	eventSinks map[string]bool

	connectionCounter atomic.Uint64
}

func NewRelayHandler() *RelayHandler {
	return &RelayHandler{
		bridges:    make(map[string]RelayBridge),
		sessions:   make(map[string]*RelayConnection),
		eventSinks: make(map[string]bool),
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

	h.mu.Lock()
	if old, exists := h.sessions[platform]; exists {
		old.Close()
	}
	connectionID := h.connectionCounter.Add(1)
	relayConn := NewRelayConnection(platform, conn, connectionID)
	h.sessions[platform] = relayConn
	h.mu.Unlock()

	attachedGeneration := bridge.AttachRelaySession(relayConn.Transport)

	relayConn.StartPongLoop()

	go func() {
		defer func() {
			bridge.DetachRelaySession(attachedGeneration)
			h.mu.Lock()
			if h.sessions[platform] == relayConn {
				delete(h.sessions, platform)
			}
			h.mu.Unlock()
			relayConn.Close()
		}()

		err := relayConn.ReadLoop(func(data []byte) error {
			return bridge.HandleRelayEnvelope(data)
		})
		if err != nil {
			return
		}
	}()
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

func (h *RelayHandler) SetEventSink(platform string, sink NativeEventSink) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.bridges[platform]; !ok {
		return
	}
	if b, ok := h.bridges[platform].(interface{ SetEventSink(NativeEventSink) }); ok {
		b.SetEventSink(sink)
	}
	h.eventSinks[platform] = true
}

func (h *RelayHandler) HasEventSink(platform string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.eventSinks[platform]
}

type relayEventSink struct {
	handler  *RelayHandler
	platform string
}

func (s *relayEventSink) PublishNativeEvent(ctx context.Context, platform string, generation uint64, payload json.RawMessage) error {
	s.handler.mu.RLock()
	bridge, ok := s.handler.bridges[platform]
	s.handler.mu.RUnlock()
	if !ok {
		return nil
	}
	if bridge.Generation() != generation {
		return nil
	}
	return nil
}
