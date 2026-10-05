package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestConnectionConcurrentSendsAllocateSequenceInWireOrder(t *testing.T) {
	ready := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			ready <- connection
		}
	}))
	defer server.Close()
	reader, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	writer := <-ready
	defer writer.Close()
	connection := NewMeshConnection(writer, "session", 1, "core", "device", "runtime")
	const count = 128
	errors := make(chan error, count)
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			payload := json.RawMessage(`{"time":"2026-10-04T00:00:00Z"}`)
			encoded, err := json.Marshal(protocol.Envelope{MessageType: protocol.MessageTypePing, Sequence: int64(count - index), Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)})
			if err == nil {
				err = connection.Send(encoded)
			}
			errors <- err
		}(i)
	}
	reader.SetReadDeadline(time.Now().Add(5 * time.Second))
	for expected := int64(1); expected <= count; expected++ {
		var envelope protocol.Envelope
		if err := reader.ReadJSON(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Sequence != expected || !envelope.VerifyPayloadHash() {
			t.Fatalf("out of order or corrupted envelope: %+v expected=%d", envelope, expected)
		}
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}
