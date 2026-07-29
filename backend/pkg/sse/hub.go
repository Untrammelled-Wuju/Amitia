// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sse

import (
	"encoding/json"
	"sync"

	"github.com/gin-gonic/gin"
)

type Client struct {
	ID     string
	Events chan map[string]interface{}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

var Global = &Hub{
	clients: make(map[string]*Client),
}

func (h *Hub) Subscribe(clientID string) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := &Client{ID: clientID, Events: make(chan map[string]interface{}, 20)}
	h.clients[clientID] = c
	return c
}

func (h *Hub) Unsubscribe(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[clientID]; ok {
		close(c.Events)
		delete(h.clients, clientID)
	}
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

	clientID := c.Query("clientId")
	if clientID == "" {
		clientID = "default"
	}
	client := Global.Subscribe(clientID)
	defer Global.Unsubscribe(clientID)

	c.Writer.Flush()
	for {
		select {
		case msg := <-client.Events:
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
