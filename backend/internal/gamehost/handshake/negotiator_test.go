package handshake_test

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
)

func TestProtocolNegotiator_Match(t *testing.T) {
	n := handshake.NewProtocolNegotiator([]string{"amitia-game-host/1"})

	result, err := n.Negotiate([]string{"amitia-game-host/1"})
	if err != nil {
		t.Fatalf("negotiation failed: %v", err)
	}
	if result != "amitia-game-host/1" {
		t.Errorf("expected amitia-game-host/1, got %s", result)
	}
}

func TestProtocolNegotiator_UnknownProtocol(t *testing.T) {
	n := handshake.NewProtocolNegotiator([]string{"amitia-game-host/1"})

	_, err := n.Negotiate([]string{"amitia-game-host/2"})
	if err == nil {
		t.Error("unknown protocol should fail")
	}
}

func TestProtocolNegotiator_EmptyList(t *testing.T) {
	n := handshake.NewProtocolNegotiator([]string{"amitia-game-host/1"})

	_, err := n.Negotiate([]string{})
	if err == nil {
		t.Error("empty protocol list should fail")
	}
}

func TestProtocolNegotiator_AliasNotAccepted(t *testing.T) {
	n := handshake.NewProtocolNegotiator([]string{"amitia-game-host/1"})

	_, err := n.Negotiate([]string{"v1"})
	if err == nil {
		t.Error("alias v1 should not match amitia-game-host/1")
	}

	_, err = n.Negotiate([]string{"game-host/1"})
	if err == nil {
		t.Error("alias game-host/1 should not match")
	}
}

func TestCapabilityNegotiator_BasicMatch(t *testing.T) {
	n := handshake.NewCapabilityNegotiator([]domain.Capability{
		domain.CapabilityCustomRPC,
		domain.CapabilityEventStreaming,
	})

	result, err := n.Negotiate(
		[]domain.Capability{domain.CapabilityCustomRPC, domain.CapabilityEventStreaming},
		[]domain.Capability{domain.CapabilityCustomRPC},
	)
	if err != nil {
		t.Fatalf("capability negotiation failed: %v", err)
	}
	if len(result) != 1 || result[0] != domain.CapabilityCustomRPC {
		t.Errorf("expected [custom_rpc], got %v", result)
	}
}

func TestCapabilityNegotiator_ExceedsDescriptor(t *testing.T) {
	n := handshake.NewCapabilityNegotiator([]domain.Capability{
		domain.CapabilityCustomRPC,
	})

	_, err := n.Negotiate(
		[]domain.Capability{domain.CapabilityCustomRPC},
		[]domain.Capability{domain.CapabilityCustomRPC, domain.CapabilityHostAPI},
	)
	if err == nil {
		t.Error("exceeding descriptor capabilities should fail")
	}
}

func TestCapabilityNegotiator_HostUnsupported(t *testing.T) {
	n := handshake.NewCapabilityNegotiator([]domain.Capability{
		domain.CapabilityCustomRPC,
	})

	result, err := n.Negotiate(
		[]domain.Capability{domain.CapabilityCustomRPC, domain.CapabilityEventStreaming},
		[]domain.Capability{domain.CapabilityCustomRPC, domain.CapabilityEventStreaming},
	)
	if err != nil {
		t.Fatalf("negotiation failed: %v", err)
	}
	if len(result) != 1 || result[0] != domain.CapabilityCustomRPC {
		t.Errorf("host unsupported capability should be filtered, got %v", result)
	}
}

func TestCapabilityNegotiator_Empty(t *testing.T) {
	n := handshake.NewCapabilityNegotiator([]domain.Capability{domain.CapabilityCustomRPC})

	result, err := n.Negotiate(nil, nil)
	if err != nil {
		t.Fatalf("empty negotiation failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestCapabilityNegotiator_NoDuplicate(t *testing.T) {
	err := handshake.ValidateCapabilitiesNoDuplicate([]domain.Capability{"custom_rpc", "event_streaming"})
	if err != nil {
		t.Errorf("no duplicate should not error: %v", err)
	}

	err = handshake.ValidateCapabilitiesNoDuplicate([]domain.Capability{"custom_rpc", "custom_rpc"})
	if err == nil {
		t.Error("duplicate capability should error")
	}
}
