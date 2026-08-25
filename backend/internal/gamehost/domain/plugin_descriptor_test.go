package domain

import (
	"testing"
)

func TestPluginDescriptorValid(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",

		Capabilities: []Capability{
			CapabilityStateStreaming,
			CapabilityEventStreaming,
			CapabilityCustomRPC,
		},
		Services: []ServiceDescriptor{
			{
				ID:       ServiceID("agent-service"),
				Name:     "Agent Service",
				Kind:     ServiceKindProcess,
				Required: true,
			},
			{
				ID:   ServiceID("game-bridge"),
				Name: "Game Bridge",
				Kind: ServiceKindProcess,
				DependsOn: []ServiceID{
					ServiceID("agent-service"),
				},
			},
		},
		Channels: []ChannelDescriptor{
			{
				ID:        ChannelID("agent.events"),
				ServiceID: ServiceID("agent-service"),
				Kind:      ChannelKindEvent,
			},
			{
				ID:        ChannelID("game.state"),
				ServiceID: ServiceID("game-bridge"),
				Kind:      ChannelKindState,
			},
		},
		Metadata: map[string]string{
			"vendor": "example",
		},
	}

	if err := desc.Validate(); err != nil {
		t.Fatalf("expected valid descriptor, got error: %v", err)
	}
}

func TestPluginDescriptorRejectsEmptyPluginID(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID(""),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for empty plugin id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsEmptyExtensionID(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for empty extension id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsEmptyName(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsEmptyVersion(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "",
		ProtocolVersion: "amitia-game-host/1",
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for empty version")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsEmptyProtocolVersion(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "",
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for empty protocol version")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsDuplicateCapability(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Capabilities: []Capability{
			CapabilityCustomRPC,
			CapabilityCustomRPC,
		},
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate capability")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsGameToolCapabilityAsHostFeature(t *testing.T) {
	desc := PluginDescriptor{
		ID: PluginID("test-plugin"), ExtensionID: "com.example.test", Name: "Test",
		Version: "1.0.0", ProtocolVersion: "amitia-game-host/1",
		Capabilities: []Capability{Capability("example.navigation")},
	}
	if err := desc.Validate(); err == nil {
		t.Fatal("game/tool capability must not be accepted as a GameHost feature")
	}
}

func TestPluginDescriptorRejectsDuplicateServiceID(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{
			{
				ID:   ServiceID("agent-service"),
				Name: "Agent",
				Kind: ServiceKindProcess,
			},
			{
				ID:   ServiceID("agent-service"),
				Name: "Another Agent",
				Kind: ServiceKindExternal,
			},
		},
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate service id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsMissingServiceDependency(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{
			{
				ID:   ServiceID("agent-service"),
				Name: "Agent",
				Kind: ServiceKindProcess,
				DependsOn: []ServiceID{
					ServiceID("bridge"),
				},
			},
		},
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for missing service dependency")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorRejectsDuplicateChannelID(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{{
			ID: ServiceID("svc-1"), Name: "Service 1", Kind: ServiceKindProcess,
		}},
		Channels: []ChannelDescriptor{
			{
				ID: ChannelID("state"), ServiceID: ServiceID("svc-1"), Kind: ChannelKindState,
			},
			{
				ID: ChannelID("state"), ServiceID: ServiceID("svc-1"), Kind: ChannelKindState,
			},
		},
	}

	err := desc.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate channel id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestPluginDescriptorAllowsSameChannelIDInDifferentServices(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{
			{ID: ServiceID("svc-1"), Name: "Service 1", Kind: ServiceKindProcess},
			{ID: ServiceID("svc-2"), Name: "Service 2", Kind: ServiceKindProcess},
		},
		Channels: []ChannelDescriptor{
			{ID: ChannelID("state"), ServiceID: ServiceID("svc-1"), Kind: ChannelKindState},
			{ID: ChannelID("state"), ServiceID: ServiceID("svc-2"), Kind: ChannelKindState},
		},
	}

	if err := desc.Validate(); err != nil {
		t.Fatalf("same channel id in different services must be valid: %v", err)
	}
}

func TestPluginDescriptorCloneDeepCopy(t *testing.T) {
	original := PluginDescriptor{
		ID:              PluginID("test-plugin"),
		ExtensionID:     "com.example.test",
		Name:            "Test",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
		Capabilities: []Capability{
			CapabilityRealtimeControl,
			CapabilityHostAPI,
		},
		Services: []ServiceDescriptor{
			{
				ID:   ServiceID("svc-1"),
				Name: "Service 1",
				Kind: ServiceKindProcess,
				DependsOn: []ServiceID{
					ServiceID("svc-2"),
				},
				Metadata: map[string]string{
					"key": "value",
				},
			},
		},
		Channels: []ChannelDescriptor{
			{
				ID: ChannelID("events"), ServiceID: ServiceID("svc-1"), Kind: ChannelKindEvent,
				Metadata: map[string]string{
					"schema": "v1",
				},
			},
		},
		Metadata: map[string]string{
			"vendor":   "example",
			"category": "utility",
		},
	}

	cloned := original.Clone()

	cloned.Metadata["vendor"] = "modified"
	cloned.Capabilities[0] = Capability("modified")
	cloned.Services[0].DependsOn[0] = ServiceID("modified")
	cloned.Services[0].Metadata["key"] = "modified"
	cloned.Channels[0].Metadata["schema"] = "modified"

	if original.Metadata["vendor"] != "example" {
		t.Error("original Metadata should not be affected by clone modification")
	}
	if original.Capabilities[0] != CapabilityRealtimeControl {
		t.Error("original Capabilities should not be affected by clone modification")
	}
	if original.Services[0].DependsOn[0] != ServiceID("svc-2") {
		t.Error("original Services DependsOn should not be affected by clone modification")
	}
	if original.Services[0].Metadata["key"] != "value" {
		t.Error("original Services Metadata should not be affected by clone modification")
	}
	if original.Channels[0].Metadata["schema"] != "v1" {
		t.Error("original Channels Metadata should not be affected by clone modification")
	}
}

func TestGameToolCapabilitiesStayOutsideGameHostDescriptor(t *testing.T) {
	desc := PluginDescriptor{
		ID: PluginID("example-game-plugin"), ExtensionID: "com.example.game",
		Name: "Example Game Plugin", Version: "1.0.0", ProtocolVersion: "amitia-game-host/1",
		Capabilities: []Capability{Capability("example.build")},
	}
	if err := desc.Validate(); err == nil {
		t.Fatal("game-specific tool capability must be registered through Extension Kernel, not GameHost")
	}
}

func TestDescriptorHasNoGameSpecificRequirement(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("generic-controller"),
		ExtensionID:     "com.example.generic",
		Name:            "Generic Controller",
		Version:         "0.1.0",
		ProtocolVersion: "amitia-game-host/1",
		Capabilities: []Capability{
			CapabilityCustomRPC,
		},
		Services: []ServiceDescriptor{
			{
				ID:   ServiceID("controller"),
				Name: "Controller",
				Kind: ServiceKindProcess,
			},
		},
		Channels: []ChannelDescriptor{
			{
				ID: ChannelID("rpc"), ServiceID: ServiceID("controller"), Kind: ChannelKindCustom,
			},
		},
	}

	if err := desc.Validate(); err != nil {
		t.Fatalf("generic non-game descriptor should be valid, got: %v", err)
	}
}

func TestPluginIDExtensionIDSeparation(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("game-plugin-contribution-1"),
		ExtensionID:     "com.example.extension-package",
		Name:            "Game Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "amitia-game-host/1",
	}

	if desc.ID == PluginID(desc.ExtensionID) {
		t.Error("PluginID and ExtensionID should be allowed to be different")
	}

	if err := desc.Validate(); err != nil {
		t.Fatalf("descriptor with different PluginID and ExtensionID should be valid, got: %v", err)
	}
}

func TestPluginDescriptorHasCapability(t *testing.T) {
	desc := PluginDescriptor{
		Capabilities: []Capability{
			CapabilityStateStreaming,
			CapabilityEventStreaming,
			CapabilityBinaryStreaming,
		},
	}

	if !desc.HasCapability(CapabilityStateStreaming) {
		t.Error("expected to find state_streaming capability")
	}
	if !desc.HasCapability(CapabilityEventStreaming) {
		t.Error("expected to find event_streaming capability")
	}
	if !desc.HasCapability(CapabilityBinaryStreaming) {
		t.Error("expected to find binary_streaming capability")
	}
}

func TestPluginDescriptorRejectsChannelUnknownService(t *testing.T) {
	desc := PluginDescriptor{
		ID: "test-plugin", ExtensionID: "com.example.test", Name: "Test", Version: "1.0.0", ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{{ID: "svc-1", Name: "Service 1", Kind: ServiceKindProcess}},
		Channels: []ChannelDescriptor{{ID: "events", ServiceID: "missing", Kind: ChannelKindEvent}},
	}
	if err := desc.Validate(); err == nil {
		t.Fatal("expected unknown channel service to be rejected")
	}
}

func TestEqualPluginDescriptorDetectsChannelServiceChange(t *testing.T) {
	a := PluginDescriptor{ID: "p", ExtensionID: "ext", Name: "P", Version: "1", ProtocolVersion: "amitia-game-host/1",
		Services: []ServiceDescriptor{{ID: "svc-a", Name: "A", Kind: ServiceKindProcess}, {ID: "svc-b", Name: "B", Kind: ServiceKindProcess}},
		Channels: []ChannelDescriptor{{ID: "events", ServiceID: "svc-a", Kind: ChannelKindEvent}},
	}
	b := a.Clone()
	b.Channels[0].ServiceID = "svc-b"
	if EqualPluginDescriptor(a, b) {
		t.Fatal("channel service ownership must participate in descriptor equality")
	}
}
