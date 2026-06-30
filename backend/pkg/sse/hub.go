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
