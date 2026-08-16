package sdk

import (
	"context"
	"sync"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type MockTransport struct {
	mu       sync.Mutex
	messages []protocol.Envelope
	receiveC chan protocol.Envelope
	sendErr  error
	recvErr  error
	closed   bool
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages: make([]protocol.Envelope, 0),
		receiveC: make(chan protocol.Envelope, 16),
	}
}

func (m *MockTransport) Send(ctx context.Context, message protocol.Envelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.messages = append(m.messages, message)
	return nil
}

func (m *MockTransport) Receive(ctx context.Context) (protocol.Envelope, error) {
	select {
	case msg := <-m.receiveC:
		m.mu.Lock()
		err := m.recvErr
		m.mu.Unlock()
		if err != nil {
			return protocol.Envelope{}, err
		}
		return msg, nil
	case <-ctx.Done():
		return protocol.Envelope{}, ctx.Err()
	}
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.receiveC)
	return nil
}

func (m *MockTransport) QueueMessage(msg protocol.Envelope) {
	m.receiveC <- msg
}

func (m *MockTransport) GetSentMessages() []protocol.Envelope {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]protocol.Envelope, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *MockTransport) GetSentMessagesLen() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *MockTransport) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func (m *MockTransport) SetReceiveError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvErr = err
}

func (m *MockTransport) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

type FixedIDGenerator struct {
	ids       []string
	current   int
}

func NewFixedIDGenerator(ids ...string) *FixedIDGenerator {
	return &FixedIDGenerator{
		ids:     ids,
		current: 0,
	}
}

func (g *FixedIDGenerator) NewID() string {
	if g.current >= len(g.ids) {
		return "msg-overflow"
	}
	id := g.ids[g.current]
	g.current++
	return id
}
