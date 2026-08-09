package handshake_test

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/handshake"
)

func TestValidateProtocols_Valid(t *testing.T) {
	err := handshake.ValidateProtocols([]string{"amitia-game-host/1"})
	if err != nil {
		t.Errorf("valid protocol should not error: %v", err)
	}
}

func TestValidateProtocols_Empty(t *testing.T) {
	err := handshake.ValidateProtocols([]string{})
	if err == nil {
		t.Error("empty protocol should error")
	}
}

func TestValidateProtocols_Duplicate(t *testing.T) {
	err := handshake.ValidateProtocols([]string{"amitia-game-host/1", "amitia-game-host/1"})
	if err == nil {
		t.Error("duplicate protocol should error")
	}
}

func TestValidateProtocols_TooLong(t *testing.T) {
	longProto := strings.Repeat("x", 200)
	err := handshake.ValidateProtocols([]string{longProto})
	if err == nil {
		t.Error("too long protocol should error")
	}
}

func TestValidateCapabilities_Valid(t *testing.T) {
	err := handshake.ValidateCapabilities([]string{"custom_rpc", "event_streaming"})
	if err != nil {
		t.Errorf("valid capabilities should not error: %v", err)
	}
}

func TestValidateCapabilities_Empty(t *testing.T) {
	err := handshake.ValidateCapabilities([]string{})
	if err != nil {
		t.Error("empty capabilities should be valid")
	}
}

func TestValidateCapabilities_TooMany(t *testing.T) {
	caps := make([]string, 100)
	for i := range caps {
		caps[i] = "custom_rpc"
	}
	err := handshake.ValidateCapabilities(caps)
	if err == nil {
		t.Error("too many capabilities should error")
	}
}

func TestValidateCapabilities_Duplicate(t *testing.T) {
	err := handshake.ValidateCapabilities([]string{"custom_rpc", "custom_rpc"})
	if err == nil {
		t.Error("duplicate capability should error")
	}
}

func TestValidateCapabilities_UnknownStillPassesLocal(t *testing.T) {
	err := handshake.ValidateCapabilities([]string{"vendor.something"})
	if err != nil {
		t.Errorf("namespaced capability should pass local validation: %v", err)
	}
}

func TestValidateChannelAdvertisements_Valid(t *testing.T) {
	err := handshake.ValidateChannelAdvertisements([]handshake.ChannelAdvertisement{
		{ID: "events"},
		{ID: "state"},
	})
	if err != nil {
		t.Errorf("valid channels should not error: %v", err)
	}
}

func TestValidateChannelAdvertisements_Duplicate(t *testing.T) {
	err := handshake.ValidateChannelAdvertisements([]handshake.ChannelAdvertisement{
		{ID: "events"},
		{ID: "events"},
	})
	if err == nil {
		t.Error("duplicate channel should error")
	}
}

func TestValidateChannelAdvertisements_EmptyID(t *testing.T) {
	err := handshake.ValidateChannelAdvertisements([]handshake.ChannelAdvertisement{
		{ID: ""},
	})
	if err == nil {
		t.Error("empty channel ID should error")
	}
}
