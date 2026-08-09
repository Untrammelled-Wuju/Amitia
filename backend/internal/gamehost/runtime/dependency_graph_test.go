package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func buildTestSnapshot(runtimeID domain.RuntimeInstanceID, services []ServiceInstanceSnapshot) RuntimeTopologySnapshot {
	return RuntimeTopologySnapshot{
		RuntimeID: runtimeID,
		Services:  services,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func svcSnapshot(id domain.ServiceID, deps []domain.ServiceID) ServiceInstanceSnapshot {
	return ServiceInstanceSnapshot{
		ID:           BuildServiceInstanceID("test-runtime", id),
		RuntimeID:    "test-runtime",
		PluginID:     "test-plugin",
		ServiceID:    id,
		State:        ServiceStateCreated,
		Required:     true,
		Dependencies: deps,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestDependencyGraph_Empty(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", graph.NodeCount())
	}
	if graph.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", graph.EdgeCount())
	}

	roots := graph.Roots()
	if len(roots) != 0 {
		t.Errorf("expected 0 roots, got %d", len(roots))
	}

	leaves := graph.Leaves()
	if len(leaves) != 0 {
		t.Errorf("expected 0 leaves, got %d", len(leaves))
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("expected empty sort, got %v", sorted)
	}
}

func TestDependencyGraph_SingleNode(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", graph.NodeCount())
	}

	roots := graph.Roots()
	if len(roots) != 1 || roots[0] != "bridge" {
		t.Errorf("expected roots [bridge], got %v", roots)
	}

	leaves := graph.Leaves()
	if len(leaves) != 1 || leaves[0] != "bridge" {
		t.Errorf("expected leaves [bridge], got %v", leaves)
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 1 || sorted[0] != "bridge" {
		t.Errorf("expected [bridge], got %v", sorted)
	}
}

func TestDependencyGraph_Linear(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", graph.NodeCount())
	}
	if graph.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", graph.EdgeCount())
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []domain.ServiceID{"bridge", "agent", "planner"}
	if len(sorted) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, sorted)
	}
	for i, id := range expected {
		if sorted[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, sorted[i])
		}
	}
}

func TestDependencyGraph_Diamond(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", []domain.ServiceID{"A"}),
		svcSnapshot("D", []domain.ServiceID{"B", "C"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.NodeCount() != 4 {
		t.Errorf("expected 4 nodes, got %d", graph.NodeCount())
	}
	if graph.EdgeCount() != 4 {
		t.Errorf("expected 4 edges, got %d", graph.EdgeCount())
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sorted[0] != "A" {
		t.Errorf("expected first element to be A, got %s", sorted[0])
	}
	if sorted[3] != "D" {
		t.Errorf("expected last element to be D, got %s", sorted[3])
	}

	posA := -1
	posB := -1
	posC := -1
	posD := -1
	for i, id := range sorted {
		switch id {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		case "D":
			posD = i
		}
	}

	if posA >= posB || posA >= posC {
		t.Error("A should come before B and C")
	}
	if posB >= posD || posC >= posD {
		t.Error("B and C should come before D")
	}
}

func TestDependencyGraph_MultipleRoots(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", nil),
		svcSnapshot("C", []domain.ServiceID{"A"}),
		svcSnapshot("D", []domain.ServiceID{"B"}),
		svcSnapshot("E", nil),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roots := graph.Roots()
	expected := []domain.ServiceID{"A", "B", "E"}
	if len(roots) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, roots)
	}
	for i, id := range expected {
		if roots[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, roots[i])
		}
	}
}

func TestDependencyGraph_MultipleIndependentGraphs(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", nil),
		svcSnapshot("D", []domain.ServiceID{"C"}),
		svcSnapshot("E", nil),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.NodeCount() != 5 {
		t.Errorf("expected 5 nodes, got %d", graph.NodeCount())
	}

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 5 {
		t.Errorf("expected 5 sorted, got %v", sorted)
	}
}

func TestDependencyGraph_ReverseTopologicalSort(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reversed, err := graph.ReverseTopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []domain.ServiceID{"planner", "agent", "bridge"}
	if len(reversed) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, reversed)
	}
	for i, id := range expected {
		if reversed[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, reversed[i])
		}
	}
}

func TestDependencyGraph_DependenciesOf(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps, err := graph.DependenciesOf("planner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 || deps[0] != "agent" {
		t.Errorf("expected [agent], got %v", deps)
	}

	deps, err = graph.DependenciesOf("bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected empty, got %v", deps)
	}
}

func TestDependencyGraph_DependentsOf(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("vision", []domain.ServiceID{"bridge"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dependents, err := graph.DependentsOf("bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []domain.ServiceID{"agent", "vision"}
	if len(dependents) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, dependents)
	}
	for i, id := range expected {
		if dependents[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, dependents[i])
		}
	}
}

func TestDependencyGraph_TransitiveDependencies(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transitive, err := graph.TransitiveDependencies("planner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []domain.ServiceID{"bridge", "agent"}
	if len(transitive) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, transitive)
	}
	for i, id := range expected {
		if transitive[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, transitive[i])
		}
	}
}

func TestDependencyGraph_TransitiveDependents(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transitive, err := graph.TransitiveDependents("bridge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []domain.ServiceID{"planner", "agent"}
	if len(transitive) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, transitive)
	}
	for i, id := range expected {
		if transitive[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, transitive[i])
		}
	}
}

func TestDependencyGraph_TransitiveDependencies_Diamond(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", []domain.ServiceID{"A"}),
		svcSnapshot("D", []domain.ServiceID{"B", "C"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps, err := graph.TransitiveDependencies("D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := make(map[domain.ServiceID]struct{})
	for _, d := range deps {
		if _, ok := seen[d]; ok {
			t.Errorf("duplicate dependency in result: %s", d)
		}
		seen[d] = struct{}{}
	}

	if len(deps) != 3 {
		t.Errorf("expected 3 transitive dependencies (A, B, C), got %v", deps)
	}
}

func TestDependencyGraph_Snapshot(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graphSnapshot := graph.Snapshot()
	if graphSnapshot.RuntimeID != "rt-001" {
		t.Errorf("expected runtime rt-001, got %s", graphSnapshot.RuntimeID)
	}
	if len(graphSnapshot.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graphSnapshot.Nodes))
	}

	expectedOrder := []domain.ServiceID{"agent", "bridge"}
	for i, node := range graphSnapshot.Nodes {
		if node.ServiceID != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], node.ServiceID)
		}
	}
}

func TestDependencyGraph_SnapshotDeepCopy(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graphSnapshot := graph.Snapshot()
	if len(graphSnapshot.Nodes) > 0 && len(graphSnapshot.Nodes[0].Dependencies) > 0 {
		graphSnapshot.Nodes[0].Dependencies[0] = "modified"
	}

	node, _ := graph.Node("agent")
	if node.Dependencies[0] != "bridge" {
		t.Error("modifying snapshot affected original graph")
	}
}

func TestDependencyGraph_NilTopology(t *testing.T) {
	builder := NewDependencyGraphBuilder()
	_, err := builder.Build(nil)
	if err == nil {
		t.Fatal("expected error for nil topology")
	}
	if !IsTopologyError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestDependencyGraph_DependenciesOf_NotFound(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = graph.DependenciesOf("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
	if !IsTopologyError(err, ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestDependencyGraph_TopologicalSort_Deterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
			svcSnapshot("bridge", nil),
			svcSnapshot("agent", []domain.ServiceID{"bridge"}),
			svcSnapshot("vision", []domain.ServiceID{"bridge"}),
			svcSnapshot("planner", []domain.ServiceID{"agent", "vision"}),
		})

		builder := NewDependencyGraphBuilder()
		graph, err := builder.Build(&snapshot)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		sorted, err := graph.TopologicalSort()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		if sorted[0] != "bridge" {
			t.Errorf("iteration %d: expected first to be bridge, got %s", i, sorted[0])
		}
		if sorted[3] != "planner" {
			t.Errorf("iteration %d: expected last to be planner, got %s", i, sorted[3])
		}
	}
}

func TestDependencyGraph_RootsAreLeaves(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roots := graph.Roots()
	if len(roots) != 1 || roots[0] != "A" {
		t.Errorf("expected roots [A], got %v", roots)
	}

	leaves := graph.Leaves()
	if len(leaves) != 1 || leaves[0] != "B" {
		t.Errorf("expected leaves [B], got %v", leaves)
	}
}
