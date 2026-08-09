package ipc

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type ConnectionID string

type ConnectionState string

const (
	ConnectionStateAttached ConnectionState = "attached"
	ConnectionStateClosing  ConnectionState = "closing"
	ConnectionStateClosed   ConnectionState = "closed"
)

type Connection struct {
	ID   ConnectionID
	Peer Peer

	transport Transport

	mu         sync.Mutex
	state      ConnectionState
	createdAt  time.Time
	closedAt   *time.Time
	cancelFunc context.CancelFunc
}

func newConnection(
	id ConnectionID,
	peer Peer,
	transport Transport,
	now time.Time,
	cancelFunc context.CancelFunc,
) *Connection {
	return NewConnection(id, peer, transport, now, cancelFunc)
}

func NewConnection(
	id ConnectionID,
	peer Peer,
	transport Transport,
	now time.Time,
	cancelFunc context.CancelFunc,
) *Connection {
	return &Connection{
		ID:         id,
		Peer:       peer,
		transport:  transport,
		state:      ConnectionStateAttached,
		createdAt:  now,
		cancelFunc: cancelFunc,
	}
}

func (c *Connection) State() ConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *Connection) CreatedAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.createdAt
}

func (c *Connection) ClosedAt() *time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closedAt
}

func (c *Connection) Transport() Transport {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transport
}

func (c *Connection) IsActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == ConnectionStateAttached
}

func (c *Connection) markClosing(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != ConnectionStateAttached {
		return false
	}
	c.state = ConnectionStateClosing
	return true
}

func (c *Connection) markClosed(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = ConnectionStateClosed
	c.closedAt = &now
}

func (c *Connection) setCancelFunc(cancelFunc context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelFunc = cancelFunc
}

func (c *Connection) cancel() {
	c.mu.Lock()
	cancelFunc := c.cancelFunc
	c.mu.Unlock()
	if cancelFunc != nil {
		cancelFunc()
	}
}

type Transport interface {
	Send(ctx context.Context, envelope protocol.Envelope) error
	Receive(ctx context.Context) (protocol.Envelope, error)
	Close() error
}
