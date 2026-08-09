package runtime

import (
	"sort"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type DependencyNodeSnapshot struct {
	ServiceID    domain.ServiceID
	Dependencies []domain.ServiceID
	Dependents   []domain.ServiceID
}

type DependencyGraphSnapshot struct {
	RuntimeID domain.RuntimeInstanceID
	Nodes     []DependencyNodeSnapshot
}

type DependencyGraph struct {
	runtimeID domain.RuntimeInstanceID
	nodes     map[domain.ServiceID]*DependencyNode
}

func newDependencyGraph(runtimeID domain.RuntimeInstanceID) *DependencyGraph {
	return &DependencyGraph{
		runtimeID: runtimeID,
		nodes:     make(map[domain.ServiceID]*DependencyNode),
	}
}

func (g *DependencyGraph) RuntimeID() domain.RuntimeInstanceID {
	return g.runtimeID
}

func (g *DependencyGraph) NodeCount() int {
	return len(g.nodes)
}

func (g *DependencyGraph) EdgeCount() int {
	count := 0
	for _, node := range g.nodes {
		count += len(node.Dependencies)
	}
	return count
}

func (g *DependencyGraph) HasNode(serviceID domain.ServiceID) bool {
	_, exists := g.nodes[serviceID]
	return exists
}

func (g *DependencyGraph) Node(serviceID domain.ServiceID) (*DependencyNode, bool) {
	node, exists := g.nodes[serviceID]
	return node, exists
}

func (g *DependencyGraph) DependenciesOf(serviceID domain.ServiceID) ([]domain.ServiceID, error) {
	node, exists := g.nodes[serviceID]
	if !exists {
		return nil, NewTopologyErrorWithCause(ErrNotFound,
			"service not found in dependency graph",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	result := make([]domain.ServiceID, len(node.Dependencies))
	copy(result, node.Dependencies)
	return result, nil
}

func (g *DependencyGraph) DependentsOf(serviceID domain.ServiceID) ([]domain.ServiceID, error) {
	node, exists := g.nodes[serviceID]
	if !exists {
		return nil, NewTopologyErrorWithCause(ErrNotFound,
			"service not found in dependency graph",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	result := make([]domain.ServiceID, len(node.Dependents))
	copy(result, node.Dependents)
	return result, nil
}

func (g *DependencyGraph) TransitiveDependencies(serviceID domain.ServiceID) ([]domain.ServiceID, error) {
	_, exists := g.nodes[serviceID]
	if !exists {
		return nil, NewTopologyErrorWithCause(ErrNotFound,
			"service not found in dependency graph",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	visited := make(map[domain.ServiceID]struct{})
	var result []domain.ServiceID

	var visit func(sid domain.ServiceID)
	visit = func(sid domain.ServiceID) {
		currentNode, ok := g.nodes[sid]
		if !ok {
			return
		}
		for _, depID := range currentNode.Dependencies {
			if _, seen := visited[depID]; !seen {
				visited[depID] = struct{}{}
				visit(depID)
				result = append(result, depID)
			}
		}
	}

	visit(serviceID)
	return result, nil
}

func (g *DependencyGraph) TransitiveDependents(serviceID domain.ServiceID) ([]domain.ServiceID, error) {
	_, exists := g.nodes[serviceID]
	if !exists {
		return nil, NewTopologyErrorWithCause(ErrNotFound,
			"service not found in dependency graph",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	visited := make(map[domain.ServiceID]struct{})
	var result []domain.ServiceID

	var visit func(sid domain.ServiceID)
	visit = func(sid domain.ServiceID) {
		currentNode, ok := g.nodes[sid]
		if !ok {
			return
		}
		for _, depID := range currentNode.Dependents {
			if _, seen := visited[depID]; !seen {
				visited[depID] = struct{}{}
				visit(depID)
				result = append(result, depID)
			}
		}
	}

	visit(serviceID)
	return result, nil
}

func (g *DependencyGraph) Roots() []domain.ServiceID {
	var roots []domain.ServiceID
	for serviceID, node := range g.nodes {
		if node.IsRoot() {
			roots = append(roots, serviceID)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i] < roots[j]
	})
	return roots
}

func (g *DependencyGraph) Leaves() []domain.ServiceID {
	var leaves []domain.ServiceID
	for serviceID, node := range g.nodes {
		if node.IsLeaf() {
			leaves = append(leaves, serviceID)
		}
	}
	sort.Slice(leaves, func(i, j int) bool {
		return leaves[i] < leaves[j]
	})
	return leaves
}

func (g *DependencyGraph) TopologicalSort() ([]domain.ServiceID, error) {
	indegree := make(map[domain.ServiceID]int, len(g.nodes))
	for serviceID := range g.nodes {
		indegree[serviceID] = len(g.nodes[serviceID].Dependencies)
	}

	sortedIDs := make([]domain.ServiceID, 0, len(g.nodes))
	for serviceID := range g.nodes {
		sortedIDs = append(sortedIDs, serviceID)
	}
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i] < sortedIDs[j]
	})

	var queue []domain.ServiceID
	for _, serviceID := range sortedIDs {
		if indegree[serviceID] == 0 {
			queue = append(queue, serviceID)
		}
	}

	head := 0
	var result []domain.ServiceID
	for head < len(queue) {
		current := queue[head]
		head++
		result = append(result, current)

		if node, exists := g.nodes[current]; exists {
			var dependents []domain.ServiceID
			dependents = append(dependents, node.Dependents...)
			sort.Slice(dependents, func(i, j int) bool {
				return dependents[i] < dependents[j]
			})
			for _, dependent := range dependents {
				indegree[dependent]--
				if indegree[dependent] == 0 {
					queue = append(queue, dependent)
				}
			}
		}
	}

	if len(result) != len(g.nodes) {
		cyclePath, _ := g.findCycle()
		if cyclePath != nil {
			return nil, NewDependencyCycleError("dependency cycle detected", cyclePath)
		}
		return nil, NewDependencyCycleError("dependency cycle detected", nil)
	}

	return result, nil
}

func (g *DependencyGraph) ReverseTopologicalSort() ([]domain.ServiceID, error) {
	sorted, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	reversed := make([]domain.ServiceID, len(sorted))
	for i, j := 0, len(sorted)-1; i <= j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = sorted[j], sorted[i]
	}
	return reversed, nil
}

func (g *DependencyGraph) findCycle() ([]domain.ServiceID, bool) {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	state := make(map[domain.ServiceID]int, len(g.nodes))
	for serviceID := range g.nodes {
		state[serviceID] = white
	}

	sortedIDs := make([]domain.ServiceID, 0, len(g.nodes))
	for serviceID := range g.nodes {
		sortedIDs = append(sortedIDs, serviceID)
	}
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i] < sortedIDs[j]
	})

	var path []domain.ServiceID
	var cycleFound bool
	var cyclePath []domain.ServiceID

	var dfs func(sid domain.ServiceID) bool
	dfs = func(sid domain.ServiceID) bool {
		if cycleFound {
			return true
		}

		state[sid] = gray
		path = append(path, sid)

		var neighborIDs []domain.ServiceID
		if node, exists := g.nodes[sid]; exists {
			neighborIDs = append(neighborIDs, node.Dependencies...)
		}
		sort.Slice(neighborIDs, func(i, j int) bool {
			return neighborIDs[i] < neighborIDs[j]
		})

		for _, neighbor := range neighborIDs {
			if state[neighbor] == gray {
				cycleStart := -1
				for i, p := range path {
					if p == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cyclePath = make([]domain.ServiceID, len(path)-cycleStart)
					copy(cyclePath, path[cycleStart:])
					cycleFound = true
					return true
				}
			}
			if state[neighbor] == white {
				if dfs(neighbor) {
					return true
				}
			}
		}

		state[sid] = black
		path = path[:len(path)-1]
		return false
	}

	for _, serviceID := range sortedIDs {
		if state[serviceID] == white {
			path = nil
			if dfs(serviceID) {
				return cyclePath, true
			}
		}
	}

	return nil, false
}

func (g *DependencyGraph) Snapshot() DependencyGraphSnapshot {
	nodes := make([]DependencyNodeSnapshot, 0, len(g.nodes))

	sortedIDs := make([]domain.ServiceID, 0, len(g.nodes))
	for serviceID := range g.nodes {
		sortedIDs = append(sortedIDs, serviceID)
	}
	sort.Slice(sortedIDs, func(i, j int) bool {
		return sortedIDs[i] < sortedIDs[j]
	})

	for _, serviceID := range sortedIDs {
		node := g.nodes[serviceID]
		nodes = append(nodes, node.Snapshot())
	}

	return DependencyGraphSnapshot{
		RuntimeID: g.runtimeID,
		Nodes:     nodes,
	}
}
