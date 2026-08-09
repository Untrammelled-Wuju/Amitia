package handshake

import (
	"sync"
)

type HandshakeState string

const (
	HandshakeStateAttached    HandshakeState = "attached"
	HandshakeStateHandshaking HandshakeState = "handshaking"
	HandshakeStateReady       HandshakeState = "ready"
	HandshakeStateRejected    HandshakeState = "rejected"
	HandshakeStateClosed      HandshakeState = "closed"
)

func (s HandshakeState) IsTerminal() bool {
	switch s {
	case HandshakeStateReady, HandshakeStateRejected, HandshakeStateClosed:
		return true
	}
	return false
}

type stateCell struct {
	mu    sync.Mutex
	state HandshakeState
}

func newStateCell(initial HandshakeState) *stateCell {
	return &stateCell{state: initial}
}

func (c *stateCell) Get() HandshakeState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *stateCell) compareAndSwap(from, to HandshakeState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != from {
		return false
	}
	c.state = to
	return true
}

func (c *stateCell) transitionNonTerminal(to HandshakeState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.IsTerminal() {
		return false
	}
	c.state = to
	return true
}

func (c *stateCell) transitionAlways(to HandshakeState) HandshakeState {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.state
	c.state = to
	return prev
}
