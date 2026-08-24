package domain

import (
	"testing"
)

func TestPluginIDConstruction(t *testing.T) {
	id := PluginID("test-plugin-001")
	if string(id) != "test-plugin-001" {
		t.Errorf("unexpected plugin id: %s", id)
	}

	id2 := PluginID("test-plugin-001")
	if id != id2 {
		t.Errorf("expected equal plugin ids: %s vs %s", id, id2)
	}
}

func TestPluginDescriptorConstruction(t *testing.T) {
	desc := PluginDescriptor{
		ID:              PluginID("my-plugin"),
		ExtensionID:     "ext-001",
		Name:            "My Plugin",
		Version:         "1.0.0",
		ProtocolVersion: "v1",

		Capabilities: []Capability{
			CapabilityRealtimeControl,
			CapabilityStateStreaming,
		},
		Services: []ServiceDescriptor{
			{
				ID:       ServiceID("main-service"),
				Name:     "Main",
				Kind:     ServiceKindProcess,
				Required: true,
			},
		},
		Channels: []ChannelDescriptor{
			{
				ID:        ChannelID("events"),
				ServiceID: ServiceID("main-service"),
				Kind:      ChannelKindEvent,
			},
		},
		Metadata: map[string]string{
			"author": "test",
		},
	}

	if desc.ID != PluginID("my-plugin") {
		t.Error("unexpected ID")
	}
	if len(desc.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(desc.Capabilities))
	}
	if len(desc.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(desc.Services))
	}
	if len(desc.Channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(desc.Channels))
	}
}

func TestCustomCapability(t *testing.T) {
	customCap := Capability("example.navigation")
	if string(customCap) != "example.navigation" {
		t.Fatalf("custom capability changed unexpectedly: %s", customCap)
	}

	desc := PluginDescriptor{
		Capabilities: []Capability{
			"example.navigation",
			"example.build",
			"custom.visual-agent",
		},
	}

	for _, cap := range desc.Capabilities {
		if string(cap) == "" {
			t.Error("capability should not be empty")
		}
	}
}

func TestServiceDependsOn(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("worker"),
		Name: "Worker",
		DependsOn: []ServiceID{
			ServiceID("database"),
			ServiceID("logger"),
		},
	}

	if len(svc.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(svc.DependsOn))
	}
	if svc.DependsOn[0] != ServiceID("database") {
		t.Error("unexpected dependency")
	}
}

func TestChannelDescriptorDoesNotInterpretSchema(t *testing.T) {
	ch := ChannelDescriptor{
		ID:        ChannelID("metrics"),
		ServiceID: ServiceID("main-service"),
		Kind:      ChannelKindMetric,
		SchemaID:  "metric-schema-v1",
	}

	if ch.SchemaID != "metric-schema-v1" {
		t.Error("schema id should be stored but not interpreted")
	}
	// Channel Descriptor should not contain buffer, websocket, or cursor info
}
