// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sse

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type Client struct {
	ID        string
	SpaceID   string
	Events    chan map[string]interface{}
	closeOnce sync.Once
}

func (c *Client) close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.Events)
	})
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

var (
	Global                 = &Hub{clients: make(map[string]*Client)}
	anonymousClientCounter atomic.Uint64
)

func newAnonymousClientID() string {
	return fmt.Sprintf("sse-%d-%d", time.Now().UnixNano(), anonymousClientCounter.Add(1))
}

func (h *Hub) Subscribe(clientID string) *Client {
	return h.SubscribeScoped(clientID, "")
}

func (h *Hub) SubscribeScoped(clientID string, spaceID string) *Client {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = newAnonymousClientID()
	}

	c := &Client{ID: clientID, SpaceID: strings.TrimSpace(spaceID), Events: make(chan map[string]interface{}, 20)}

	h.mu.Lock()
	previous := h.clients[clientID]
	h.clients[clientID] = c
	h.mu.Unlock()

	// A reconnect using the same stable client ID atomically replaces the old
	// subscription. Closing the old instance wakes its handler without giving
	// that handler a chance to remove the replacement from the registry.
	if previous != nil && previous != c {
		previous.close()
	}

	return c
}

func (h *Hub) Unsubscribe(clientID string) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return
	}

	h.mu.Lock()
	client := h.clients[clientID]
	if client != nil {
		delete(h.clients, clientID)
	}
	h.mu.Unlock()

	client.close()
}

// UnsubscribeClient removes exactly the supplied subscription instance. This
// matters when a stable client ID reconnects: an old HTTP handler must never
// delete or close the newer subscription that replaced it.
func (h *Hub) UnsubscribeClient(client *Client) {
	if client == nil {
		return
	}

	h.mu.Lock()
	if current, ok := h.clients[client.ID]; ok && current == client {
		delete(h.clients, client.ID)
	}
	h.mu.Unlock()

	client.close()
}

func (h *Hub) Broadcast(event string, data map[string]interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg := map[string]interface{}{"event": event, "data": data}
	for _, c := range h.clients {
		select {
		case c.Events <- msg:
		default:
		}
	}
}

// BroadcastToSpace delivers an event only to SSE subscriptions authenticated as
// the requested user. It is intended for user-content events such as proactive
// messages; global UI/catalog invalidation events should continue to use
// Broadcast. Empty user IDs fail closed and are never broadcast.
func (h *Hub) BroadcastToSpace(spaceID string, event string, data map[string]interface{}) int {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg := map[string]interface{}{"event": event, "data": data}
	delivered := 0
	for _, c := range h.clients {
		if c.SpaceID != spaceID {
			continue
		}
		select {
		case c.Events <- msg:
			delivered++
		default:
		}
	}
	return delivered
}

func (h *Hub) HasClients() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients) > 0
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) SendToClient(clientID string, event string, data map[string]interface{}) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[clientID]
	if !ok {
		return false
	}
	msg := map[string]interface{}{"event": event, "data": data}
	select {
	case c.Events <- msg:
		return true
	default:
		return false
	}
}

func (h *Hub) SendToClients(clientIDs []string, event string, data map[string]interface{}) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg := map[string]interface{}{"event": event, "data": data}
	delivered := 0
	for _, id := range clientIDs {
		if c, ok := h.clients[id]; ok {
			select {
			case c.Events <- msg:
				delivered++
			default:
			}
		}
	}
	return delivered
}

func (h *Hub) ClientExists(clientID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[clientID]
	return ok
}

func (h *Hub) ListClientIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

func SSEHandler(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	spaceID := ""
	if raw, ok := c.Get("spaceId"); ok && raw != nil {
		spaceID = strings.TrimSpace(fmt.Sprint(raw))
	}
	client := Global.SubscribeScoped(c.Query("clientId"), spaceID)
	defer Global.UnsubscribeClient(client)

	c.Writer.Flush()
	for {
		select {
		case msg, ok := <-client.Events:
			if !ok {
				return
			}
			eventName, _ := msg["event"].(string)
			data, _ := msg["data"].(map[string]interface{})
			jsonData, _ := json.Marshal(data)
			c.SSEvent(eventName, string(jsonData))
			c.Writer.Flush()
		case <-c.Done():
			return
		}
	}
}
