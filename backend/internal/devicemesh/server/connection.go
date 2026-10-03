package server

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type MeshConnection struct {
	Conn           *websocket.Conn
	SessionID      runtimeidentity.RuntimeSessionID
	Generation     int64
	SpaceID        runtimeidentity.SpaceID
	DeviceID       runtimeidentity.DeviceID
	RuntimeID      runtimeidentity.RuntimeID
	ConnectedAt    time.Time
	LastPongAt     time.Time
	sendMu         sync.Mutex
	writeCloseSent bool
	outboundSeq    int64
}

func NewMeshConnection(conn *websocket.Conn, sessionID runtimeidentity.RuntimeSessionID, generation int64, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) *MeshConnection {
	now := time.Now().UTC()
	return &MeshConnection{
		Conn:        conn,
		SessionID:   sessionID,
		Generation:  generation,
		SpaceID:     spaceID,
		DeviceID:    deviceID,
		RuntimeID:   runtimeID,
		ConnectedAt: now,
		LastPongAt:  now,
	}
}

func (c *MeshConnection) Send(data []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, data)
}

func (c *MeshConnection) Close(code int, reason string) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.writeCloseSent {
		return c.Conn.Close()
	}
	c.writeCloseSent = true
	msg := websocket.FormatCloseMessage(code, reason)
	_ = c.Conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(2*time.Second))
	return c.Conn.Close()
}

func (c *MeshConnection) nextOutboundSequence() int64 {
	return atomic.AddInt64(&c.outboundSeq, 1)
}
