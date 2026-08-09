package runtime

import (
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func (p *LifecyclePlanner) BuildShutdownPlan(topology RuntimeTopologySnapshot, graph DependencyGraphSnapshot) (LifecyclePlan, error) {
	startupPlan, err := p.BuildStartupPlan(topology, graph)
	if err != nil {
		return LifecyclePlan{}, err
	}

	stageCount := len(startupPlan.Stages)
	shutdownStages := make([]LifecycleStage, stageCount)

	for i, stage := range startupPlan.Stages {
		shutdownIdx := stageCount - 1 - i
		shutdownStages[shutdownIdx] = LifecycleStage{
			Index:    shutdownIdx,
			Services: make([]ServicePlanEntry, len(stage.Services)),
		}
		copy(shutdownStages[shutdownIdx].Services, stage.Services)

		sort.Slice(shutdownStages[shutdownIdx].Services, func(a, b int) bool {
			return shutdownStages[shutdownIdx].Services[a].ServiceID < shutdownStages[shutdownIdx].Services[b].ServiceID
		})
	}

	return LifecyclePlan{
		RuntimeID: topology.RuntimeID,
		PluginID:  topology.PluginID,
		Action:    LifecycleActionStop,
		Stages:    shutdownStages,
		CreatedAt: time.Now(),
	}, nil
}

func (p *LifecyclePlanner) BuildShutdownPlanFor(
	topology RuntimeTopologySnapshot,
	graph DependencyGraphSnapshot,
	targetServiceIDs []domain.ServiceID,
) (LifecyclePlan, error) {
	if topology.RuntimeID != graph.RuntimeID {
		return LifecyclePlan{}, NewLifecyclePlanError(ErrTopologyGraphMismatch,
			"runtime id mismatch between topology and graph")
	}

	targetSet := make(map[domain.ServiceID]struct{})
	for _, id := range targetServiceIDs {
		if _, exists := targetSet[id]; exists {
			continue
		}
		targetSet[id] = struct{}{}
	}

	for id := range targetSet {
		found := false
		for _, svc := range topology.Services {
			if svc.ServiceID == id {
				found = true
				break
			}
		}
		if !found {
			return LifecyclePlan{}, NewLifecyclePlanErrorWithCause(ErrLifecycleServiceNotFound,
				"target service not found in topology",
				NewLifecyclePlanError(ErrLifecycleServiceNotFound, string(id)))
		}
	}

	includedSet := make(map[domain.ServiceID]struct{})
	for id := range targetSet {
		includedSet[id] = struct{}{}
	}

	for _, node := range graph.Nodes {
		if _, isTarget := targetSet[node.ServiceID]; !isTarget {
			continue
		}
		for _, depID := range node.Dependents {
			includedSet[depID] = struct{}{}
		}
	}

	visited := make(map[domain.ServiceID]struct{})
	for id := range targetSet {
		p.collectTransitiveDependents(id, graphNodesMap(graph), includedSet, visited)
	}

	filteredTopology := RuntimeTopologySnapshot{
		RuntimeID: topology.RuntimeID,
		PluginID:  topology.PluginID,
		CreatedAt: topology.CreatedAt,
		UpdatedAt: topology.UpdatedAt,
	}

	for _, svc := range topology.Services {
		if _, included := includedSet[svc.ServiceID]; included {
			filteredTopology.Services = append(filteredTopology.Services, svc)
		}
	}

	filteredGraph := DependencyGraphSnapshot{
		RuntimeID: graph.RuntimeID,
	}

	for _, node := range graph.Nodes {
		if _, included := includedSet[node.ServiceID]; included {
			filteredNode := DependencyNodeSnapshot{
				ServiceID:    node.ServiceID,
				Dependencies: make([]domain.ServiceID, 0),
				Dependents:   make([]domain.ServiceID, 0),
			}
			for _, depID := range node.Dependencies {
				if _, depIncluded := includedSet[depID]; depIncluded {
					filteredNode.Dependencies = append(filteredNode.Dependencies, depID)
				}
			}
			for _, depID := range node.Dependents {
				if _, depIncluded := includedSet[depID]; depIncluded {
					filteredNode.Dependents = append(filteredNode.Dependents, depID)
				}
			}
			filteredGraph.Nodes = append(filteredGraph.Nodes, filteredNode)
		}
	}

	return p.BuildShutdownPlan(filteredTopology, filteredGraph)
}

func (p *LifecyclePlanner) collectTransitiveDependents(
	serviceID domain.ServiceID,
	nodes map[domain.ServiceID]DependencyNodeSnapshot,
	includedSet map[domain.ServiceID]struct{},
	visited map[domain.ServiceID]struct{},
) {
	if _, done := visited[serviceID]; done {
		return
	}
	visited[serviceID] = struct{}{}

	node, exists := nodes[serviceID]
	if !exists {
		return
	}

	for _, depID := range node.Dependents {
		includedSet[depID] = struct{}{}
		p.collectTransitiveDependents(depID, nodes, includedSet, visited)
	}
}

func ValidateLifecyclePlan(plan LifecyclePlan, topology RuntimeTopologySnapshot, graph DependencyGraphSnapshot) error {
	if plan.RuntimeID != topology.RuntimeID {
		return NewLifecyclePlanError(ErrTopologyGraphMismatch, "plan runtime id mismatch with topology")
	}
	if plan.RuntimeID != graph.RuntimeID {
		return NewLifecyclePlanError(ErrTopologyGraphMismatch, "plan runtime id mismatch with graph")
	}

	graphNodes := make(map[domain.ServiceID]DependencyNodeSnapshot)
	for _, node := range graph.Nodes {
		graphNodes[node.ServiceID] = node
	}

	planStages := make(map[domain.ServiceID]int)
	for _, stage := range plan.Stages {
		for _, svc := range stage.Services {
			if _, exists := planStages[svc.ServiceID]; exists {
				return NewLifecyclePlanErrorWithCause(ErrLifecycleInvalidTarget,
					"service appears in multiple stages",
					NewLifecyclePlanError(ErrLifecycleInvalidTarget, string(svc.ServiceID)))
			}
			planStages[svc.ServiceID] = stage.Index
		}
	}

	topoServices := make(map[domain.ServiceID]struct{})
	for _, svc := range topology.Services {
		topoServices[svc.ServiceID] = struct{}{}
	}
	for serviceID := range planStages {
		if _, exists := topoServices[serviceID]; !exists {
			return NewLifecyclePlanErrorWithCause(ErrLifecycleServiceNotFound,
				"service in plan not found in topology",
				NewLifecyclePlanError(ErrLifecycleServiceNotFound, string(serviceID)))
		}
	}

	if plan.Action == LifecycleActionStart {
		for serviceID, stageIdx := range planStages {
			node, exists := graphNodes[serviceID]
			if !exists {
				return NewLifecyclePlanErrorWithCause(ErrTopologyGraphMismatch,
					"service in plan not found in graph",
					NewLifecyclePlanError(ErrTopologyGraphMismatch, string(serviceID)))
			}
			for _, depID := range node.Dependencies {
				depStageIdx, depExists := planStages[depID]
				if !depExists {
					return NewLifecyclePlanErrorWithCause(ErrLifecycleDependencyError,
						"dependency not in plan",
						NewLifecyclePlanError(ErrLifecycleDependencyError, string(serviceID)+" -> "+string(depID)))
				}
				if depStageIdx >= stageIdx {
					return NewLifecyclePlanErrorWithCause(ErrLifecycleDependencyError,
						"dependency must be in earlier stage",
						NewLifecyclePlanError(ErrLifecycleDependencyError, string(serviceID)+" depends on "+string(depID)))
				}
			}
		}
	} else if plan.Action == LifecycleActionStop {
		for serviceID, stageIdx := range planStages {
			node, exists := graphNodes[serviceID]
			if !exists {
				return NewLifecyclePlanErrorWithCause(ErrTopologyGraphMismatch,
					"service in plan not found in graph",
					NewLifecyclePlanError(ErrTopologyGraphMismatch, string(serviceID)))
			}
			for _, depID := range node.Dependents {
				depStageIdx, depExists := planStages[depID]
				if !depExists {
					continue
				}
				if depStageIdx >= stageIdx {
					return NewLifecyclePlanErrorWithCause(ErrLifecycleDependencyError,
						"dependent must be in earlier shutdown stage",
						NewLifecyclePlanError(ErrLifecycleDependencyError, string(serviceID)+" has dependent "+string(depID)))
				}
			}
		}
	}

	return nil
}
