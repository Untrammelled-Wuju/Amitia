package javascript_main

import (
	"testing"
)

func TestRuntimeFactoryGetByExtensionModulePrefersReady(t *testing.T) {
	factory := NewRuntimeFactory()
	factory.hosts["starting"] = &PluginHost{
		instanceID:  "starting",
		extensionID: "com.example/plugin",
		moduleID:    "main",
		state:       HostStateStarting,
	}
	ready := &PluginHost{
		instanceID:  "ready",
		extensionID: "com.example/plugin",
		moduleID:    "main",
		state:       HostStateReady,
	}
	factory.hosts["ready"] = ready

	host, err := factory.GetByExtensionModule("com.example/plugin", "main")
	if err != nil {
		t.Fatalf("get by extension module: %v", err)
	}
	if host != ready {
		t.Fatalf("expected ready host, got %s", host.InstanceID())
	}
}

func TestRuntimeFactoryGetByExtensionModuleFallsBackToExisting(t *testing.T) {
	factory := NewRuntimeFactory()
	starting := &PluginHost{
		instanceID:  "starting",
		extensionID: "com.example/plugin",
		moduleID:    "main",
		state:       HostStateStarting,
	}
	factory.hosts["starting"] = starting

	host, err := factory.GetByExtensionModule("com.example/plugin", "main")
	if err != nil {
		t.Fatalf("get by extension module: %v", err)
	}
	if host != starting {
		t.Fatalf("expected existing host, got %s", host.InstanceID())
	}
}

func TestRuntimeFactoryGetByExtensionModuleRejectsCrossExtensionLookup(t *testing.T) {
	factory := NewRuntimeFactory()
	factory.hosts["other"] = &PluginHost{
		instanceID:  "other",
		extensionID: "com.example/other",
		moduleID:    "main",
		state:       HostStateReady,
	}

	if _, err := factory.GetByExtensionModule("com.example/plugin", "main"); err == nil {
		t.Fatal("expected missing runtime error")
	}
}
