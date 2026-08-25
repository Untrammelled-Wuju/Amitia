package ipc

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestMainDispatcher_Dispatch(t *testing.T) {
	var dispatched []protocol.Envelope

	disp := &testDispatcher{
		fn: func(p Peer, e protocol.Envelope) error {
			dispatched = append(dispatched, e)
			return nil
		},
	}

	env := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "test-1",
		Method:   "vendor.test.action",
	}

	err := disp.Dispatch(context.Background(), Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "svc-1",
	}, env)

	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if len(dispatched) != 1 {
		t.Errorf("expected 1 dispatched envelope, got %d", len(dispatched))
	}
}

func TestValidateEnvelopePeer_Spoof(t *testing.T) {
	peer := Peer{
		PluginID:  "correct.plugin",
		RuntimeID: "runtime-a",
		ServiceID: "svc-a",
	}

	tests := []struct {
		name    string
		env     protocol.Envelope
		wantErr bool
	}{
		{
			name: "valid matching",
			env: protocol.Envelope{
				RuntimeID: "runtime-a",
				PluginID:  "correct.plugin",
				ServiceID: "svc-a",
			},
			wantErr: false,
		},
		{
			name: "runtime spoof",
			env: protocol.Envelope{
				RuntimeID: "runtime-b",
				PluginID:  "correct.plugin",
				ServiceID: "svc-a",
			},
			wantErr: true,
		},
		{
			name: "plugin spoof",
			env: protocol.Envelope{
				RuntimeID: "runtime-a",
				PluginID:  "wrong.plugin",
				ServiceID: "svc-a",
			},
			wantErr: true,
		},
		{
			name: "service spoof",
			env: protocol.Envelope{
				RuntimeID: "runtime-a",
				PluginID:  "correct.plugin",
				ServiceID: "svc-b",
			},
			wantErr: true,
		},
		{
			name:    "empty fields allowed",
			env:     protocol.Envelope{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelopePeer(tt.env, peer)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateEnvelopePeer_GenerationBootstrap(t *testing.T) {
	peer := Peer{
		PluginID:   "correct.plugin",
		RuntimeID:  "runtime-a",
		ServiceID:  "svc-a",
		Generation: 7,
	}

	tests := []struct {
		name    string
		env     protocol.Envelope
		wantErr bool
	}{
		{
			name: "initial handshake may bootstrap without generation",
			env: protocol.Envelope{
				Type:     protocol.MessageTypeRequest,
				Method:   HandshakeMethod,
				PluginID: "correct.plugin",
			},
			wantErr: false,
		},
		{
			name: "ordinary request cannot omit generation",
			env: protocol.Envelope{
				Type:   protocol.MessageTypeRequest,
				Method: "vendor.test.action",
			},
			wantErr: true,
		},
		{
			name: "handshake cannot spoof nonzero generation",
			env: protocol.Envelope{
				Type:       protocol.MessageTypeRequest,
				Method:     HandshakeMethod,
				Generation: 6,
			},
			wantErr: true,
		},
		{
			name: "matching generation is accepted",
			env: protocol.Envelope{
				Type:       protocol.MessageTypeRequest,
				Method:     "vendor.test.action",
				Generation: 7,
			},
			wantErr: false,
		},
		{
			name: "bootstrap still cannot spoof route identity",
			env: protocol.Envelope{
				Type:     protocol.MessageTypeRequest,
				Method:   HandshakeMethod,
				PluginID: "wrong.plugin",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelopePeer(tt.env, peer)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFillRouting(t *testing.T) {
	peer := Peer{
		PluginID:  "plugin-a",
		RuntimeID: "runtime-1",
		ServiceID: "svc-x",
	}

	env := protocol.Envelope{}

	FillRouting(&env, peer)

	if env.RuntimeID != string(peer.RuntimeID) {
		t.Errorf("runtime ID should be filled from peer")
	}
	if env.PluginID != string(peer.PluginID) {
		t.Errorf("plugin ID should be filled from peer")
	}
	if env.ServiceID != string(peer.ServiceID) {
		t.Errorf("service ID should be filled from peer")
	}

	existing := protocol.Envelope{
		RuntimeID: "existing-runtime",
		PluginID:  "existing-plugin",
		ServiceID: "existing-svc",
	}

	FillRouting(&existing, peer)

	if existing.RuntimeID != "existing-runtime" {
		t.Error("existing runtime ID should be preserved")
	}
	if existing.PluginID != "existing-plugin" {
		t.Error("existing plugin ID should be preserved")
	}
	if existing.ServiceID != "existing-svc" {
		t.Error("existing service ID should be preserved")
	}
}

func TestIsRequestResponseNotification(t *testing.T) {
	tests := []struct {
		env      protocol.Envelope
		check    func(protocol.Envelope) bool
		expected bool
	}{
		{
			env:      protocol.Envelope{Type: protocol.MessageTypeRequest},
			check:    IsRequest,
			expected: true,
		},
		{
			env:      protocol.Envelope{Type: protocol.MessageTypeResponse},
			check:    IsResponse,
			expected: true,
		},
		{
			env:      protocol.Envelope{Type: protocol.MessageTypeNotification},
			check:    IsNotification,
			expected: true,
		},
		{
			env:      protocol.Envelope{Type: protocol.MessageTypeError},
			check:    IsError,
			expected: true,
		},
	}

	for _, tt := range tests {
		if got := tt.check(tt.env); got != tt.expected {
			t.Errorf("expected %v, got %v for type %s", tt.expected, got, tt.env.Type)
		}
	}
}

type testDispatcher struct {
	fn func(peer Peer, envelope protocol.Envelope) error
}

func (d *testDispatcher) Dispatch(ctx context.Context, peer Peer, envelope protocol.Envelope) error {
	if d.fn != nil {
		return d.fn(peer, envelope)
	}
	return nil
}

func TestNoopDispatcher(t *testing.T) {
	disp := NewNoopDispatcher()

	err := disp.Dispatch(context.Background(), DispatchSource{}, protocol.Envelope{})
	if err != nil {
		t.Errorf("noop dispatcher should never fail: %v", err)
	}
}

func TestIPCError(t *testing.T) {
	err := NewIPCError(IPCErrorTransport, domain.ErrRuntimeUnavailable, "test error")
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
	if err.Type != IPCErrorTransport {
		t.Errorf("error type mismatch: got %s", err.Type)
	}
	if err.Code != domain.ErrRuntimeUnavailable {
		t.Errorf("error code mismatch: got %s", err.Code)
	}
}

func TestToHostError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if ToHostError(nil) != nil {
			t.Error("nil error should return nil")
		}
	})

	t.Run("HostError passthrough", func(t *testing.T) {
		he := domain.NewHostError(domain.ErrNotFound, "not found")
		result := ToHostError(he)
		if result == nil {
			t.Fatal("should not return nil")
		}
		if !domain.IsHostError(result, domain.ErrNotFound) {
			t.Error("should preserve original HostError code")
		}
	})

	t.Run("IPCError conversion", func(t *testing.T) {
		ipcErr := NewIPCError(IPCErrorTransport, domain.ErrRuntimeUnavailable, "transport failed")
		result := ToHostError(ipcErr)
		if result == nil {
			t.Fatal("should not return nil")
		}
		if !domain.IsHostError(result, domain.ErrRuntimeUnavailable) {
			t.Errorf("should convert IPCError code to HostError")
		}
	})
}
