package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func newTestRuntimeAndDescriptor(rtID domain.RuntimeInstanceID, pluginID domain.PluginID) (*domain.RuntimeInstance, domain.PluginDescriptor) {
	rt, _ := domain.NewRuntimeInstance(rtID, pluginID, time.Now())
	descriptor := domain.PluginDescriptor{
		ID:          pluginID,
		ExtensionID: "ext-test",
		Name:        "Test Plugin",
		Services: []domain.ServiceDescriptor{
			{
				ID:       "core",
				Name:     "Core Service",
				Kind:     domain.ServiceKindProcess,
				Required: true,
			},
		},
	}
	return rt, descriptor
}

func TestTopologyStore_PutAndGet(t *testing.T) {
	store := NewTopologyStore()

	rt, desc := newTestRuntimeAndDescriptor("rt-a", "plugin-a")
	defIDs := map[domain.ServiceID]string{
		"core": "ext-test/core",
	}

	err := store.PutRuntimeGraph(rt, desc, defIDs)
	if err != nil {
		t.Fatalf("PutRuntimeGraph error: %v", err)
	}

	snap, err := store.GetTopologySnapshot("rt-a")
	if err != nil {
		t.Fatalf("GetTopologySnapshot error: %v", err)
	}
	if len(snap.Services) != 1 {
		t.Errorf("topology has %d services, want 1", len(snap.Services))
	}
}

func TestTopologyStore_MultiRuntime_Isolation(t *testing.T) {
	store := NewTopologyStore()

	rtA, descA := newTestRuntimeAndDescriptor("rt-a", "plugin-a")
	rtB, descB := newTestRuntimeAndDescriptor("rt-b", "plugin-b")

	descA.Services = []domain.ServiceDescriptor{{ID: "core", Name: "Core", Kind: domain.ServiceKindProcess, Required: true}}
	descB.Services = []domain.ServiceDescriptor{{ID: "core", Name: "Core", Kind: domain.ServiceKindProcess, Required: true}}

	err := store.PutRuntimeGraph(rtA, descA, map[domain.ServiceID]string{"core": "def-a"})
	if err != nil {
		t.Fatalf("PutRuntimeGraph A error: %v", err)
	}
	err = store.PutRuntimeGraph(rtB, descB, map[domain.ServiceID]string{"core": "def-b"})
	if err != nil {
		t.Fatalf("PutRuntimeGraph B error: %v", err)
	}

	topologyA, err := store.GetTopology("rt-a")
	if err != nil {
		t.Fatalf("GetTopology A error: %v", err)
	}
	svcA, err := topologyA.GetService("core")
	if err != nil {
		t.Fatalf("GetService A/core error: %v", err)
	}
	if svcA.RuntimeID != "rt-a" {
		t.Errorf("runtime A service has RuntimeID %q, want rt-a", svcA.RuntimeID)
	}

	defA, err := store.ResolveDefinitionID("rt-a", "core")
	if err != nil {
		t.Fatalf("ResolveDefinitionID A error: %v", err)
	}
	if defA != "def-a" {
		t.Errorf("definition for A/core = %q, want def-a", defA)
	}

	defB, err := store.ResolveDefinitionID("rt-b", "core")
	if err != nil {
		t.Fatalf("ResolveDefinitionID B error: %v", err)
	}
	if defB != "def-b" {
		t.Errorf("definition for B/core = %q, want def-b", defB)
	}
}

func TestTopologyStore_UnknownRuntime_FailClosed(t *testing.T) {
	store := NewTopologyStore()

	_, err := store.GetTopologySnapshot("rt-nonexistent")
	if err == nil {
		t.Error("expected error for unknown runtime")
	}

	_, err = store.GetDependencyGraphSnapshot("rt-nonexistent")
	if err == nil {
		t.Error("expected error for unknown runtime graph")
	}

	_, err = store.GetTopology("rt-nonexistent")
	if err == nil {
		t.Error("expected error for unknown topology")
	}
}

func TestTopologyStore_ResolveDefinitionID_UnknownService_FailClosed(t *testing.T) {
	store := NewTopologyStore()

	rt, desc := newTestRuntimeAndDescriptor("rt-a", "plugin-a")
	_ = store.PutRuntimeGraph(rt, desc, map[domain.ServiceID]string{"core": "def-core"})

	_, err := store.ResolveDefinitionID("rt-a", "unknown-svc")
	if err == nil {
		t.Error("expected error for unknown service")
	}
}

func TestTopologyStore_DependencyGraphSnapshot(t *testing.T) {
	store := NewTopologyStore()

	rt, _ := domain.NewRuntimeInstance("rt-a", "plugin-a", time.Now())
	descriptor := domain.PluginDescriptor{
		ID:          "plugin-a",
		ExtensionID: "ext-test",
		Services: []domain.ServiceDescriptor{
			{ID: "svc-1", Name: "S1", Kind: domain.ServiceKindProcess, Required: true},
			{ID: "svc-2", Name: "S2", Kind: domain.ServiceKindProcess, Required: true, DependsOn: []domain.ServiceID{"svc-1"}},
		},
	}

	err := store.PutRuntimeGraph(rt, descriptor, map[domain.ServiceID]string{
		"svc-1": "def-1",
		"svc-2": "def-2",
	})
	if err != nil {
		t.Fatalf("PutRuntimeGraph error: %v", err)
	}

	graphSnap, err := store.GetDependencyGraphSnapshot("rt-a")
	if err != nil {
		t.Fatalf("GetDependencyGraphSnapshot error: %v", err)
	}
	if len(graphSnap.Nodes) != 2 {
		t.Errorf("graph has %d nodes, want 2", len(graphSnap.Nodes))
	}
}

func TestTopologyStore_RemoveRuntime(t *testing.T) {
	store := NewTopologyStore()

	rt, desc := newTestRuntimeAndDescriptor("rt-a", "plugin-a")
	_ = store.PutRuntimeGraph(rt, desc, nil)

	err := store.RemoveRuntime("rt-a")
	if err != nil {
		t.Fatalf("RemoveRuntime error: %v", err)
	}

	_, err = store.GetTopologySnapshot("rt-a")
	if err == nil {
		t.Error("expected error after runtime removed")
	}
}

func TestTopologyStore_RemoveRuntime_NotFound(t *testing.T) {
	store := NewTopologyStore()

	err := store.RemoveRuntime("rt-nonexistent")
	if err == nil {
		t.Error("expected error removing nonexistent runtime")
	}
}

func TestTopologyStoreResolveServiceIDByModule(t *testing.T) {
	store := NewTopologyStore()
	rt, desc := newTestRuntimeAndDescriptor("rt-module", "plugin-module")
	if err := store.PutRuntimeGraph(rt, desc, map[domain.ServiceID]string{"core": "def-core"}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindModuleID("rt-module", "core", "module-core"); err != nil {
		t.Fatal(err)
	}
	serviceID, err := store.ResolveServiceIDByModule("rt-module", "module-core")
	if err != nil {
		t.Fatal(err)
	}
	if serviceID != "core" {
		t.Fatalf("service id = %q, want core", serviceID)
	}
}

func TestTopologyStoreResolveServiceIDByModuleRejectsAmbiguousMapping(t *testing.T) {
	store := NewTopologyStore()
	rt, _ := domain.NewRuntimeInstance("rt-ambiguous", "plugin-ambiguous", time.Now())
	desc := domain.PluginDescriptor{
		ID: "plugin-ambiguous", ExtensionID: "ext-test", Name: "Test",
		Services: []domain.ServiceDescriptor{
			{ID: "svc-a", Name: "A", Kind: domain.ServiceKindProcess, Required: true},
			{ID: "svc-b", Name: "B", Kind: domain.ServiceKindProcess, Required: true},
		},
	}
	if err := store.PutRuntimeGraph(rt, desc, map[domain.ServiceID]string{"svc-a": "def-a", "svc-b": "def-b"}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindModuleID("rt-ambiguous", "svc-a", "same-module"); err != nil {
		t.Fatal(err)
	}
	if err := store.BindModuleID("rt-ambiguous", "svc-b", "same-module"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveServiceIDByModule("rt-ambiguous", "same-module"); err == nil {
		t.Fatal("expected ambiguous module mapping to fail closed")
	}
}
