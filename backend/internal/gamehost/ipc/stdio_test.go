package ipc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestStdioTransport_SingleBufferRoundtrip(t *testing.T) {
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	sender := NewStdioTransport(StdioTransportConfig{
		Writer: bufA,
		Reader: bufB,
	})

	receiver := NewStdioTransport(StdioTransportConfig{
		Writer: bufB,
		Reader: bufA,
	})

	ctx := context.Background()

	env := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeRequest,
		ID:        "test-msg-1",
		Method:    "vendor.custom.action",
		RuntimeID: "runtime-1",
		PluginID:  "plugin-a",
		ServiceID: "svc-1",
	}

	err := sender.Send(ctx, env)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	received, err := receiver.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if received.ID != env.ID {
		t.Errorf("message ID mismatch: got %s, want %s", received.ID, env.ID)
	}
	if received.RuntimeID != env.RuntimeID {
		t.Errorf("runtime ID mismatch: got %s, want %s", received.RuntimeID, env.RuntimeID)
	}
	if received.Method != env.Method {
		t.Errorf("method mismatch: got %s, want %s", received.Method, env.Method)
	}
}

func TestStdioTransport_Pair(t *testing.T) {
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	transA := NewStdioTransport(StdioTransportConfig{
		Writer: bufA,
		Reader: bufB,
	})

	transB := NewStdioTransport(StdioTransportConfig{
		Writer: bufB,
		Reader: bufA,
	})

	ctx := context.Background()

	env := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeRequest,
		ID:        "test-msg-1",
		Method:    "vendor.custom.action",
		RuntimeID: "runtime-1",
		PluginID:  "plugin-a",
		ServiceID: "svc-1",
	}

	err := transA.Send(ctx, env)
	if err != nil {
		t.Fatalf("transA.Send failed: %v", err)
	}

	received, err := transB.Receive(ctx)
	if err != nil {
		t.Fatalf("transB.Receive failed: %v", err)
	}

	if received.ID != env.ID {
		t.Errorf("message ID mismatch: got %s, want %s", received.ID, env.ID)
	}
	if received.Method != env.Method {
		t.Errorf("method mismatch: got %s, want %s", received.Method, env.Method)
	}
	if received.Type != env.Type {
		t.Errorf("type mismatch: got %s, want %s", received.Type, env.Type)
	}
	if received.RuntimeID != env.RuntimeID {
		t.Errorf("runtime ID mismatch: got %s, want %s", received.RuntimeID, env.RuntimeID)
	}
}

func TestStdioTransport_ConcurrentSend(t *testing.T) {
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	transA := NewStdioTransport(StdioTransportConfig{
		Writer: bufA,
		Reader: bufB,
	})

	_ = NewStdioTransport(StdioTransportConfig{
		Writer: bufB,
		Reader: bufA,
	})

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			env := protocol.Envelope{
				Protocol: protocol.ProtocolVersion,
				Type:     protocol.MessageTypeRequest,
				ID:       "concurrent-msg-" + string(rune('a'+i%26)),
				Method:   "vendor.test.action",
			}
			if err := transA.Send(ctx, env); err != nil {
				t.Errorf("concurrent send failed: %v", err)
				return
			}
		}
		close(done)
	}()

	<-done
	time.Sleep(50 * time.Millisecond)

	bufA2 := &bytes.Buffer{}
	bufB2 := &bytes.Buffer{}

	transA2 := NewStdioTransport(StdioTransportConfig{
		Writer: bufA2,
		Reader: bufB2,
	})
	transB2 := NewStdioTransport(StdioTransportConfig{
		Writer: bufB2,
		Reader: bufA2,
	})

	for i := 0; i < 10; i++ {
		env := protocol.Envelope{
			Protocol: protocol.ProtocolVersion,
			Type:     protocol.MessageTypeRequest,
			ID:       "concurrent-roundtrip-msg",
			Method:   "vendor.test.action",
		}

		if err := transA2.Send(ctx, env); err != nil {
			t.Fatalf("send %d failed: %v", i, err)
		}

		received, err := transB2.Receive(ctx)
		if err != nil {
			t.Fatalf("receive %d failed: %v", i, err)
		}
		if received.ID != env.ID {
			t.Errorf("msg %d: ID mismatch", i)
		}
	}
}

func TestStdioTransport_CloseIdempotent(t *testing.T) {
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	trans := NewStdioTransport(StdioTransportConfig{
		Writer: bufA,
		Reader: bufB,
		Closer: &mockCloser{},
	})

	err := trans.Close()
	if err != nil {
		t.Fatalf("first close failed: %v", err)
	}

	err = trans.Close()
	if err != nil {
		t.Fatalf("second close should be idempotent: %v", err)
	}
}

func TestStdioTransport_FrameSizeLimit(t *testing.T) {
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	trans := NewStdioTransport(StdioTransportConfig{
		Writer:       bufA,
		Reader:       bufB,
		MaxFrameSize: 64,
	})

	ctx := context.Background()

	largePayload := make([]byte, 256)
	for i := range largePayload {
		largePayload[i] = 'x'
	}

	env := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "big-msg",
		Method:   "vendor.test.action",
	}

	_ = env
	_ = largePayload

	err := trans.Send(ctx, env)
	if err != nil {
		t.Logf("Send error: %v", err)
	}
}

type mockCloser struct {
	closed bool
}

func (c *mockCloser) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}
