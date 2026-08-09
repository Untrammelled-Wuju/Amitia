package runtime

import (
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func (p *LifecyclePlanner) BuildStartupPlan(topology RuntimeTopologySnapshot, graph DependencyGraphSnapshot) (LifecyclePlan, error) {
	if topology.RuntimeID != graph.RuntimeID {
		return LifecyclePlan{}, NewLifecyclePlanError(ErrTopologyGraphMismatch,
			"runtime id mismatch between topology and graph")
	}

	topoServices := make(map[domain.ServiceID]ServiceInstanceSnapshot)
	for _, svc := range topology.Services {
		topoServices[svc.ServiceID] = svc
	}

	graphNodes := make(map[domain.ServiceID]DependencyNodeSnapshot)
	for _, node := range graph.Nodes {
		graphNodes[node.ServiceID] = node
	}

	if len(topoServices) != len(graphNodes) {
		return LifecyclePlan{}, NewLifecyclePlanError(ErrTopologyGraphMismatch,
			"service count mismatch between topology and graph")
	}

	for serviceID := range topoServices {
		if _, exists := graphNodes[serviceID]; !exists {
			return LifecyclePlan{}, NewLifecyclePlanErrorWithCause(ErrTopologyGraphMismatch,
				"service exists in topology but not in graph",
				NewLifecyclePlanError(ErrTopologyGraphMismatch, string(serviceID)))
		}
	}

	sorted, err := p.topologicalSortFromGraph(graph)
	if err != nil {
		if IsDependencyCycleError(err) {
			return LifecyclePlan{}, NewLifecyclePlanErrorWithCause(ErrLifecycleDependencyError,
				"dependency cycle detected", err)
		}
		return LifecyclePlan{}, err
	}

	indegree := make(map[domain.ServiceID]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		indegree[node.ServiceID] = len(node.Dependencies)
	}

	stageMap := make(map[domain.ServiceID]int)
	processed := make(map[domain.ServiceID]struct{})

	currentLevel := make([]domain.ServiceID, 0)
	for _, serviceID := range sorted {
		if indegree[serviceID] == 0 {
			currentLevel = append(currentLevel, serviceID)
		}
	}

	stageIndex := 0
	for len(currentLevel) > 0 {
		sort.Slice(currentLevel, func(i, j int) bool {
			return currentLevel[i] < currentLevel[j]
		})

		for _, serviceID := range currentLevel {
			stageMap[serviceID] = stageIndex
			processed[serviceID] = struct{}{}
		}

		nextLevelSet := make(map[domain.ServiceID]struct{})
		for _, serviceID := range currentLevel {
			node := graphNodes[serviceID]
			for _, dependentID := range node.Dependents {
				indegree[dependentID]--
				if indegree[dependentID] == 0 {
					nextLevelSet[dependentID] = struct{}{}
				}
			}
		}

		nextLevel := make([]domain.ServiceID, 0, len(nextLevelSet))
		for serviceID := range nextLevelSet {
			nextLevel = append(nextLevel, serviceID)
		}

		stageIndex++
		currentLevel = nextLevel
	}

	stages := make([]LifecycleStage, stageIndex)
	for serviceID, idx := range stageMap {
		svc := topoServices[serviceID]
		deps := make([]domain.ServiceID, len(svc.Dependencies))
		copy(deps, svc.Dependencies)

		entry := ServicePlanEntry{
			ServiceInstanceID: svc.ID,
			ServiceID:         svc.ServiceID,
			Required:          svc.Required,
			Dependencies:      deps,
		}

		stages[idx].Index = idx
		stages[idx].Services = append(stages[idx].Services, entry)
	}

	for i := range stages {
		sort.Slice(stages[i].Services, func(a, b int) bool {
			return stages[i].Services[a].ServiceID < stages[i].Services[b].ServiceID
		})
	}

	return LifecyclePlan{
		RuntimeID: topology.RuntimeID,
		PluginID:  topology.PluginID,
		Action:    LifecycleActionStart,
		Stages:    stages,
		CreatedAt: time.Now(),
	}, nil
}

func (p *LifecyclePlanner) BuildStartupPlanFor(
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
		for _, depID := range node.Dependencies {
			includedSet[depID] = struct{}{}
		}
	}

	visited := make(map[domain.ServiceID]struct{})
	for id := range targetSet {
		p.collectTransitiveDependencies(id, graphNodesMap(graph), includedSet, visited)
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
				Dependents:   make([]domain.ServiceID, len(node.Dependents)),
				Dependencies: make([]domain.ServiceID, 0),
			}
			copy(filteredNode.Dependents, node.Dependents)
			for _, depID := range node.Dependencies {
				if _, depIncluded := includedSet[depID]; depIncluded {
					filteredNode.Dependencies = append(filteredNode.Dependencies, depID)
				}
			}
			filteredGraph.Nodes = append(filteredGraph.Nodes, filteredNode)
		}
	}

	return p.BuildStartupPlan(filteredTopology, filteredGraph)
}

func (p *LifecyclePlanner) collectTransitiveDependencies(
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

	for _, depID := range node.Dependencies {
		includedSet[depID] = struct{}{}
		p.collectTransitiveDependencies(depID, nodes, includedSet, visited)
	}
}

func graphNodesMap(graph DependencyGraphSnapshot) map[domain.ServiceID]DependencyNodeSnapshot {
	result := make(map[domain.ServiceID]DependencyNodeSnapshot, len(graph.Nodes))
	for _, node := range graph.Nodes {
		result[node.ServiceID] = node
	}
	return result
}
