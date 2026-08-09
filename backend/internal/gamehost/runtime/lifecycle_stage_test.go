package runtime

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestLifecyclePlanner_TargetStartupSingle(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("planner", []domain.ServiceID{"agent"}),
		svcSnapshot("vision", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, []domain.ServiceID{"planner"}),
		lnode("planner", []domain.ServiceID{"agent"}, nil),
		lnode("vision", nil, nil),
	})

	planner := NewLifecyclePlanner()

	plan, err := planner.BuildStartupPlanFor(topo, graph, []domain.ServiceID{"planner"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ServiceCount() != 3 {
		t.Errorf("expected 3 services in plan, got %d", plan.ServiceCount())
	}

	if !plan.ContainsService("bridge") {
		t.Error("expected bridge to be in plan")
	}
	if !plan.ContainsService("agent") {
		t.Error("expected agent to be in plan")
	}
	if !plan.ContainsService("planner") {
		t.Error("expected planner to be in plan")
	}
	if plan.ContainsService("vision") {
		t.Error("did not expect vision in plan")
	}

	sorted := plan.Flatten()
	bridgeIdx := -1
	agentIdx := -1
	plannerIdx := -1
	for i, id := range sorted {
		switch id {
		case "bridge":
			bridgeIdx = i
		case "agent":
			agentIdx = i
		case "planner":
			plannerIdx = i
		}
	}

	if bridgeIdx >= agentIdx || agentIdx >= plannerIdx {
		t.Errorf("wrong order: bridge=%d, agent=%d, planner=%d", bridgeIdx, agentIdx, plannerIdx)
	}
}

func TestLifecyclePlanner_TargetStartupMultiple(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("vision", nil),
		svcSnapshot("planner", []domain.ServiceID{"agent", "vision"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, []domain.ServiceID{"planner"}),
		lnode("vision", nil, []domain.ServiceID{"planner"}),
		lnode("planner", []domain.ServiceID{"agent", "vision"}, nil),
	})

	planner := NewLifecyclePlanner()

	plan, err := planner.BuildStartupPlanFor(topo, graph, []domain.ServiceID{"planner", "vision"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plan.ContainsService("bridge") {
		t.Error("expected bridge")
	}
	if !plan.ContainsService("agent") {
		t.Error("expected agent")
	}
	if !plan.ContainsService("vision") {
		t.Error("expected vision")
	}
	if !plan.ContainsService("planner") {
		t.Error("expected planner")
	}
}

func TestLifecyclePlanner_TargetStartupUnknown(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	planner := NewLifecyclePlanner()

	_, err := planner.BuildStartupPlanFor(topo, graph, []domain.ServiceID{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
	if !IsLifecyclePlanError(err, ErrLifecycleServiceNotFound) {
		t.Errorf("expected service_not_found, got %v", err)
	}
}

func TestLifecyclePlanner_TargetStartupDuplicate(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
	})

	planner := NewLifecyclePlanner()

	plan, err := planner.BuildStartupPlanFor(topo, graph, []domain.ServiceID{"agent", "agent", "agent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agentCount := 0
	for _, stage := range plan.Stages {
		for _, svc := range stage.Services {
			if svc.ServiceID == "agent" {
				agentCount++
			}
		}
	}
	if agentCount != 1 {
		t.Errorf("expected agent to appear once, got %d", agentCount)
	}
}

func TestLifecyclePlanner_TargetShutdownLeaf(t *testing.T) {
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

	plan, err := planner.BuildShutdownPlanFor(topo, graph, []domain.ServiceID{"planner"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ServiceCount() != 1 {
		t.Errorf("expected 1 service, got %d", plan.ServiceCount())
	}
	if !plan.ContainsService("planner") {
		t.Error("expected planner in plan")
	}
}

func TestLifecyclePlanner_TargetShutdownDependency(t *testing.T) {
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

	plan, err := planner.BuildShutdownPlanFor(topo, graph, []domain.ServiceID{"bridge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ServiceCount() != 3 {
		t.Errorf("expected 3 services, got %d", plan.ServiceCount())
	}
	if !plan.ContainsService("bridge") {
		t.Error("expected bridge")
	}
	if !plan.ContainsService("agent") {
		t.Error("expected agent")
	}
	if !plan.ContainsService("planner") {
		t.Error("expected planner")
	}
}

func TestLifecyclePlanner_TargetShutdownMultiple(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("vision", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
		lnode("vision", nil, nil),
	})

	planner := NewLifecyclePlanner()

	plan, err := planner.BuildShutdownPlanFor(topo, graph, []domain.ServiceID{"agent", "vision"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ServiceCount() != 3 {
		t.Errorf("expected 3 services, got %d", plan.ServiceCount())
	}
}

func TestLifecyclePlanner_RollbackBasic(t *testing.T) {
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

	progress := StartupProgress{
		RuntimeID:            "rt-001",
		StartedThisOperation: []domain.ServiceID{"bridge", "agent"},
	}

	rollback, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rollback.ServiceCount() != 2 {
		t.Errorf("expected 2 services in rollback, got %d", rollback.ServiceCount())
	}
	if !rollback.ContainsService("bridge") {
		t.Error("expected bridge in rollback")
	}
	if !rollback.ContainsService("agent") {
		t.Error("expected agent in rollback")
	}
	if rollback.ContainsService("planner") {
		t.Error("did not expect planner in rollback")
	}

	if len(rollback.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(rollback.Stages))
	}

	if len(rollback.Stages[0].Services) != 1 || rollback.Stages[0].Services[0].ServiceID != "agent" {
		t.Errorf("stage 0 should be [agent], got %v", rollback.Stages[0].Services)
	}

	if len(rollback.Stages[1].Services) != 1 || rollback.Stages[1].Services[0].ServiceID != "bridge" {
		t.Errorf("stage 1 should be [bridge], got %v", rollback.Stages[1].Services)
	}
}

func TestLifecyclePlanner_RollbackEmpty(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-001",
		StartedThisOperation: []domain.ServiceID{},
	}

	rollback, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rollback.ServiceCount() != 0 {
		t.Errorf("expected 0 services, got %d", rollback.ServiceCount())
	}
}

func TestLifecyclePlanner_RollbackParallel(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
		svcSnapshot("vision", []domain.ServiceID{"bridge"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent", "vision"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
		lnode("vision", []domain.ServiceID{"bridge"}, nil),
	})

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-001",
		StartedThisOperation: []domain.ServiceID{"bridge", "agent", "vision"},
	}

	rollback, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rollback.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(rollback.Stages))
	}

	if len(rollback.Stages[0].Services) != 2 {
		t.Errorf("expected 2 services in stage 0, got %d", len(rollback.Stages[0].Services))
	}
	if len(rollback.Stages[1].Services) != 1 || rollback.Stages[1].Services[0].ServiceID != "bridge" {
		t.Errorf("stage 1 should be [bridge], got %v", rollback.Stages[1].Services)
	}
}

func TestLifecyclePlanner_RollbackDoesNotStopOldService(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
		svcSnapshot("agent", []domain.ServiceID{"bridge"}),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, []domain.ServiceID{"agent"}),
		lnode("agent", []domain.ServiceID{"bridge"}, nil),
	})

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-001",
		StartedThisOperation: []domain.ServiceID{"agent"},
	}

	rollback, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rollback.ContainsService("bridge") {
		t.Error("rollback should not include bridge (was running before this operation)")
	}
	if !rollback.ContainsService("agent") {
		t.Error("rollback should include agent")
	}
}

func TestLifecyclePlanner_RollbackProgressRuntimeMismatch(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-002",
		StartedThisOperation: []domain.ServiceID{"bridge"},
	}

	_, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err == nil {
		t.Fatal("expected error for runtime mismatch")
	}
}

func TestLifecyclePlanner_RollbackStoppedServiceNotInTopology(t *testing.T) {
	topo := buildTestLifecycleTopology("rt-001", []ServiceInstanceSnapshot{
		svcSnapshot("bridge", nil),
	})
	graph := buildTestLifecycleGraph("rt-001", []DependencyNodeSnapshot{
		lnode("bridge", nil, nil),
	})

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-001",
		StartedThisOperation: []domain.ServiceID{"nonexistent"},
	}

	_, err := planner.BuildRollbackPlan(progress, topo, graph)
	if err == nil {
		t.Fatal("expected error for unknown service in progress")
	}
}
