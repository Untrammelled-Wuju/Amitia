package capability

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/server"
)

type fastReplyConnection struct {
	net.Conn
	onWrite func()
}

func (c fastReplyConnection) Write(data []byte) (int, error) {
	c.onWrite()
	return c.Conn.Write(data)
}

func TestMeshInvocationKeepsResultCompletedBeforeSendReturns(t *testing.T) {
	upgrader := websocket.Upgrader{}
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer peer.Close()

	pending := NewPendingInvocationManager()
	var ready, delivered atomic.Bool
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return fastReplyConnection{Conn: conn, onWrite: func() {
			if ready.CompareAndSwap(true, false) {
				delivered.Store(pending.Complete("fast", NewToolSuccessResult("fast", "tool")))
			}
		}}, nil
	}}
	conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(peer.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	hub := server.NewConnectionHub()
	hub.Attach("session", server.NewMeshConnection(conn, "session", 1, "space", "device", "runtime"))
	port := NewMeshDeviceRuntimeInvocationPort(&MeshRuntimePorts{Hub: hub, PendingInvocations: pending})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ready.Store(true)
	result := port.Execute(ctx, DeviceRuntimeInvocationRequest{
		Route: RuntimeExecutionRoute{
			SpaceID: "space", DeviceID: "device", RuntimeID: "runtime",
			Binding: RuntimeBinding{HandlerName: "coordination.data"},
		},
		Invocation: ToolInvocationContext{InvocationID: "fast"},
	})
	if !delivered.Load() || result.Status != NewToolSuccessResult("fast", "tool").Status || result.ToolID != "tool" || pending.PendingCount() != 0 {
		t.Fatalf("early completion lost: delivered=%v result=%+v error=%+v pending=%d", delivered.Load(), result, result.Error, pending.PendingCount())
	}
}
