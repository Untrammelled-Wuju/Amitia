package ipc_test

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type dummyTransport struct {
	closed bool
}

func (d *dummyTransport) Send(ctx context.Context, envelope protocol.Envelope) error {
	return nil
}

func (d *dummyTransport) Receive(ctx context.Context) (protocol.Envelope, error) {
	return protocol.Envelope{}, context.Canceled
}

func (d *dummyTransport) Close() error {
	d.closed = true
	return nil
}

func makeTestPeer(id string) ipc.Peer {
	return ipc.Peer{
		PluginID:  domain.PluginID("test.plugin"),
		RuntimeID: domain.RuntimeInstanceID(id),
		ServiceID: domain.ServiceID(id + "-service"),
	}
}

func TestConnectionRegistry_RegisterAndGet(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	peer := makeTestPeer("runtime-1")
	transport := &dummyTransport{}
	now := time.Now()

	conn := ipc.NewConnection("conn-1", peer, transport, now, nil)

	err := reg.Register(conn)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	got, ok := reg.Get("conn-1")
	if !ok {
		t.Fatal("connection should be retrievable by ID")
	}
	if got.ID != "conn-1" {
		t.Errorf("connection ID mismatch: got %s, want conn-1", got.ID)
	}

	gotByPeer, ok := reg.GetByPeer(conn.Peer.Key())
	if !ok {
		t.Fatal("connection should be retrievable by peer key")
	}
	if gotByPeer.ID != conn.ID {
		t.Errorf("connection mismatch by peer lookup")
	}
}

func TestConnectionRegistry_RegisterNil(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	err := reg.Register(nil)
	if err == nil {
		t.Fatal("registering nil connection should fail")
	}
}

func TestConnectionRegistry_DuplicateActivePeer(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	peer := makeTestPeer("runtime-1")
	transport := &dummyTransport{}
	now := time.Now()

	conn1 := ipc.NewConnection("conn-1", peer, transport, now, nil)
	err := reg.Register(conn1)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	conn2 := ipc.NewConnection("conn-2", peer, &dummyTransport{}, now, nil)
	err = reg.Register(conn2)
	if err == nil {
		t.Fatal("registering duplicate active peer should fail with already_exists")
	}
}

func TestConnectionRegistry_Remove(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	peer := makeTestPeer("runtime-1")
	transport := &dummyTransport{}
	now := time.Now()

	conn := ipc.NewConnection("conn-1", peer, transport, now, nil)

	err := reg.Register(conn)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	removed, ok := reg.Remove("conn-1")
	if !ok {
		t.Fatal("remove should return true")
	}
	if removed.ID != "conn-1" {
		t.Errorf("removed connection ID mismatch: got %s, want conn-1", removed.ID)
	}

	_, ok = reg.Remove("conn-1")
	if ok {
		t.Error("second remove should return false (idempotent)")
	}
}

func TestConnectionRegistry_List(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	now := time.Now()
	for i := 0; i < 3; i++ {
		peer := makeTestPeer(string(rune('a' + i)))
		conn := ipc.NewConnection(ipc.ConnectionID("conn"), peer, &dummyTransport{}, now, nil)
		if err := reg.Register(conn); err != nil {
			t.Fatalf("register failed: %v", err)
		}
		break
	}

	list := reg.List()
	if len(list) != 1 {
		t.Errorf("expected 1 connection, got %d", len(list))
	}
}

func TestConnectionRegistry_ActiveCount(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	now := time.Now()
	peer := makeTestPeer("runtime-1")
	conn := ipc.NewConnection("conn-1", peer, &dummyTransport{}, now, nil)

	if err := reg.Register(conn); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if reg.ActiveCount() != 1 {
		t.Errorf("active count should be 1, got %d", reg.ActiveCount())
	}
}

func TestConnectionRegistry_ConcurrentAccess(t *testing.T) {
	reg := ipc.NewConnectionRegistry()

	done := make(chan struct{})
	go func() {
		now := time.Now()
		for i := 0; i < 100; i++ {
			peer := makeTestPeer(string(rune('a' + (i % 26))))
			c := ipc.NewConnection(ipc.ConnectionID(string(rune('a'+i%26))), peer, &dummyTransport{}, now, nil)
			_ = reg.Register(c)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = reg.List()
		_ = reg.ActiveCount()
	}

	<-done
}
