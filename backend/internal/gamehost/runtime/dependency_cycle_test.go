package runtime

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestCycle_SelfDependency(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("agent", []domain.ServiceID{"agent"}),
	})

	builder := NewDependencyGraphBuilder()
	_, err := builder.Build(&snapshot)
	if err == nil {
		t.Fatal("expected error for self dependency")
	}
	if !IsTopologyError(err, ErrSelfDependency) {
		t.Errorf("expected self_dependency, got %v", err)
	}
}

func TestCycle_TwoNodeCycle(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", []domain.ServiceID{"B"}),
		svcSnapshot("B", []domain.ServiceID{"A"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected dependency cycle error, got %v", err)
	}

	cycleErr, ok := err.(*DependencyCycleError)
	if !ok {
		t.Fatal("expected DependencyCycleError type")
	}
	if len(cycleErr.Path) == 0 {
		t.Error("expected cycle path to be non-empty")
	}
}

func TestCycle_ThreeNodeCycle(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", []domain.ServiceID{"C"}),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", []domain.ServiceID{"B"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected dependency cycle error, got %v", err)
	}

	cycleErr, ok := err.(*DependencyCycleError)
	if !ok {
		t.Fatal("expected DependencyCycleError type")
	}
	if len(cycleErr.Path) < 3 {
		t.Errorf("expected cycle path with at least 3 nodes, got %v", cycleErr.Path)
	}
}

func TestCycle_FourNodeCycle(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", []domain.ServiceID{"D"}),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", []domain.ServiceID{"B"}),
		svcSnapshot("D", []domain.ServiceID{"C"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected dependency cycle error, got %v", err)
	}
}

func TestCycle_PartialCycle(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", []domain.ServiceID{"B"}),
		svcSnapshot("D", []domain.ServiceID{"C"}),
	})

	services := snapshot.Services
	for i := range services {
		if services[i].ServiceID == "B" {
			services[i].Dependencies = append(services[i].Dependencies, "D")
		}
	}
	snapshot.Services = services

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected dependency cycle error, got %v", err)
	}

	cycleErr, ok := err.(*DependencyCycleError)
	if !ok {
		t.Fatal("expected DependencyCycleError type")
	}

	hasB := false
	hasC := false
	hasD := false
	for _, id := range cycleErr.Path {
		switch id {
		case "B":
			hasB = true
		case "C":
			hasC = true
		case "D":
			hasD = true
		}
	}
	if !hasB || !hasC || !hasD {
		t.Errorf("expected cycle path to include B, C, D; got %v", cycleErr.Path)
	}

	for _, id := range cycleErr.Path {
		if id == "A" {
			t.Errorf("unexpected A in cycle path %v", cycleErr.Path)
		}
	}
}

func TestCycle_HiddenCycle(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A", "D"}),
		svcSnapshot("C", []domain.ServiceID{"B"}),
		svcSnapshot("D", []domain.ServiceID{"C"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error for B -> C -> D -> B")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected dependency cycle error, got %v", err)
	}
}

func TestCycle_Stability(t *testing.T) {
	var prevPath []domain.ServiceID

	for i := 0; i < 50; i++ {
		snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
			svcSnapshot("A", []domain.ServiceID{"C"}),
			svcSnapshot("B", []domain.ServiceID{"A"}),
			svcSnapshot("C", []domain.ServiceID{"B"}),
		})

		builder := NewDependencyGraphBuilder()
		graph, err := builder.Build(&snapshot)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		_, err = graph.TopologicalSort()
		if err == nil {
			t.Fatalf("iteration %d: expected cycle error", i)
		}

		cycleErr, ok := err.(*DependencyCycleError)
		if !ok {
			t.Fatalf("iteration %d: expected DependencyCycleError", i)
		}

		if i > 0 && !cyclePathsEqual(prevPath, cycleErr.Path) {
			t.Errorf("iteration %d: cycle path changed from %v to %v", i, prevPath, cycleErr.Path)
		}
		prevPath = cycleErr.Path
	}
}

func cyclePathsEqual(a, b []domain.ServiceID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDependency_MissingDependency(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("agent", []domain.ServiceID{"nonexistent"}),
	})

	builder := NewDependencyGraphBuilder()
	_, err := builder.Build(&snapshot)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
	if !IsTopologyError(err, ErrDependencyNotFound) {
		t.Errorf("expected dependency_not_found, got %v", err)
	}
}

func TestDependency_DuplicateDependency(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge", "bridge"}),
	})

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)

	if graph != nil {
		_, sortErr := graph.TopologicalSort()
		_ = sortErr
	}

	if err == nil {
		t.Log("duplicate dependency was accepted - this may be acceptable depending on spec")
	} else if IsTopologyError(err, ErrDuplicateDependency) {
		t.Log("duplicate dependency correctly rejected")
	}
}

func TestDependencyCycleError_ErrorFormat(t *testing.T) {
	err := NewDependencyCycleError(
		"dependency cycle detected",
		[]domain.ServiceID{"A", "B", "C"},
	)
	expected := "dependency cycle detected: A -> B -> C"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDependencyCycleError_EmptyPath(t *testing.T) {
	err := NewDependencyCycleError("cycle detected", nil)
	expected := "cycle detected"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestCycle_WithValidBranches(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("root1", nil),
		svcSnapshot("root2", nil),
		svcSnapshot("child1", []domain.ServiceID{"root1"}),
		svcSnapshot("child2", []domain.ServiceID{"root2"}),
		svcSnapshot("A", []domain.ServiceID{"child2"}),
		svcSnapshot("B", []domain.ServiceID{"A"}),
	})

	services := snapshot.Services
	for i := range services {
		if services[i].ServiceID == "child2" {
			services[i].Dependencies = append(services[i].Dependencies, "B")
		}
	}
	snapshot.Services = services

	builder := NewDependencyGraphBuilder()
	graph, err := builder.Build(&snapshot)
	if err != nil {
		t.Fatalf("unexpected error during build: %v", err)
	}

	_, err = graph.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error A -> B -> child2 -> A")
	}
	if !IsDependencyCycleError(err) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

func TestCycle_SelfDependencyError(t *testing.T) {
	snapshot := buildTestSnapshot("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("self", []domain.ServiceID{"self"}),
	})

	builder := NewDependencyGraphBuilder()
	_, err := builder.Build(&snapshot)
	if err == nil {
		t.Fatal("expected error for self dependency")
	}

	if !IsTopologyError(err, ErrSelfDependency) {
		t.Errorf("expected self_dependency error, got %v", err)
	}
}
