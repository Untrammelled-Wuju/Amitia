package ipc_test

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type MemoryTransportSide struct {
	sendCh    chan protocol.Envelope
	recvCh    chan protocol.Envelope
	closeOnce sync.Once
	closeCh   chan struct{}
}

func NewMemoryTransportPair() (ipc.Transport, ipc.Transport) {
	chA := make(chan protocol.Envelope, 64)
	chB := make(chan protocol.Envelope, 64)

	a := &MemoryTransportSide{
		sendCh:  chB,
		recvCh:  chA,
		closeCh: make(chan struct{}),
	}
	b := &MemoryTransportSide{
		sendCh:  chA,
		recvCh:  chB,
		closeCh: make(chan struct{}),
	}

	return a, b
}

func (m *MemoryTransportSide) Send(ctx context.Context, envelope protocol.Envelope) error {
	select {
	case m.sendCh <- envelope:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-m.closeCh:
		return context.Canceled
	}
}

func (m *MemoryTransportSide) Receive(ctx context.Context) (protocol.Envelope, error) {
	select {
	case env := <-m.recvCh:
		return env, nil
	case <-ctx.Done():
		return protocol.Envelope{}, ctx.Err()
	case <-m.closeCh:
		return protocol.Envelope{}, context.Canceled
	}
}

func (m *MemoryTransportSide) Close() error {
	m.closeOnce.Do(func() {
		close(m.closeCh)
	})
	return nil
}

type MockResolver struct {
	PluginID domain.PluginID
	Err      error
}

func (r *MockResolver) ResolveService(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
) (domain.PluginID, error) {
	if r.Err != nil {
		return "", r.Err
	}
	return r.PluginID, nil
}
