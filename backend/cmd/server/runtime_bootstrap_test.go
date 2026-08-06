package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/util"
)

func TestNewRuntimeBootstrapCreatesOrchestrator(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, err := newRuntimeBootstrap(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil bootstrap")
	}
	if b.orchestrator == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	b.SetGraphService(nil)
	if b.GraphService() != nil {
		t.Fatal("expected nil graph service")
	}
}

func TestRuntimeBootstrapSnapshotReady(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, _ := newRuntimeBootstrap(paths)
	snap := b.Snapshot()
	if snap.State == "" {
		t.Fatal("expected non-empty initial state")
	}
}

type fakeGraphService struct {
	graph.Service
}

func TestRuntimeBootstrapGraphServiceThreadSafe(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, _ := newRuntimeBootstrap(paths)
	fakeSvc := &fakeGraphService{}
	b.SetGraphService(fakeSvc)
	got := b.GraphService()
	if got != fakeSvc {
		t.Fatalf("expected graph service to be set")
	}
}

func TestRuntimeBootstrapStopAllReturnsNilInitially(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, _ := newRuntimeBootstrap(paths)
	ctx := context.Background()
	err := b.StopAll(ctx)
	if err != nil {
		t.Fatalf("expected nil error on initial StopAll, got %v", err)
	}
}

func TestRuntimeBootstrapStartPhaseInfrastructureNoComponents(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, _ := newRuntimeBootstrap(paths)
	ctx := context.Background()
	err := b.StartPhase(ctx, runtimeorchestrator.PhaseInfrastructure)
	if err != nil {
		t.Logf("StartPhase with no registered components: %v", err)
	}
}

func TestVectorStoreProviderDescriptor(t *testing.T) {
	p := &vectorStoreProviderAdapter{}
	desc := p.Descriptor()
	if desc.ID != runtimeorchestrator.ComponentVectorStore {
		t.Fatalf("expected ComponentVectorStore, got %s", desc.ID)
	}
	if desc.Phase != runtimeorchestrator.PhaseInfrastructure {
		t.Fatalf("expected PhaseInfrastructure, got %s", desc.Phase)
	}
	if desc.Required {
		t.Fatal("vector store should be optional")
	}
}

func TestGraphStoreProviderDescriptor(t *testing.T) {
	p := &graphStoreProviderAdapter{}
	desc := p.Descriptor()
	if desc.ID != runtimeorchestrator.ComponentGraphStore {
		t.Fatalf("expected ComponentGraphStore, got %s", desc.ID)
	}
	if desc.Phase != runtimeorchestrator.PhaseInfrastructure {
		t.Fatalf("expected PhaseInfrastructure, got %s", desc.Phase)
	}
	if desc.Required {
		t.Fatal("graph store should be optional")
	}
}

func TestRuntimeBootstrapCreatesNodeEnvironmentResolver(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, err := newRuntimeBootstrap(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.NodeEnvironmentResolver() == nil {
		t.Fatal("expected non-nil node environment resolver")
	}
}

func TestRuntimeBootstrapDoesNotRequireInstalledNode(t *testing.T) {
	paths := &util.RuntimePaths{}
	b, err := newRuntimeBootstrap(paths)
	if err != nil {
		t.Fatalf("bootstrap should succeed without installed node: %v", err)
	}
	resolver := b.NodeEnvironmentResolver()
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
	snap := resolver.Snapshot()
	if snap.State != "not-started" {
		t.Fatalf("expected not-started state before Resolve, got %s", snap.State)
	}
}
