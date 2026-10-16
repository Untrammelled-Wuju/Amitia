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
	"github.com/u-ai/backend/internal/auth"
)

type Client struct {
	ID        string
	SpaceID   string
	DeviceID  string
	key       string
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
	return h.SubscribeDevice(clientID, spaceID, "")
}

func (h *Hub) SubscribeDevice(clientID, spaceID, deviceID string) *Client {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = newAnonymousClientID()
	}

	c := &Client{ID: clientID, SpaceID: strings.TrimSpace(spaceID), DeviceID: strings.TrimSpace(deviceID), Events: make(chan map[string]interface{}, 20)}
	c.key = clientID
	if c.SpaceID != "" && c.DeviceID != "" {
		c.key = c.SpaceID + "\x00" + c.DeviceID + "\x00" + clientID
	}

	h.mu.Lock()
	previous := h.clients[c.key]
	h.clients[c.key] = c
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
	client := h.clientLocked(clientID)
	if client != nil {
		delete(h.clients, client.key)
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
	if current, ok := h.clients[client.key]; ok && current == client {
		delete(h.clients, client.key)
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

func (h *Hub) SendToDevice(spaceID, deviceID, event string, data map[string]interface{}) int {
	if strings.TrimSpace(spaceID) == "" || strings.TrimSpace(deviceID) == "" {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	delivered := 0
	for _, client := range h.clients {
		if client.SpaceID != spaceID || client.DeviceID != deviceID {
			continue
		}
		select {
		case client.Events <- map[string]interface{}{"event": event, "data": data}:
			delivered++
		default:
		}
	}
	return delivered
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) SendToClient(clientID string, event string, data map[string]interface{}) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c := h.clientLocked(clientID)
	if c == nil {
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
		if c := h.clientLocked(id); c != nil {
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
	return h.clientLocked(clientID) != nil
}

func (h *Hub) ListClientIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for _, client := range h.clients {
		ids = append(ids, client.ID)
	}
	return ids
}

func (h *Hub) clientLocked(id string) *Client {
	var result *Client
	for _, client := range h.clients {
		if client.ID == id {
			if result != nil {
				return nil
			}
			result = client
		}
	}
	return result
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
	deviceID := ""
	if actor, ok := auth.FromContext(c.Request.Context()); ok && actor != nil {
		spaceID = actor.SpaceID.String()
		deviceID = actor.DeviceID.String()
		if deviceID == "" && actor.IsLocalTrusted {
			deviceID = strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
		}
	}
	clientID := c.Query("clientId")
	client := Global.SubscribeDevice(clientID, spaceID, deviceID)
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
