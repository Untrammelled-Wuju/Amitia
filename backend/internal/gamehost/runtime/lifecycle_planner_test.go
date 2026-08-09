package runtime

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func buildTestLifecycleTopology(runtimeID domain.RuntimeInstanceID, services []ServiceInstanceSnapshot) RuntimeTopologySnapshot {
	return RuntimeTopologySnapshot{
		RuntimeID: runtimeID,
		PluginID:  "test-plugin",
		Services:  services,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func buildTestLifecycleGraph(runtimeID domain.RuntimeInstanceID, nodes []DependencyNodeSnapshot) DependencyGraphSnapshot {
	return DependencyGraphSnapshot{
		RuntimeID: runtimeID,
		Nodes:     nodes,
	}
}

func lnode(serviceID domain.ServiceID, deps []domain.ServiceID, dependents []domain.ServiceID) DependencyNodeSnapshot {
	return DependencyNodeSnapshot{
		ServiceID:    serviceID,
		Dependencies: deps,
		Dependents:   dependents,
	}
}

func TestLifecyclePlanner_Empty(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startup.StageCount() != 0 {
		t.Errorf("expected 0 startup stages, got %d", startup.StageCount())
	}

	shutdown, err := planner.BuildShutdownPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown.StageCount() != 0 {
		t.Errorf("expected 0 shutdown stages, got %d", shutdown.StageCount())
	}
}

func TestLifecyclePlanner_SingleService(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startup.StageCount() != 1 {
		t.Errorf("expected 1 startup stage, got %d", startup.StageCount())
	}
	if len(startup.Stages[0].Services) != 1 || startup.Stages[0].Services[0].ServiceID != "bridge" {
		t.Errorf("expected bridge in stage 0, got %v", startup.Stages[0].Services)
	}

	shutdown, err := planner.BuildShutdownPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown.StageCount() != 1 {
		t.Errorf("expected 1 shutdown stage, got %d", shutdown.StageCount())
	}
}

func TestLifecyclePlanner_LinearDependency(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, []domain.ServiceID{"planner"}),
		lnode("planner", []domain.ServiceID{"agent"}, nil),
	})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if startup.StageCount() != 3 {
		t.Errorf("expected 3 startup stages, got %d", startup.StageCount())
	}

	expectedStages := []struct {
		index    int
		services []domain.ServiceID
	}{
		{0, []domain.ServiceID{"bridge"}},
		{1, []domain.ServiceID{"agent"}},
		{2, []domain.ServiceID{"planner"}},
	}

	for _, expected := range expectedStages {
		stage := startup.Stages[expected.index]
		if len(stage.Services) != len(expected.services) {
			t.Fatalf("stage %d: expected %v, got %v", expected.index, expected.services, stage.Services)
		}
		for i, svc := range stage.Services {
			if svc.ServiceID != expected.services[i] {
				t.Errorf("stage %d position %d: expected %s, got %s", expected.index, i, expected.services[i], svc.ServiceID)
			}
		}
	}

	shutdown, err := planner.BuildShutdownPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown.StageCount() != 3 {
		t.Errorf("expected 3 shutdown stages, got %d", shutdown.StageCount())
	}

	reverseExpected := []struct {
		index    int
		services []domain.ServiceID
	}{
		{0, []domain.ServiceID{"planner"}},
		{1, []domain.ServiceID{"agent"}},
		{2, []domain.ServiceID{"bridge"}},
	}

	for _, expected := range reverseExpected {
		stage := shutdown.Stages[expected.index]
		if len(stage.Services) != len(expected.services) {
			t.Fatalf("shutdown stage %d: expected %v, got %v", expected.index, expected.services, stage.Services)
		}
		for i, svc := range stage.Services {
			if svc.ServiceID != expected.services[i] {
				t.Errorf("shutdown stage %d position %d: expected %s, got %s", expected.index, i, expected.services[i], svc.ServiceID)
			}
		}
	}
}

func TestLifecyclePlanner_DiamondGraph(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("vision", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent", "vision"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent", "vision"}),
		lnode("agent", []domain.ServiceID{"bridge"}, []domain.ServiceID{"planner"}),
		lnode("vision", []domain.ServiceID{"bridge"}, []domain.ServiceID{"planner"}),
		lnode("planner", []domain.ServiceID{"agent", "vision"}, nil),
	})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if startup.StageCount() != 3 {
		t.Errorf("expected 3 stages, got %d", startup.StageCount())
	}

	if len(startup.Stages[0].Services) != 1 || startup.Stages[0].Services[0].ServiceID != "bridge" {
		t.Errorf("stage 0 should be [bridge], got %v", startup.Stages[0].Services)
	}

	if len(startup.Stages[1].Services) != 2 {
		t.Errorf("stage 1 should have 2 services, got %d", len(startup.Stages[1].Services))
	}

	stage1IDs := []domain.ServiceID{
		startup.Stages[1].Services[0].ServiceID,
		startup.Stages[1].Services[1].ServiceID,
	}
	if stage1IDs[0] != "agent" || stage1IDs[1] != "vision" {
		t.Errorf("stage 1 should be [agent, vision], got %v", stage1IDs)
	}

	if len(startup.Stages[2].Services) != 1 || startup.Stages[2].Services[0].ServiceID != "planner" {
		t.Errorf("stage 2 should be [planner], got %v", startup.Stages[2].Services)
	}
}

func TestLifecyclePlanner_IndependentServices(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", nil),
		svcSnapshot("C", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("A", nil, nil),
		lnode("B", nil, nil),
		lnode("C", nil, nil),
	})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if startup.StageCount() != 1 {
		t.Errorf("expected 1 stage, got %d", startup.StageCount())
	}
	if len(startup.Stages[0].Services) != 3 {
		t.Errorf("expected 3 services in stage 0, got %d", len(startup.Stages[0].Services))
	}
}

func TestLifecyclePlanner_MultipleIndependentGraphs(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
		svcSnapshot("C", nil),
		svcSnapshot("D", []domain.ServiceID{"C"}),
		svcSnapshot("E", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("A", nil, []domain.ServiceID{"B"}),
		lnode("B", []domain.ServiceID{"A"}, nil),
		lnode("C", nil, []domain.ServiceID{"D"}),
		lnode("D", []domain.ServiceID{"C"}, nil),
		lnode("E", nil, nil),
	})

	planner := NewLifecyclePlanner()

	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if startup.StageCount() != 2 {
		t.Errorf("expected 2 stages, got %d", startup.StageCount())
	}

	stage0IDs := []domain.ServiceID{
		startup.Stages[0].Services[0].ServiceID,
		startup.Stages[0].Services[1].ServiceID,
		startup.Stages[0].Services[2].ServiceID,
	}
	expected0 := []domain.ServiceID{"A", "C", "E"}
	for i, id := range expected0 {
		if stage0IDs[i] != id {
			t.Errorf("stage 0 position %d: expected %s, got %s", i, id, stage0IDs[i])
		}
	}

	stage1IDs := []domain.ServiceID{
		startup.Stages[1].Services[0].ServiceID,
		startup.Stages[1].Services[1].ServiceID,
	}
	expected1 := []domain.ServiceID{"B", "D"}
	for i, id := range expected1 {
		if stage1IDs[i] != id {
			t.Errorf("stage 1 position %d: expected %s, got %s", i, id, stage1IDs[i])
		}
	}
}

func TestLifecyclePlan_Flatten(t *testing.T) {
	plan := LifecyclePlan{
		RuntimeID: "rt-001",
		Action:    LifecycleActionStart,
		Stages: []LifecycleStage{
			{Index: 0, Services: []ServicePlanEntry{
				{ServiceID: "bridge"},
			}},
			{Index: 1, Services: []ServicePlanEntry{
				{ServiceID: "agent"},
				{ServiceID: "vision"},
			}},
			{Index: 2, Services: []ServicePlanEntry{
				{ServiceID: "planner"},
			}},
		},
	}

	flat := plan.Flatten()
	expected := []domain.ServiceID{"bridge", "agent", "vision", "planner"}
	if len(flat) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, flat)
	}
	for i, id := range expected {
		if flat[i] != id {
			t.Errorf("position %d: expected %s, got %s", i, id, flat[i])
		}
	}
}

func TestLifecyclePlan_ContainsService(t *testing.T) {
	plan := LifecyclePlan{
		Stages: []LifecycleStage{
			{Index: 0, Services: []ServicePlanEntry{
				{ServiceID: "bridge"},
			}},
			{Index: 1, Services: []ServicePlanEntry{
				{ServiceID: "agent"},
			}},
		},
	}

	if !plan.ContainsService("bridge") {
		t.Error("expected to find bridge")
	}
	if !plan.ContainsService("agent") {
		t.Error("expected to find agent")
	}
	if plan.ContainsService("planner") {
		t.Error("did not expect to find planner")
	}
}

func TestLifecyclePlanner_TopologyGraphMismatch(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", nil),
		svcSnapshot("C", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("A", nil, nil),
		lnode("B", nil, nil),
	})

	planner := NewLifecyclePlanner()

	_, err := planner.BuildStartupPlan(topo, graph)
	if err == nil {
		t.Fatal("expected error for topology/graph mismatch")
	}
	if !IsLifecyclePlanError(err, ErrTopologyGraphMismatch) {
		t.Errorf("expected topology_graph_mismatch, got %v", err)
	}
}

func TestLifecyclePlanner_RuntimeIDMismatch(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
	})
	graph := buildTestLifecycleGraph("rt-002", []DependencyNodeSnapshot{
		lnode("A", nil, nil),
	})

	planner := NewLifecyclePlanner()

	_, err := planner.BuildStartupPlan(topo, graph)
	if err == nil {
		t.Fatal("expected error for runtime id mismatch")
	}
	if !IsLifecyclePlanError(err, ErrTopologyGraphMismatch) {
		t.Errorf("expected topology_graph_mismatch, got %v", err)
	}
}

func TestLifecyclePlanner_DependencyCycle(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", []domain.ServiceID{"B"}),
		svcSnapshot("B", []domain.ServiceID{"A"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("A", []domain.ServiceID{"B"}, []domain.ServiceID{"B"}),
		lnode("B", []domain.ServiceID{"A"}, []domain.ServiceID{"A"}),
	})

	planner := NewLifecyclePlanner()

	_, err := planner.BuildStartupPlan(topo, graph)
	if err == nil {
		t.Fatal("expected error for dependency cycle")
	}
}

func TestLifecyclePlanner_StageOrderIsStable(t *testing.T) {
	for i := 0; i < 50; i++ {
		topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
			svcSnapshot("zebra", nil),
			svcSnapshot("alpha", nil),
			svcSnapshot("mango", []domain.ServiceID{"alpha"}),
		})
		graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
			lnode("zebra", nil, nil),
			lnode("alpha", nil, []domain.ServiceID{"mango"}),
			lnode("mango", []domain.ServiceID{"alpha"}, nil),
		})

		planner := NewLifecyclePlanner()
		startup, err := planner.BuildStartupPlan(topo, graph)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		if len(startup.Stages) < 1 {
			continue
		}

		stage0IDs := startup.Stages[0].Services[0].ServiceID
		if i == 0 {
			if stage0IDs != "alpha" {
				t.Errorf("expected first service in stage 0 to be alpha, got %s", stage0IDs)
			}
		}
	}
}

func TestLifecyclePlanner_HasNoSideEffects(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
	})

	planner := NewLifecyclePlanner()

	_, _ = planner.BuildStartupPlan(topo, graph)

	if len(topo.Services) != 2 {
		t.Error("topology was modified")
	}
	if len(graph.Nodes) != 2 {
		t.Error("graph was modified")
	}
}

func TestValidateLifecyclePlan_Startup(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
	})

	planner := NewLifecyclePlanner()
	startup, err := planner.BuildStartupPlan(topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = ValidateLifecyclePlan(startup, topo, graph)
	if err != nil {
		t.Errorf("expected valid plan, got error: %v", err)
	}
}

func TestValidateLifecyclePlan_DuplicateService(t *testing.T) {
	plan := LifecyclePlan{
		RuntimeID: "rt-001",
		Action:    LifecycleActionStart,
		Stages: []LifecycleStage{
			{Index: 0, Services: []ServicePlanEntry{
				{ServiceID: "bridge"},
				{ServiceID: "bridge"},
			}},
		},
	}

	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	err := ValidateLifecyclePlan(plan, topo, graph)
	if err == nil {
		t.Fatal("expected error for duplicate service")
	}
}

func TestValidateLifecyclePlan_WrongDependencyOrder(t *testing.T) {
	plan := LifecyclePlan{
		RuntimeID: "rt-001",
		Action:    LifecycleActionStart,
		Stages: []LifecycleStage{
			{Index: 0, Services: []ServicePlanEntry{
				{ServiceID: "agent"},
				{ServiceID: "bridge"},
			}},
		},
	}

	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
	})

	err := ValidateLifecyclePlan(plan, topo, graph)
	if err == nil {
		t.Fatal("expected error for wrong dependency order")
	}
}

func TestLifecyclePlanner_ConcurrentBuild(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("A", nil),
		svcSnapshot("B", []domain.ServiceID{"A"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("A", nil, []domain.ServiceID{"B"}),
		lnode("B", []domain.ServiceID{"A"}, nil),
	})

	planner := NewLifecyclePlanner()

	for i := 0; i < 20; i++ {
		go func() {
			_, _ = planner.BuildStartupPlan(topo, graph)
			_, _ = planner.BuildShutdownPlan(topo, graph)
		}()
	}
}
